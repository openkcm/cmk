//go:build helmtests

package unit_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"gopkg.in/yaml.v3"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/plugins/identity-management/scim"
	scimconfig "github.com/openkcm/cmk/internal/plugins/identity-management/scim/config"
)

const (
	valuesDev      = chartPath + "/values-dev.yaml"
	valuesExamples = "testdata/values-examples.yaml"
	// alternatives holds the documented examples that replace one in valuesExamples, for
	// example another secretRef type; each file is applied on top of valuesExamples.
	alternatives = "testdata/alternatives"
)

// documentedExamples returns the helm arguments of valuesExamples and of every alternative
// applied on top of it, by test name.
func documentedExamples(t *testing.T) map[string][]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(alternatives, "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no documented alternatives in %s: %v", alternatives, err)
	}
	result := map[string][]string{"documented examples": {"-f", valuesExamples}}
	for _, file := range files {
		result["documented alternative "+filepath.Base(file)] = []string{"-f", valuesExamples, "-f", file}
	}
	return result
}

// process is how a process loads its config.yaml at startup (cmd/cmkd/commands, cmd/cmkctl,
// migrations/tenant/data/00006_crypto_subject_from_config.go): config.LoadConfig, which
// rejects unknown keys and validates the configuration, with the prefix of the process's
// environment overrides (empty: none).
type process struct {
	name string
	env  string
	// validate runs the checks of a section that the process needs, when the values
	// configure it: the tenant manager checks config.tenantManager at startup, the event
	// reconciler needs complete config.eventProcessor targets.
	validate func(cfg *config.Config) error
}

// processes lists the processes that read each ConfigMap.
var processes = map[string][]process{
	"cmk-config": {{name: "api-server", env: "CMK"}},
	"cmk-tenant-manager-config": {{name: "tenant-manager", env: "TENANT_MANAGER", validate: func(cfg *config.Config) error {
		return cfg.TenantManager.Validate()
	}}},
	"cmk-event-reconciler-config": {{name: "event-reconciler", env: "event_reconciler", validate: func(cfg *config.Config) error {
		return cfg.EventProcessor.Validate()
	}}},
	"cmk-task-scheduler-config": {{name: "task-scheduler", env: "CMK_task_scheduler"}},
	"cmk-task-worker-config":    {{name: "task-worker", env: "CMK_task_worker"}},
	"cmk-cmkctl-config": {
		{name: "cmkctl sleep"},
		{name: "cmkctl tenant", env: "TENANT_MANAGER_CLI"},
		{name: "cmkctl task", env: "CMK_task_cli"},
	},
	"cmk-db-migrator-config": {
		{name: "db-migrator", env: "DB_MIGRATOR"},
		{name: "data migration 00006"},
	},
}

// TestConfigLoads renders the chart with every component and loads each process's
// config.yaml the way the process does at startup. CMK rejects unknown keys, so a key in a
// ConfigMap that CMK does not know, or a documented example that CMK does not accept, fails
// here instead of stopping the pods.
func TestConfigLoads(t *testing.T) {
	type configTest struct {
		name   string
		values []string
		// configured: the values configure the tenant manager and the event reconciler.
		configured bool
	}
	tests := []configTest{
		{name: "default values", values: []string{"--set", "cmkctl.deploy=true"}},
		{name: "values-dev.yaml", values: []string{"-f", valuesDev}, configured: true},
	}
	for name, values := range documentedExamples(t) {
		tests = append(tests, configTest{name: name, values: values, configured: true})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := map[string]string{}
			for _, m := range manifests(t, render(t, tt.values...)) {
				if m.Kind == "ConfigMap" {
					rendered[m.Metadata.Name] = m.Data["config.yaml"]
				}
			}
			for configMap, procs := range processes {
				content, ok := rendered[configMap]
				if !ok || content == "" {
					t.Errorf("ConfigMap %s with config.yaml not rendered", configMap)
					continue
				}
				dir := filepath.Join(t.TempDir(), configMap)
				err := os.MkdirAll(dir, 0o700)
				if err == nil {
					err = os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o600)
				}
				if err != nil {
					t.Fatalf("writing %s: %v", configMap, err)
				}
				for _, p := range procs {
					options := []commoncfg.Option{commoncfg.WithPaths(dir)}
					if p.env != "" {
						options = append(options, commoncfg.WithEnvOverride(p.env))
					}
					cfg, err := config.LoadConfig(options...)
					if err != nil {
						t.Errorf("%s does not start with ConfigMap %s: %v\nconfig.yaml:\n%s", p.name, configMap, err, content)
						continue
					}
					if tt.configured && p.validate != nil {
						err = p.validate(cfg)
						if err != nil {
							t.Errorf("%s rejects ConfigMap %s: %v", p.name, configMap, err)
						}
					}
				}
			}
		})
	}
}

