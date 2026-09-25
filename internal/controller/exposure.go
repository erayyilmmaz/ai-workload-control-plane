// SPDX-License-Identifier: Apache-2.0
package controller

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platformv1alpha1 "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/capability"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/resource"
)

const conditionExposureReady = "ExposureReady"

var gatewayGVK = schema.GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1", Kind: "Gateway"}

// exposureConfigurationError is a permanent, curated configuration/dependency
// observation. The reason and message are safe for status and Events.
type exposureConfigurationError struct {
	reason  string
	message string
}

func (e exposureConfigurationError) Error() string { return e.reason }

type exposureState struct {
	manageRoute bool
	active      bool
}

// validateExposure makes Gateway API discovery and direct Gateway reads only
// for a workload that opts in (or needs to safely delete its previous route).
// It never creates a Gateway or reads cross-namespace traffic configuration.
func (r *AIWorkloadReconciler) validateExposure(ctx context.Context, workload *platformv1alpha1.AIWorkload) (exposureState, error) {
	active := resource.HTTPRouteEnabled(workload)
	previous := meta.FindStatusCondition(workload.Status.Conditions, conditionExposureReady) != nil
	if !active && !previous {
		return exposureState{}, nil
	}
	if r.CapabilityLookup == nil {
		return exposureState{}, exposureConfigurationError{reason: "GatewayAPIUnavailable", message: "Gateway API v1 is unavailable; install it or select ClusterLocal exposure."}
	}
	requirement, ok := capability.RequirementFor(capability.GatewayAPI)
	if !ok {
		return exposureState{}, fmt.Errorf("gateway API capability contract is missing")
	}
	observation := capability.Detect(ctx, r.CapabilityLookup, requirement)
	if observation.State == capability.Unavailable {
		// A removed HTTPRoute CRD also removes its instances. If the user has
		// switched back to ClusterLocal, no optional API operation remains and a
		// stale condition must not block the V0 reconciliation path.
		if !active {
			meta.RemoveStatusCondition(&workload.Status.Conditions, conditionExposureReady)
			return exposureState{}, nil
		}
		return exposureState{}, exposureConfigurationError{reason: "GatewayAPIUnavailable", message: "Gateway API v1 is unavailable; install it or select ClusterLocal exposure."}
	}
	if observation.State == capability.Unknown {
		return exposureState{}, fmt.Errorf("discover Gateway API: %w", observation.Err)
	}
	state := exposureState{manageRoute: true, active: active}
	if !active {
		return state, nil
	}
	if !resource.ServiceEnabled(workload) {
		return exposureState{}, exposureConfigurationError{reason: "ExposureServiceDisabled", message: "HTTPRoute exposure requires the workload ClusterIP Service to be enabled."}
	}
	if resource.NetworkEnabled(workload) {
		return exposureState{}, exposureConfigurationError{reason: "ExposureNetworkPolicyEnabled", message: "HTTPRoute exposure requires spec.network.enabled=false; Gateway data-plane ingress policy is platform-managed."}
	}
	reader := r.GatewayReader
	if reader == nil {
		reader = r.Client
	}
	gateway := &unstructured.Unstructured{}
	gateway.SetGroupVersionKind(gatewayGVK)
	if err := reader.Get(ctx, client.ObjectKey{Namespace: workload.Namespace, Name: workload.Spec.Exposure.Gateway}, gateway); err != nil {
		if apierrors.IsNotFound(err) {
			return exposureState{}, exposureConfigurationError{reason: "GatewayNotFound", message: "The referenced Gateway is unavailable in the workload namespace."}
		}
		return exposureState{}, fmt.Errorf("get Gateway metadata: %w", err)
	}
	if !hasTrueCondition(gateway, "Programmed") {
		return exposureState{}, exposureConfigurationError{reason: "GatewayNotReady", message: "The referenced Gateway is not Programmed; inspect the platform Gateway status."}
	}
	return state, nil
}

