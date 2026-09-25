// SPDX-License-Identifier: Apache-2.0
package resource

import (
	"testing"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/utils/ptr"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
)

func TestAutoscalingBuildsHPAAndPreservesLiveDeploymentScale(t *testing.T) {
	p := source()
	p.Spec.Autoscaling = &platform.AutoscalingSpec{Enabled: ptr.To(true), MinReplicas: ptr.To(int32(2)), MaxReplicas: ptr.To(int32(8)), Metrics: []platform.AutoscalingMetricSpec{{Type: platform.AutoscalingCPU, TargetUtilization: ptr.To(int32(70))}}}
	plan, err := (WorkloadBuilder{}).Build(p)
	if err != nil {
		t.Fatal(err)
	}
	var intent Intent
	for _, candidate := range plan {
		if _, ok := candidate.Object.(*autoscalingv2.HorizontalPodAutoscaler); ok {
			intent = candidate
		}
	}
	if intent.Object == nil || intent.Absent {
		t.Fatal("enabled autoscaling must create an HPA intent")
	}
	hpa := intent.Object.(*autoscalingv2.HorizontalPodAutoscaler)
	if err := intent.Mutate(hpa); err != nil {
		t.Fatal(err)
	}
	if hpa.Spec.ScaleTargetRef.Name != ChildName(p.Name) || *hpa.Spec.MinReplicas != 2 || hpa.Spec.MaxReplicas != 8 || len(hpa.Spec.Metrics) != 1 || hpa.Spec.Metrics[0].Resource.Target.AverageUtilization == nil || *hpa.Spec.Metrics[0].Resource.Target.AverageUtilization != 70 {
		t.Fatalf("unexpected HPA: %#v", hpa.Spec)
	}
	live := build(t, p)
	live.ResourceVersion = "7"
	live.UID = "deployment-uid"
	live.Spec.Replicas = ptr.To(int32(6))
	deploymentIntent := intentFor(t, p)
	if err := deploymentIntent.Mutate(live); err != nil {
		t.Fatal(err)
	}
	if *live.Spec.Replicas != 6 {
		t.Fatalf("AWCP fought the HPA-owned live scale: %d", *live.Spec.Replicas)
	}
}
