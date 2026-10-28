//go:build helmtests

package unit_test

import (
	"strings"
	"testing"
)

// sections returns how the given top-level sections of config.yaml start in a rendered ConfigMap.
func sections(names ...string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, "\n    "+n+":")
	}
	return out
}

// TestConfigMapSections checks that each ConfigMap writes the sections its process reads,
// and not the sections of other processes.
func TestConfigMapSections(t *testing.T) {
	tests := []struct {
		configMap  string
		values     string
		name       string
		expected   []string
		unexpected []string
	}{
		{
			configMap: "cmk-config",
			name:      "cmk",
			expected: sections("featureGates", "featureFlags", "http", "logger", "status", "certificates",
				"cryptoLayer", "telemetry", "scheduler", "database", "databaseReplicas", "services", "plugins",
				"configurableContext", "provisioning", "audit", "clientData", "landscape",
				"keystorePool", "tenant", "keys", "workflow"),
			// Only the event reconciler reads eventProcessor, only the tenant manager tenantManager.
			unexpected: sections("tenantManager", "eventProcessor"),
		},
		{
			configMap: "cmk-tenant-manager-config",
			name:      "tenant-manager",
			expected: sections("featureGates", "featureFlags", "logger", "status", "telemetry", "database",
				"databaseReplicas", "services", "tenantManager", "plugins", "audit", "certificates", "cryptoLayer",
				"landscape", "keystorePool", "configurableContext", "tenant", "keys", "workflow"),
			unexpected: sections("http", "clientData", "eventProcessor", "scheduler", "provisioning"),
		},
		{
			configMap: "cmk-event-reconciler-config",
			name:      "event-reconciler",
			expected: sections("logger", "status", "audit", "plugins", "services", "telemetry", "database",
				"databaseReplicas", "eventProcessor", "landscape", "cryptoLayer", "featureGates", "featureFlags",
				"certificates", "keystorePool", "tenant", "workflow"),
			unexpected: sections("http", "clientData", "tenantManager", "scheduler", "provisioning", "keys"),
		},
		{
			configMap: "cmk-task-scheduler-config",
			name:      "task-scheduler",
			expected:  sections("status", "logger", "telemetry", "database", "scheduler", "certificates"),
			unexpected: sections("http", "plugins", "services", "featureFlags", "tenant", "workflow",
				"tenantManager", "eventProcessor"),
		},
		{
			configMap: "cmk-task-worker-config",
			name:      "task-worker",
			expected: sections("featureGates", "featureFlags", "logger", "status", "telemetry", "database",
				"databaseReplicas", "scheduler", "configurableContext", "certificates", "services", "plugins",
				"audit", "keystorePool", "keys", "landscape", "cryptoLayer", "tenant", "workflow"),
			unexpected: sections("http", "clientData", "tenantManager", "eventProcessor", "provisioning"),
		},
		{
			configMap: "cmk-cmkctl-config",
			values:    "--set cmkctl.deploy=true",
			name:      "cmkctl",
			expected: sections("featureGates", "featureFlags", "logger", "status", "telemetry", "database",
				"databaseReplicas", "scheduler", "plugins", "services", "audit", "certificates",
				"cryptoLayer", "landscape", "keystorePool", "configurableContext", "tenant", "keys", "workflow"),
			unexpected: sections("http", "clientData", "tenantManager", "eventProcessor", "provisioning"),
		},
		{
			configMap:  "cmk-db-migrator-config",
			name:       "db-migrator",
			expected:   sections("status", "logger", "telemetry", "database", "databaseReplicas", "cryptoLayer"),
			unexpected: sections("http", "plugins", "services", "scheduler", "tenantManager"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.configMap, func(t *testing.T) {
			doc := document(render(t, split(tt.values)...), "ConfigMap", tt.configMap)
			if doc == "" {
				t.Fatalf("ConfigMap %s not rendered", tt.configMap)
			}
			expected := append([]string{
				"namespace: default\n",
				// cmk.config writes the keys sorted, as toYaml does.
				"application:\n      environment: production\n      name: " + tt.name + "\n",
			}, tt.expected...)
			checkContains(t, doc, expected, tt.unexpected)
		})
	}
}

func TestConfigMapValues(t *testing.T) {
	tests := []struct {
		name       string
		values     string
		configMap  string
		expected   []string
		unexpected []string
	}{
		{
			name:      "API server defaults",
			configMap: "cmk-config",
			expected: []string{
				"immutable: false\n",
				"address: :8081\n",
				"swaggerEnabled: false\n",
				"signingKeysPath: /etc/signing-keys",
				"mode: verify-full",
				"migrator:",
				"certX509Trusts:",
				"systemLimit: 50",
				"defaultRequiredApprovals: 2",
				"pendingCreationTimeout: 15m",
			},
			// Chart settings are not CMK settings: CMK rejects unknown keys.
			unexpected: []string{"isImmutable", "port: 8081"},
		},
		{
			name:      "HTTP port and immutable ConfigMaps",
			values:    "--set config.http.port=9000 --set config.isImmutable=true",
			configMap: "cmk-config",
			expected:  []string{"address: :9000\n", "immutable: true\n"},
		},
		{
			name:      "a section reaches the processes that read it",
			values:    "--set config.audit.endpoint=http://collector:4318/v1/logs",
			configMap: "cmk-task-worker-config",
			expected:  []string{"endpoint: http://collector:4318/v1/logs"},
		},
		{
			name:       "and no other process",
			values:     "--set config.audit.endpoint=http://collector:4318/v1/logs",
			configMap:  "cmk-task-scheduler-config",
			unexpected: []string{"http://collector:4318/v1/logs"},
		},
		{
			name:      "data migrations get the crypto layer trusts",
			values:    "--set config.cryptoLayer.certX509Trusts.value=trusts-marker",
			configMap: "cmk-db-migrator-config",
			expected:  []string{"value: trusts-marker"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := document(render(t, split(tt.values)...), "ConfigMap", tt.configMap)
			if doc == "" {
				t.Fatalf("ConfigMap %s not rendered", tt.configMap)
			}
			checkContains(t, doc, tt.expected, tt.unexpected)
		})
	}
}

// TestConfigMapNames checks that the ConfigMap names include the release name.
func TestConfigMapNames(t *testing.T) {
	out, err := renderRelease(t, "cmk-eu10")
	if err != nil {
		t.Fatalf("helm template failed: %v\n%s", err, out)
	}
	for _, name := range []string{
		"cmk-eu10-config", "cmk-eu10-tenant-manager-config", "cmk-eu10-event-reconciler-config",
		"cmk-eu10-task-scheduler-config", "cmk-eu10-task-worker-config", "cmk-eu10-db-migrator-config",
	} {
		if document(out, "ConfigMap", name) == "" {
			t.Errorf("ConfigMap %s not rendered", name)
		}
	}
}

// TestConfigMapReferences checks that each Deployment mounts its own ConfigMap and is
// restarted when its configuration changes.
func TestConfigMapReferences(t *testing.T) {
	output := render(t, "--set", "cmkctl.deploy=true")
	changed := render(t, "--set", "cmkctl.deploy=true", "--set", "config.logger.level=warn")
	deployments := map[string]string{
		"cmk":                  "cmk-config",
		"cmk-tenant-manager":   "cmk-tenant-manager-config",
		"cmk-event-reconciler": "cmk-event-reconciler-config",
		"cmk-task-scheduler":   "cmk-task-scheduler-config",
		"cmk-task-worker":      "cmk-task-worker-config",
		"cmk-cmkctl":           "cmk-cmkctl-config",
	}
	for deploymentName, configMap := range deployments {
		t.Run(deploymentName, func(t *testing.T) {
			if document(output, "ConfigMap", configMap) == "" {
				t.Fatalf("ConfigMap %s not rendered", configMap)
			}
			deployment := document(output, "Deployment", deploymentName)
			if !strings.Contains(deployment, "configMap:\n            name: "+configMap+"\n") {
				t.Errorf("Deployment %s does not mount ConfigMap %s.\n%s", deploymentName, configMap, deployment)
			}
			checksum := func(doc string) string {
				i := strings.Index(doc, "checksum/config: ")
				if i < 0 {
					return ""
				}
				return doc[i : i+81]
			}
			if checksum(deployment) == "" || checksum(deployment) == checksum(document(changed, "Deployment", deploymentName)) {
				t.Errorf("Deployment %s: checksum/config missing or unchanged after a configuration change", deploymentName)
			}
		})
	}
}

// TestChecksumPerComponent checks that checksum/config hashes each component's own
// ConfigMap: a change to a section that only one process reads restarts only that
// component.
func TestChecksumPerComponent(t *testing.T) {
	base := checksums(t, render(t, "--set", "cmkctl.deploy=true"))
	for _, name := range []string{
		"cmk", "cmk-tenant-manager", "cmk-event-reconciler", "cmk-task-scheduler",
		"cmk-task-worker", "cmk-cmkctl", "cmk-schema-db-migrator", "cmk-data-db-migrator",
	} {
		if base[name] == "" {
			t.Fatalf("%s has no checksum/config annotation", name)
		}
	}
	tests := []struct {
		name    string
		values  string
		changed string
	}{
		{name: "clientData: API server", values: "--set config.clientData.signingKeysPath=/etc/other-keys", changed: "cmk"},
		{name: "provisioning: API server", values: "--set config.provisioning.initKeystoreConfig.provider=AWS", changed: "cmk"},
		{name: "tenantManager: tenant manager", values: "--set config.tenantManager.terminationTimeout=24h", changed: "cmk-tenant-manager"},
		{name: "eventProcessor: event reconciler", values: "--set config.eventProcessor.maxReconcileCount=5", changed: "cmk-event-reconciler"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed := checksums(t, render(t, append([]string{"--set", "cmkctl.deploy=true"}, split(tt.values)...)...))
			for name, checksum := range base {
				if name == tt.changed && changed[name] == checksum {
					t.Errorf("%s: checksum/config unchanged", name)
				}
				if name != tt.changed && changed[name] != checksum {
					t.Errorf("%s: checksum/config changed, but its process does not read the section", name)
				}
			}
		})
	}
}
