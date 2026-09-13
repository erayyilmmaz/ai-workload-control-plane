// SPDX-License-Identifier: Apache-2.0
package controller

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platformv1alpha1 "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
)

// ErrSecretNotFound is deliberately name- and payload-free for safe status/events.
var ErrSecretNotFound = errors.New("one or more referenced Secrets are unavailable")

// validateSecretReferences reads only partial Secret metadata. RBAC still authorizes
// complete Secret reads at the API boundary; this controller does not request or use data.
func (r *AIWorkloadReconciler) validateSecretReferences(ctx context.Context, workload *platformv1alpha1.AIWorkload) error {
	for _, name := range workload.Spec.SecretRefs {
		secret := &metav1.PartialObjectMetadata{}
		secret.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))
		err := r.Get(ctx, client.ObjectKey{Namespace: workload.Namespace, Name: string(name)}, secret)
		if apierrors.IsNotFound(err) {
			return ErrSecretNotFound
		}
		if err != nil {
			return fmt.Errorf("get referenced Secret metadata: %w", err)
		}
	}
	return nil
}
