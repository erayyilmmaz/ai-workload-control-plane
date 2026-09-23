// SPDX-License-Identifier: Apache-2.0
// Package capability declares V1 optional platform contracts without making
// manager startup depend on any optional API or controller.
package capability

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/discovery"
)

// Feature names one V1 feature family that can require a platform capability.
// These strings are API/status reasons only after the owning feature story wires
// them into reconciliation; catalog membership alone does not enable a feature.
type Feature string

const (
	ExternalSecrets             Feature = "ExternalSecrets"
	GatewayAPI                  Feature = "GatewayAPI"
	ArgoRollouts                Feature = "ArgoRollouts"
	ExternalMetrics             Feature = "ExternalMetrics"
	PrometheusMonitoring        Feature = "PrometheusMonitoring"
	ValidatingAdmissionPolicies Feature = "ValidatingAdmissionPolicies"
)

// APIResource identifies one discoverable Kubernetes API resource.
type APIResource struct {
	GroupVersion string
	Resource     string
}

// Requirement describes all Kubernetes API resources that must be discoverable
// before a feature can create its child resources. Non-Kubernetes dependencies,
// such as an OpenCost endpoint, deliberately do not appear here: their health is
// evaluated by the owning feature rather than inferred from CRD discovery.
type Requirement struct {
	Feature     Feature
	Description string
	Resources   []APIResource
}

var requirements = []Requirement{
	{
		Feature:     ExternalSecrets,
		Description: "External Secrets Operator namespaced SecretStore and ExternalSecret APIs",
		Resources: []APIResource{
			{GroupVersion: "external-secrets.io/v1", Resource: "secretstores"},
			{GroupVersion: "external-secrets.io/v1", Resource: "externalsecrets"},
		},
	},
	{
		Feature:     GatewayAPI,
		Description: "Gateway API Gateway and HTTPRoute APIs",
		Resources: []APIResource{
			{GroupVersion: "gateway.networking.k8s.io/v1", Resource: "gateways"},
			{GroupVersion: "gateway.networking.k8s.io/v1", Resource: "httproutes"},
		},
	},
	{
		Feature:     ArgoRollouts,
		Description: "Argo Rollouts Rollout API",
		Resources:   []APIResource{{GroupVersion: "argoproj.io/v1alpha1", Resource: "rollouts"}},
	},
	{
		Feature:     ExternalMetrics,
		Description: "external.metrics.k8s.io API required for HPA scale-to-zero or external metrics",
		Resources:   []APIResource{{GroupVersion: "external.metrics.k8s.io/v1beta1", Resource: "*"}},
	},
	{
		Feature:     PrometheusMonitoring,
		Description: "Prometheus Operator monitoring API for optional ServiceMonitor integration",
		Resources:   []APIResource{{GroupVersion: "monitoring.coreos.com/v1", Resource: "servicemonitors"}},
	},
	{
		Feature:     ValidatingAdmissionPolicies,
		Description: "Kubernetes admissionregistration ValidatingAdmissionPolicy API",
		Resources:   []APIResource{{GroupVersion: "admissionregistration.k8s.io/v1", Resource: "validatingadmissionpolicies"}},
	},
}

// Requirements returns a copy so callers cannot mutate the process-wide V1
// contract. Feature stories should use RequirementFor instead of duplicating API
// group/version strings.
func Requirements() []Requirement {
	copy := make([]Requirement, len(requirements))
	for i, requirement := range requirements {
		copy[i] = requirement
		copy[i].Resources = append([]APIResource(nil), requirement.Resources...)
	}
	return copy
}

// RequirementFor returns the declared platform requirement for one feature.
func RequirementFor(feature Feature) (Requirement, bool) {
	for _, requirement := range requirements {
		if requirement.Feature == feature {
			requirement.Resources = append([]APIResource(nil), requirement.Resources...)
			return requirement, true
		}
	}
	return Requirement{}, false
}

// State distinguishes an absent optional API from a discovery failure. A
// discovery failure must be retried by the owning reconciler; it must never be
// silently misreported as an absent dependency.
type State string

const (
	Available   State = "Available"
	Unavailable State = "Unavailable"
	Unknown     State = "Unknown"
)

// ResourceLookup is intentionally small. Its production adapter will be added
// by the feature story that needs it, so manager construction does not make a
// discovery request or gain broad dependency ownership.
type ResourceLookup interface {
	HasResource(context.Context, APIResource) (bool, error)
}

// DiscoveryLookup adapts Kubernetes discovery without preloading optional APIs
// at manager startup. It is intentionally read-only and does not grant a
// feature permission to create the discovered resource.
type DiscoveryLookup struct {
	Discovery discovery.DiscoveryInterface
}

func (l DiscoveryLookup) HasResource(_ context.Context, resource APIResource) (bool, error) {
	list, err := l.Discovery.ServerResourcesForGroupVersion(resource.GroupVersion)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, apiResource := range list.APIResources {
		if resource.Resource == "*" || apiResource.Name == resource.Resource {
			return true, nil
		}
	}
	return false, nil
}

// Observation is the sanitized result for one optional feature. Error is kept
// for retry classification and must not be copied verbatim into status/events.
type Observation struct {
	Requirement Requirement
	State       State
	Err         error
}

// Detect evaluates a feature requirement on demand. It has no side effects and
// is deliberately not called during manager startup. A missing resource is a
// normal Unavailable result; lookup errors are Unknown so callers can apply
// their existing transient-error/backoff policy.
func Detect(ctx context.Context, lookup ResourceLookup, requirement Requirement) Observation {
	for _, resource := range requirement.Resources {
		found, err := lookup.HasResource(ctx, resource)
		if err != nil {
			return Observation{Requirement: requirement, State: Unknown, Err: err}
		}
		if !found {
			return Observation{Requirement: requirement, State: Unavailable}
		}
	}
	return Observation{Requirement: requirement, State: Available}
}
