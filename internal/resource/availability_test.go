// SPDX-License-Identifier: Apache-2.0
package resource

import (
	"testing"

	policyv1 "k8s.io/api/policy/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/ptr"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
)

func availabilityIntentFor(t *testing.T, p *platform.AIWorkload) Intent {
	t.Helper()
	plan, err := (WorkloadBuilder{}).Build(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, intent := range plan {
		if _, ok := intent.Object.(*policyv1.PodDisruptionBudget); ok {
			return intent
		}
	}
	t.Fatal("PodDisruptionBudget intent missing")
	return Intent{}
}

func TestAvailabilityBuildsPDBWithExactWorkloadSelector(t *testing.T) {
	p := source()
	p.Spec.Availability = &platform.AvailabilitySpec{Enabled: ptr.To(true), MinAvailable: ptr.To(int32(1))}
	before := p.DeepCopy()
	intent := availabilityIntentFor(t, p)
	if intent.Absent {
		t.Fatal("enabled availability must create a PDB")
	}
	pdb := intent.Object.(*policyv1.PodDisruptionBudget)
	if err := intent.Mutate(pdb); err != nil {
		t.Fatal(err)
	}
	if !apiequality.Semantic.DeepEqual(p, before) || pdb.Spec.MinAvailable == nil || pdb.Spec.MinAvailable.IntValue() != 1 || pdb.Spec.MaxUnavailable != nil || pdb.Spec.Selector == nil || !apiequality.Semantic.DeepEqual(pdb.Spec.Selector.MatchLabels, SelectorLabels(p)) {
		t.Fatalf("unexpected PDB contract: %#v", pdb.Spec)
	}
	// Existing policy defaults and unrelated metadata are not owned by AWCP.
	pdb.Labels["user.example/keep"] = "yes"
	if err := intent.Mutate(pdb); err != nil || pdb.Labels["user.example/keep"] != "yes" {
		t.Fatal("PDB mutation lost unmanaged metadata")
	}
}

func TestAvailabilityRejectsUnsafeOrAmbiguousBudget(t *testing.T) {
	for _, availability := range []*platform.AvailabilitySpec{
		{Enabled: ptr.To(true)},
		{Enabled: ptr.To(true), MinAvailable: ptr.To(int32(1)), MaxUnavailable: ptr.To(int32(0))},
		{Enabled: ptr.To(true), MinAvailable: ptr.To(int32(3))},
		{Enabled: ptr.To(true), MaxUnavailable: ptr.To(int32(2))},
	} {
		p := source()
		p.Spec.Availability = availability
		if _, err := (WorkloadBuilder{}).Build(p); err == nil {
			t.Fatalf("unsafe availability accepted: %#v", availability)
		}
	}
	p := source()
	p.Spec.Replicas = ptr.To(int32(1))
	p.Spec.Availability = &platform.AvailabilitySpec{Enabled: ptr.To(true), MinAvailable: ptr.To(int32(1))}
	if _, err := (WorkloadBuilder{}).Build(p); err != nil {
		t.Fatalf("single-replica PDB must be valid but observable: %v", err)
	}
}

func TestAvailabilityUsesAutoscalingLowerBound(t *testing.T) {
	p := source()
	p.Spec.Autoscaling = &platform.AutoscalingSpec{Enabled: ptr.To(true), MinReplicas: ptr.To(int32(2)), MaxReplicas: ptr.To(int32(8)), Metrics: []platform.AutoscalingMetricSpec{{Type: platform.AutoscalingCPU, TargetUtilization: ptr.To(int32(70))}}}
	p.Spec.Availability = &platform.AvailabilitySpec{Enabled: ptr.To(true), MaxUnavailable: ptr.To(int32(1))}
	if AvailabilityReplicaFloor(p) != 2 {
		t.Fatal("PDB must use HPA lower bound")
	}
	if _, err := (WorkloadBuilder{}).Build(p); err != nil {
		t.Fatal(err)
	}
}
