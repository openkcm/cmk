//go:build helmtests

package unit_test

import "testing"

func TestService(t *testing.T) {
	doc := document(render(t), "Service", "cmk")
	checkContains(t, doc, []string{
		"type: ClusterIP",
		"port: 8081",
		"targetPort: http",
		"port: 8888",
		"app.kubernetes.io/name: cmk\n    app.kubernetes.io/instance: cmk\n    app.kubernetes.io/component: cmk\n",
	}, nil)
}

func TestNetworkPolicies(t *testing.T) {
	values := []string{
		"--set", "networkPolicy.enabled=true",
		"--set", "networkPolicy.http.from[0].namespaceSelector.matchLabels.kubernetes\\.io/metadata\\.name=envoy-gateway-system",
		"--set", "networkPolicy.status.from[0].namespaceSelector.matchLabels.kubernetes\\.io/metadata\\.name=monitoring",
	}
	output := render(t, values...)

	t.Run("API server admits Envoy Gateway on http", func(t *testing.T) {
		checkContains(t, document(output, "NetworkPolicy", "cmk"), []string{
			"port: http\n",
			"kubernetes.io/metadata.name: envoy-gateway-system",
			"port: http-status",
		}, nil)
	})
	for _, name := range []string{"cmk-tenant-manager", "cmk-event-reconciler", "cmk-task-scheduler", "cmk-task-worker"} {
		t.Run(name+" admits only the status port", func(t *testing.T) {
			doc := document(output, "NetworkPolicy", name)
			if doc == "" {
				t.Fatalf("NetworkPolicy %s not rendered", name)
			}
			checkContains(t, doc, []string{"port: http-status", "kubernetes.io/metadata.name: monitoring"},
				[]string{"port: http\n", "envoy-gateway-system"})
		})
	}
	t.Run("disabled by default", func(t *testing.T) {
		checkContains(t, render(t), nil, []string{"kind: NetworkPolicy"})
	})
}

func TestAutoscalingAndDisruptionBudgets(t *testing.T) {
	output := render(t,
		"--set", "apiServer.autoscaling.enabled=true",
		"--set", "tenantManager.autoscaling.enabled=true",
		"--set", "podDisruptionBudget.enabled=true",
	)
	checkContains(t, document(output, "HorizontalPodAutoscaler", "cmk"), []string{"minReplicas: 2", "maxReplicas: 5", "name: cpu", "name: memory"}, nil)
	checkContains(t, document(output, "HorizontalPodAutoscaler", "cmk-tenant-manager"), []string{"name: cmk-tenant-manager"}, nil)
	checkContains(t, document(output, "PodDisruptionBudget", "cmk"), []string{"maxUnavailable: 1", "app.kubernetes.io/component: cmk\n"}, nil)
	checkContains(t, document(output, "PodDisruptionBudget", "cmk-tenant-manager"), []string{"maxUnavailable: 1", "app.kubernetes.io/component: cmk-tenant-manager"}, nil)
	// The tenant manager Deployment leaves its replicas to the autoscaler.
	checkContains(t, document(output, "Deployment", "cmk-tenant-manager"), nil, []string{"replicas:"})
}
