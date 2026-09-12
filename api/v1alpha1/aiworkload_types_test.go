// SPDX-License-Identifier: Apache-2.0
package v1alpha1

import (
	"encoding/json"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExplicitZeroFalseSerialization(t *testing.T) {
	zero, disabled := int32(0), false
	spec := AIWorkloadSpec{Image: "example.invalid/app:v1", Container: ContainerSpec{Port: 8080}, Replicas: &zero, Service: &ServiceSpec{Enabled: &disabled}, Network: &NetworkSpec{Enabled: &disabled}}
	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"replicas":0`, `"enabled":false`} {
		if !strings.Contains(string(data), field) {
			t.Fatalf("lost explicit value %s in %s", field, data)
		}
	}
	var decoded AIWorkloadSpec
	if err = json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Replicas == nil || *decoded.Replicas != 0 || decoded.Service.Enabled == nil || *decoded.Service.Enabled || decoded.Network.Enabled == nil || *decoded.Network.Enabled {
		t.Fatal("zero/false round-trip failed")
	}
}

func TestDeepCopyIsolation(t *testing.T) {
	replicas, enabled, port := int32(2), true, int32(80)
	original := AIWorkload{Spec: AIWorkloadSpec{
		Replicas: &replicas, Service: &ServiceSpec{Enabled: &enabled, Port: &port}, Network: &NetworkSpec{Enabled: &enabled},
		Resources: &ResourceRequirements{Requests: ResourceQuantities{"cpu": "100m"}, Limits: ResourceQuantities{"cpu": "1"}},
		Health:    &HealthSpec{Readiness: &HTTPProbeSpec{Path: "/ready"}, Liveness: &HTTPProbeSpec{Path: "/health"}}, SecretRefs: []SecretReference{"first", "second"},
	}, Status: AIWorkloadStatus{Conditions: []metav1.Condition{{Type: "Ready", Status: metav1.ConditionUnknown}}}}
	copy := original.DeepCopy()
	*copy.Spec.Replicas = 0
	*copy.Spec.Service.Enabled = false
	*copy.Spec.Service.Port = 90
	*copy.Spec.Network.Enabled = false
	copy.Spec.Resources.Requests["cpu"] = "500m"
	copy.Spec.Resources.Limits["cpu"] = "2"
	copy.Spec.Health.Readiness.Path = "/changed"
	copy.Spec.Health.Liveness.Path = "/changed"
	copy.Spec.SecretRefs[0] = "changed"
	copy.Status.Conditions[0].Status = metav1.ConditionTrue
	if *original.Spec.Replicas != 2 || !*original.Spec.Service.Enabled || *original.Spec.Service.Port != 80 || !*original.Spec.Network.Enabled || original.Spec.Resources.Requests["cpu"] != "100m" || original.Spec.Resources.Limits["cpu"] != "1" || original.Spec.Health.Readiness.Path != "/ready" || original.Spec.Health.Liveness.Path != "/health" || original.Spec.SecretRefs[0] != "first" || original.Status.Conditions[0].Status != metav1.ConditionUnknown {
		t.Fatal("deep copy shares mutable API state")
	}
}
