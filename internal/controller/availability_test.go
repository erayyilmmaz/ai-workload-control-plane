// SPDX-License-Identifier: Apache-2.0
package controller

import (
	"testing"

	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/resource"
)

func TestAvailabilityConditionReportsPDBAndSingleReplicaBoundary(t *testing.T) {
	p, scheme, _ := setup(t)
	p.Spec.Replicas = ptr.To(int32(2))
	p.Spec.Availability = &platform.AvailabilitySpec{Enabled: ptr.To(true), MinAvailable: ptr.To(int32(1))}
	pdb := &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: resource.ChildName(p.Name), Namespace: p.Namespace}}
	r := &AIWorkloadReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(pdb).Build()}
	if err := r.observeAvailability(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	condition := meta.FindStatusCondition(p.Status.Conditions, conditionAvailabilityReady)
	if condition == nil || condition.Status != metav1.ConditionTrue || condition.Reason != "PDBActive" {
		t.Fatalf("expected active PDB condition: %+v", condition)
	}
	p.Spec.Replicas = ptr.To(int32(1))
	if err := r.observeAvailability(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	condition = meta.FindStatusCondition(p.Status.Conditions, conditionAvailabilityReady)
	if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "SingleReplicaDisruptionBlocked" {
		t.Fatalf("expected single replica warning: %+v", condition)
	}
}

func TestAvailabilityConditionHandlesDisabledAndPendingPDB(t *testing.T) {
	p, scheme, _ := setup(t)
	p.Spec.Availability = &platform.AvailabilitySpec{Enabled: ptr.To(true), MaxUnavailable: ptr.To(int32(1))}
	r := &AIWorkloadReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}
	if err := r.observeAvailability(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	condition := meta.FindStatusCondition(p.Status.Conditions, conditionAvailabilityReady)
	if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "PDBReconciling" {
		t.Fatalf("expected pending PDB condition: %+v", condition)
	}
	p.Spec.Availability.Enabled = ptr.To(false)
	if err := r.observeAvailability(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	if meta.FindStatusCondition(p.Status.Conditions, conditionAvailabilityReady) != nil {
		t.Fatal("disabled availability must remove its condition")
	}
}
