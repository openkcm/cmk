//go:build helmtests

package unit_test

import "testing"

func TestMigrationJobs(t *testing.T) {
	common := []string{
		"automountServiceAccountToken: false",
		"restartPolicy: OnFailure",
		"backoffLimit: 6",
		"helm.sh/hook-delete-policy: before-hook-creation,hook-succeeded",
		`- "/cmkd"`,
		"- db-migrator",
		"- --target=all",
		"mountPath: /etc/cmk/config.yaml",
		"subPath: config.yaml",
		"name: cmk-db-migrator-config",
		"runAsUser: 65532",
		"ephemeral-storage: 512Mi",
	}
	tests := []struct {
		name       string
		values     string
		job        string
		expected   []string
		unexpected []string
	}{
		{
			name: "schema migration before install and upgrade",
			job:  "cmk-schema-db-migrator",
			expected: []string{
				"helm.sh/hook: pre-install,pre-upgrade",
				"- --type=schema",
			},
			// The release's ServiceAccounts do not exist yet when the hook runs.
			unexpected: []string{"serviceAccountName:"},
		},
		{
			name: "data migration after install and upgrade",
			job:  "cmk-data-db-migrator",
			expected: []string{
				"helm.sh/hook: post-install,post-upgrade",
				"- --type=data",
			},
		},
		{
			name:     "a ServiceAccount that exists before the installation",
			values:   "--set serviceAccount.create=false --set serviceAccount.name=existing",
			job:      "cmk-schema-db-migrator",
			expected: []string{"serviceAccountName: existing"},
		},
		{
			name:       "no sidecars of the Deployments",
			values:     "--set extraContainers[0].name=sidecar --set extraContainers[0].image=s --set dbMigrator.extraContainers[0].name=exits --set dbMigrator.extraContainers[0].image=e",
			job:        "cmk-schema-db-migrator",
			expected:   []string{"name: exits"},
			unexpected: []string{"name: sidecar"},
		},
		{
			name:     "job settings and pod annotations",
			values:   "--set dbMigrator.activeDeadlineSeconds=1800 --set dbMigrator.ttlSecondsAfterFinished=600 --set dbMigrator.podAnnotations.sidecar\\.istio\\.io/inject=false",
			job:      "cmk-data-db-migrator",
			expected: []string{"activeDeadlineSeconds: 1800", "ttlSecondsAfterFinished: 600", `sidecar.istio.io/inject: "false"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := document(render(t, split(tt.values)...), "Job", tt.job)
			if doc == "" {
				t.Fatalf("Job %s not rendered", tt.job)
			}
			checkContains(t, doc, append(common, tt.expected...), tt.unexpected)
		})
	}

	t.Run("hook ConfigMap", func(t *testing.T) {
		doc := document(render(t), "ConfigMap", "cmk-db-migrator-config")
		checkContains(t, doc, []string{
			`"helm.sh/hook": "pre-install, pre-upgrade, post-install, post-upgrade"`,
			`"helm.sh/hook-weight": "-1"`,
			"name: db-migrator\n",
		}, nil)
	})
}
