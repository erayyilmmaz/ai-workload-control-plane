// SPDX-License-Identifier: Apache-2.0
package telemetry

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	platformv1alpha1 "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
)

func TestMetricsUseBoundedLabelsAndRefreshCacheGauges(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := New(registry)
	if err != nil {
		t.Fatal(err)
	}
	metrics.RecordResource("Deployment", "created")
	metrics.RecordFailure("SecretNotFound")
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := platformv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	ready := &platformv1alpha1.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: "ready", Namespace: "workloads"}, Status: platformv1alpha1.AIWorkloadStatus{Conditions: []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue}}}}
	degraded := &platformv1alpha1.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: "degraded", Namespace: "workloads"}, Status: platformv1alpha1.AIWorkloadStatus{Conditions: []metav1.Condition{{Type: "Degraded", Status: metav1.ConditionTrue}}}}
	outside := ready.DeepCopy()
	outside.Name, outside.Namespace = "outside", "other"
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ready, degraded, outside).Build()
	if err := metrics.Refresh(context.Background(), reader, "workloads"); err != nil {
		t.Fatal(err)
	}
	if metricValue(t, registry, "awcp_controller_resource_reconciliations_total", map[string]string{"kind": "Deployment", "operation": "created"}) != 1 || metricValue(t, registry, "awcp_controller_reconcile_failures_total", map[string]string{"reason": "SecretNotFound"}) != 1 || metricValue(t, registry, "awcp_controller_managed_workloads", nil) != 2 || metricValue(t, registry, "awcp_controller_ready_workloads", nil) != 1 || metricValue(t, registry, "awcp_controller_degraded_workloads", nil) != 1 {
		t.Fatal("metric values or namespace filtering wrong")
	}
}

func metricValue(t *testing.T, registry *prometheus.Registry, name string, labels map[string]string) float64 {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.Metric {
			actual := map[string]string{}
			for _, label := range metric.Label {
				actual[label.GetName()] = label.GetValue()
			}
			if len(actual) != len(labels) {
				continue
			}
			match := true
			for key, value := range labels {
				if actual[key] != value {
					match = false
				}
			}
			if match {
				if metric.Counter != nil {
					return metric.Counter.GetValue()
				}
				return metric.Gauge.GetValue()
			}
		}
	}
	t.Fatalf("metric %q with labels %v not found", name, labels)
	return 0
}
