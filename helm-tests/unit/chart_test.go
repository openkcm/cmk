//go:build helmtests

package unit_test

import (
	"strings"
	"testing"
)

func TestChartResources(t *testing.T) {
	tests := []struct {
		name       string
		values     string
		expected   []string
		unexpected []string
	}{
		{
			name:   "default values deploy the API server and the four background processes",
			values: "",
			expected: []string{
				"kind: Deployment\nmetadata:\n  name: cmk\n",
				"kind: Deployment\nmetadata:\n  name: cmk-tenant-manager\n",
				"kind: Deployment\nmetadata:\n  name: cmk-event-reconciler\n",
				"kind: Deployment\nmetadata:\n  name: cmk-task-scheduler\n",
				"kind: Deployment\nmetadata:\n  name: cmk-task-worker\n",
				"kind: ServiceAccount\nmetadata:\n  name: cmk-event-reconciler\n",
				"kind: Service\nmetadata:\n  name: cmk\n",
				"name: cmk-schema-db-migrator",
				"name: cmk-data-db-migrator",
				"name: cmk-db-migrator-config",
				"kind: Pod\nmetadata:\n  name: cmk-test-connection",
			},
			unexpected: []string{
				"name: cmk-cmkctl",
				"kind: HorizontalPodAutoscaler",
				"kind: PodDisruptionBudget",
				"kind: NetworkPolicy",
			},
		},
		{
			name:   "only the API server",
			values: "--set tenantManager.deploy=false --set eventReconciler.deploy=false --set taskScheduler.deploy=false --set taskWorker.deploy=false",
			expected: []string{
				"kind: Deployment\nmetadata:\n  name: cmk\n",
			},
			unexpected: []string{
				"name: cmk-tenant-manager",
				"name: cmk-event-reconciler",
				"name: cmk-task-scheduler",
				"name: cmk-task-worker",
			},
		},
		{
			name:   "cmkctl and no migrations",
			values: "--set cmkctl.deploy=true --set dbMigrator.enabled=false",
			expected: []string{
				"kind: Deployment\nmetadata:\n  name: cmk-cmkctl\n",
				"kind: ServiceAccount\nmetadata:\n  name: cmk-cmkctl\n",
			},
			unexpected: []string{
				"kind: Job",
				"db-migrator-config",
			},
		},
		{
			name:   "without ServiceAccounts",
			values: "--set serviceAccount.create=false",
			expected: []string{
				"serviceAccountName: default",
			},
			unexpected: []string{
				"kind: ServiceAccount",
				"serviceAccountName: default-",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkContains(t, render(t, split(tt.values)...), tt.expected, tt.unexpected)
		})
	}
}

func TestValidationErrors(t *testing.T) {
	tests := []struct {
		name   string
		values string
		errMsg string
	}{
		{
			name:   "image tag or digest is required",
			values: "--set image.tag=",
			errMsg: "Either image.tag or image.digest must be set",
		},
		{
			name:   "feature flags need their ConfigMap",
			values: "--set config.featureFlags.enabled=true",
			errMsg: "set featureFlagsConfigMap",
		},
		{
			name:   "autoscaling needs a target",
			values: "--set autoscaling.enabled=true --set autoscaling.targetCPUUtilizationPercentage=null --set autoscaling.targetMemoryUtilizationPercentage=null",
			errMsg: "autoscaling.enabled requires",
		},
		{
			name:   "tenant manager autoscaling needs a target",
			values: "--set tenantManager.autoscaling.enabled=true --set tenantManager.autoscaling.targetCPUUtilizationPercentage=null --set tenantManager.autoscaling.targetMemoryUtilizationPercentage=null",
			errMsg: "tenantManager.autoscaling.enabled requires",
		},
		{
			name:   "PodDisruptionBudget takes minAvailable or maxUnavailable",
			values: "--set podDisruptionBudget.enabled=true --set podDisruptionBudget.minAvailable=1 --set podDisruptionBudget.maxUnavailable=1",
			errMsg: "set either minAvailable or maxUnavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := renderErr(t, split(tt.values)...)
			if err == nil {
				t.Fatalf("expected helm template to fail, but it succeeded")
			}
			if !strings.Contains(out, tt.errMsg) {
				t.Errorf("expected error to contain %q, but got: %s", tt.errMsg, out)
			}
		})
	}
}
