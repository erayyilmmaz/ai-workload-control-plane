// SPDX-License-Identifier: Apache-2.0
package resource

import (
	"errors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	quantity "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
)

var (
	ErrInvalidConfiguration = errors.New("InvalidConfiguration")
	ErrImmutableSelector    = errors.New("owned Deployment has an incompatible immutable selector")
)

const (
	ContainerName          = "workload"
	HTTPPortName           = "http"
	InstanceLabel          = "app.kubernetes.io/instance"
	UIDLabel               = "platform.example.io/workload-uid"
	EnvironmentLabel       = "platform.example.io/environment"
	TenantLabel            = "platform.example.io/tenant"
	WorkloadNameAnnotation = "platform.example.io/workload-name"
)

// WorkloadBuilder is the production composition root for V0 direct children.
type WorkloadBuilder struct{}

func (WorkloadBuilder) Build(p *platform.AIWorkload) ([]Intent, error) {
	if p == nil {
		return nil, ErrInvalidConfiguration
	}
	// Snapshot the source so callbacks cannot observe caller mutation or share maps.
	desired := p.DeepCopy()
	requests, limits, err := resourceLists(desired.Spec.Resources)
	if err != nil {
		return nil, err
	}
	identity := serviceAccountIntent(desired)
	deployment := Intent{Object: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: ChildName(p.Name), Namespace: p.Namespace}}, Mutate: func(o client.Object) error {
		d, ok := o.(*appsv1.Deployment)
		if !ok {
			return ErrInvalidConfiguration
		}
		return mutateDeployment(d, desired, requests, limits)
	}}
	service, err := serviceIntent(desired)
	if err != nil {
		return nil, err
	}
	autoscaling, err := autoscalingIntent(desired)
	if err != nil {
		return nil, err
	}
	availability, err := availabilityIntent(desired)
	if err != nil {
		return nil, err
	}
	return []Intent{identity, deployment, service, networkPolicyIntent(desired), autoscaling, availability}, nil
}

// SelectorLabels returns fresh stable identity labels, independent of generation/image.
func SelectorLabels(p *platform.AIWorkload) map[string]string {
	return map[string]string{InstanceLabel: ChildName(p.Name), UIDLabel: string(p.UID)}
}

func managedMetadata(m *metav1.ObjectMeta, p *platform.AIWorkload) {
	if m.Labels == nil {
		m.Labels = map[string]string{}
	}
	for k, v := range SelectorLabels(p) {
		m.Labels[k] = v
	}
	m.Labels["app.kubernetes.io/name"] = "ai-workload"
	m.Labels["app.kubernetes.io/part-of"] = "ai-workload-control-plane"
	m.Labels["app.kubernetes.io/managed-by"] = "awcp-controller"
	if p.Spec.Environment != "" {
		m.Labels[EnvironmentLabel] = p.Spec.Environment
	} else {
		delete(m.Labels, EnvironmentLabel)
	}
	if p.Spec.Tenant != "" {
		m.Labels[TenantLabel] = p.Spec.Tenant
	} else {
		delete(m.Labels, TenantLabel)
	}
	if m.Annotations == nil {
		m.Annotations = map[string]string{}
	}
	m.Annotations[WorkloadNameAnnotation] = p.Name
}

