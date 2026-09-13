// SPDX-License-Identifier: Apache-2.0
package resource

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apimachinery "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
)

func networkFor(t *testing.T, p *platform.AIWorkload) Intent {
	t.Helper()
	plan, err := (WorkloadBuilder{}).Build(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, intent := range plan {
		if _, ok := intent.Object.(*networkingv1.NetworkPolicy); ok {
			return intent
		}
	}
	t.Fatal("NetworkPolicy intent missing")
	return Intent{}
}

func buildNetworkPolicy(t *testing.T, p *platform.AIWorkload) *networkingv1.NetworkPolicy {
	t.Helper()
	intent := networkFor(t, p)
	if intent.Absent {
		t.Fatal("expected present NetworkPolicy intent")
	}
	policy := intent.Object.(*networkingv1.NetworkPolicy)
	if err := intent.Mutate(policy); err != nil {
		t.Fatal(err)
	}
	return policy
}

func TestNetworkPolicyMapping(t *testing.T) {
	p := source()
	before := p.DeepCopy()
	policy := buildNetworkPolicy(t, p)
	if !apimachinery.Semantic.DeepEqual(p, before) || policy.Name != ChildName(p.Name) || policy.Namespace != p.Namespace || len(policy.OwnerReferences) != 0 {
		t.Fatal("network builder changed primary or identity boundary")
	}
	expected := networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{MatchLabels: SelectorLabels(p)},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		Ingress: []networkingv1.NetworkPolicyIngressRule{{
			From:  []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{}}},
			Ports: []networkingv1.NetworkPolicyPort{{Protocol: ptr.To(corev1.ProtocolTCP), Port: ptr.To(intstr.FromString(HTTPPortName))}},
		}},
	}
	if !reflect.DeepEqual(policy.Spec, expected) || len(policy.Spec.Egress) != 0 {
		t.Fatalf("NetworkPolicy contract wrong: %#v", policy.Spec)
	}
	if policy.Annotations[WorkloadNameAnnotation] != p.Name {
		t.Fatal("common metadata missing")
	}
}

func TestNetworkPolicyDisableAndDrift(t *testing.T) {
	p := source()
	p.Spec.Network = &platform.NetworkSpec{Enabled: ptr.To(false)}
	intent := networkFor(t, p)
	if !intent.Absent || NetworkEnabled(p) {
		t.Fatal("explicitly disabled NetworkPolicy must be absent")
	}
	p.Spec.Network.Enabled = ptr.To(true)
	policy := buildNetworkPolicy(t, p)
	policy.ResourceVersion = "1"
	policy.Labels["user.example/keep"] = "yes"
	policy.Annotations["user.example/keep"] = "yes"
	before := policy.DeepCopy()
	if err := networkFor(t, p).Mutate(policy); err != nil {
		t.Fatal(err)
	}
	if !apimachinery.Semantic.DeepEqual(before, policy) {
		t.Fatal("unchanged NetworkPolicy mutate wrote unmanaged metadata")
	}
	policy.Spec.PolicyTypes = []networkingv1.PolicyType{networkingv1.PolicyTypeEgress}
	policy.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{{}}
	policy.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{{}}
	if err := networkFor(t, p).Mutate(policy); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(policy.Spec.PolicyTypes, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}) || len(policy.Spec.Egress) != 0 || len(policy.Spec.Ingress) != 1 || policy.Spec.Ingress[0].From[0].NamespaceSelector != nil || policy.Labels["user.example/keep"] != "yes" {
		t.Fatal("NetworkPolicy drift was not repaired within the managed spec boundary")
	}
}