// observeExposure reads only HTTPRoute status after AWCP has applied the route.
// Periodic rechecks intentionally replace an optional-API watch so a manager can
// start on clusters where Gateway API is not installed.
func (r *AIWorkloadReconciler) observeExposure(ctx context.Context, workload *platformv1alpha1.AIWorkload, state exposureState) (time.Duration, error) {
	if !state.manageRoute {
		return 0, nil
	}
	if !state.active {
		meta.RemoveStatusCondition(&workload.Status.Conditions, conditionExposureReady)
		return 0, nil
	}
	reader := r.GatewayReader
	if reader == nil {
		reader = r.Client
	}
	route := &unstructured.Unstructured{}
	route.SetGroupVersionKind(resource.HTTPRouteGVK)
	if err := reader.Get(ctx, client.ObjectKey{Namespace: workload.Namespace, Name: resource.ChildName(workload.Name)}, route); err != nil {
		if apierrors.IsNotFound(err) {
			setExposureCondition(workload, metav1.ConditionFalse, "HTTPRoutePending", "The owned HTTPRoute is awaiting Gateway API observation.")
			return 15 * time.Second, nil
		}
		return 0, fmt.Errorf("observe HTTPRoute status: %w", err)
	}
	accepted, resolved, observed := httpRouteConditions(route, workload.Spec.Exposure.Gateway, workload.Spec.Exposure.SectionName)
	switch {
	case !observed:
		setExposureCondition(workload, metav1.ConditionFalse, "HTTPRoutePending", "The owned HTTPRoute is awaiting a matching Gateway status observation.")
		return 15 * time.Second, nil
	case accepted == metav1.ConditionFalse:
		setExposureCondition(workload, metav1.ConditionFalse, "HTTPRouteRejected", "The platform Gateway rejected the owned HTTPRoute; inspect Gateway listener policy and route configuration.")
		return time.Minute, nil
	case accepted != metav1.ConditionTrue:
		setExposureCondition(workload, metav1.ConditionFalse, "HTTPRoutePending", "The owned HTTPRoute is awaiting Gateway acceptance.")
		return 15 * time.Second, nil
	case resolved == metav1.ConditionFalse:
		setExposureCondition(workload, metav1.ConditionFalse, "HTTPRouteReferencesNotResolved", "The platform Gateway cannot resolve the HTTPRoute backend reference.")
		return time.Minute, nil
	case resolved != metav1.ConditionTrue:
		setExposureCondition(workload, metav1.ConditionFalse, "HTTPRoutePending", "The owned HTTPRoute is awaiting backend reference resolution.")
		return 15 * time.Second, nil
	default:
		setExposureCondition(workload, metav1.ConditionTrue, "HTTPRouteReady", "The owned HTTPRoute is accepted and its backend reference is resolved.")
		return time.Minute, nil
	}
}

func setExposureCondition(workload *platformv1alpha1.AIWorkload, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&workload.Status.Conditions, metav1.Condition{Type: conditionExposureReady, Status: status, Reason: reason, Message: message, ObservedGeneration: workload.Generation})
}

func hasTrueCondition(object *unstructured.Unstructured, typ string) bool {
	conditions, found, err := unstructured.NestedSlice(object.Object, "status", "conditions")
	if err != nil || !found {
		return false
	}
	for _, value := range conditions {
		condition, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if condition["type"] == typ && condition["status"] == "True" {
			return true
		}
	}
	return false
}

func httpRouteConditions(route *unstructured.Unstructured, gateway, sectionName string) (metav1.ConditionStatus, metav1.ConditionStatus, bool) {
	parents, found, err := unstructured.NestedSlice(route.Object, "status", "parents")
	if err != nil || !found {
		return metav1.ConditionUnknown, metav1.ConditionUnknown, false
	}
	for _, value := range parents {
		parent, ok := value.(map[string]any)
		if !ok {
			continue
		}
		parentRef, ok := parent["parentRef"].(map[string]any)
		if !ok || parentRef["name"] != gateway || parentRef["sectionName"] != sectionName {
			continue
		}
		conditions, ok := parent["conditions"].([]any)
		if !ok {
			return metav1.ConditionUnknown, metav1.ConditionUnknown, true
		}
		accepted, resolved := metav1.ConditionUnknown, metav1.ConditionUnknown
		for _, value := range conditions {
			condition, ok := value.(map[string]any)
			if !ok {
				continue
			}
			status, ok := condition["status"].(string)
			if !ok {
				continue
			}
			switch condition["type"] {
			case "Accepted":
				accepted = metav1.ConditionStatus(status)
			case "ResolvedRefs":
				resolved = metav1.ConditionStatus(status)
			}
		}
		return accepted, resolved, true
	}
	return metav1.ConditionUnknown, metav1.ConditionUnknown, false
}
