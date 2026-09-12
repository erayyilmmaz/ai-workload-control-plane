// SPDX-License-Identifier: Apache-2.0
package controller

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/resource"
	"github.com/erayyilmmaz/ai-workload-control-plane/test/fixtures"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func setup(t *testing.T) (*platform.AIWorkload, *runtime.Scheme, []resource.Intent) {
	t.Helper()
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := platform.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	p := &platform.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: "sample", Namespace: "workloads", UID: "parent-uid", Generation: 1}, Spec: platform.AIWorkloadSpec{Image: "example.invalid/app:v1", Replicas: ptr.To(int32(1)), Container: platform.ContainerSpec{Port: 8080}}}
	intents, err := fixtures.Plan(p)
	if err != nil {
		t.Fatal(err)
	}
	intents, err = ValidatePlan(p, intents)
	if err != nil {
		t.Fatal(err)
	}
	return p, s, intents
}

func TestEngineIdempotenceDriftAndDeletion(t *testing.T) {
	p, s, plan := setup(t)
	for index, intent := range plan {
		t.Run(string(rune('0'+index)), func(t *testing.T) {
			base := fake.NewClientBuilder().WithScheme(s).Build()
			writes := 0
			c := interceptor.NewClient(base, interceptor.Funcs{
				Create: func(ctx context.Context, c client.WithWatch, o client.Object, opts ...client.CreateOption) error {
					writes++
					return c.Create(ctx, o, opts...)
				},
				Patch: func(ctx context.Context, c client.WithWatch, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
					writes++
					data, err := patch.Data(o)
					if err != nil {
						return err
					}
					if !strings.Contains(string(data), "resourceVersion") {
						t.Error("patch must include optimistic resourceVersion")
					}
					return c.Patch(ctx, o, patch, opts...)
				},
			})
			e := Engine{Client: c, Scheme: s}
			apply := func(want Outcome) {
				t.Helper()
				out, err := e.Apply(t.Context(), p, intent)
				if err != nil || out != want {
					t.Fatalf("outcome=%s error=%v want=%s", out, err, want)
				}
			}
			apply(Created)
			apply(Unchanged)
			apply(Unchanged)
			if writes != 1 {
				t.Fatalf("no-op wrote API: %d", writes)
			}
			obj := intent.Object.DeepCopyObject().(client.Object)
			key := client.ObjectKeyFromObject(obj)
			if err := base.Get(t.Context(), key, obj); err != nil {
				t.Fatal(err)
			}
			owner := metav1.GetControllerOf(obj)
			if !owns(p, obj) || owner.BlockOwnerDeletion == nil || *owner.BlockOwnerDeletion {
				t.Fatal("owner must use parent UID and blockOwnerDeletion=false")
			}
			labels := obj.GetLabels()
			labels["test.awcp/managed"] = "drift"
			labels["user.example/keep"] = "yes"
			obj.SetLabels(labels)
			if err := base.Update(t.Context(), obj); err != nil {
				t.Fatal(err)
			}
			apply(Patched)
			apply(Unchanged)
			if err := base.Get(t.Context(), key, obj); err != nil {
				t.Fatal(err)
			}
			if obj.GetLabels()["user.example/keep"] != "yes" {
				t.Fatal("unmanaged metadata lost")
			}
			if err := base.Delete(t.Context(), obj); err != nil {
				t.Fatal(err)
			}
			apply(Created)
			apply(Unchanged)
			if writes != 3 {
				t.Fatalf("expected create, patch, recreate only; got %d", writes)
			}
		})
	}
}

