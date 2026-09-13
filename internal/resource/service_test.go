// SPDX-License-Identifier: Apache-2.0
package resource

import (
	"errors"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apimachinery "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
)

func serviceFor(t *testing.T, p *platform.AIWorkload) Intent {
	t.Helper()
	plan, err := (WorkloadBuilder{}).Build(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, intent := range plan {
		if _, ok := intent.Object.(*corev1.Service); ok {
			return intent
		}
	}
	t.Fatal("Service intent missing")
	return Intent{}
}

func buildService(t *testing.T, p *platform.AIWorkload) *corev1.Service {
	t.Helper()
	intent := serviceFor(t, p)
	if intent.Absent {
		t.Fatal("expected present Service intent")
	}
	s := intent.Object.(*corev1.Service)
	if err := intent.Mutate(s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestServiceMappingAndDiscovery(t *testing.T) {
	p := source()
	before := p.DeepCopy()
	s := buildService(t, p)
	if !apimachinery.Semantic.DeepEqual(p, before) {
		t.Fatal("builder mutated primary")
	}
	if s.Name != ChildName(p.Name) || s.Namespace != p.Namespace || len(s.OwnerReferences) != 0 {
		t.Fatal("identity wrong; ownerReference belongs to engine")
	}
	if s.Spec.Type != corev1.ServiceTypeClusterIP || !reflect.DeepEqual(s.Spec.Selector, SelectorLabels(p)) {
		t.Fatal("ClusterIP or selector mapping wrong")
	}
	expectedPorts := []corev1.ServicePort{{Name: HTTPPortName, Protocol: corev1.ProtocolTCP, Port: 80, TargetPort: intstr.FromString(HTTPPortName)}}
	if !reflect.DeepEqual(s.Spec.Ports, expectedPorts) {
		t.Fatalf("ports wrong: %#v", s.Spec.Ports)
	}
	for k, v := range SelectorLabels(p) {
		if s.Labels[k] != v {
			t.Fatalf("identity label %q missing", k)
		}
	}
	if s.Annotations[WorkloadNameAnnotation] != p.Name || s.Spec.ClusterIP != "" || len(s.Spec.ClusterIPs) != 0 || len(s.Spec.IPFamilies) != 0 || s.Spec.IPFamilyPolicy != nil {
		t.Fatal("metadata or API-assigned fields wrong")
	}
	if got, want := ServiceEndpoint(p), ChildName(p.Name)+".workloads.svc:80"; got != want {
		t.Fatalf("endpoint = %q, want %q", got, want)
	}

	p.Spec.Service = &platform.ServiceSpec{Port: ptr.To(int32(8081))}
	if got := buildService(t, p).Spec.Ports[0]; got.Port != 8081 || got.TargetPort != intstr.FromString(HTTPPortName) {
		t.Fatalf("custom service port = %#v", got)
	}
	if got, want := ServiceEndpoint(p), ChildName(p.Name)+".workloads.svc:8081"; got != want {
		t.Fatalf("endpoint = %q, want %q", got, want)
	}
}

func TestServiceDisableAndPortValidation(t *testing.T) {
	p := source()
	p.Spec.Service = &platform.ServiceSpec{Enabled: ptr.To(false)}
	intent := serviceFor(t, p)
	if !intent.Absent || ServiceEnabled(p) || ServiceEndpoint(p) != "" {
		t.Fatal("explicitly disabled Service must be absent with no endpoint")
	}
	for _, port := range []int32{0, 65536} {
		t.Run("invalid port", func(t *testing.T) {
			invalid := source()
			invalid.Spec.Service = &platform.ServiceSpec{Port: ptr.To(port)}
			if _, err := (WorkloadBuilder{}).Build(invalid); !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("Build error = %v, want InvalidConfiguration", err)
			}
		})
	}
}

func TestServiceDriftAndAllocatedFields(t *testing.T) {
	p := source()
	s := buildService(t, p)
	s.ResourceVersion = "1"
	s.Spec.ClusterIP = "10.96.12.34"
	s.Spec.ClusterIPs = []string{"10.96.12.34"}
	s.Spec.IPFamilies = []corev1.IPFamily{corev1.IPv4Protocol}
	s.Spec.IPFamilyPolicy = ptr.To(corev1.IPFamilyPolicySingleStack)
	s.Labels["user.example/label"] = "keep"
	s.Annotations["service.kubernetes.io/managed"] = "external"
	before := s.DeepCopy()
	if err := serviceFor(t, p).Mutate(s); err != nil {
		t.Fatal(err)
	}
	if !apimachinery.Semantic.DeepEqual(before, s) || !HasAllocatedClusterNetwork(s) {
		t.Fatal("unchanged mutate modified API allocation or unmanaged metadata")
	}

	s.Spec.Selector = map[string]string{"drift": "true"}
	s.Spec.Ports = []corev1.ServicePort{{Name: "wrong", Protocol: corev1.ProtocolUDP, Port: 1234, TargetPort: intstr.FromInt(1234)}}
	if err := serviceFor(t, p).Mutate(s); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.Spec.Selector, SelectorLabels(p)) || !reflect.DeepEqual(s.Spec.Ports, []corev1.ServicePort{{Name: HTTPPortName, Protocol: corev1.ProtocolTCP, Port: 80, TargetPort: intstr.FromString(HTTPPortName)}}) {
		t.Fatal("managed Service drift was not repaired")
	}
	if s.Spec.ClusterIP != "10.96.12.34" || !reflect.DeepEqual(s.Spec.ClusterIPs, []string{"10.96.12.34"}) || !reflect.DeepEqual(s.Spec.IPFamilies, []corev1.IPFamily{corev1.IPv4Protocol}) || ptr.Deref(s.Spec.IPFamilyPolicy, "") != corev1.IPFamilyPolicySingleStack {
		t.Fatal("allocated cluster network fields changed")
	}
}

func TestServiceRejectsImmutableExposureModes(t *testing.T) {
	p := source()
	for _, mutate := range []func(*corev1.Service){
		func(s *corev1.Service) { s.Spec.Type = corev1.ServiceTypeNodePort },
		func(s *corev1.Service) { s.Spec.Type = corev1.ServiceTypeLoadBalancer },
		func(s *corev1.Service) { s.Spec.Type = corev1.ServiceTypeExternalName },
		func(s *corev1.Service) { s.Spec.ClusterIP = corev1.ClusterIPNone },
	} {
		t.Run("incompatible allocation", func(t *testing.T) {
			s := buildService(t, p)
			s.ResourceVersion = "1"
			mutate(s)
			before := s.DeepCopy()
			if err := serviceFor(t, p).Mutate(s); !errors.Is(err, ErrImmutableServiceAllocation) {
				t.Fatalf("Mutate error = %v, want immutable allocation error", err)
			}
			if !apimachinery.Semantic.DeepEqual(before, s) {
				t.Fatal("immutable conflict must not partially mutate Service")
			}
		})
	}
}
