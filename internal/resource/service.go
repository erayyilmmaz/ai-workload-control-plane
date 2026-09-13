// SPDX-License-Identifier: Apache-2.0
package resource

import (
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
)

var ErrImmutableServiceAllocation = errors.New("owned Service has an incompatible immutable cluster allocation")

// ServiceEnabled preserves an explicit false while treating omitted/defaulted values as true.
func ServiceEnabled(p *platform.AIWorkload) bool {
	return p != nil && (p.Spec.Service == nil || p.Spec.Service.Enabled == nil || *p.Spec.Service.Enabled)
}

// ServicePort returns the validated API default when the parent has not yet round-tripped.
func ServicePort(p *platform.AIWorkload) int32 {
	if p != nil && p.Spec.Service != nil && p.Spec.Service.Port != nil {
		return *p.Spec.Service.Port
	}
	return 80
}

// ServiceEndpoint is discovery information only; callers publish it after a successful apply.
func ServiceEndpoint(p *platform.AIWorkload) string {
	if !ServiceEnabled(p) {
		return ""
	}
	return fmt.Sprintf("%s.%s.svc:%d", ChildName(p.Name), p.Namespace, ServicePort(p))
}

func serviceIntent(p *platform.AIWorkload) (Intent, error) {
	intent := Intent{Object: &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: ChildName(p.Name), Namespace: p.Namespace}}, Absent: !ServiceEnabled(p)}
	if intent.Absent {
		return intent, nil
	}
	if port := ServicePort(p); port < 1 || port > 65535 {
		return Intent{}, ErrInvalidConfiguration
	}
	intent.Mutate = func(o client.Object) error {
		s, ok := o.(*corev1.Service)
		if !ok {
			return ErrInvalidConfiguration
		}
		return mutateService(s, p)
	}
	return intent, nil
}

func mutateService(s *corev1.Service, p *platform.AIWorkload) error {
	// clusterIP=None makes the Service headless and cannot be changed to normal
	// ClusterIP without delete/recreate. Non-ClusterIP types are likewise outside V0.
	if s.Spec.Type != "" && s.Spec.Type != corev1.ServiceTypeClusterIP {
		return ErrImmutableServiceAllocation
	}
	if s.Spec.ClusterIP == corev1.ClusterIPNone {
		return ErrImmutableServiceAllocation
	}
	managedMetadata(&s.ObjectMeta, p)
	s.Spec.Type = corev1.ServiceTypeClusterIP
	s.Spec.Selector = SelectorLabels(p)
	// Ports are an intentional whole-list ownership boundary: V0 has exactly one
	// TCP port named http. Allocated network fields remain completely untouched.
	s.Spec.Ports = []corev1.ServicePort{{Name: HTTPPortName, Protocol: corev1.ProtocolTCP, Port: ServicePort(p), TargetPort: intstr.FromString(HTTPPortName)}}
	return nil
}

// HasAllocatedClusterNetwork reports fields the Service API assigns and AWCP never writes.
func HasAllocatedClusterNetwork(s *corev1.Service) bool {
	return s.Spec.ClusterIP != "" || len(s.Spec.ClusterIPs) != 0 || len(s.Spec.IPFamilies) != 0 || s.Spec.IPFamilyPolicy != nil
}