func TestOwnershipNeverAdoptsOrDeletesForeignResources(t *testing.T) {
	p, s, plan := setup(t)
	for _, intent := range plan {
		for _, variant := range []string{"none", "stale-uid", "other-name", "other-kind", "other-group", "not-controller"} {
			t.Run(strings.TrimPrefix(string(intent.Object.GetName()), "awcp-")+"/"+variant+"/"+string(rune('0'+childOrder(intent.Object))), func(t *testing.T) {
				obj := intent.Object.DeepCopyObject().(client.Object)
				obj.SetUID("foreign-uid")
				owner := metav1.OwnerReference{APIVersion: platform.GroupVersion.String(), Kind: "AIWorkload", Name: p.Name, UID: p.UID, Controller: ptr.To(true)}
				switch variant {
				case "stale-uid":
					owner.UID = "old-parent"
				case "other-name":
					owner.Name = "other"
				case "other-kind":
					owner.Kind = "Other"
				case "other-group":
					owner.APIVersion = "other.io/v1"
				case "not-controller":
					owner.Controller = ptr.To(false)
				}
				if variant != "none" {
					obj.SetOwnerReferences([]metav1.OwnerReference{owner})
				}
				c := fake.NewClientBuilder().WithScheme(s).WithObjects(obj).Build()
				before := obj.DeepCopyObject().(client.Object)
				if err := c.Get(t.Context(), client.ObjectKeyFromObject(obj), before); err != nil {
					t.Fatal(err)
				}
				e := Engine{Client: c, Scheme: s}
				_, err := e.Apply(t.Context(), p, intent)
				if !errors.Is(err, ErrOwnershipConflict) {
					t.Fatalf("must reject foreign owner: %v", err)
				}
				if childOrder(obj) >= 2 {
					intent.Absent = true
					_, err = e.Apply(t.Context(), p, intent)
					if !errors.Is(err, ErrOwnershipConflict) {
						t.Fatalf("foreign delete accepted: %v", err)
					}
				}
				if err := c.Get(t.Context(), client.ObjectKeyFromObject(obj), obj); err != nil {
					t.Fatal(err)
				}
				if obj.GetResourceVersion() != before.GetResourceVersion() {
					t.Fatal("foreign resource modified")
				}
			})
		}
	}
}

func TestPlanBoundaryAndDeletePreconditions(t *testing.T) {
	p, s, plan := setup(t)
	for i, intent := range plan {
		if childOrder(intent.Object) != i {
			t.Fatal("dependency order drift")
		}
	}
	for _, o := range []client.Object{nil, (*corev1.Service)(nil), &corev1.Secret{}, &corev1.ConfigMap{}, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: p.Namespace}}, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: resource.ChildName(p.Name), Namespace: "outside"}}} {
		if _, err := ValidatePlan(p, []resource.Intent{{Object: o, Mutate: func(client.Object) error { return nil }}}); !errors.Is(err, ErrInvalidPlan) {
			t.Fatalf("unsafe plan accepted: %T", o)
		}
	}
	if _, err := ValidatePlan(p, []resource.Intent{plan[0], plan[0]}); !errors.Is(err, ErrInvalidPlan) {
		t.Fatal("duplicate kind accepted")
	}
	bad := plan[0]
	bad.Absent = true
	if _, err := ValidatePlan(p, []resource.Intent{bad}); !errors.Is(err, ErrInvalidPlan) {
		t.Fatal("SA deletion accepted")
	}
	for _, kind := range []int{2, 3} {
		c := fake.NewClientBuilder().WithScheme(s).Build()
		e := Engine{Client: c, Scheme: s}
		intent := plan[kind]
		if _, err := e.Apply(t.Context(), p, intent); err != nil {
			t.Fatal(err)
		}
		o := intent.Object.DeepCopyObject().(client.Object)
		if err := c.Get(t.Context(), client.ObjectKeyFromObject(o), o); err != nil {
			t.Fatal(err)
		}
		o.SetUID("child-uid")
		if err := c.Update(t.Context(), o); err != nil {
			t.Fatal(err)
		}
		called := false
		e.Client = interceptor.NewClient(c, interceptor.Funcs{Delete: func(ctx context.Context, c client.WithWatch, o client.Object, opts ...client.DeleteOption) error {
			called = true
			d := (&client.DeleteOptions{}).ApplyOptions(opts)
			if d.Preconditions == nil || ptr.Deref(d.Preconditions.UID, "") != "child-uid" || ptr.Deref(d.Preconditions.ResourceVersion, "") == "" {
				t.Fatal("delete lacks exact UID+RV preconditions")
			}
			return c.Delete(ctx, o, opts...)
		}})
		intent.Absent = true
		out, err := e.Apply(t.Context(), p, intent)
		if err != nil || out != Deleted || !called {
			t.Fatalf("optional cleanup failed: %s %v", out, err)
		}
		out, err = e.Apply(t.Context(), p, intent)
		if err != nil || out != Unchanged {
			t.Fatal("absent must be no-op")
		}
	}
}

