// SPDX-License-Identifier: Apache-2.0
package resource

import (
	"errors"
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	quantity "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
)

func source() *platform.AIWorkload {
	return &platform.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "workloads", UID: "00000000-0000-0000-0000-000000000001", Generation: 1}, Spec: platform.AIWorkloadSpec{
		Image: "example.invalid/demo:v1", Replicas: ptr.To(int32(2)), Container: platform.ContainerSpec{Port: 8080},
		Resources:  &platform.ResourceRequirements{Requests: platform.ResourceQuantities{"cpu": "100m", "memory": "128Mi"}, Limits: platform.ResourceQuantities{"cpu": "1", "memory": "1Gi"}},
		Health:     &platform.HealthSpec{Readiness: &platform.HTTPProbeSpec{Path: "/ready"}, Liveness: &platform.HTTPProbeSpec{Path: "/health"}},
		SecretRefs: []platform.SecretReference{"first", "second"},
	}}
}

func intentFor(t *testing.T, p *platform.AIWorkload) Intent {
	t.Helper()
	plan, err := (WorkloadBuilder{}).Build(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, intent := range plan {
		if _, ok := intent.Object.(*appsv1.Deployment); ok {
			return intent
		}
	}
	t.Fatal("Deployment intent missing")
	return Intent{}
}
func build(t *testing.T, p *platform.AIWorkload) *appsv1.Deployment {
	t.Helper()
	intent := intentFor(t, p)
	d := intent.Object.(*appsv1.Deployment)
	if err := intent.Mutate(d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDeploymentMapping(t *testing.T) {
	p := source()
	p.Spec.Environment = "staging"
	p.Spec.Tenant = "alpha"
	before := p.DeepCopy()
	d := build(t, p)
	pod := d.Spec.Template.Spec
	app := pod.Containers[0]
	if !apiequality.Semantic.DeepEqual(p, before) {
		t.Fatal("builder mutated primary")
	}
	if d.Name != ChildName(p.Name) || d.Namespace != p.Namespace || len(d.OwnerReferences) != 0 {
		t.Fatal("identity wrong; ownerReference belongs to engine")
	}
	if !reflect.DeepEqual(d.Spec.Selector.MatchLabels, SelectorLabels(p)) || len(d.Spec.Selector.MatchExpressions) != 0 {
		t.Fatal("selector identity wrong")
	}
	for _, labels := range []map[string]string{d.Labels, d.Spec.Template.Labels} {
		for k, v := range SelectorLabels(p) {
			if labels[k] != v {
				t.Fatalf("identity label %s missing", k)
			}
		}
		if labels["app.kubernetes.io/name"] != "ai-workload" || labels["app.kubernetes.io/managed-by"] != "awcp-controller" || labels["app.kubernetes.io/part-of"] != "ai-workload-control-plane" {
			t.Fatal("common labels wrong")
		}
		if labels[EnvironmentLabel] != "staging" {
			t.Fatal("environment label missing")
		}
		if labels[TenantLabel] != "alpha" {
			t.Fatal("tenant label missing")
		}
	}
	if d.Annotations[WorkloadNameAnnotation] != p.Name || d.Spec.Template.Annotations[WorkloadNameAnnotation] != p.Name {
		t.Fatal("full name annotation missing")
	}
	if ptr.Deref(d.Spec.Replicas, 0) != 2 || app.Name != ContainerName || app.Image != p.Spec.Image || app.ImagePullPolicy != corev1.PullIfNotPresent {
		t.Fatal("image/replicas/pull mapping wrong")
	}
	if !reflect.DeepEqual(app.Ports, []corev1.ContainerPort{{Name: HTTPPortName, ContainerPort: 8080, Protocol: corev1.ProtocolTCP}}) {
		t.Fatalf("ports wrong: %+v", app.Ports)
	}
	for k, v := range p.Spec.Resources.Requests {
		if q := app.Resources.Requests[corev1.ResourceName(k)]; q.Cmp(quantity.MustParse(string(v))) != 0 {
			t.Fatalf("request %s wrong", k)
		}
	}
	for k, v := range p.Spec.Resources.Limits {
		if q := app.Resources.Limits[corev1.ResourceName(k)]; q.Cmp(quantity.MustParse(string(v))) != 0 {
			t.Fatalf("limit %s wrong", k)
		}
	}
	if len(app.EnvFrom) != 2 {
		t.Fatal("Secret refs missing")
	}
	for i, name := range p.Spec.SecretRefs {
		ref := app.EnvFrom[i]
		if ref.ConfigMapRef != nil || ref.Prefix != "" || ref.SecretRef.Name != string(name) || ptr.Deref(ref.SecretRef.Optional, true) {
			t.Fatal("ordered non-optional envFrom contract violated")
		}
	}
	if pod.ServiceAccountName != d.Name || pod.DeprecatedServiceAccount != d.Name || ptr.Deref(pod.AutomountServiceAccountToken, true) {
		t.Fatal("dedicated identity/token binding wrong")
	}
	if !ptr.Deref(pod.SecurityContext.RunAsNonRoot, false) || pod.SecurityContext.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault || !ptr.Deref(app.SecurityContext.RunAsNonRoot, false) || ptr.Deref(app.SecurityContext.AllowPrivilegeEscalation, true) || ptr.Deref(app.SecurityContext.Privileged, true) || app.SecurityContext.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault || len(app.SecurityContext.Capabilities.Add) != 0 || !reflect.DeepEqual(app.SecurityContext.Capabilities.Drop, []corev1.Capability{"ALL"}) {
		t.Fatal("security defaults wrong")
	}
	for _, tc := range []struct {
		probe         *corev1.Probe
		path          string
		delay, period int32
	}{{app.ReadinessProbe, "/ready", 0, 5}, {app.LivenessProbe, "/health", 10, 10}} {
		probe := tc.probe
		if probe == nil || probe.HTTPGet == nil || probe.HTTPGet.Path != tc.path || probe.HTTPGet.Port != intstr.FromString(HTTPPortName) || probe.HTTPGet.Scheme != corev1.URISchemeHTTP || probe.InitialDelaySeconds != tc.delay || probe.PeriodSeconds != tc.period || probe.TimeoutSeconds != 1 || probe.SuccessThreshold != 1 || probe.FailureThreshold != 3 || probe.Exec != nil || probe.TCPSocket != nil || probe.GRPC != nil {
			t.Fatalf("probe contract wrong: %+v", probe)
		}
	}
	if d.Spec.Strategy.Type != appsv1.RollingUpdateDeploymentStrategyType || *d.Spec.Strategy.RollingUpdate.MaxSurge != intstr.FromInt32(1) || *d.Spec.Strategy.RollingUpdate.MaxUnavailable != intstr.FromInt32(0) || d.Spec.MinReadySeconds != 0 || ptr.Deref(d.Spec.ProgressDeadlineSeconds, 0) != 120 || d.Spec.Paused {
		t.Fatal("rolling-update contract wrong")
	}
}

func TestEnvironmentLabelIsRemovedWhenOmitted(t *testing.T) {
	p := source()
	p.Spec.Environment = "dev"
	d := build(t, p)
	p.Spec.Environment = ""
	if err := intentFor(t, p).Mutate(d); err != nil {
		t.Fatal(err)
	}
	if _, found := d.Labels[EnvironmentLabel]; found {
		t.Fatal("omitted environment must not leave a stale child label")
	}
}

func TestTenantLabelIsRemovedWhenOmitted(t *testing.T) {
	p := source()
	p.Spec.Tenant = "alpha"
	d := build(t, p)
	p.Spec.Tenant = ""
	if err := intentFor(t, p).Mutate(d); err != nil {
		t.Fatal(err)
	}
	if _, found := d.Labels[TenantLabel]; found {
		t.Fatal("omitted tenant must not leave a stale child label")
	}
}

func TestDeploymentScaleDoesNotChangeTemplate(t *testing.T) {
	p := source()
	d := build(t, p)
	template := d.Spec.Template.DeepCopy()
	selector := d.Spec.Selector.DeepCopy()
	for _, replicas := range []int32{1, 2, 0} {
		p.Spec.Replicas = ptr.To(replicas)
		p.Generation++
		next := build(t, p)
		if *next.Spec.Replicas != replicas || !apiequality.Semantic.DeepEqual(template, &next.Spec.Template) || !apiequality.Semantic.DeepEqual(selector, next.Spec.Selector) {
			t.Fatal("scaling changed pod template or selector")
		}
	}
	p.Spec.Replicas = nil
	if got := build(t, p); ptr.Deref(got.Spec.Replicas, 0) != 1 {
		t.Fatal("nil replicas should default to 1")
	}
}

func TestDeploymentTemplateChangesAndRemoval(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*platform.AIWorkload)
	}{
		{"image tag", func(p *platform.AIWorkload) { p.Spec.Image = "example.invalid/demo:v2" }},
		{"image digest", func(p *platform.AIWorkload) {
			p.Spec.Image = "example.invalid/demo@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
		{"port", func(p *platform.AIWorkload) { p.Spec.Container.Port = 9090 }},
		{"requests", func(p *platform.AIWorkload) { p.Spec.Resources.Requests["cpu"] = "200m" }},
		{"limits", func(p *platform.AIWorkload) { p.Spec.Resources.Limits["memory"] = "2Gi" }},
		{"readiness", func(p *platform.AIWorkload) { p.Spec.Health.Readiness.Path = "/ready/v2" }},
		{"liveness", func(p *platform.AIWorkload) { p.Spec.Health.Liveness.Path = "/health/v2" }},
		{"Secret order", func(p *platform.AIWorkload) { p.Spec.SecretRefs = []platform.SecretReference{"second", "first"} }},
		{"Secret removal", func(p *platform.AIWorkload) { p.Spec.SecretRefs = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := source()
			old := build(t, p)
			tc.change(p)
			next := build(t, p)
			if apiequality.Semantic.DeepEqual(old.Spec.Template, next.Spec.Template) {
				t.Fatal("template did not change")
			}
			if !apiequality.Semantic.DeepEqual(old.Spec.Selector, next.Spec.Selector) {
				t.Fatal("selector changed")
			}
		})
	}
	p := source()
	d := build(t, p)
	p.Spec.Health = nil
	p.Spec.Resources = nil
	p.Spec.SecretRefs = nil
	intent := intentFor(t, p)
	if err := intent.Mutate(d); err != nil {
		t.Fatal(err)
	}
	app := d.Spec.Template.Spec.Containers[0]
	if app.ReadinessProbe != nil || app.LivenessProbe != nil || len(app.EnvFrom) != 0 || len(app.Resources.Requests) != 0 || len(app.Resources.Limits) != 0 {
		t.Fatal("removed settings remained in Deployment")
	}
	for _, health := range []*platform.HealthSpec{nil, {}, {Readiness: &platform.HTTPProbeSpec{Path: "/r"}}, {Liveness: &platform.HTTPProbeSpec{Path: "/l"}}} {
		p.Spec.Health = health
		app := build(t, p).Spec.Template.Spec.Containers[0]
		if (app.ReadinessProbe != nil) != (health != nil && health.Readiness != nil) || (app.LivenessProbe != nil) != (health != nil && health.Liveness != nil) {
			t.Fatal("omitted probe enabled")
		}
	}
}

func TestDeploymentPreservesUnmanagedFieldsAndUsesContainerName(t *testing.T) {
	p := source()
	d := build(t, p)
	d.ResourceVersion = "1"
	d.Labels["user.example/label"] = "keep"
	d.Annotations["deployment.kubernetes.io/revision"] = "4"
	d.Spec.Template.Annotations["sidecar.example/inject"] = "yes"
	d.Spec.Template.Labels["user.example/pod"] = "keep"
	d.Spec.RevisionHistoryLimit = ptr.To(int32(3))
	pod := &d.Spec.Template.Spec
	pod.DNSPolicy = corev1.DNSClusterFirst
	pod.RestartPolicy = corev1.RestartPolicyAlways
	pod.TerminationGracePeriodSeconds = ptr.To(int64(30))
	pod.SecurityContext.FSGroup = ptr.To(int64(1234))
	pod.Volumes = []corev1.Volume{{Name: "injected", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}}
	app := &pod.Containers[0]
	app.Command = []string{"server"}
	app.Env = []corev1.EnvVar{{Name: "INJECTED", Value: "synthetic"}}
	app.TerminationMessagePath = "/dev/termination-log"
	app.TerminationMessagePolicy = corev1.TerminationMessageReadFile
	app.SecurityContext.ReadOnlyRootFilesystem = ptr.To(true)
	app.SecurityContext.RunAsUser = ptr.To(int64(1000))
	app.Ports = append(app.Ports, corev1.ContainerPort{Name: "metrics", ContainerPort: 9091, Protocol: corev1.ProtocolTCP})
	app.Resources.Limits[corev1.ResourceEphemeralStorage] = quantity.MustParse("1Gi")
	sidecar := corev1.Container{Name: "injected", Image: "example.invalid/sidecar:v1"}
	pod.Containers = append([]corev1.Container{sidecar}, pod.Containers...)
	before := d.DeepCopy()
	intent := intentFor(t, p)
	if err := intent.Mutate(d); err != nil {
		t.Fatal(err)
	}
	if !apiequality.Semantic.DeepEqual(before, d) {
		t.Fatal("unchanged mutate lost defaults or injected fields")
	}
	p.Spec.Image = "example.invalid/demo:v2"
	intent = intentFor(t, p)
	if err := intent.Mutate(d); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d.Spec.Template.Spec.Containers[0], sidecar) || d.Spec.Template.Spec.Containers[1].Image != p.Spec.Image {
		t.Fatal("container position used instead of name")
	}
	before.Spec.Template.Spec.Containers[1].Image = p.Spec.Image
	if !apiequality.Semantic.DeepEqual(before, d) {
		t.Fatal("image change touched unmanaged fields")
	}
}

func TestDeploymentImmutableSelectorFailsBeforeMutation(t *testing.T) {
	p := source()
	for _, selector := range []*metav1.LabelSelector{nil, {MatchLabels: map[string]string{"foreign": "value"}}, {MatchLabels: SelectorLabels(p), MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "extra", Operator: metav1.LabelSelectorOpExists}}}} {
		d := build(t, p)
		d.ResourceVersion = "1"
		d.Spec.Selector = selector
		before := d.DeepCopy()
		intent := intentFor(t, p)
		if err := intent.Mutate(d); !errors.Is(err, ErrImmutableSelector) {
			t.Fatalf("immutable conflict not detected: %v", err)
		}
		if !apiequality.Semantic.DeepEqual(before, d) {
			t.Fatal("immutable conflict mutated object")
		}
	}
}

