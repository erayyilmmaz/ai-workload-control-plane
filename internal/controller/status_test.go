// SPDX-License-Identifier: Apache-2.0
package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/resource"
	"github.com/erayyilmmaz/ai-workload-control-plane/test/fixtures"
)

func TestAssessDeployment(t *testing.T) {
	p, _, _ := setup(t)
	p.Generation = 7
	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Generation: 4}}

	for _, tc := range []struct {
		name                         string
		prepare                      func()
		reason                       string
		ready, progressing, degraded metav1.ConditionStatus
	}{
		{
			name: "current rollout ready",
			prepare: func() {
				deployment.Status = appsv1.DeploymentStatus{ObservedGeneration: 4, UpdatedReplicas: 1, Replicas: 1, ReadyReplicas: 1, AvailableReplicas: 1, Conditions: []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue}}}
			},
			reason: "WorkloadReady", ready: metav1.ConditionTrue, progressing: metav1.ConditionFalse, degraded: metav1.ConditionFalse,
		},
		{
			name: "old rollout cannot be ready",
			prepare: func() {
				deployment.Status = appsv1.DeploymentStatus{ObservedGeneration: 3, UpdatedReplicas: 1, Replicas: 1, ReadyReplicas: 1, AvailableReplicas: 1, Conditions: []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue}}}
			},
			reason: "Reconciling", ready: metav1.ConditionFalse, progressing: metav1.ConditionTrue, degraded: metav1.ConditionFalse,
		},
		{
			name: "deployment unavailable",
			prepare: func() {
				deployment.Status = appsv1.DeploymentStatus{ObservedGeneration: 4, Conditions: []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionFalse}}}
			},
			reason: "DeploymentUnavailable", ready: metav1.ConditionFalse, progressing: metav1.ConditionTrue, degraded: metav1.ConditionFalse,
		},
		{
			name: "progress deadline exceeded",
			prepare: func() {
				deployment.Status = appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse, Reason: "ProgressDeadlineExceeded"}}}
			},
			reason: "ProgressDeadlineExceeded", ready: metav1.ConditionFalse, progressing: metav1.ConditionFalse, degraded: metav1.ConditionTrue,
		},
		{
			name: "scaled to zero overrides rollout observation",
			prepare: func() {
				zero := int32(0)
				p.Spec.Replicas = &zero
				deployment.Status = appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse, Reason: "ProgressDeadlineExceeded"}}}
			},
			reason: "ScaledToZero", ready: metav1.ConditionFalse, progressing: metav1.ConditionFalse, degraded: metav1.ConditionFalse,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			one := int32(1)
			p.Spec.Replicas = &one
			tc.prepare()
			actual := assessDeployment(p, deployment)
			if actual.reason != tc.reason || actual.ready != tc.ready || actual.progressing != tc.progressing || actual.degraded != tc.degraded {
				t.Fatalf("assessment=%+v", actual)
			}
		})
	}
}

func TestSetAssessmentConditionsPreservesTransitionTimeForSameStatus(t *testing.T) {
	p, _, _ := setup(t)
	p.Generation = 2
	then := metav1.NewTime(time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC))
	p.Status.Conditions = []metav1.Condition{
		{Type: conditionReady, Status: metav1.ConditionFalse, Reason: "Old", Message: "old", ObservedGeneration: 1, LastTransitionTime: then},
		{Type: conditionProgressing, Status: metav1.ConditionTrue, Reason: "Old", Message: "old", ObservedGeneration: 1, LastTransitionTime: then},
		{Type: conditionDegraded, Status: metav1.ConditionFalse, Reason: "Old", Message: "old", ObservedGeneration: 1, LastTransitionTime: then},
	}
	setAssessmentConditions(p, assessDeployment(p, &appsv1.Deployment{}))
	for _, typ := range []string{conditionReady, conditionProgressing, conditionDegraded} {
		condition := meta.FindStatusCondition(p.Status.Conditions, typ)
		if condition == nil || !condition.LastTransitionTime.Equal(&then) || condition.ObservedGeneration != p.Generation || condition.Reason != "Reconciling" {
			t.Fatalf("condition %q changed transition or was not refreshed safely: %+v", typ, condition)
		}
	}
}

