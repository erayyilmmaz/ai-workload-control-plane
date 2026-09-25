// SPDX-License-Identifier: Apache-2.0
package resource

import (
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
)

// AvailabilityEnabled keeps PDB creation opt-in so existing V0 workloads have
// no new eviction behavior until their owner explicitly declares a budget.
func AvailabilityEnabled(p *platform.AIWorkload) bool {
	return p != nil && p.Spec.Availability != nil && ptr.Deref(p.Spec.Availability.Enabled, false)
}

func availabilityIntent(p *platform.AIWorkload) (Intent, error) {
	pdb := &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: ChildName(p.Name), Namespace: p.Namespace}}
	if !AvailabilityEnabled(p) {
		return Intent{Object: pdb, Absent: true}, nil
	}
	if err := validateAvailability(p); err != nil {
		return Intent{}, err
	}
	desired := p.DeepCopy()
	return Intent{Object: pdb, Mutate: func(object client.Object) error {
		target, ok := object.(*policyv1.PodDisruptionBudget)
		if !ok {
			return ErrInvalidConfiguration
		}
		return mutatePDB(target, desired)
	}}, nil
}

// AvailabilityReplicaFloor is the lowest replica count that the declared
// workload contract can maintain. HPA uses its lower bound; static workloads
// use spec.replicas (default one). A PDB above that count would permanently
// block voluntary eviction without improving availability, so it is rejected.
func AvailabilityReplicaFloor(p *platform.AIWorkload) int32 {
	if AutoscalingEnabled(p) {
		return ptr.Deref(p.Spec.Autoscaling.MinReplicas, int32(1))
	}
	return ptr.Deref(p.Spec.Replicas, int32(1))
}

func validateAvailability(p *platform.AIWorkload) error {
	if p == nil || p.Spec.Availability == nil {
		return ErrInvalidConfiguration
	}
	spec := p.Spec.Availability
	if (spec.MinAvailable == nil) == (spec.MaxUnavailable == nil) {
		return ErrInvalidConfiguration
	}
	floor := AvailabilityReplicaFloor(p)
	if floor < 1 || (spec.MinAvailable != nil && (*spec.MinAvailable < 1 || *spec.MinAvailable > floor)) ||
		(spec.MaxUnavailable != nil && (*spec.MaxUnavailable < 0 || *spec.MaxUnavailable >= floor)) {
		return ErrInvalidConfiguration
	}
	return nil
}

func mutatePDB(pdb *policyv1.PodDisruptionBudget, p *platform.AIWorkload) error {
	if err := validateAvailability(p); err != nil {
		return err
	}
	managedMetadata(&pdb.ObjectMeta, p)
	pdb.Spec.Selector = &metav1.LabelSelector{MatchLabels: SelectorLabels(p)}
	pdb.Spec.MinAvailable = nil
	pdb.Spec.MaxUnavailable = nil
	if p.Spec.Availability.MinAvailable != nil {
		value := intstr.FromInt32(*p.Spec.Availability.MinAvailable)
		pdb.Spec.MinAvailable = &value
	} else {
		value := intstr.FromInt32(*p.Spec.Availability.MaxUnavailable)
		pdb.Spec.MaxUnavailable = &value
	}
	return nil
}
