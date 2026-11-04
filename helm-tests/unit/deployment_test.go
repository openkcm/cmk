//go:build helmtests

package unit_test

import (
	"strings"
	"testing"
)

// TestDeploymentDefaults checks each component's Deployment with the default values. The
// selector labels must never change: a Deployment's selector cannot be changed on upgrade.
func TestDeploymentDefaults(t *testing.T) {
	common := []string{
		`image: "ghcr.io/openkcm/cmk/images/cmk-full-plugins:` + appVersion(t) + `"`,
		"checksum/config:",
		"automountServiceAccountToken: false",
		"enableServiceLinks: false",
		"runAsNonRoot: true",
		"runAsUser: 65532",
		"runAsGroup: 65532",
		"fsGroup: 65532",
		"readOnlyRootFilesystem: true",
		"allowPrivilegeEscalation: false",
		"name: http-status",
		"path: /probe/liveness",
		"path: /probe/readiness",
		"ephemeral-storage: 512Mi",
		"subPath: config.yaml",
		"mountPath: /tmp",
		`- "/cmkd"`,
	}
	tests := []struct {
		name       string
		deployment string
		expected   []string
		unexpected []string
	}{
		{
			name:       "API server",
			deployment: "cmk",
			expected: []string{
				"app.kubernetes.io/name: cmk\n      app.kubernetes.io/instance: cmk\n      app.kubernetes.io/component: cmk\n",
				"serviceAccountName: cmk\n",
				"- name: cmk\n",
				"- api-server",
				"replicas: 1",
				"containerPort: 8081",
				"mountPath: /etc/cmk/config.yaml",
			},
		},
		{
			name:       "tenant manager",
			deployment: "cmk-tenant-manager",
			expected: []string{
				"app.kubernetes.io/name: cmk-tenant-manager\n      app.kubernetes.io/instance: cmk-tenant-manager\n      app.kubernetes.io/component: cmk-tenant-manager\n",
				"serviceAccountName: cmk-tenant-manager",
				"- tenant-manager",
				"replicas: 1",
				// The tenant manager reads /etc/tenant-manager.
				"mountPath: /etc/tenant-manager/config.yaml",
			},
			unexpected: []string{"containerPort: 8081"},
		},
		{
			name:       "event reconciler",
			deployment: "cmk-event-reconciler",
			expected: []string{
				"app.kubernetes.io/name: cmk-event-reconciler\n      app.kubernetes.io/instance: cmk-event-reconciler\n      app.kubernetes.io/component: cmk-event-reconciler\n",
				"serviceAccountName: cmk-event-reconciler",
				"- event-reconciler",
				"mountPath: /etc/cmk/config.yaml",
			},
			unexpected: []string{"containerPort: 8081"},
		},
		{
			name:       "task scheduler",
			deployment: "cmk-task-scheduler",
			expected: []string{
				"app.kubernetes.io/name: cmk\n      app.kubernetes.io/instance: cmk-task-scheduler\n      app.kubernetes.io/component: cmk-task-scheduler\n",
				"serviceAccountName: cmk-task-scheduler",
				"- task-scheduler",
				"replicas: 1",
				"type: Recreate",
			},
			unexpected: []string{"containerPort: 8081"},
		},
		{
			name:       "task worker",
			deployment: "cmk-task-worker",
			expected: []string{
				"app.kubernetes.io/name: cmk\n      app.kubernetes.io/instance: cmk-task-worker\n      app.kubernetes.io/component: cmk-task-worker\n",
				"serviceAccountName: cmk-task-worker",
				"- task-worker",
			},
			unexpected: []string{"containerPort: 8081"},
		},
	}

	output := render(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := document(output, "Deployment", tt.deployment)
			if doc == "" {
				t.Fatalf("Deployment %s not rendered", tt.deployment)
			}
			checkContains(t, doc, append(common, tt.expected...), tt.unexpected)
		})
	}
}

