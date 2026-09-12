// SPDX-License-Identifier: Apache-2.0
// Package fixtures contains deliberately minimal test plans, NOT production workload mappings.
package fixtures

import (
	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/resource"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Plan tests all four child kinds and server defaults while preserving unmanaged fields.
func Plan(p *platform.AIWorkload) ([]resource.Intent, error) {
	identity := metav1.ObjectMeta{Name: resource.ChildName(p.Name), Namespace: p.Namespace}
	label := map[string]string{"test.awcp/owner": string(p.UID)}
	mark := func(o client.Object) {
		labels := o.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels["test.awcp/managed"] = "true"
		o.SetLabels(labels)
	}
	return []resource.Intent{
		{Object: &networkingv1.NetworkPolicy{ObjectMeta: identity}, Absent: p.Spec.Network != nil && p.Spec.Network.Enabled != nil && !*p.Spec.Network.Enabled, Mutate: func(o client.Object) error {
			mark(o)
			n := o.(*networkingv1.NetworkPolicy)
			n.Spec.PodSelector = metav1.LabelSelector{MatchLabels: label}
			n.Spec.PolicyTypes = []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}
			return nil
		}},
		{Object: &corev1.Service{ObjectMeta: identity}, Absent: p.Spec.Service != nil && p.Spec.Service.Enabled != nil && !*p.Spec.Service.Enabled, Mutate: func(o client.Object) error {
			mark(o)
			s := o.(*corev1.Service)
			s.Spec.Selector = label
			s.Spec.Type = corev1.ServiceTypeClusterIP
			if len(s.Spec.Ports) == 0 {
				s.Spec.Ports = []corev1.ServicePort{{Name: "http"}}
			}
			s.Spec.Ports[0].Port = 80
			s.Spec.Ports[0].Protocol = corev1.ProtocolTCP
			s.Spec.Ports[0].TargetPort = intstr.FromString("http")
			return nil
		}},
		{Object: &appsv1.Deployment{ObjectMeta: identity}, Mutate: func(o client.Object) error {
			mark(o)
			d := o.(*appsv1.Deployment)
			d.Spec.Replicas = ptr.To(ptr.Deref(p.Spec.Replicas, 1))
			d.Spec.Selector = &metav1.LabelSelector{MatchLabels: label}
			if d.Spec.Template.Labels == nil {
				d.Spec.Template.Labels = map[string]string{}
			}
			d.Spec.Template.Labels["test.awcp/owner"] = string(p.UID)
			if len(d.Spec.Template.Spec.Containers) == 0 {
				d.Spec.Template.Spec.Containers = []corev1.Container{{Name: "workload", Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 8080}}}}
			}
			d.Spec.Template.Spec.Containers[0].Image = p.Spec.Image
			return nil
		}},
		{Object: &corev1.ServiceAccount{ObjectMeta: identity}, Mutate: func(o client.Object) error {
			mark(o)
			o.(*corev1.ServiceAccount).AutomountServiceAccountToken = ptr.To(false)
			return nil
		}},
	}, nil
}