func mutateDeployment(d *appsv1.Deployment, p *platform.AIWorkload, requests, limits corev1.ResourceList) error {
	selector := SelectorLabels(p)
	if d.ResourceVersion != "" || d.UID != "" {
		if d.Spec.Selector == nil || len(d.Spec.Selector.MatchExpressions) != 0 || len(d.Spec.Selector.MatchLabels) != len(selector) {
			return ErrImmutableSelector
		}
		for k, v := range selector {
			if d.Spec.Selector.MatchLabels[k] != v {
				return ErrImmutableSelector
			}
		}
	}
	d.Spec.Selector = &metav1.LabelSelector{MatchLabels: selector}
	managedMetadata(&d.ObjectMeta, p)
	managedMetadata(&d.Spec.Template.ObjectMeta, p)
	// HPA writes Deployment.spec.replicas through the scale subresource. Preserve
	// a live value so AWCP never reverts an autoscaler decision; on first create,
	// initialize at the declared HPA lower bound.
	if !AutoscalingEnabled(p) {
		d.Spec.Replicas = ptr.To(ptr.Deref(p.Spec.Replicas, 1))
	} else if d.ResourceVersion == "" && d.UID == "" {
		d.Spec.Replicas = ptr.To(ptr.Deref(p.Spec.Autoscaling.MinReplicas, 1))
	}
	d.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType, RollingUpdate: &appsv1.RollingUpdateDeployment{MaxUnavailable: ptr.To(intstr.FromInt32(0)), MaxSurge: ptr.To(intstr.FromInt32(1))}}
	d.Spec.MinReadySeconds = 0
	d.Spec.ProgressDeadlineSeconds = ptr.To(int32(120))
	d.Spec.Paused = false
	pod := &d.Spec.Template.Spec
	pod.ServiceAccountName = ChildName(p.Name)
	// The API round-trips the legacy alias too; normalize both to the same identity.
	pod.DeprecatedServiceAccount = pod.ServiceAccountName
	pod.AutomountServiceAccountToken = ptr.To(false)
	if pod.SecurityContext == nil {
		pod.SecurityContext = &corev1.PodSecurityContext{}
	}
	pod.SecurityContext.RunAsNonRoot = ptr.To(true)
	pod.SecurityContext.SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}
	index := -1
	for i := range pod.Containers {
		if pod.Containers[i].Name == ContainerName {
			index = i
			break
		}
	}
	if index < 0 {
		pod.Containers = append(pod.Containers, corev1.Container{Name: ContainerName})
		index = len(pod.Containers) - 1
	}
	app := &pod.Containers[index]
	app.Image = p.Spec.Image
	app.ImagePullPolicy = corev1.PullIfNotPresent
	app.Resources.Requests = replaceCompute(app.Resources.Requests, requests)
	app.Resources.Limits = replaceCompute(app.Resources.Limits, limits)
	port := corev1.ContainerPort{Name: HTTPPortName, ContainerPort: p.Spec.Container.Port, Protocol: corev1.ProtocolTCP}
	portIndex := -1
	for i := range app.Ports {
		if app.Ports[i].Name == HTTPPortName {
			portIndex = i
			break
		}
	}
	if portIndex < 0 {
		app.Ports = append(app.Ports, port)
	} else {
		app.Ports[portIndex] = port
	}
	app.EnvFrom = nil
	for _, name := range p.Spec.SecretRefs {
		app.EnvFrom = append(app.EnvFrom, corev1.EnvFromSource{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: string(name)}, Optional: ptr.To(false)}})
	}
	app.ReadinessProbe = nil
	app.LivenessProbe = nil
	if p.Spec.Health != nil {
		app.ReadinessProbe = httpProbe(p.Spec.Health.Readiness, 0, 5)
		app.LivenessProbe = httpProbe(p.Spec.Health.Liveness, 10, 10)
	}
	if app.SecurityContext == nil {
		app.SecurityContext = &corev1.SecurityContext{}
	}
	app.SecurityContext.RunAsNonRoot = ptr.To(true)
	app.SecurityContext.Privileged = ptr.To(false)
	app.SecurityContext.AllowPrivilegeEscalation = ptr.To(false)
	app.SecurityContext.Capabilities = &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}
	app.SecurityContext.SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}
	return nil
}

func httpProbe(spec *platform.HTTPProbeSpec, delay, period int32) *corev1.Probe {
	if spec == nil {
		return nil
	}
	return &corev1.Probe{ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: spec.Path, Port: intstr.FromString(HTTPPortName), Scheme: corev1.URISchemeHTTP}}, InitialDelaySeconds: delay, PeriodSeconds: period, TimeoutSeconds: 1, SuccessThreshold: 1, FailureThreshold: 3}
}

func resourceLists(spec *platform.ResourceRequirements) (corev1.ResourceList, corev1.ResourceList, error) {
	parse := func(input platform.ResourceQuantities) (corev1.ResourceList, error) {
		result := corev1.ResourceList{}
		for name, value := range input {
			if name != "cpu" && name != "memory" {
				return nil, ErrInvalidConfiguration
			}
			q, err := quantity.ParseQuantity(string(value))
			if err != nil || q.Sign() < 0 {
				return nil, ErrInvalidConfiguration
			}
			result[corev1.ResourceName(name)] = q
		}
		return result, nil
	}
	if spec == nil {
		return nil, nil, nil
	}
	requests, err := parse(spec.Requests)
	if err != nil {
		return nil, nil, err
	}
	limits, err := parse(spec.Limits)
	if err != nil {
		return nil, nil, err
	}
	for name, limit := range limits {
		if request, exists := requests[name]; exists {
			if request.Cmp(limit) > 0 {
				return nil, nil, ErrInvalidConfiguration
			}
		} else {
			// Match Kubernetes limit-only request defaulting without changing the CR spec.
			requests[name] = limit.DeepCopy()
		}
	}
	return requests, limits, nil
}

// CPU/memory are owned; preserve admission-supplied non-API resource keys/claims.
func replaceCompute(current, desired corev1.ResourceList) corev1.ResourceList {
	result := current.DeepCopy()
	if result == nil {
		result = corev1.ResourceList{}
	}
	delete(result, corev1.ResourceCPU)
	delete(result, corev1.ResourceMemory)
	for k, v := range desired {
		result[k] = v.DeepCopy()
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
