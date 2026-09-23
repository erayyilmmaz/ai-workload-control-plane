// SPDX-License-Identifier: Apache-2.0
package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	platformv1alpha1 "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
)

const SecretReferenceIndex = "spec.secretRefs"

func primaryPredicate() predicate.Predicate {
	return predicate.Funcs{UpdateFunc: func(e event.UpdateEvent) bool {
		return e.ObjectOld.GetUID() != e.ObjectNew.GetUID() || e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() || !e.ObjectOld.GetDeletionTimestamp().Equal(e.ObjectNew.GetDeletionTimestamp())
	}}
}

func secretIndex(o client.Object) []string {
	p, ok := o.(*platformv1alpha1.AIWorkload)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(p.Spec.SecretRefs)+len(p.Spec.ExternalSecrets))
	seen := map[string]bool{}
	for _, name := range p.Spec.SecretRefs {
		if name != "" && !seen[string(name)] {
			keys = append(keys, string(name))
			seen[string(name)] = true
		}
	}
	for _, reference := range p.Spec.ExternalSecrets {
		if reference.TargetSecret != "" && !seen[string(reference.TargetSecret)] {
			keys = append(keys, string(reference.TargetSecret))
			seen[string(reference.TargetSecret)] = true
		}
	}
	return keys
}

// RequestsForSecret is payload-free and namespace-scoped, including delete events.
func (r *AIWorkloadReconciler) RequestsForSecret(ctx context.Context, o client.Object) []ctrl.Request {
	if !r.watchesNamespace(o.GetNamespace()) {
		return nil
	}
	var parents platformv1alpha1.AIWorkloadList
	if err := r.List(ctx, &parents, client.InNamespace(o.GetNamespace()), client.MatchingFields{SecretReferenceIndex: o.GetName()}); err != nil {
		log.FromContext(ctx).Error(retryError(err), "Could not map Secret event")
		return nil
	}
	requests := make([]ctrl.Request, 0, len(parents.Items))
	for _, p := range parents.Items {
		requests = append(requests, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: p.Namespace, Name: p.Name}})
	}
	return requests
}
