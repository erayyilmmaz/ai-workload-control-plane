// SPDX-License-Identifier: Apache-2.0
package controller

import (
	"context"
	"errors"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platformv1alpha1 "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
)

// AIWorkloadReconciler is a read-only bootstrap, not a workload implementation.
type AIWorkloadReconciler struct {
	client.Client
	WatchNamespace string
}

// +kubebuilder:rbac:groups=platform.example.io,namespace=awcp-workloads,resources=aiworkloads,verbs=get;list;watch

// Reconcile verifies primary access without creating resources or writing status.
func (r *AIWorkloadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if r.WatchNamespace == "" {
		return ctrl.Result{}, errors.New("watch namespace is required")
	}
	if req.Namespace != r.WatchNamespace {
		return ctrl.Result{}, nil
	}
	var workload platformv1alpha1.AIWorkload
	if err := r.Get(ctx, req.NamespacedName, &workload); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// AWCP-4 defines the spec; AWCP-5 adds child reconciliation and ownership.
	return ctrl.Result{}, nil
}

// SetupWithManager registers the primary watch with the namespace-scoped cache.
func (r *AIWorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.AIWorkload{}).
		Named("aiworkload").
		Complete(r)
}
