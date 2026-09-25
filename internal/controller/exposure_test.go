// SPDX-License-Identifier: Apache-2.0
package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/resource"
)

type gatewayMetadataReader map[string]*unstructured.Unstructured

func (r gatewayMetadataReader) Get(_ context.Context, key client.ObjectKey, object client.Object, _ ...client.GetOption) error {
	source, ok := r[key.Namespace+"/"+key.Name]
	if !ok {
		return apierrors.NewNotFound(schema.GroupResource{Group: "gateway.networking.k8s.io", Resource: "metadata"}, key.Name)
	}
	target, ok := object.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("unexpected Gateway API metadata object %T", object)
	}
	target.Object = source.DeepCopy().Object
	target.SetGroupVersionKind(source.GroupVersionKind())
	return nil
}

func (gatewayMetadataReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return nil
}

func readyGateway(namespace, name string) *unstructured.Unstructured {
	gateway := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": gatewayGVK.GroupVersion().String(), "kind": gatewayGVK.Kind,
		"metadata": map[string]any{"namespace": namespace, "name": name},
		"status":   map[string]any{"conditions": []any{map[string]any{"type": "Programmed", "status": "True"}}},
	}}
	gateway.SetGroupVersionKind(gatewayGVK)
	return gateway
}

func readyHTTPRoute(namespace, name, gateway, section string) *unstructured.Unstructured {
	route := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": resource.HTTPRouteGVK.GroupVersion().String(), "kind": resource.HTTPRouteGVK.Kind,
		"metadata": map[string]any{"namespace": namespace, "name": name},
		"status": map[string]any{"parents": []any{map[string]any{
			"parentRef": map[string]any{"name": gateway, "sectionName": section},
			"conditions": []any{
				map[string]any{"type": "Accepted", "status": "True"},
				map[string]any{"type": "ResolvedRefs", "status": "True"},
			},
		}}},
	}}
	route.SetGroupVersionKind(resource.HTTPRouteGVK)
	return route
}

func TestHTTPRouteExposureIsNamespaceLocalAndTracksGatewayStatus(t *testing.T) {
	workload := &platform.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: "sample", Namespace: "workloads", UID: "uid"}, Spec: platform.AIWorkloadSpec{
		Image: "example.invalid/app:v1", Container: platform.ContainerSpec{Port: 8080},
		Network:  &platform.NetworkSpec{Enabled: ptr.To(false)},
		Exposure: &platform.ExposureSpec{Mode: platform.ExposureHTTPRoute, Gateway: "platform-gateway", SectionName: "http", Hostname: "api.example.test", Path: "/v1"},
	}}
	child := resource.ChildName(workload.Name)
	reader := gatewayMetadataReader{
		"workloads/platform-gateway": readyGateway("workloads", "platform-gateway"),
		"workloads/" + child:         readyHTTPRoute("workloads", child, "platform-gateway", "http"),
	}
	r := AIWorkloadReconciler{GatewayReader: reader, CapabilityLookup: availableCapabilities{}}
	state, err := r.validateExposure(t.Context(), workload)
	if err != nil || !state.manageRoute || !state.active {
		t.Fatalf("state=%+v err=%v", state, err)
	}
	intent, err := resource.HTTPRouteIntent(workload)
	if err != nil || intent.Absent {
		t.Fatalf("intent=%#v err=%v", intent, err)
	}
	route := intent.Object.DeepCopyObject().(*unstructured.Unstructured)
	if err := intent.Mutate(route); err != nil {
		t.Fatal(err)
	}
	parents, _, err := unstructured.NestedSlice(route.Object, "spec", "parentRefs")
	if err != nil || len(parents) != 1 || parents[0].(map[string]any)["name"] != "platform-gateway" {
		t.Fatalf("unexpected parentRefs=%#v err=%v", parents, err)
	}
	routeSpec, found, err := unstructured.NestedMap(route.Object, "spec")
	if err != nil || !found {
		t.Fatalf("route spec missing: %v", err)
	}
	rules := routeSpec["rules"].([]any)
	backend := rules[0].(map[string]any)["backendRefs"].([]any)[0].(map[string]any)
	if backend["name"] != child || backend["port"] != int64(80) {
		t.Fatalf("route backend escaped Service boundary: %#v", backend)
	}
	requeue, err := r.observeExposure(t.Context(), workload, state)
	if err != nil || requeue != time.Minute {
		t.Fatalf("requeue=%v err=%v", requeue, err)
	}
	condition := findCondition(workload.Status.Conditions, conditionExposureReady)
	if condition == nil || condition.Status != "True" || condition.Reason != "HTTPRouteReady" {
		t.Fatalf("exposure condition=%#v", condition)
	}
}

func TestHTTPRouteExposureRejectsUnreadyGatewayAndV0DoesNotDiscoverGatewayAPI(t *testing.T) {
	workload := &platform.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: "sample", Namespace: "workloads", UID: "uid"}, Spec: platform.AIWorkloadSpec{
		Image: "example.invalid/app:v1", Container: platform.ContainerSpec{Port: 8080},
		Network:  &platform.NetworkSpec{Enabled: ptr.To(false)},
		Exposure: &platform.ExposureSpec{Mode: platform.ExposureHTTPRoute, Gateway: "platform-gateway", SectionName: "http", Hostname: "api.example.test", Path: "/"},
	}}
	r := AIWorkloadReconciler{GatewayReader: gatewayMetadataReader{"workloads/platform-gateway": &unstructured.Unstructured{Object: map[string]any{"status": map[string]any{}}}}, CapabilityLookup: availableCapabilities{}}
	if _, err := r.validateExposure(t.Context(), workload); err == nil {
		t.Fatal("unready Gateway was accepted")
	} else if configuration, ok := err.(exposureConfigurationError); !ok || configuration.reason != "GatewayNotReady" {
		t.Fatalf("unexpected error=%v", err)
	}
	protected := workload.DeepCopy()
	protected.Spec.Network = nil
	readyReader := &AIWorkloadReconciler{GatewayReader: gatewayMetadataReader{"workloads/platform-gateway": readyGateway("workloads", "platform-gateway")}, CapabilityLookup: availableCapabilities{}}
	if _, err := readyReader.validateExposure(t.Context(), protected); err == nil {
		t.Fatal("HTTPRoute exposure bypassed the default same-namespace NetworkPolicy boundary")
	} else if configuration, ok := err.(exposureConfigurationError); !ok || configuration.reason != "ExposureNetworkPolicyEnabled" {
		t.Fatalf("unexpected network boundary error=%v", err)
	}
	legacy := workload.DeepCopy()
	legacy.Spec.Exposure = nil
	state, err := (&AIWorkloadReconciler{CapabilityLookup: unavailableCapabilities{}}).validateExposure(t.Context(), legacy)
	if err != nil || state.manageRoute || state.active {
		t.Fatalf("V0 workload must not discover Gateway API: state=%+v err=%v", state, err)
	}
	legacy.Status.Conditions = []metav1.Condition{{Type: conditionExposureReady, Status: metav1.ConditionFalse, Reason: "GatewayAPIUnavailable"}}
	state, err = (&AIWorkloadReconciler{CapabilityLookup: unavailableCapabilities{}}).validateExposure(t.Context(), legacy)
	if err != nil || state.manageRoute || findCondition(legacy.Status.Conditions, conditionExposureReady) != nil {
		t.Fatalf("ClusterLocal cleanup must not retain a stale exposure condition when the API was removed: state=%+v conditions=%#v err=%v", state, legacy.Status.Conditions, err)
	}
}
