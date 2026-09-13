// SPDX-License-Identifier: Apache-2.0
package resource

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
)

// NetworkEnabled preserves an explicit false while treating omitted/defaulted values as true.
func NetworkEnabled(p *platform.AIWorkload) bool {
	return p != nil && (p.Spec.Network == nil || p.Spec.Network.Enabled == nil || *p.Spec.Network.Enabled)
}

func networkPolicyIntent(p *platform.AIWorkload) Intent {
	intent := Intent{Object: &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: ChildName(p.Name), Namespace: p.Namespace}}, Absent: !NetworkEnabled(p)}
	if intent.Absent {
		return intent
	}
	intent.Mutate = func(o client.Object) error {
		policy, ok := o.(*networkingv1.NetworkPolicy)
		if !ok {
			return ErrInvalidConfiguration
		}
		managedMetadata(&policy.ObjectMeta, p)
		policy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: SelectorLabels(p)},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{{
				From:  []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{}}},
				Ports: []networkingv1.NetworkPolicyPort{{Protocol: ptr.To(corev1.ProtocolTCP), Port: ptr.To(intstr.FromString(HTTPPortName))}},
			}},
		}
		return nil
	}
	return intent
}