func TestDeploymentBuilderDoesNotAliasInputsOrOutputs(t *testing.T) {
	p := source()
	intent := intentFor(t, p)
	p.Spec.Image = "caller-changed"
	p.Spec.Resources.Requests["cpu"] = "999m"
	p.Spec.SecretRefs[0] = "changed"
	first := intent.Object.DeepCopyObject().(*appsv1.Deployment)
	if err := intent.Mutate(first); err != nil {
		t.Fatal(err)
	}
	app := first.Spec.Template.Spec.Containers[0]
	if app.Image != "example.invalid/demo:v1" || app.EnvFrom[0].SecretRef.Name != "first" || app.Resources.Requests.Cpu().Cmp(quantity.MustParse("100m")) != 0 {
		t.Fatal("plan aliased caller")
	}
	want := first.DeepCopy()
	first.Spec.Selector.MatchLabels[UIDLabel] = "changed"
	first.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU] = quantity.MustParse("888m")
	second := intent.Object.DeepCopyObject().(*appsv1.Deployment)
	if err := intent.Mutate(second); err != nil {
		t.Fatal(err)
	}
	if !apiequality.Semantic.DeepEqual(want, second) {
		t.Fatal("plan aliased previous output")
	}
}

func TestDeploymentResourceValidationAndNormalization(t *testing.T) {
	for _, spec := range []*platform.ResourceRequirements{
		{Requests: platform.ResourceQuantities{"cpu": "not-quantity"}}, {Requests: platform.ResourceQuantities{"memory": "-1Gi"}},
		{Limits: platform.ResourceQuantities{"gpu": "1"}}, {Requests: platform.ResourceQuantities{"cpu": "2"}, Limits: platform.ResourceQuantities{"cpu": "1"}},
	} {
		p := source()
		p.Spec.Resources = spec
		if _, err := (WorkloadBuilder{}).Build(p); !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("invalid resources accepted: %v", err)
		}
	}
	p := source()
	p.Spec.Resources = &platform.ResourceRequirements{Limits: platform.ResourceQuantities{"cpu": "1", "memory": "128Mi"}}
	d := build(t, p)
	app := d.Spec.Template.Spec.Containers[0]
	if !apiequality.Semantic.DeepEqual(app.Resources.Requests, app.Resources.Limits) || len(p.Spec.Resources.Requests) != 0 {
		t.Fatal("limit-only normalization must affect Deployment, never CR")
	}
	p.Spec.Resources.Requests = platform.ResourceQuantities{"cpu": "0", "memory": "128Mi"}
	app = build(t, p).Spec.Template.Spec.Containers[0]
	if !app.Resources.Requests.Cpu().IsZero() {
		t.Fatal("explicit zero request lost")
	}
	if _, err := (WorkloadBuilder{}).Build(nil); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatal("nil parent not rejected")
	}
}