func TestDeploymentOptions(t *testing.T) {
	tests := []struct {
		name       string
		values     string
		deployment string
		expected   []string
		unexpected []string
	}{
		{
			name:       "cmkctl runs cmkctl sleep",
			values:     "--set cmkctl.deploy=true",
			deployment: "cmk-cmkctl",
			expected: []string{
				`- "/cmkctl"`,
				"- sleep",
				"app.kubernetes.io/name: cmk-cmkctl",
			},
		},
		{
			name:       "binary paths of the development image",
			values:     "--set image.cmkdPath=/bin/cmkd",
			deployment: "cmk-task-worker",
			expected:   []string{`- "/bin/cmkd"`},
		},
		{
			name:       "command as a list, arguments and extra arguments",
			values:     "--set apiServer.image.command={/a,/b} --set apiServer.extraArgs={--graceful-shutdown=10}",
			deployment: "cmk",
			expected: []string{
				"command:\n            - /a\n            - /b\n",
				"args:\n            - api-server\n            - --graceful-shutdown=10\n",
			},
		},
		{
			name:       "arguments override the subcommand",
			values:     "--set taskWorker.image.args={task-worker,--graceful-shutdown=5}",
			deployment: "cmk-task-worker",
			expected:   []string{"args:\n            - task-worker\n            - --graceful-shutdown=5\n"},
		},
		{
			name:       "replicas from replicaCount",
			values:     "--set apiServer.replicaCount=3 --set taskWorker.replicaCount=4",
			deployment: "cmk",
			expected:   []string{"replicas: 3"},
		},
		{
			name:       "no replicas with autoscaling",
			values:     "--set apiServer.autoscaling.enabled=true",
			deployment: "cmk",
			unexpected: []string{"replicas:"},
		},
		{
			name:       "task scheduler always one replica",
			values:     "--set apiServer.replicaCount=3",
			deployment: "cmk-task-scheduler",
			expected:   []string{"replicas: 1"},
		},
		{
			name:       "component resources override the shared ones",
			values:     "--set taskWorker.resources.limits.memory=2Gi",
			deployment: "cmk-task-worker",
			expected:   []string{"memory: 2Gi"},
			unexpected: []string{"memory: 1Gi"},
		},
		{
			name:       "feature flags are mounted",
			values:     "--set config.featureFlags.enabled=true --set featureFlagsConfigMap=cmk-flags",
			deployment: "cmk-tenant-manager",
			expected: []string{
				"mountPath: /etc/featureflags",
				"name: cmk-flags",
			},
		},
		{
			name:       "pod annotations are strings",
			values:     "--set podAnnotations.prometheus\\.io/scrape=true",
			deployment: "cmk",
			expected:   []string{`prometheus.io/scrape: "true"`},
		},
		{
			name:       "extra volumes, mounts and environment",
			values:     "--set extraVolumes[0].name=keys --set extraVolumes[0].secret.secretName=cmk-keys --set extraVolumeMounts[0].name=keys --set extraVolumeMounts[0].mountPath=/etc/signing-keys --set extraEnvs[0].name=FOO --set extraEnvs[0].value=bar",
			deployment: "cmk-event-reconciler",
			expected: []string{
				"secretName: cmk-keys",
				"mountPath: /etc/signing-keys",
				"name: FOO",
			},
		},
		{
			name:       "topology spread constraints per component",
			values:     "--set apiServer.topologySpreadConstraints[0].maxSkew=1 --set apiServer.topologySpreadConstraints[0].topologyKey=topology.kubernetes.io/zone",
			deployment: "cmk",
			expected:   []string{"topologyKey: topology.kubernetes.io/zone"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := document(render(t, split(tt.values)...), "Deployment", tt.deployment)
			if doc == "" {
				t.Fatalf("Deployment %s not rendered", tt.deployment)
			}
			checkContains(t, doc, tt.expected, tt.unexpected)
		})
	}
}

// TestImage checks the image of every CMK container: the tag defaults to the chart's
// appVersion, and a digest takes precedence over the tag.
func TestImage(t *testing.T) {
	const repository = "ghcr.io/openkcm/cmk/images/cmk-full-plugins"
	// Five Deployments and the two migration jobs; cmkctl is off by default.
	const containers = 7
	tests := []struct {
		name       string
		values     string
		image      string
		unexpected []string
	}{
		{
			name:  "no tag: the chart's appVersion",
			image: repository + ":" + appVersion(t),
		},
		{
			name:   "tag",
			values: "--set image.tag=v9.9.9",
			image:  repository + ":v9.9.9",
		},
		{
			name:       "the digest takes precedence over the tag",
			values:     "--set image.tag=v9.9.9 --set image.digest=sha256:0123456789abcdef",
			image:      repository + "@sha256:0123456789abcdef",
			unexpected: []string{":v9.9.9\""},
		},
		{
			name:   "no registry",
			values: "--set image.registry= --set image.repository=cmk-api-server-dev --set image.tag=latest",
			image:  "cmk-api-server-dev:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := render(t, split(tt.values)...)
			if n := strings.Count(output, `image: "`+tt.image+`"`); n != containers {
				t.Errorf("expected %d containers with image %s, found %d", containers, tt.image, n)
			}
			checkContains(t, output, nil, tt.unexpected)
		})
	}

	t.Run("version label", func(t *testing.T) {
		checkContains(t, render(t), []string{`app.kubernetes.io/version: "` + appVersion(t) + `"`}, nil)
	})
}