func TestFailureClassificationConditionsAndRecovery(t *testing.T) {
	gr := schema.GroupResource{Group: "apps", Resource: "deployments"}
	private := errors.New("DO-NOT-LOG-SECRET")
	cases := []struct {
		name      string
		err       error
		permanent bool
	}{
		{"Conflict", apierrors.NewConflict(gr, "child", private), false}, {"AlreadyExists", apierrors.NewAlreadyExists(gr, "child"), false},
		{"Forbidden", apierrors.NewForbidden(gr, "child", private), false}, {"Timeout", apierrors.NewTimeoutError(private.Error(), 1), false},
		{"Throttled", apierrors.NewTooManyRequests(private.Error(), 1), false}, {"UnexpectedError", private, false},
		{"ResourceOwnershipConflict", ErrOwnershipConflict, true}, {"InvalidConfiguration", ErrInvalidPlan, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, s, _ := setup(t)
			base := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(p).WithObjects(p).Build()
			failure := tc.err
			statusWrites := 0
			recorder := events.NewFakeRecorder(10)
			wrapped := interceptor.NewClient(base, interceptor.Funcs{
				Create: func(ctx context.Context, c client.WithWatch, o client.Object, opts ...client.CreateOption) error {
					if failure != nil {
						return failure
					}
					return c.Create(ctx, o, opts...)
				},
				SubResourcePatch: func(ctx context.Context, c client.Client, sub string, o client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
					statusWrites++
					return c.SubResource(sub).Patch(ctx, o, patch, opts...)
				},
			})
			r := AIWorkloadReconciler{Client: wrapped, WatchNamespace: p.Namespace, Scheme: s, Builder: resource.BuilderFunc(fixtures.Plan), Recorder: recorder}
			req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(p)}
			for attempt := 0; attempt < 2; attempt++ {
				result, err := r.Reconcile(t.Context(), req)
				if tc.permanent {
					if err != nil || result.RequeueAfter != time.Minute {
						t.Fatalf("permanent error must use bounded retry: %+v %v", result, err)
					}
				} else {
					if !errors.Is(err, tc.err) || !strings.Contains(err.Error(), tc.name) || strings.Contains(err.Error(), private.Error()) || result.RequeueAfter != 0 {
						t.Fatalf("retry category or redaction wrong: %+v %v", result, err)
					}
				}
			}
			if statusWrites != 1 || len(recorder.Events) != 1 {
				t.Fatalf("failure hot-loop: status=%d events=%d", statusWrites, len(recorder.Events))
			}
			if err := base.Get(t.Context(), req.NamespacedName, p); err != nil {
				t.Fatal(err)
			}
			cond := meta.FindStatusCondition(p.Status.Conditions, "Degraded")
			if cond == nil || cond.Status != metav1.ConditionTrue || cond.ObservedGeneration != p.Generation || strings.Contains(cond.Message, private.Error()) {
				t.Fatalf("failure not safely visible: %+v", cond)
			}
			// A healthy parent progresses even while this parent's builder fails.
			other := p.DeepCopy()
			other.Name = "healthy"
			other.UID = "healthy-uid"
			other.ResourceVersion = ""
			other.Status = platform.AIWorkloadStatus{}
			if err := base.Create(t.Context(), other); err != nil {
				t.Fatal(err)
			}
			failure = nil
			if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(other)}); err != nil {
				t.Fatal(err)
			}
			if _, err := r.Reconcile(t.Context(), req); err != nil {
				t.Fatal(err)
			}
			if err := base.Get(t.Context(), req.NamespacedName, p); err != nil {
				t.Fatal(err)
			}
			if meta.IsStatusConditionTrue(p.Status.Conditions, "Ready") || !meta.IsStatusConditionFalse(p.Status.Conditions, "Degraded") {
				t.Fatal("recovery must clear failure without claiming readiness")
			}
		})
	}
}

