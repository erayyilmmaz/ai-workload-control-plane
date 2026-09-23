// SPDX-License-Identifier: Apache-2.0
package controller

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/capability"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/resource"
)

type availableCapabilities struct{}

func (availableCapabilities) HasResource(_ context.Context, _ capability.APIResource) (bool, error) {
	return true, nil
}

type unavailableCapabilities struct{}

func (unavailableCapabilities) HasResource(_ context.Context, _ capability.APIResource) (bool, error) {
	return false, nil
}

type externalMetadataReader map[string]*unstructured.Unstructured

func (r externalMetadataReader) Get(_ context.Context, key client.ObjectKey, object client.Object, _ ...client.GetOption) error {
	source, ok := r[key.Namespace+"/"+key.Name]
	if !ok {
		return apierrors.NewNotFound(schema.GroupResource{Group: "external-secrets.io", Resource: "metadata"}, key.Name)
	}
	target, ok := object.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("unexpected external metadata object %T", object)
	}
	target.Object = source.DeepCopy().Object
	target.SetGroupVersionKind(source.GroupVersionKind())
	return nil
}

func (externalMetadataReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return nil
}

func readyExternalObject(gvk schema.GroupVersionKind, namespace, name string, spec map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": gvk.GroupVersion().String(), "kind": gvk.Kind,
		"metadata": map[string]any{"namespace": namespace, "name": name},
		"spec":     spec,
		"status":   map[string]any{"conditions": []any{map[string]any{"type": "Ready", "status": "True"}}},
	}}
}

func TestExternalSecretReferencesAreNamespacedAndRotateWithoutPayload(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	workload := &platform.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: "sample", Namespace: "workloads", Generation: 4, UID: "uid"}, Spec: platform.AIWorkloadSpec{
		Image: "example.invalid/app:v1", Container: platform.ContainerSpec{Port: 8080},
		ExternalSecrets: []platform.ExternalSecretReference{{ExternalSecret: "settings-sync", TargetSecret: "settings"}},
	}}
	target := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "settings", Namespace: "workloads", ResourceVersion: "42"}}
	cache := fake.NewClientBuilder().WithScheme(scheme).WithObjects(target).Build()
	store := readyExternalObject(secretStoreGVK, "workloads", "team-store", map[string]any{})
	external := readyExternalObject(externalSecretGVK, "workloads", "settings-sync", map[string]any{
		"secretStoreRef": map[string]any{"name": "team-store", "kind": "SecretStore"},
		"target":         map[string]any{"name": "settings"},
	})
	r := AIWorkloadReconciler{Client: cache, ExternalSecretsReader: externalMetadataReader{"workloads/team-store": store, "workloads/settings-sync": external}, CapabilityLookup: availableCapabilities{}}
	resolved, err := r.validateExternalSecrets(t.Context(), workload)
	if err != nil || len(resolved) != 1 || resolved[0].targetSecret != "settings" || resolved[0].resourceVersion != "42" {
		t.Fatalf("resolved=%#v err=%v", resolved, err)
	}
	if condition := findCondition(workload.Status.Conditions, conditionExternalSecretsReady); condition == nil || condition.Status != metav1.ConditionTrue {
		t.Fatalf("external readiness condition = %#v", condition)
	}
	desired := workload.DeepCopy()
	desired.Spec.SecretRefs = append(desired.Spec.SecretRefs, resolved[0].targetSecret)
	plan, err := (resource.WorkloadBuilder{}).Build(desired)
	if err != nil {
		t.Fatal(err)
	}
	plan = decorateExternalSecretRevision(plan, externalSecretRevision(resolved))
	for _, intent := range plan {
		deployment, ok := intent.Object.(*appsv1.Deployment)
		if !ok {
			continue
		}
		if err := intent.Mutate(deployment); err != nil {
			t.Fatal(err)
		}
		if len(deployment.Spec.Template.Spec.Containers[0].EnvFrom) != 1 || deployment.Spec.Template.Spec.Containers[0].EnvFrom[0].SecretRef.Name != "settings" || deployment.Spec.Template.Annotations[externalSecretRevisionKey] == "" {
			t.Fatalf("external target was not injected/revisioned: %#v", deployment.Spec.Template)
		}
	}
}

func TestExternalSecretUnavailableSetsBoundedConfigurationStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := platform.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	workload := &platform.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: "sample", Namespace: "workloads", Generation: 7}, Spec: platform.AIWorkloadSpec{
		Image: "example.invalid/app:v1", Container: platform.ContainerSpec{Port: 8080},
		ExternalSecrets: []platform.ExternalSecretReference{{ExternalSecret: "settings-sync", TargetSecret: "settings"}},
	}}
	cache := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(workload).WithObjects(workload).Build()
	r := AIWorkloadReconciler{
		Client:           cache,
		WatchNamespace:   workload.Namespace,
		CapabilityLookup: unavailableCapabilities{},
		Builder: resource.BuilderFunc(func(*platform.AIWorkload) ([]resource.Intent, error) {
			t.Fatal("builder must not run when the optional ESO API is unavailable")
			return nil, nil
		}),
	}
	result, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(workload)})
	if err != nil || result.RequeueAfter != time.Minute {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	actual := &platform.AIWorkload{}
	if err := cache.Get(t.Context(), client.ObjectKeyFromObject(workload), actual); err != nil {
		t.Fatal(err)
	}
	condition := findCondition(actual.Status.Conditions, conditionExternalSecretsReady)
	if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "ExternalSecretsUnavailable" {
		t.Fatalf("external secret failure condition=%#v", condition)
	}
}

func TestExternalSecretRejectsClusterStoreAndSecretIndexIncludesTarget(t *testing.T) {
	workload := &platform.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: "sample", Namespace: "workloads"}, Spec: platform.AIWorkloadSpec{ExternalSecrets: []platform.ExternalSecretReference{{ExternalSecret: "settings-sync", TargetSecret: "settings"}}}}
	external := readyExternalObject(externalSecretGVK, "workloads", "settings-sync", map[string]any{"secretStoreRef": map[string]any{"name": "shared", "kind": "ClusterSecretStore"}, "target": map[string]any{"name": "settings"}})
	r := AIWorkloadReconciler{ExternalSecretsReader: externalMetadataReader{"workloads/settings-sync": external}, CapabilityLookup: availableCapabilities{}}
	if _, err := r.validateExternalSecrets(t.Context(), workload); err == nil {
		t.Fatal("ClusterSecretStore reference was accepted")
	} else {
		var configuration externalSecretConfigurationError
		if !errors.As(err, &configuration) || configuration.reason != "ExternalSecretStoreScopeInvalid" {
			t.Fatalf("error=%v", err)
		}
	}
	keys := secretIndex(workload)
	if len(keys) != 1 || keys[0] != "settings" {
		t.Fatalf("external target missing from Secret watch index: %#v", keys)
	}
}

func findCondition(conditions []metav1.Condition, typ string) *metav1.Condition {
	for index := range conditions {
		if conditions[index].Type == typ {
			return &conditions[index]
		}
	}
	return nil
}
