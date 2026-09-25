// SPDX-License-Identifier: Apache-2.0
package resource

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	quantity "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
)

// AutoscalingEnabled keeps the V0 static-replica contract unless the developer
// expressly opts into HPA ownership.
func AutoscalingEnabled(p *platform.AIWorkload) bool {
	return p != nil && p.Spec.Autoscaling != nil && ptr.Deref(p.Spec.Autoscaling.Enabled, false)
}

func autoscalingIntent(p *platform.AIWorkload) (Intent, error) {
	hpa := &autoscalingv2.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: ChildName(p.Name), Namespace: p.Namespace}}
	if !AutoscalingEnabled(p) {
		return Intent{Object: hpa, Absent: true}, nil
	}
	if p.Spec.Autoscaling.MinReplicas == nil || p.Spec.Autoscaling.MaxReplicas == nil || len(p.Spec.Autoscaling.Metrics) == 0 {
		return Intent{}, ErrInvalidConfiguration
	}
	desired := p.DeepCopy()
	return Intent{Object: hpa, Mutate: func(object client.Object) error {
		target, ok := object.(*autoscalingv2.HorizontalPodAutoscaler)
		if !ok {
			return ErrInvalidConfiguration
		}
		return mutateHPA(target, desired)
	}}, nil
}

func mutateHPA(hpa *autoscalingv2.HorizontalPodAutoscaler, p *platform.AIWorkload) error {
	managedMetadata(&hpa.ObjectMeta, p)
	metrics, err := hpaMetrics(p.Spec.Autoscaling.Metrics)
	if err != nil {
		return err
	}
	hpa.Spec.ScaleTargetRef = autoscalingv2.CrossVersionObjectReference{APIVersion: "apps/v1", Kind: "Deployment", Name: ChildName(p.Name)}
	hpa.Spec.MinReplicas = ptr.To(*p.Spec.Autoscaling.MinReplicas)
	hpa.Spec.MaxReplicas = *p.Spec.Autoscaling.MaxReplicas
	hpa.Spec.Metrics = metrics
	if seconds := p.Spec.Autoscaling.ScaleDownStabilizationSeconds; seconds != nil {
		hpa.Spec.Behavior = &autoscalingv2.HorizontalPodAutoscalerBehavior{ScaleDown: &autoscalingv2.HPAScalingRules{StabilizationWindowSeconds: seconds}}
	} else {
		hpa.Spec.Behavior = nil
	}
	return nil
}

func hpaMetrics(specs []platform.AutoscalingMetricSpec) ([]autoscalingv2.MetricSpec, error) {
	metrics := make([]autoscalingv2.MetricSpec, 0, len(specs))
	for _, spec := range specs {
		switch spec.Type {
		case platform.AutoscalingCPU, platform.AutoscalingMemory:
			if spec.TargetUtilization == nil {
				return nil, ErrInvalidConfiguration
			}
			name := corev1.ResourceCPU
			if spec.Type == platform.AutoscalingMemory {
				name = corev1.ResourceMemory
			}
			metrics = append(metrics, autoscalingv2.MetricSpec{Type: autoscalingv2.ResourceMetricSourceType, Resource: &autoscalingv2.ResourceMetricSource{Name: name, Target: autoscalingv2.MetricTarget{Type: autoscalingv2.UtilizationMetricType, AverageUtilization: ptr.To(*spec.TargetUtilization)}}})
		case platform.AutoscalingPods, platform.AutoscalingExternal:
			if spec.MetricName == "" || spec.TargetValue == nil {
				return nil, ErrInvalidConfiguration
			}
			value, err := quantity.ParseQuantity(string(*spec.TargetValue))
			if err != nil || value.Sign() < 0 {
				return nil, ErrInvalidConfiguration
			}
			identifier := autoscalingv2.MetricIdentifier{Name: spec.MetricName}
			target := autoscalingv2.MetricTarget{Type: autoscalingv2.AverageValueMetricType, AverageValue: &value}
			if spec.Type == platform.AutoscalingPods {
				metrics = append(metrics, autoscalingv2.MetricSpec{Type: autoscalingv2.PodsMetricSourceType, Pods: &autoscalingv2.PodsMetricSource{Metric: identifier, Target: target}})
			} else {
				target.Type = autoscalingv2.ValueMetricType
				target.AverageValue = nil
				target.Value = &value
				metrics = append(metrics, autoscalingv2.MetricSpec{Type: autoscalingv2.ExternalMetricSourceType, External: &autoscalingv2.ExternalMetricSource{Metric: identifier, Target: target}})
			}
		default:
			return nil, ErrInvalidConfiguration
		}
	}
	return metrics, nil
}
