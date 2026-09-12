// SPDX-License-Identifier: Apache-2.0
package controller

import (
	"context"
	"errors"
	"reflect"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	platformv1alpha1 "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
)

func TestBootstrapReconcile(t *testing.T) {
	forbidden := apierrors.NewForbidden(schema.GroupResource{Group: "platform.example.io", Resource: "aiworkloads"}, "sample", errors.New("denied"))
	transient := errors.New("temporary API failure")
	for _, tc := range []struct {
		name, namespace, watch string
		exists, deleting       bool
		readError              error
		wantReads              int
		wantError              bool
	}{
		{name: "existing is read only", namespace: "workloads", watch: "workloads", exists: true, wantReads: 1},
		{name: "not found is success", namespace: "workloads", watch: "workloads", wantReads: 1},
		{name: "deleting is read only", namespace: "workloads", watch: "workloads", exists: true, deleting: true, wantReads: 1},
		{name: "outside scope is ignored", namespace: "other", watch: "workloads"},
		{name: "empty scope fails closed", namespace: "workloads", wantError: true},
		{name: "forbidden is not missing", namespace: "workloads", watch: "workloads", readError: forbidden, wantReads: 1, wantError: true},
		{name: "transient is retried by runtime", namespace: "workloads", watch: "workloads", readError: transient, wantReads: 1, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			if err := platformv1alpha1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			object := &platformv1alpha1.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: "sample", Namespace: tc.namespace}}
			if tc.deleting {
				now := metav1.Now()
				object.DeletionTimestamp = &now
				object.Finalizers = []string{"test.example.io/hold"}
			}
			reads, writes := 0, 0
			builder := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(object)
			if tc.exists {
				builder = builder.WithObjects(object)
			}
			base := builder.Build()
			before := object.DeepCopy()
			if tc.exists {
				if err := base.Get(t.Context(), client.ObjectKeyFromObject(object), before); err != nil {
					t.Fatal(err)
				}
			}
			mutation := func() error { writes++; return errors.New("bootstrap must not mutate") }
			wrapped := interceptor.NewClient(base, interceptor.Funcs{
				Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					reads++
					if tc.readError != nil {
						return tc.readError
					}
					return c.Get(ctx, key, obj, opts...)
				},
				Create: func(context.Context, client.WithWatch, client.Object, ...client.CreateOption) error {
					return mutation()
				},
				Update: func(context.Context, client.WithWatch, client.Object, ...client.UpdateOption) error {
					return mutation()
				},
				Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
					return mutation()
				},
				Delete: func(context.Context, client.WithWatch, client.Object, ...client.DeleteOption) error {
					return mutation()
				},
				DeleteAllOf: func(context.Context, client.WithWatch, client.Object, ...client.DeleteAllOfOption) error {
					return mutation()
				},
				SubResourceUpdate: func(context.Context, client.Client, string, client.Object, ...client.SubResourceUpdateOption) error {
					return mutation()
				},
				SubResourcePatch: func(context.Context, client.Client, string, client.Object, client.Patch, ...client.SubResourcePatchOption) error {
					return mutation()
				},
			})
			r := AIWorkloadReconciler{Client: wrapped, WatchNamespace: tc.watch}
			result, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "sample", Namespace: tc.namespace}})
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v, wantError = %v", err, tc.wantError)
			}
			if tc.readError != nil && !errors.Is(err, tc.readError) {
				t.Fatalf("API error was lost: %v", err)
			}
			if result != (ctrl.Result{}) || reads != tc.wantReads || writes != 0 {
				t.Fatalf("result=%+v reads=%d writes=%d", result, reads, writes)
			}
			if tc.exists {
				var after platformv1alpha1.AIWorkload
				if err := base.Get(t.Context(), client.ObjectKeyFromObject(object), &after); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(before, &after) {
					t.Fatal("bootstrap changed primary")
				}
			}
		})
	}
}
