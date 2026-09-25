// SPDX-License-Identifier: Apache-2.0
package controller

import (
	"context"
	"errors"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platformv1alpha1 "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/resource"
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
	if errors.Is(cause, resource.ErrImmutableSelector) {
		reason = "ResourceOwnershipConflict"
		message = "The owned Deployment has an incompatible immutable selector; administrative resolution is required. Automatic deletion or replacement is disabled."
		permanent = true
	}
	if errors.Is(cause, resource.ErrImmutableServiceAllocation) {
		reason = "ResourceOwnershipConflict"
		message = "The owned Service has an incompatible immutable cluster allocation; administrative resolution is required. Automatic deletion or replacement is disabled."
		permanent = true
	}
	if errors.Is(cause, ErrSecretNotFound) {
		reason = "SecretNotFound"
		message = "One or more referenced Secrets are unavailable in the workload namespace."
		permanent = true
	}
	var tenantErr tenantConfigurationError
	if errors.As(cause, &tenantErr) {
		reason = tenantErr.reason
		message = tenantErr.message
		permanent = true
	}
	var externalSecretErr externalSecretConfigurationError
	if errors.As(cause, &externalSecretErr) {
		reason = externalSecretErr.reason
		message = externalSecretErr.message
		permanent = true
	}
	var exposureErr exposureConfigurationError
	if errors.As(cause, &exposureErr) {
		reason = exposureErr.reason
		message = exposureErr.message
		permanent = true
	}
	if apierrors.IsForbidden(cause) && strings.Contains(strings.ToLower(cause.Error()), "exceeded quota") {
		reason = "QuotaExceeded"
		message = "Tenant quota blocked an AWCP child; reduce requested resources or ask a platform administrator to adjust the tenant profile."
	}
	before := p.DeepCopy()
	if apierrors.IsNotFound(cause) {
		// A missing observed child has no ready replicas; never preserve a stale
		// healthy count while reporting the failed observation.
		p.Status.ReadyReplicas = 0
	}
	for _, typ := range []string{conditionReady, conditionProgressing, conditionDegraded} {
		value := metav1.ConditionUnknown
		if permanent {
			value = metav1.ConditionFalse
		}
		if typ == "Degraded" {
			value = metav1.ConditionTrue
		}
		meta.SetStatusCondition(&p.Status.Conditions, metav1.Condition{Type: typ, Status: value, Reason: reason, Message: message, ObservedGeneration: p.Generation})
	}
	if errors.As(cause, &tenantErr) {
		meta.SetStatusCondition(&p.Status.Conditions, metav1.Condition{Type: conditionTenantReady, Status: metav1.ConditionFalse, Reason: reason, Message: message, ObservedGeneration: p.Generation})
	}
	if errors.As(cause, &externalSecretErr) {
		meta.SetStatusCondition(&p.Status.Conditions, metav1.Condition{Type: conditionExternalSecretsReady, Status: metav1.ConditionFalse, Reason: reason, Message: message, ObservedGeneration: p.Generation})
	}
	if errors.As(cause, &exposureErr) {
		meta.SetStatusCondition(&p.Status.Conditions, metav1.Condition{Type: conditionExposureReady, Status: metav1.ConditionFalse, Reason: reason, Message: message, ObservedGeneration: p.Generation})
	}
	p.Status.ObservedGeneration = p.Generation
	p.Status.DesiredReplicas = desiredReplicas(p)
	if !resource.ServiceEnabled(p) {
		p.Status.Endpoint = ""
	}
	changed, err := r.patchStatus(ctx, before, p)
	if err != nil {
		return ctrl.Result{}, retryError(err)
	}
	if changed && r.Recorder != nil {
		r.Recorder.Eventf(p, nil, corev1.EventTypeWarning, reason, "Reconcile", message)
	}
	if r.Telemetry != nil {
		r.Telemetry.RecordFailure(reason)
	}
	if permanent {
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}
	return ctrl.Result{}, retryError(cause)
}

func (r *AIWorkloadReconciler) patchStatus(ctx context.Context, before, after *platformv1alpha1.AIWorkload) (bool, error) {
	if apiequality.Semantic.DeepEqual(before.Status, after.Status) {
		return false, nil
	}
	err := r.Status().Patch(ctx, after, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{}))
	if err == nil || !apierrors.IsConflict(err) {
		return err == nil, err
	}
	// Child status events can race this controller's own status patch. Retry once
	// from a fresh primary object, but never apply an old generation's observation
	// over a newer desired spec.
	fresh := &platformv1alpha1.AIWorkload{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(after), fresh); err != nil {
		return false, err
	}
	if fresh.Generation != after.Generation {
		return false, err
	}
	freshBefore := fresh.DeepCopy()
	fresh.Status = after.Status
	fresh.Status.Conditions = preserveUnmanagedConditions(after.Status.Conditions, freshBefore.Status.Conditions)
	if apiequality.Semantic.DeepEqual(freshBefore.Status, fresh.Status) {
		return false, nil
	}
	err = r.Status().Patch(ctx, fresh, client.MergeFromWithOptions(freshBefore, client.MergeFromWithOptimisticLock{}))
	return err == nil, err
}

func preserveUnmanagedConditions(desired, current []metav1.Condition) []metav1.Condition {
	result := append([]metav1.Condition(nil), desired...)
	for _, condition := range current {
		if condition.Type == conditionReady || condition.Type == conditionProgressing || condition.Type == conditionDegraded || condition.Type == conditionTenantReady || condition.Type == conditionExternalSecretsReady || condition.Type == conditionExposureReady {
			continue
		}
		if meta.FindStatusCondition(result, condition.Type) == nil {
			result = append(result, condition)
		}
	}
	return result
}
