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

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
)

func TestSecretValidationUsesMetadataAndSafeMissingError(t *testing.T) {
	p, scheme, _ := setup(t)
	p.Spec.SecretRefs = []platform.SecretReference{"present", "missing"}
	sentinel := "AWCP-8-SENTINEL-MUST-NOT-LEAK"
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "present", Namespace: p.Namespace}, Data: map[string][]byte{"token": []byte(sentinel)}}
	base := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	metadataReads := 0
	wrapped := interceptor.NewClient(base, interceptor.Funcs{Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, object client.Object, opts ...client.GetOption) error {
		if _, ok := object.(*metav1.PartialObjectMetadata); !ok {
			t.Fatalf("Secret validation requested typed object %T", object)
		}
		metadataReads++
		return c.Get(ctx, key, object, opts...)
	}})
	r := AIWorkloadReconciler{Client: wrapped}
	err := r.validateSecretReferences(t.Context(), p)
	if !errors.Is(err, ErrSecretNotFound) || err.Error() != ErrSecretNotFound.Error() || metadataReads != 2 {
		t.Fatalf("missing Secret result must be generic and metadata-only: %v, reads=%d", err, metadataReads)
	}
	if errors.Is(err, errors.New(sentinel)) {
		t.Fatal("sentinel unexpectedly became part of error")
	}
}

func TestSecretValidationPreservesAPIErrors(t *testing.T) {
	p, scheme, _ := setup(t)
	p.Spec.SecretRefs = []platform.SecretReference{"present"}
	denied := apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, "present", errors.New("denied"))
	base := fake.NewClientBuilder().WithScheme(scheme).Build()
	wrapped := interceptor.NewClient(base, interceptor.Funcs{Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
		return denied
	}})
	err := (&AIWorkloadReconciler{Client: wrapped}).validateSecretReferences(context.Background(), p)
	if errors.Is(err, ErrSecretNotFound) || !errors.Is(err, denied) {
		t.Fatalf("API error was mislabeled as missing Secret: %v", err)
	}
}
