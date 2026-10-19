//go:build helmtests

package unit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"

	"github.com/openkcm/cmk/internal/config"
)

const (
	valuesDev      = chartPath + "/values-dev.yaml"
	valuesExamples = "testdata/values-examples.yaml"
)

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
	tests := []struct {
		name   string
		values []string
		// configured: the values configure the tenant manager and the event reconciler.
		configured bool
	}{
		{name: "default values", values: []string{"--set", "cmkctl.deploy=true"}},
		{name: "values-dev.yaml", values: []string{"-f", valuesDev}, configured: true},
		{name: "documented examples", values: []string{"-f", valuesExamples}, configured: true},
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
