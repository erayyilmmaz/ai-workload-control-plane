// SPDX-License-Identifier: Apache-2.0
package controller

import (
	"context"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/resource"
)

const conditionAutoscalingReady = "AutoscalingReady"

// observeAutoscaling mirrors only safe HPA controller observations. It neither
// queries a metric adapter nor invents capacity; provider errors remain visible
// through ScalingActive=False and reconcile normally through the HPA watch.
func (r *AIWorkloadReconciler) observeAutoscaling(ctx context.Context, workload *platform.AIWorkload) error {
	if !resource.AutoscalingEnabled(workload) {
		meta.RemoveStatusCondition(&workload.Status.Conditions, conditionAutoscalingReady)
		return nil
	}
	hpa := &autoscalingv2.HorizontalPodAutoscaler{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: workload.Namespace, Name: resource.ChildName(workload.Name)}, hpa); err != nil {
		if apierrors.IsNotFound(err) {
			setAutoscalingCondition(workload, metav1.ConditionFalse, "HPAReconciling", "The owned HorizontalPodAutoscaler is awaiting API observation.")
			return nil
		}
		return err
	}
	for _, condition := range hpa.Status.Conditions {
		if condition.Type == autoscalingv2.ScalingActive && condition.Status == corev1.ConditionFalse {
			setAutoscalingCondition(workload, metav1.ConditionFalse, "MetricUnavailable", "The HorizontalPodAutoscaler cannot obtain one or more configured metrics; inspect its status and the platform metric adapter.")
			return nil
		}
	}
	if active := findHPACondition(hpa.Status.Conditions, autoscalingv2.ScalingActive); active != nil && active.Status == corev1.ConditionTrue {
		setAutoscalingCondition(workload, metav1.ConditionTrue, "HPAActive", "The owned HorizontalPodAutoscaler is actively evaluating configured metrics.")
		return nil
	}
	setAutoscalingCondition(workload, metav1.ConditionFalse, "HPAReconciling", "The owned HorizontalPodAutoscaler is awaiting metric controller status.")
	return nil
}

func findHPACondition(conditions []autoscalingv2.HorizontalPodAutoscalerCondition, typ autoscalingv2.HorizontalPodAutoscalerConditionType) *autoscalingv2.HorizontalPodAutoscalerCondition {
	for i := range conditions {
		if conditions[i].Type == typ {
			return &conditions[i]
		}
	}
	return nil
}

func setAutoscalingCondition(workload *platform.AIWorkload, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&workload.Status.Conditions, metav1.Condition{Type: conditionAutoscalingReady, Status: status, Reason: reason, Message: message, ObservedGeneration: workload.Generation})
}
