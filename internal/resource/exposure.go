// SPDX-License-Identifier: Apache-2.0
package resource

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
)

// HTTPRouteGVK deliberately uses an unstructured object: Gateway API remains
// optional, so the manager must neither register its Go types nor require its
// CRDs at startup.
var HTTPRouteGVK = schema.GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1", Kind: "HTTPRoute"}

// HTTPRouteEnabled reports whether this desired workload asks AWCP to own a
// route. Nil and ClusterLocal exposure preserve the V0 Service-only contract.
func HTTPRouteEnabled(p *platform.AIWorkload) bool {
	return p != nil && p.Spec.Exposure != nil && p.Spec.Exposure.Mode == platform.ExposureHTTPRoute
}

// HTTPRouteIntent creates one same-namespace HTTPRoute targeting the existing
// AWCP ClusterIP Service. The caller decides whether the optional API must be
// managed at all, allowing untouched V0 workloads to run without Gateway API.
func HTTPRouteIntent(p *platform.AIWorkload) (Intent, error) {
	if p == nil {
		return Intent{}, ErrInvalidConfiguration
	}
	route := &unstructured.Unstructured{}
	route.SetGroupVersionKind(HTTPRouteGVK)
	route.SetNamespace(p.Namespace)
	route.SetName(ChildName(p.Name))
	intent := Intent{Object: route, Absent: !HTTPRouteEnabled(p)}
	if intent.Absent {
		return intent, nil
	}
	if !ServiceEnabled(p) || p.Spec.Exposure.Gateway == "" || p.Spec.Exposure.SectionName == "" || p.Spec.Exposure.Hostname == "" || p.Spec.Exposure.Path == "" {
		return Intent{}, ErrInvalidConfiguration
	}
	intent.Mutate = func(object client.Object) error {
		target, ok := object.(*unstructured.Unstructured)
		if !ok || target.GroupVersionKind() != HTTPRouteGVK {
			return ErrInvalidConfiguration
		}
		mutateHTTPRoute(target, p)
		return nil
	}
	return intent, nil
}

func mutateHTTPRoute(route *unstructured.Unstructured, p *platform.AIWorkload) {
	labels := route.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	for key, value := range SelectorLabels(p) {
		labels[key] = value
	}
	labels["app.kubernetes.io/name"] = "ai-workload"
	labels["app.kubernetes.io/part-of"] = "ai-workload-control-plane"
	labels["app.kubernetes.io/managed-by"] = "awcp-controller"
	if p.Spec.Environment != "" {
		labels[EnvironmentLabel] = p.Spec.Environment
	} else {
		delete(labels, EnvironmentLabel)
	}
	if p.Spec.Tenant != "" {
		labels[TenantLabel] = p.Spec.Tenant
	} else {
		delete(labels, TenantLabel)
	}
	route.SetLabels(labels)
	annotations := route.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[WorkloadNameAnnotation] = p.Name
	route.SetAnnotations(annotations)
	_ = unstructured.SetNestedField(route.Object, map[string]any{
		"parentRefs": []any{map[string]any{
			"name":        p.Spec.Exposure.Gateway,
			"sectionName": p.Spec.Exposure.SectionName,
		}},
		"hostnames": []any{p.Spec.Exposure.Hostname},
		"rules": []any{map[string]any{
			"matches": []any{map[string]any{
				"path": map[string]any{"type": "PathPrefix", "value": p.Spec.Exposure.Path},
			}},
			"backendRefs": []any{map[string]any{
				"name": ChildName(p.Name), "port": int64(ServicePort(p)),
			}},
		}},
	}, "spec")
}
