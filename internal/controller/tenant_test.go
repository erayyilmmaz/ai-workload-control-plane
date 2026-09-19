// SPDX-License-Identifier: Apache-2.0
package controller

import (
	"testing"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/resource"
	"github.com/erayyilmmaz/ai-workload-control-plane/test/fixtures"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestTenantProfileValidationAndMismatchCondition(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := platform.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	profile := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: tenantProfileConfigMap, Namespace: "awcp-tenant-alpha"}, Data: map[string]string{"tenant": "alpha", "profile": "small"}}
	workload := &platform.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: "sample", Namespace: "awcp-tenant-alpha", Generation: 1}, Spec: platform.AIWorkloadSpec{Tenant: "bravo", Image: "example.invalid/app:v1", Container: platform.ContainerSpec{Port: 8080}}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(workload).WithObjects(workload, profile).Build()
	r := AIWorkloadReconciler{Client: c, TenantReader: c, WatchNamespace: workload.Namespace, Builder: resource.BuilderFunc(fixtures.Plan)}
	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(workload)}); err != nil {
		t.Fatal(err)
	}
	actual := &platform.AIWorkload{}
	if err := c.Get(t.Context(), client.ObjectKeyFromObject(workload), actual); err != nil {
		t.Fatal(err)
	}
	for _, typ := range []string{conditionReady, conditionDegraded, conditionTenantReady} {
		condition := meta.FindStatusCondition(actual.Status.Conditions, typ)
		if condition == nil || condition.Reason != "TenantMismatch" {
			t.Fatalf("%s condition = %#v, want TenantMismatch", typ, condition)
		}
	}
	if meta.FindStatusCondition(actual.Status.Conditions, conditionTenantReady).Status != metav1.ConditionFalse {
		t.Fatal("mismatched tenant must not be marked ready")
	}
}

func TestTenantProfileIsNamespaceLocalAndQuotaFailuresAreSafe(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := platform.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	workload := &platform.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: "sample", Namespace: "awcp-tenant-alpha", Generation: 1}, Spec: platform.AIWorkloadSpec{Tenant: "alpha", Image: "example.invalid/app:v1", Container: platform.ContainerSpec{Port: 8080}}}
	profile := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: tenantProfileConfigMap, Namespace: "awcp-tenant-bravo"}, Data: map[string]string{"tenant": "alpha", "profile": "small"}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(workload).WithObjects(workload, profile).Build()
	r := AIWorkloadReconciler{Client: c, TenantReader: c, WatchNamespace: workload.Namespace}
	if err := r.validateTenant(t.Context(), workload.DeepCopy()); err == nil {
		t.Fatal("a tenant profile in another namespace must not satisfy the workload")
	}
	forbidden := apierrors.NewForbidden(schema.GroupResource{Group: "apps", Resource: "deployments"}, "sample", &apierrors.StatusError{ErrStatus: metav1.Status{Message: "exceeded quota: awcp-tenant-quota"}})
	if _, err := r.reportFailure(t.Context(), workload, forbidden); err == nil {
		t.Fatal("quota failure should remain retryable without crashing the reconciler")
	}
	actual := &platform.AIWorkload{}
	if err := c.Get(t.Context(), client.ObjectKeyFromObject(workload), actual); err != nil {
		t.Fatal(err)
	}
	if condition := meta.FindStatusCondition(actual.Status.Conditions, conditionDegraded); condition == nil || condition.Reason != "QuotaExceeded" || condition.Status != metav1.ConditionTrue {
		t.Fatalf("quota condition = %#v", condition)
	}
}

func TestExplicitTenantWatchScopeNeverExpands(t *testing.T) {
	r := AIWorkloadReconciler{WatchNamespaces: []string{"awcp-tenant-alpha", "awcp-tenant-bravo"}}
	for namespace, want := range map[string]bool{
		"awcp-tenant-alpha":   true,
		"awcp-tenant-bravo":   true,
		"awcp-tenant-charlie": false,
		"awcp-workloads":      false,
	} {
		if got := r.watchesNamespace(namespace); got != want {
			t.Fatalf("watchesNamespace(%q) = %v, want %v", namespace, got, want)
		}
	}
}
