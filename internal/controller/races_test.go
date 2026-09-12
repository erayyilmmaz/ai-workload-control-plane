// SPDX-License-Identifier: Apache-2.0
package controller

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestPatchConflictPreservesConcurrentChangesThenRecovers(t *testing.T) {
	p, s, plan := setup(t)
	base := fake.NewClientBuilder().WithScheme(s).Build()
	engine := Engine{Client: base, Scheme: s}
	intent := plan[0]
	if _, err := engine.Apply(t.Context(), p, intent); err != nil {
		t.Fatal(err)
	}
	o := intent.Object.DeepCopyObject().(client.Object)
	key := client.ObjectKeyFromObject(o)
	if err := base.Get(t.Context(), key, o); err != nil {
		t.Fatal(err)
	}
	o.SetLabels(map[string]string{"test.awcp/managed": "drift"})
	if err := base.Update(t.Context(), o); err != nil {
		t.Fatal(err)
	}
	engine.Client = interceptor.NewClient(base, interceptor.Funcs{Patch: func(ctx context.Context, c client.WithWatch, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
		latest := o.DeepCopyObject().(client.Object)
		if err := c.Get(ctx, key, latest); err != nil {
			return err
		}
		latest.SetAnnotations(map[string]string{"concurrent.example/keep": "yes"})
		if err := c.Update(ctx, latest); err != nil {
			return err
		}
		return c.Patch(ctx, o, patch, opts...)
	}})
	if _, err := engine.Apply(t.Context(), p, intent); !apierrors.IsConflict(err) {
		t.Fatalf("stale patch must fail: %v", err)
	}
	engine.Client = base
	if out, err := engine.Apply(t.Context(), p, intent); err != nil || out != Patched {
		t.Fatalf("retry failed: %s %v", out, err)
	}
	if err := base.Get(t.Context(), key, o); err != nil {
		t.Fatal(err)
	}
	if o.GetAnnotations()["concurrent.example/keep"] != "yes" {
		t.Fatal("concurrent annotation overwritten")
	}
}

func TestCreateRaceDoesNotAdoptWinner(t *testing.T) {
	p, s, plan := setup(t)
	base := fake.NewClientBuilder().WithScheme(s).Build()
	intent := plan[0]
	engine := Engine{Scheme: s, Client: interceptor.NewClient(base, interceptor.Funcs{Create: func(ctx context.Context, c client.WithWatch, o client.Object, opts ...client.CreateOption) error {
		foreign := intent.Object.DeepCopyObject().(client.Object)
		foreign.SetUID("race-winner")
		if err := c.Create(ctx, foreign); err != nil {
			return err
		}
		return c.Create(ctx, o, opts...)
	}})}
	if _, err := engine.Apply(t.Context(), p, intent); !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create race must retry: %v", err)
	}
	engine.Client = base
	if _, err := engine.Apply(t.Context(), p, intent); !errors.Is(err, ErrOwnershipConflict) {
		t.Fatalf("create race winner adopted: %v", err)
	}
}

func TestOptionalDeleteRaceCannotRemoveReplacement(t *testing.T) {
	p, s, plan := setup(t)
	base := fake.NewClientBuilder().WithScheme(s).Build()
	intent := plan[2]
	engine := Engine{Client: base, Scheme: s}
	if _, err := engine.Apply(t.Context(), p, intent); err != nil {
		t.Fatal(err)
	}
	engine.Client = interceptor.NewClient(base, interceptor.Funcs{Delete: func(ctx context.Context, c client.WithWatch, o client.Object, opts ...client.DeleteOption) error {
		options := (&client.DeleteOptions{}).ApplyOptions(opts)
		if options.Preconditions == nil || options.Preconditions.UID == nil || options.Preconditions.ResourceVersion == nil {
			t.Fatal("delete not guarded")
		}
		if err := c.Delete(ctx, o); err != nil {
			return err
		}
		replacement := intent.Object.DeepCopyObject().(client.Object)
		replacement.SetUID("replacement")
		if err := c.Create(ctx, replacement); err != nil {
			return err
		}
		// Fake client does not implement server UID delete admission; emulate the API rejection.
		return apierrors.NewConflict(schema.GroupResource{Resource: "services"}, o.GetName(), errors.New("UID precondition failed"))
	}})
	intent.Absent = true
	if _, err := engine.Apply(t.Context(), p, intent); !apierrors.IsConflict(err) {
		t.Fatalf("delete race should retry: %v", err)
	}
	engine.Client = base
	if _, err := engine.Apply(t.Context(), p, intent); !errors.Is(err, ErrOwnershipConflict) {
		t.Fatalf("replacement not protected: %v", err)
	}
	var replacement corev1.Service
	if err := base.Get(t.Context(), client.ObjectKeyFromObject(intent.Object), &replacement); err != nil || replacement.UID != "replacement" {
		t.Fatalf("replacement removed: %v", err)
	}
}

func TestDeletingChildWaitsWithoutRecreation(t *testing.T) {
	p, s, plan := setup(t)
	base := fake.NewClientBuilder().WithScheme(s).Build()
	intent := plan[0]
	engine := Engine{Client: base, Scheme: s}
	if _, err := engine.Apply(t.Context(), p, intent); err != nil {
		t.Fatal(err)
	}
	o := intent.Object.DeepCopyObject().(client.Object)
	if err := base.Get(t.Context(), client.ObjectKeyFromObject(o), o); err != nil {
		t.Fatal(err)
	}
	o.SetFinalizers([]string{"test.example/hold"})
	if err := base.Update(t.Context(), o); err != nil {
		t.Fatal(err)
	}
	if err := base.Delete(t.Context(), o); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Apply(t.Context(), p, intent); !errors.Is(err, ErrChildDeleting) {
		t.Fatalf("must wait for terminating child: %v", err)
	}
	now := metav1.Now()
	p.DeletionTimestamp = &now
	if out, err := engine.Apply(t.Context(), p, intent); err != nil || out != Unchanged {
		t.Fatalf("deleting parent should stop: %s %v", out, err)
	}
}