// TestPluginSettingsExamples checks the documented settings (yamlConfiguration) of the
// identity management plugins, which CMK passes to the plugins without reading them: the
// `scim` settings with the parser and the checks of CMK's built-in scim plugin, which takes
// the same settings as the scim binary of the cmk-full-plugins image, and the `json_data` of
// scim-static. The documented `host`, `params` and `authContext` are embedded values, so the
// checks can load them.
func TestPluginSettingsExamples(t *testing.T) {
	for name, values := range documentedExamples(t) {
		t.Run(name, func(t *testing.T) {
			var content string
			for _, m := range manifests(t, render(t, values...)) {
				if m.Kind == "ConfigMap" && m.Metadata.Name == "cmk-config" {
					content = m.Data["config.yaml"]
				}
			}
			var cfg struct {
				Plugins []struct {
					Name              string `yaml:"name"`
					Type              string `yaml:"type"`
					Path              string `yaml:"path"`
					YamlConfiguration string `yaml:"yamlConfiguration"`
				} `yaml:"plugins"`
			}
			err := yaml.Unmarshal([]byte(content), &cfg)
			if err != nil {
				t.Fatalf("parsing the API server's config.yaml: %v", err)
			}

			checked := 0
			for _, p := range cfg.Plugins {
				if p.Type != "IdentityManagementService" {
					continue
				}
				switch {
				case strings.HasSuffix(p.Path, "/scim") || (p.Path == "" && p.Name == "scim"):
					var settings scimconfig.Config
					decodeStrict(t, p.Name, p.YamlConfiguration, &settings)
					_, err = scim.CreateParams(&settings)
					if err != nil {
						t.Errorf("plugin %s: the scim plugin rejects its settings: %v", p.Name, err)
					}
					if settings.AuthContext.Source == commoncfg.EmbeddedSourceValue {
						decodeStrict(t, p.Name+" authContext", settings.AuthContext.Value, &scimconfig.AuthContextConfig{})
					}
					checked++
				case strings.HasSuffix(p.Path, "/scim-static"):
					var settings struct {
						JSONData string `yaml:"json_data"`
					}
					decodeStrict(t, p.Name, p.YamlConfiguration, &settings)
					var data struct {
						Users  []json.RawMessage `json:"users"`
						Groups []json.RawMessage `json:"groups"`
					}
					err = json.Unmarshal([]byte(settings.JSONData), &data)
					if err != nil {
						t.Errorf("plugin %s: json_data is not valid JSON: %v", p.Name, err)
					}
					checked++
				}
			}
			if checked == 0 {
				t.Errorf("no scim or scim-static plugin in the documented examples")
			}
		})
	}
}

// decodeStrict decodes YAML into out and reports unknown keys and wrong types.
func decodeStrict(t *testing.T, name, content string, out any) {
	t.Helper()
	decoder := yaml.NewDecoder(strings.NewReader(content))
	decoder.KnownFields(true)
	err := decoder.Decode(out)
	if err != nil {
		t.Errorf("plugin %s: %v\n%s", name, err, content)
	}
}

// TestValuesDev checks the render with the values for local development (make start-cmk).
func TestValuesDev(t *testing.T) {
	output := render(t, "-f", valuesDev)

	// Five Deployments, cmkctl and the two migration jobs, from the development image.
	if n := strings.Count(output, `image: "cmk-api-server-dev:latest"`); n != 8 {
		t.Errorf("expected 8 containers with the development image, found %d", n)
	}
	checkContains(t, document(output, "Deployment", "cmk-cmkctl"), []string{`- "/bin/cmkctl"`, "- sleep"}, nil)
	checkContains(t, document(output, "Deployment", "cmk"), []string{
		`- "/bin/cmkd"`,
		"name: wait-for-postgresql",
		"mountPath: /etc/signing-keys",
	}, nil)

	apiConfig := document(output, "ConfigMap", "cmk-config")
	checkContains(t, apiConfig, []string{
		"environment: dev\n",
		"provider: AWS",
		// OpenTelemetry metrics, served on /metrics.
		"metrics:\n        enabled: true\n",
		"prometheus:\n          enabled: true\n",
	}, nil)
	// The keystore is provisioned by the API server only.
	checkContains(t, document(output, "ConfigMap", "cmk-tenant-manager-config"), []string{"amqp://guest:guest@"},
		[]string{"provisioning:", "eventProcessor:"})
}

// releaseNotes renders the release notes with `helm install --dry-run=client`.
func releaseNotes(t *testing.T, args ...string) string {
	t.Helper()
	out, err := runHelm(t, append([]string{"install", releaseName, chartPath, "--namespace", "default", "--dry-run=client"}, args...)...)
	if err != nil {
		t.Fatalf("helm install --dry-run=client failed: %v\nOutput: %s", err, out)
	}
	_, notes, found := strings.Cut(out, "\nNOTES:\n")
	if !found {
		t.Fatalf("no release notes in the output:\n%s", out)
	}
	return notes
}

func TestReleaseNotes(t *testing.T) {
	const (
		provisioning = "config.provisioning.initKeystoreConfig.enabled is true"
		tmNotMTLS    = "config.tenantManager.secretRef.type is not mtls"
		tmPlain      = "config.tenantManager.amqp.url starts with amqp://"
		epNotMTLS    = "config.eventProcessor.secretRef.type is not mtls"
		epPlain      = "config.eventProcessor.targets use amqp://"
	)
	tests := []struct {
		name       string
		values     []string
		expected   []string
		unexpected []string
	}{
		{
			name:       "default values",
			expected:   []string{"config.tenantManager (secretRef, amqp.url, amqp.source, amqp.target) is not set"},
			unexpected: []string{provisioning, tmNotMTLS, tmPlain, epNotMTLS, epPlain},
		},
		{
			name:   "values-dev.yaml: plain-text AMQP and provisioning",
			values: []string{"-f", valuesDev},
			expected: []string{provisioning, tmNotMTLS, tmPlain, epNotMTLS,
				epPlain + " for the regions eu10, eu20, eu30"},
		},
		{
			name:       "documented examples: mtls and amqps",
			values:     []string{"-f", valuesExamples},
			expected:   []string{provisioning},
			unexpected: []string{tmNotMTLS, tmPlain, epNotMTLS, epPlain, "Stored in plain text"},
		},
		{
			name:       "no provisioning warning without the API server",
			values:     []string{"-f", valuesExamples, "--set", "apiServer.deploy=false"},
			unexpected: []string{provisioning},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkContains(t, releaseNotes(t, tt.values...), tt.expected, tt.unexpected)
		})
	}
}
