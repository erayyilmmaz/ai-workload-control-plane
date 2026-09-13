// SPDX-License-Identifier: Apache-2.0
package reconciliation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	quantity "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/yaml"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/controller"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/manager"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/resource"
)

func deploymentIntent(t *testing.T, plan []resource.Intent) resource.Intent {
	t.Helper()
	for _, intent := range plan {
		if _, ok := intent.Object.(*appsv1.Deployment); ok {
			return intent
		}
	}
	t.Fatal("Deployment intent missing")
	return resource.Intent{}
}

func TestProductionDeployment(t *testing.T) {
	if testing.Short() {
		t.Skip("requires envtest; run make test")
	}
	environment := &envtest.Environment{CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")}, ErrorIfCRDPathMissing: true}
	cfg, err := environment.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := environment.Stop(); err != nil {
			t.Error(err)
		}
	})
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := platform.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	api, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	for _, name := range []string{"workloads", "system"} {
		if err := api.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}); err != nil {
			t.Fatal(err)
		}
	}
	create := func(name string) *platform.AIWorkload {
		t.Helper()
		p := &platform.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "workloads"}, Spec: platform.AIWorkloadSpec{
			Image: "example.invalid/demo:v1", Container: platform.ContainerSpec{Port: 8080},
			Resources:  &platform.ResourceRequirements{Limits: platform.ResourceQuantities{"cpu": "1", "memory": "128Mi"}},
			Health:     &platform.HealthSpec{Readiness: &platform.HTTPProbeSpec{Path: "/ready"}, Liveness: &platform.HTTPProbeSpec{Path: "/health"}},
			SecretRefs: []platform.SecretReference{"first", "second"},
		}}
		if err := api.Create(ctx, p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	alpha, beta, collision := create("alpha"), create("beta"), create("immutable")
	for _, name := range []string{"first", "second"} {
		if err := api.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "workloads"}}); err != nil {
			t.Fatal(err)
		}
	}
	// Valid, current-UID-owned Deployment with a different immutable selector.
	legacy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: resource.ChildName(collision.Name), Namespace: collision.Namespace, OwnerReferences: []metav1.OwnerReference{{APIVersion: platform.GroupVersion.String(), Kind: "AIWorkload", Name: collision.Name, UID: collision.UID, Controller: ptr.To(true), BlockOwnerDeletion: ptr.To(false)}}}, Spec: appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"legacy": "yes"}}, Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"legacy": "yes"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: resource.ContainerName, Image: "example.invalid/legacy:v1"}}}}}}
	if err := api.Create(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	legacyBefore := legacy.DeepCopy()
	// No Builder override: exercise the same default composition used by cmd/main.go.
	mgr, err := manager.New(cfg, manager.Options{WatchNamespace: "workloads", ManagerNamespace: "system", ProbeAddress: "0", ControllerName: "production-deployment"})
	if err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- mgr.Start(runCtx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(15 * time.Second):
			t.Error("manager did not stop")
		}
	})
	keyFor := func(p *platform.AIWorkload) client.ObjectKey {
		return client.ObjectKey{Namespace: p.Namespace, Name: resource.ChildName(p.Name)}
	}
	converged := func(t *testing.T, p *platform.AIWorkload) *appsv1.Deployment {
		t.Helper()
		plan, err := (resource.WorkloadBuilder{}).Build(p)
		if err != nil {
			t.Fatal(err)
		}
		var d appsv1.Deployment
		eventually(t, "production Deployment converged", func() error {
			if err := api.Get(ctx, keyFor(p), &d); err != nil {
				return err
			}
			expected := d.DeepCopy()
			if err := deploymentIntent(t, plan).Mutate(expected); err != nil {
				return err
			}
			if !apiequality.Semantic.DeepEqual(&d, expected) {
				return errors.New("managed fields not converged")
			}
			owner := metav1.GetControllerOf(&d)
			if owner == nil || owner.UID != p.UID || ptr.Deref(owner.BlockOwnerDeletion, true) {
				return errors.New("owner UID wrong")
			}
			return nil
		})
		return d.DeepCopy()
	}
	update := func(t *testing.T, p *platform.AIWorkload, change func(*platform.AIWorkload)) *appsv1.Deployment {
		t.Helper()
		if err := api.Get(ctx, client.ObjectKeyFromObject(p), p); err != nil {
			t.Fatal(err)
		}
		change(p)
		if err := api.Update(ctx, p); err != nil {
			t.Fatal(err)
		}
		return converged(t, p)
	}
	current := converged(t, alpha)
	other := converged(t, beta)
	t.Run("default production mapping and API defaults", func(t *testing.T) {
		app := current.Spec.Template.Spec.Containers[0]
		pod := current.Spec.Template.Spec
		if app.Image != "example.invalid/demo:v1" || *current.Spec.Replicas != 1 || app.Ports[0].Name != "http" || app.Ports[0].ContainerPort != 8080 {
			t.Fatal("basic mapping missing")
		}
		if pod.ServiceAccountName != current.Name || ptr.Deref(pod.AutomountServiceAccountToken, true) || !ptr.Deref(pod.SecurityContext.RunAsNonRoot, false) {
			t.Fatal("identity/security mapping missing")
		}
		if app.Resources.Limits.Cpu().Cmp(quantity.MustParse("1")) != 0 || !apiequality.Semantic.DeepEqual(app.Resources.Limits, app.Resources.Requests) {
			t.Fatal("limits/defaulted requests wrong")
		}
		if app.ReadinessProbe == nil || app.ReadinessProbe.PeriodSeconds != 5 || app.LivenessProbe == nil || app.LivenessProbe.InitialDelaySeconds != 10 || len(app.EnvFrom) != 2 || app.EnvFrom[0].SecretRef.Name != "first" || ptr.Deref(app.EnvFrom[1].SecretRef.Optional, true) {
			t.Fatal("probe/Secret mapping wrong")
		}
		if pod.DNSPolicy != corev1.DNSClusterFirst || pod.TerminationGracePeriodSeconds == nil || app.TerminationMessagePath == "" || current.Spec.RevisionHistoryLimit == nil {
			t.Fatal("API defaults not round-tripped")
		}
		if err := api.Get(ctx, client.ObjectKeyFromObject(alpha), alpha); err != nil {
			t.Fatal(err)
		}
		if len(alpha.Spec.Resources.Requests) != 0 {
			t.Fatal("Deployment defaulting modified primary")
		}
		var identity corev1.ServiceAccount
		if err := api.Get(ctx, keyFor(alpha), &identity); err != nil {
			t.Fatalf("AWCP-8 must create dedicated identity: %v", err)
		}
		if ptr.Deref(identity.AutomountServiceAccountToken, true) || metav1.GetControllerOf(&identity) == nil || metav1.GetControllerOf(&identity).UID != alpha.UID {
			t.Fatal("dedicated ServiceAccount identity contract wrong")
		}
		eventually(t, "AWCP-7 Service contract", func() error {
			var service corev1.Service
			if err := api.Get(ctx, keyFor(alpha), &service); err != nil {
				return err
			}
			if service.Spec.Type != corev1.ServiceTypeClusterIP || !apiequality.Semantic.DeepEqual(service.Spec.Selector, resource.SelectorLabels(alpha)) || len(service.Spec.Ports) != 1 || service.Spec.Ports[0].Name != resource.HTTPPortName || service.Spec.Ports[0].Port != 80 || service.Spec.Ports[0].TargetPort != intstr.FromString(resource.HTTPPortName) {
				return errors.New("Service managed fields have not converged")
			}
			return nil
		})
		eventually(t, "Service endpoint status", func() error {
			var actual platform.AIWorkload
			if err := api.Get(ctx, client.ObjectKeyFromObject(alpha), &actual); err != nil {
				return err
			}
			if actual.Status.Endpoint != resource.ChildName(alpha.Name)+".workloads.svc:80" {
				return errors.New("endpoint has not converged")
			}
			return nil
		})
	})
	t.Run("intentional failure fixtures are valid CRs and preserve requested intent", func(t *testing.T) {
		for _, fixture := range []struct {
			file, image, cpu, memory string
		}{
			{"missing-image.yaml", "example.invalid/awcp/does-not-exist:v1", "", ""},
			{"insufficient-resources.yaml", "example.invalid/awcp/demo:v1", "1000000", "1Pi"},
		} {
			t.Run(fixture.file, func(t *testing.T) {
				data, err := os.ReadFile(filepath.Join("..", "fixtures", "deployment", fixture.file))
				if err != nil {
					t.Fatal(err)
				}
				var workload platform.AIWorkload
				if err := yaml.UnmarshalStrict(data, &workload); err != nil {
					t.Fatal(err)
				}
				// Fixture namespace documents the installation default; this isolated
				// manager intentionally watches the test namespace only.
				workload.Namespace = "workloads"
				if err := api.Create(ctx, &workload); err != nil {
					t.Fatalf("fixture must pass the CRD API: %v", err)
				}
				d := converged(t, &workload)
				app := d.Spec.Template.Spec.Containers[0]
				if app.Image != fixture.image {
					t.Fatalf("image fallback: %q", app.Image)
				}
				if fixture.cpu != "" && (app.Resources.Requests.Cpu().Cmp(quantity.MustParse(fixture.cpu)) != 0 || app.Resources.Requests.Memory().Cmp(quantity.MustParse(fixture.memory)) != 0) {
					t.Fatal("resource fixture intent was not preserved")
				}
			})
		}
	})
	t.Run("no-op preserves resourceVersion and template", func(t *testing.T) {
		engine := controller.Engine{Client: api, Scheme: scheme}
		plan, err := (resource.WorkloadBuilder{}).Build(alpha)
		if err != nil {
			t.Fatal(err)
		}
		for range 5 {
			outcome, err := engine.Apply(ctx, alpha, deploymentIntent(t, plan))
			if err != nil || outcome != controller.Unchanged {
				t.Fatalf("expected API-default-safe no-op: %s %v", outcome, err)
			}
		}
		time.Sleep(300 * time.Millisecond)
		var actual appsv1.Deployment
		if err := api.Get(ctx, keyFor(alpha), &actual); err != nil {
			t.Fatal(err)
		}
		if actual.ResourceVersion != current.ResourceVersion {
			t.Fatal("unchanged reconcile wrote Deployment")
		}
	})
	t.Run("Service port drift toggle and endpoint lifecycle", func(t *testing.T) {
		var service corev1.Service
		eventually(t, "initial Service allocation", func() error {
			if err := api.Get(ctx, keyFor(alpha), &service); err != nil {
				return err
			}
			if service.Spec.ClusterIP == "" || service.Spec.ClusterIP == corev1.ClusterIPNone {
				return errors.New("cluster IP not allocated")
			}
			return nil
		})
		allocatedIP := service.Spec.ClusterIP
		service.Spec.Selector = map[string]string{"drift": "true"}
		service.Spec.Ports = []corev1.ServicePort{{Name: "wrong", Protocol: corev1.ProtocolUDP, Port: 1234, TargetPort: intstr.FromInt(1234)}}
		if err := api.Update(ctx, &service); err != nil {
			t.Fatal(err)
		}
		eventually(t, "Service drift repair", func() error {
			if err := api.Get(ctx, keyFor(alpha), &service); err != nil {
				return err
			}
			if service.Spec.ClusterIP != allocatedIP || !apiequality.Semantic.DeepEqual(service.Spec.Selector, resource.SelectorLabels(alpha)) || len(service.Spec.Ports) != 1 || service.Spec.Ports[0].Port != 80 || service.Spec.Ports[0].TargetPort != intstr.FromString(resource.HTTPPortName) {
				return errors.New("Service drift has not converged")
			}
			return nil
		})
		current = update(t, alpha, func(p *platform.AIWorkload) { p.Spec.Service = &platform.ServiceSpec{Port: ptr.To(int32(8081))} })
		eventually(t, "Service port and endpoint update", func() error {
			if err := api.Get(ctx, keyFor(alpha), &service); err != nil {
				return err
			}
			var workload platform.AIWorkload
			if err := api.Get(ctx, client.ObjectKeyFromObject(alpha), &workload); err != nil {
				return err
			}
			if service.Spec.ClusterIP != allocatedIP || len(service.Spec.Ports) != 1 || service.Spec.Ports[0].Port != 8081 || workload.Status.Endpoint != resource.ChildName(alpha.Name)+".workloads.svc:8081" {
				return errors.New("Service update or endpoint has not converged")
			}
			return nil
		})
		current = update(t, alpha, func(p *platform.AIWorkload) { p.Spec.Service = &platform.ServiceSpec{Enabled: ptr.To(false)} })
		eventually(t, "disabled Service is removed and endpoint cleared", func() error {
			if err := api.Get(ctx, keyFor(alpha), &corev1.Service{}); !apierrors.IsNotFound(err) {
				return errors.New("disabled Service still exists")
			}
			var workload platform.AIWorkload
			if err := api.Get(ctx, client.ObjectKeyFromObject(alpha), &workload); err != nil {
				return err
			}
			if workload.Status.Endpoint != "" {
				return errors.New("disabled Service left stale endpoint")
			}
			return nil
		})
		current = update(t, alpha, func(p *platform.AIWorkload) {
			p.Spec.Service = &platform.ServiceSpec{Enabled: ptr.To(true), Port: ptr.To(int32(8082))}
		})
		eventually(t, "re-enabled Service and endpoint", func() error {
			if err := api.Get(ctx, keyFor(alpha), &service); err != nil {
				return err
			}
			var workload platform.AIWorkload
			if err := api.Get(ctx, client.ObjectKeyFromObject(alpha), &workload); err != nil {
				return err
			}
			if len(service.Spec.Ports) != 1 || service.Spec.Ports[0].Port != 8082 || workload.Status.Endpoint != resource.ChildName(alpha.Name)+".workloads.svc:8082" {
				return errors.New("re-enabled Service has not converged")
			}
			return nil
		})
	})
	t.Run("Secret missing restore delete and restore update only safe status", func(t *testing.T) {
		workload := &platform.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: "secret-lifecycle", Namespace: "workloads"}, Spec: platform.AIWorkloadSpec{
			Image: "example.invalid/secret-lifecycle:v1", Container: platform.ContainerSpec{Port: 8080}, SecretRefs: []platform.SecretReference{"restore-me"},
		}}
		if err := api.Create(ctx, workload); err != nil {
			t.Fatal(err)
		}
		key := client.ObjectKeyFromObject(workload)
		generation := workload.Generation
		assertMissing := func() {
			t.Helper()
			eventually(t, "missing Secret condition", func() error {
				var actual platform.AIWorkload
				if err := api.Get(ctx, key, &actual); err != nil {
					return err
				}
				condition := meta.FindStatusCondition(actual.Status.Conditions, "Degraded")
				ready := meta.FindStatusCondition(actual.Status.Conditions, "Ready")
				if condition == nil || condition.Status != metav1.ConditionTrue || condition.Reason != "SecretNotFound" || ready == nil || ready.Status != metav1.ConditionFalse || actual.Generation != generation {
					return errors.New("missing Secret condition has not converged")
				}
				return nil
			})
		}
		assertMissing()
		var account corev1.ServiceAccount
		if err := api.Get(ctx, keyFor(workload), &account); err != nil || ptr.Deref(account.AutomountServiceAccountToken, true) {
			t.Fatalf("identity must exist even while Secret is missing: %v", err)
		}
		const sentinel = "AWCP-8-SENTINEL-MUST-NOT-LEAK"
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "restore-me", Namespace: workload.Namespace}, Data: map[string][]byte{"token": []byte(sentinel)}}
		if err := api.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secret.Name, Namespace: secret.Namespace}, Data: secret.Data}); err != nil {
			t.Fatal(err)
		}
		assertRecovered := func() {
			t.Helper()
			eventually(t, "Secret recovery condition", func() error {
				var actual platform.AIWorkload
				if err := api.Get(ctx, key, &actual); err != nil {
					return err
				}
				degraded := meta.FindStatusCondition(actual.Status.Conditions, "Degraded")
				ready := meta.FindStatusCondition(actual.Status.Conditions, "Ready")
				serialized, err := json.Marshal(actual.Status)
				if err != nil {
					return err
				}
				if degraded == nil || degraded.Status != metav1.ConditionFalse || ready == nil || ready.Status != metav1.ConditionUnknown || strings.Contains(string(serialized), sentinel) || actual.Generation != generation {
					return errors.New("Secret recovery or status redaction has not converged")
				}
				return nil
			})
		}
		assertRecovered()
		if err := api.Delete(ctx, secret); err != nil {
			t.Fatal(err)
		}
		assertMissing()
		if err := api.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secret.Name, Namespace: secret.Namespace}, Data: secret.Data}); err != nil {
			t.Fatal(err)
		}
		assertRecovered()
	})
	t.Run("replicas 1 to 2 to 0 does not change template", func(t *testing.T) {
		template := current.Spec.Template.DeepCopy()
		selector := current.Spec.Selector.DeepCopy()
		for _, n := range []int32{1, 2, 0} {
			current = update(t, alpha, func(p *platform.AIWorkload) { p.Spec.Replicas = ptr.To(n) })
			if *current.Spec.Replicas != n || !apiequality.Semantic.DeepEqual(template, &current.Spec.Template) || !apiequality.Semantic.DeepEqual(selector, current.Spec.Selector) {
				t.Fatal("scale changed template or selector")
			}
		}
	})
	t.Run("image resources probes port and Secret changes carry rollout intent", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			change func(*platform.AIWorkload)
		}{
			{"image", func(p *platform.AIWorkload) { p.Spec.Image = "example.invalid/demo:v2" }},
			{"resources", func(p *platform.AIWorkload) {
				p.Spec.Resources = &platform.ResourceRequirements{Requests: platform.ResourceQuantities{"cpu": "100m", "memory": "64Mi"}, Limits: platform.ResourceQuantities{"cpu": "500m", "memory": "256Mi"}}
			}},
			{"port", func(p *platform.AIWorkload) { p.Spec.Container.Port = 9090 }},
			{"probes", func(p *platform.AIWorkload) {
				p.Spec.Health.Readiness.Path = "/ready/v2"
				p.Spec.Health.Liveness.Path = "/health/v2"
			}},
			{"secret order", func(p *platform.AIWorkload) { p.Spec.SecretRefs = []platform.SecretReference{"second", "first"} }},
			{"remove settings", func(p *platform.AIWorkload) { p.Spec.Resources = nil; p.Spec.Health = nil; p.Spec.SecretRefs = nil }},
		} {
			t.Run(tc.name, func(t *testing.T) {
				before := current
				current = update(t, alpha, tc.change)
				if current.UID != before.UID || current.Generation <= before.Generation || apiequality.Semantic.DeepEqual(before.Spec.Template, current.Spec.Template) || !apiequality.Semantic.DeepEqual(before.Spec.Selector, current.Spec.Selector) {
					t.Fatal("template rollout intent or stable identity wrong")
				}
			})
		}
		app := current.Spec.Template.Spec.Containers[0]
		if len(app.EnvFrom) != 0 || app.ReadinessProbe != nil || app.LivenessProbe != nil || len(app.Resources.Limits) != 0 || len(app.Resources.Requests) != 0 {
			t.Fatal("removed settings remained")
		}
	})
	t.Run("preserves unrelated metadata sidecars and API fields", func(t *testing.T) {
		if err := api.Get(ctx, keyFor(alpha), current); err != nil {
			t.Fatal(err)
		}
		current.Annotations["user.example/keep"] = "yes"
		current.Spec.Template.Annotations["user.example/pod"] = "yes"
		current.Spec.Template.Spec.Containers = append([]corev1.Container{{Name: "injected", Image: "example.invalid/sidecar:v1"}}, current.Spec.Template.Spec.Containers...)
		current.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: "injected", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}}
		if err := api.Update(ctx, current); err != nil {
			t.Fatal(err)
		}
		injected := current.Spec.Template.DeepCopy()
		rv := current.ResourceVersion
		time.Sleep(300 * time.Millisecond)
		current = converged(t, alpha)
		if current.ResourceVersion != rv || !apiequality.Semantic.DeepEqual(injected, &current.Spec.Template) {
			t.Fatal("unmanaged injection triggered controller write")
		}
		current = update(t, alpha, func(p *platform.AIWorkload) { p.Spec.Image = "example.invalid/demo:v3" })
		if current.Annotations["user.example/keep"] != "yes" || current.Spec.Template.Annotations["user.example/pod"] != "yes" || len(current.Spec.Template.Spec.Volumes) != 1 || current.Spec.Template.Spec.Containers[0].Image != "example.invalid/sidecar:v1" || current.Spec.Template.Spec.Containers[1].Image != alpha.Spec.Image {
			t.Fatal("unmanaged fields overwritten or wrong container updated")
		}
	})
	t.Run("immutable selector is visible bounded and never recreated", func(t *testing.T) {
		eventually(t, "immutable conflict condition", func() error {
			if err := api.Get(ctx, client.ObjectKeyFromObject(collision), collision); err != nil {
				return err
			}
			c := meta.FindStatusCondition(collision.Status.Conditions, "Degraded")
			if c == nil || c.Status != metav1.ConditionTrue || c.Reason != "ResourceOwnershipConflict" || c.ObservedGeneration != collision.Generation {
				return errors.New("immutable failure not reported")
			}
			return nil
		})
		if err := api.Get(ctx, keyFor(collision), legacy); err != nil {
			t.Fatal(err)
		}
		if !apiequality.Semantic.DeepEqual(legacy, legacyBefore) {
			t.Fatal("incompatible Deployment was modified/recreated")
		}
		engine := controller.Engine{Client: api, Scheme: scheme}
		plan, err := (resource.WorkloadBuilder{}).Build(collision)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Apply(ctx, collision, deploymentIntent(t, plan)); !errors.Is(err, resource.ErrImmutableSelector) {
			t.Fatalf("wrong immutable error: %v", err)
		}
		var actualOther appsv1.Deployment
		if err := api.Get(ctx, keyFor(beta), &actualOther); err != nil {
			t.Fatal(err)
		}
		if actualOther.ResourceVersion != other.ResourceVersion || actualOther.UID != other.UID {
			t.Fatal("unrelated workload changed")
		}
	})
	t.Run("deleted production Deployment is recreated", func(t *testing.T) {
		oldUID := current.UID
		if err := api.Delete(ctx, current); err != nil {
			t.Fatal(err)
		}
		eventually(t, "new Deployment UID", func() error {
			var d appsv1.Deployment
			if err := api.Get(ctx, keyFor(alpha), &d); err != nil {
				return err
			}
			if d.UID == oldUID {
				return errors.New("old Deployment still present")
			}
			return nil
		})
		current = converged(t, alpha)
		if len(current.Spec.Template.Spec.Containers) != 1 {
			t.Fatal("recreation should use API intent, not deleted injected fields")
		}
	})
	t.Run("unavailable image is accepted without false readiness or image fallback", func(t *testing.T) {
		current = update(t, alpha, func(p *platform.AIWorkload) {
			p.Spec.Replicas = ptr.To(int32(1))
			p.Spec.Image = "example.invalid/does-not-exist:v1"
		})
		if current.Spec.Template.Spec.Containers[0].Image != alpha.Spec.Image {
			t.Fatal("image silently replaced")
		}
		if err := api.Get(ctx, client.ObjectKeyFromObject(alpha), alpha); err != nil {
			t.Fatal(err)
		}
		if meta.IsStatusConditionTrue(alpha.Status.Conditions, "Ready") {
			t.Fatal("spec reconciliation is not application readiness")
		}
		if current.Spec.Strategy.RollingUpdate.MaxSurge == nil || *current.Spec.Strategy.RollingUpdate.MaxSurge != intstr.FromInt32(1) {
			t.Fatal("rollout strategy lost")
		}
		t.Log("envtest has no kubelet; ImagePullBackOff/CrashLoop/HTTP readiness and real rollout are NOT proven here")
	})
	t.Logf("production builder validated for %s/%s", alpha.Namespace, alpha.Name)
}