func TestDeletionGuardAndPrimaryPredicate(t *testing.T) {
	p, s, _ := setup(t)
	p.Finalizers = []string{"test.awcp/hold"}
	now := metav1.Now()
	p.DeletionTimestamp = &now
	c := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(p).WithObjects(p).Build()
	r := AIWorkloadReconciler{Client: c, WatchNamespace: p.Namespace, Scheme: s, Builder: resource.BuilderFunc(func(*platform.AIWorkload) ([]resource.Intent, error) {
		t.Fatal("deleting parent must not build")
		return nil, nil
	})}
	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(p)}); err != nil {
		t.Fatal(err)
	}
	old := p.DeepCopy()
	old.DeletionTimestamp = nil
	pred := primaryPredicate()
	if !pred.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: p}) {
		t.Fatal("deletion event lost")
	}
	newer := old.DeepCopy()
	newer.Status.ReadyReplicas = 1
	newer.ResourceVersion = "new"
	if pred.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: newer}) {
		t.Fatal("status-only parent update creates loop")
	}
	newer.Generation++
	if !pred.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: newer}) {
		t.Fatal("spec update lost")
	}
}

func TestSecretIndexMapping(t *testing.T) {
	p, s, _ := setup(t)
	p.Spec.SecretRefs = []platform.SecretReference{"shared", "shared", ""}
	other := p.DeepCopy()
	other.Name = "other"
	other.Spec.SecretRefs = []platform.SecretReference{"elsewhere"}
	outside := p.DeepCopy()
	outside.Namespace = "outside"
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(p, other, outside).WithIndex(&platform.AIWorkload{}, SecretReferenceIndex, secretIndex).Build()
	r := AIWorkloadReconciler{Client: c, WatchNamespace: p.Namespace}
	secret := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: "shared", Namespace: p.Namespace}}
	requests := r.RequestsForSecret(t.Context(), secret)
	if len(requests) != 1 || requests[0].Name != p.Name {
		t.Fatalf("wrong reference mapping: %v", requests)
	}
	secret.Namespace = "outside"
	if len(r.RequestsForSecret(t.Context(), secret)) != 0 {
		t.Fatal("cross-namespace Secret mapping")
	}
}

func TestMutatorCannotChangeIdentity(t *testing.T) {
	p, s, plan := setup(t)
	c := fake.NewClientBuilder().WithScheme(s).Build()
	e := Engine{Client: c, Scheme: s}
	bad := plan[1]
	bad.Mutate = func(o client.Object) error { o.SetNamespace("outside"); return nil }
	if _, err := e.Apply(t.Context(), p, bad); !errors.Is(err, ErrInvalidPlan) {
		t.Fatalf("unsafe fresh mutation accepted: %v", err)
	}
	if _, err := e.Apply(t.Context(), p, plan[1]); err != nil {
		t.Fatal(err)
	}
	bad.Mutate = func(o client.Object) error { o.SetOwnerReferences(nil); return nil }
	if _, err := e.Apply(t.Context(), p, bad); !errors.Is(err, ErrInvalidPlan) {
		t.Fatalf("owner mutation accepted: %v", err)
	}
	var children appsv1.DeploymentList
	if err := c.List(t.Context(), &children); err != nil || len(children.Items) != 1 {
		t.Fatalf("unexpected children: %v %v", children.Items, err)
	}
}
