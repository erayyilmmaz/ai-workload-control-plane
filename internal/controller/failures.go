// SPDX-License-Identifier: Apache-2.0
package controller

import (
	"context"
	"errors"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platformv1alpha1 "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
)

// safeRetry preserves errors.Is/As while avoiding arbitrary API error bodies in logs.
type safeRetry struct {
	cause    error
	category string
}

func (e safeRetry) Error() string { return "reconciliation failed: " + e.category }
func (e safeRetry) Unwrap() error { return e.cause }

func retryError(err error) error {
	if err == nil {
		return nil
	}
	category := "UnexpectedError"
	switch {
	case apierrors.IsConflict(err):
		category = "Conflict"
	case apierrors.IsAlreadyExists(err):
		category = "AlreadyExists"
	case apierrors.IsForbidden(err):
		category = "Forbidden"
	case apierrors.IsTimeout(err) || apierrors.IsServerTimeout(err):
		category = "Timeout"
	case apierrors.IsTooManyRequests(err):
		category = "Throttled"
	}
	return safeRetry{cause: err, category: category}
}

func (r *AIWorkloadReconciler) reportFailure(ctx context.Context, p *platformv1alpha1.AIWorkload, cause error) (ctrl.Result, error) {
	reason, message := "ReconcileFailed", "Kubernetes API operation failed; inspect access, availability and controller logs."
	permanent := false
	if errors.Is(cause, ErrOwnershipConflict) {
		reason = "ResourceOwnershipConflict"
		message = "A child name is occupied by a resource not controlled by this AIWorkload UID; resolve ownership without adoption."
		permanent = true
	}
	if errors.Is(cause, ErrInvalidPlan) || apierrors.IsInvalid(cause) {
		reason = "InvalidConfiguration"
		message = "Desired resource configuration is invalid; review the specification and builder contract."
		permanent = true
	}
	before := p.DeepCopy()
	for _, typ := range []string{"Ready", "Progressing", "Degraded"} {
		value := metav1.ConditionUnknown
		if permanent {
			value = metav1.ConditionFalse
		}
		if typ == "Degraded" {
			value = metav1.ConditionTrue
		}
		meta.SetStatusCondition(&p.Status.Conditions, metav1.Condition{Type: typ, Status: value, Reason: reason, Message: message, ObservedGeneration: p.Generation})
	}
	p.Status.ObservedGeneration = p.Generation
	if p.Spec.Replicas != nil {
		p.Status.DesiredReplicas = *p.Spec.Replicas
	}
	changed, err := r.patchStatus(ctx, before, p)
	if err != nil {
		return ctrl.Result{}, retryError(err)
	}
	if changed && r.Recorder != nil {
		r.Recorder.Eventf(p, nil, corev1.EventTypeWarning, reason, "Reconcile", message)
	}
	if permanent {
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}
	return ctrl.Result{}, retryError(cause)
}

// Recovery clears only this foundation's failure marker, never declares a workload Ready.
func (r *AIWorkloadReconciler) clearFailure(ctx context.Context, p *platformv1alpha1.AIWorkload) error {
	c := meta.FindStatusCondition(p.Status.Conditions, "Degraded")
	if c == nil || (c.Reason != "ResourceOwnershipConflict" && c.Reason != "InvalidConfiguration" && c.Reason != "ReconcileFailed") {
		return nil
	}
	before := p.DeepCopy()
	for _, typ := range []string{"Ready", "Progressing", "Degraded"} {
		value := metav1.ConditionUnknown
		if typ == "Progressing" {
			value = metav1.ConditionTrue
		}
		if typ == "Degraded" {
			value = metav1.ConditionFalse
		}
		meta.SetStatusCondition(&p.Status.Conditions, metav1.Condition{Type: typ, Status: value, Reason: "Reconciling", Message: "Resource plan applied; workload readiness has not been evaluated.", ObservedGeneration: p.Generation})
	}
	p.Status.ObservedGeneration = p.Generation
	if p.Spec.Replicas != nil {
		p.Status.DesiredReplicas = *p.Spec.Replicas
	}
	_, err := r.patchStatus(ctx, before, p)
	return err
}

func (r *AIWorkloadReconciler) patchStatus(ctx context.Context, before, after *platformv1alpha1.AIWorkload) (bool, error) {
	if apiequality.Semantic.DeepEqual(before.Status, after.Status) {
		return false, nil
	}
	err := r.Status().Patch(ctx, after, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{}))
	return err == nil, err
}
