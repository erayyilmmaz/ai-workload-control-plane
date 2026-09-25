// SPDX-License-Identifier: Apache-2.0
package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platformv1alpha1 "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/resource"
)

const (
	conditionReady       = "Ready"
	conditionProgressing = "Progressing"
	conditionDegraded    = "Degraded"
)

// statusAssessment is the complete, mutually consistent condition view of one
// AIWorkload generation. It deliberately contains only generic operational text.
type statusAssessment struct {
	reason      string
	message     string
	ready       metav1.ConditionStatus
	progressing metav1.ConditionStatus
	degraded    metav1.ConditionStatus
}

func desiredReplicas(p *platformv1alpha1.AIWorkload) int32 {
	return ptr.Deref(p.Spec.Replicas, int32(1))
}

func assessDeployment(p *platformv1alpha1.AIWorkload, deployment *appsv1.Deployment) statusAssessment {
	desired := desiredReplicas(p)
	if desired == 0 {
		return statusAssessment{
			reason: "ScaledToZero", message: "Desired replicas is zero; no ready workload Pods are required.",
			ready: metav1.ConditionFalse, progressing: metav1.ConditionFalse, degraded: metav1.ConditionFalse,
		}
	}
	if progressing := findDeploymentCondition(deployment.Status.Conditions, appsv1.DeploymentProgressing); progressing != nil && progressing.Status == corev1.ConditionFalse && progressing.Reason == "ProgressDeadlineExceeded" {
		return statusAssessment{
			reason: "ProgressDeadlineExceeded", message: "Deployment rollout exceeded its progress deadline.",
			ready: metav1.ConditionFalse, progressing: metav1.ConditionFalse, degraded: metav1.ConditionTrue,
		}
	}
	if deploymentReady(deployment, desired) {
		return statusAssessment{
			reason: "WorkloadReady", message: "Current Deployment rollout is available at the desired replica count.",
			ready: metav1.ConditionTrue, progressing: metav1.ConditionFalse, degraded: metav1.ConditionFalse,
		}
	}
	if available := findDeploymentCondition(deployment.Status.Conditions, appsv1.DeploymentAvailable); available != nil && available.Status == corev1.ConditionFalse {
		return statusAssessment{
			reason: "DeploymentUnavailable", message: "Deployment reports unavailable replicas for the current rollout.",
			ready: metav1.ConditionFalse, progressing: metav1.ConditionTrue, degraded: metav1.ConditionFalse,
		}
	}
	return statusAssessment{
		reason: "Reconciling", message: "Deployment rollout is progressing toward the desired replica count.",
		ready: metav1.ConditionFalse, progressing: metav1.ConditionTrue, degraded: metav1.ConditionFalse,
	}
}

func deploymentReady(deployment *appsv1.Deployment, desired int32) bool {
	available := findDeploymentCondition(deployment.Status.Conditions, appsv1.DeploymentAvailable)
	return available != nil && available.Status == corev1.ConditionTrue &&
		deployment.Status.ObservedGeneration >= deployment.Generation &&
		deployment.Status.UpdatedReplicas == desired &&
		deployment.Status.Replicas == desired &&
		deployment.Status.ReadyReplicas == desired &&
		deployment.Status.AvailableReplicas == desired
}

func findDeploymentCondition(conditions []appsv1.DeploymentCondition, typ appsv1.DeploymentConditionType) *appsv1.DeploymentCondition {
	for i := range conditions {
		if conditions[i].Type == typ {
			return &conditions[i]
		}
	}
	return nil
}

func setAssessmentConditions(p *platformv1alpha1.AIWorkload, assessment statusAssessment) {
	for _, condition := range []struct {
		typ    string
		status metav1.ConditionStatus
	}{
		{conditionReady, assessment.ready},
		{conditionProgressing, assessment.progressing},
		{conditionDegraded, assessment.degraded},
	} {
		meta.SetStatusCondition(&p.Status.Conditions, metav1.Condition{
			Type:               condition.typ,
			Status:             condition.status,
			Reason:             assessment.reason,
			Message:            assessment.message,
			ObservedGeneration: p.Generation,
		})
	}
}

func (r *AIWorkloadReconciler) observeAndReportStatus(ctx context.Context, workload *platformv1alpha1.AIWorkload) error {
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: workload.Namespace, Name: resource.ChildName(workload.Name)}, deployment); err != nil {
		return fmt.Errorf("observe workload Deployment: %w", err)
	}
	before := workload.DeepCopy()
	if err := r.observeAutoscaling(ctx, workload); err != nil {
		return fmt.Errorf("observe autoscaling: %w", err)
	}
	wasReady := meta.IsStatusConditionTrue(before.Status.Conditions, conditionReady)
	workload.Status.ObservedGeneration = workload.Generation
	workload.Status.DesiredReplicas = desiredReplicas(workload)
	workload.Status.ReadyReplicas = deployment.Status.ReadyReplicas
	workload.Status.Endpoint = resource.ServiceEndpoint(workload)
	assessment := assessDeployment(workload, deployment)
	setAssessmentConditions(workload, assessment)
	changed, err := r.patchStatus(ctx, before, workload)
	if err != nil {
		return err
	}
	if changed && !wasReady && assessment.ready == metav1.ConditionTrue && r.Recorder != nil {
		r.Recorder.Eventf(workload, nil, corev1.EventTypeNormal, "WorkloadReady", "Status", "Current Deployment rollout is available at the desired replica count.")
	}
	return nil
}
