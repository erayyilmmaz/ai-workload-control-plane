// SPDX-License-Identifier: Apache-2.0
package controller

import (
	"context"

	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/resource"
)

const conditionAvailabilityReady = "AvailabilityReady"

// observeAvailability reports only the PDB contract that AWCP owns. A PDB can
// constrain voluntary eviction but cannot make one replica highly available, so
// the single-replica no-disruption case remains a visible warning condition.
func (r *AIWorkloadReconciler) observeAvailability(ctx context.Context, workload *platform.AIWorkload) error {
	if !resource.AvailabilityEnabled(workload) {
		meta.RemoveStatusCondition(&workload.Status.Conditions, conditionAvailabilityReady)
		return nil
	}
	pdb := &policyv1.PodDisruptionBudget{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: workload.Namespace, Name: resource.ChildName(workload.Name)}, pdb); err != nil {
		if apierrors.IsNotFound(err) {
			setAvailabilityCondition(workload, metav1.ConditionFalse, "PDBReconciling", "The owned PodDisruptionBudget is awaiting API observation.")
			return nil
		}
		return err
	}
	if resource.AvailabilityReplicaFloor(workload) == 1 {
		setAvailabilityCondition(workload, metav1.ConditionFalse, "SingleReplicaDisruptionBlocked", "A single-replica workload cannot remain available during voluntary disruption; its PodDisruptionBudget blocks eviction until capacity is increased.")
		return nil
	}
	setAvailabilityCondition(workload, metav1.ConditionTrue, "PDBActive", "The owned PodDisruptionBudget is active for voluntary disruption protection.")
	return nil
}

func setAvailabilityCondition(workload *platform.AIWorkload, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&workload.Status.Conditions, metav1.Condition{Type: conditionAvailabilityReady, Status: status, Reason: reason, Message: message, ObservedGeneration: workload.Generation})
}