func TestReconcileReportsDeploymentStatusWithoutEventSpam(t *testing.T) {
	p, scheme, _ := setup(t)
	base := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(p).WithObjects(p).Build()
	statusWrites := 0
	wrapped := interceptor.NewClient(base, interceptor.Funcs{
		SubResourcePatch: func(ctx context.Context, c client.Client, sub string, o client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
			statusWrites++
			return c.SubResource(sub).Patch(ctx, o, patch, opts...)
		},
	})
	recorder := events.NewFakeRecorder(10)
	r := AIWorkloadReconciler{Client: wrapped, WatchNamespace: p.Namespace, Scheme: scheme, Builder: resource.BuilderFunc(fixtures.Plan), Recorder: recorder}
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(p)}
	if _, err := r.Reconcile(t.Context(), req); err != nil {
		t.Fatal(err)
	}

	deployment := &appsv1.Deployment{}
	if err := base.Get(t.Context(), client.ObjectKey{Namespace: p.Namespace, Name: resource.ChildName(p.Name)}, deployment); err != nil {
		t.Fatal(err)
	}
	deployment.Status = appsv1.DeploymentStatus{ObservedGeneration: deployment.Generation, UpdatedReplicas: 1, Replicas: 1, ReadyReplicas: 1, AvailableReplicas: 1, Conditions: []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue}}}
	if err := base.Status().Update(t.Context(), deployment); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(t.Context(), req); err != nil {
		t.Fatal(err)
	}
	actual := &platform.AIWorkload{}
	if err := base.Get(t.Context(), req.NamespacedName, actual); err != nil {
		t.Fatal(err)
	}
	if actual.Status.ObservedGeneration != actual.Generation || actual.Status.DesiredReplicas != 1 || actual.Status.ReadyReplicas != 1 || actual.Status.Endpoint != resource.ServiceEndpoint(p) || !meta.IsStatusConditionTrue(actual.Status.Conditions, conditionReady) || len(recorder.Events) != 1 {
		t.Fatalf("unexpected ready status or event: status=%+v events=%d", actual.Status, len(recorder.Events))
	}
	if _, err := r.Reconcile(t.Context(), req); err != nil {
		t.Fatal(err)
	}
	if statusWrites != 2 || len(recorder.Events) != 1 {
		t.Fatalf("unchanged reconcile wrote status or spammed Events: status=%d events=%d", statusWrites, len(recorder.Events))
	}
}

func TestPatchStatusRetriesOneSameGenerationConflict(t *testing.T) {
	p, scheme, _ := setup(t)
	base := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(p).WithObjects(p).Build()
	patches := 0
	wrapped := interceptor.NewClient(base, interceptor.Funcs{
		SubResourcePatch: func(ctx context.Context, c client.Client, sub string, o client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
			patches++
			if patches == 1 {
				concurrent := &platform.AIWorkload{}
				if err := c.Get(ctx, client.ObjectKeyFromObject(o), concurrent); err != nil {
					return err
				}
				concurrent.Status.Conditions = []metav1.Condition{{Type: "ExternalObservation", Status: metav1.ConditionTrue, Reason: "Fixture", Message: "must survive controller retry", ObservedGeneration: concurrent.Generation}}
				if err := c.Status().Update(ctx, concurrent); err != nil {
					return err
				}
				return apierrors.NewConflict(schema.GroupResource{Group: "platform.example.io", Resource: "aiworkloads"}, o.GetName(), errors.New("synthetic conflict"))
			}
			return c.SubResource(sub).Patch(ctx, o, patch, opts...)
		},
	})
	current := &platform.AIWorkload{}
	if err := base.Get(t.Context(), client.ObjectKeyFromObject(p), current); err != nil {
		t.Fatal(err)
	}
	before := current.DeepCopy()
	current.Status.DesiredReplicas = 1
	changed, err := (&AIWorkloadReconciler{Client: wrapped}).patchStatus(t.Context(), before, current)
	if err != nil || !changed || patches != 2 {
		t.Fatalf("status conflict retry changed=%t patches=%d err=%v", changed, patches, err)
	}
	if err := base.Get(t.Context(), client.ObjectKeyFromObject(p), current); err != nil || current.Status.DesiredReplicas != 1 || meta.FindStatusCondition(current.Status.Conditions, "ExternalObservation") == nil {
		t.Fatalf("retried status was not stored: status=%+v err=%v", current.Status, err)
	}
}
