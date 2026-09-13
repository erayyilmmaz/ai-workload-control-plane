// SPDX-License-Identifier: Apache-2.0
package reconciliation

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/manager"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/resource"
	"github.com/erayyilmmaz/ai-workload-control-plane/test/fixtures"
)

// The wrapper observes queue activity, not application readiness or Secret contents.
type observedBuilder struct {
	mu    sync.Mutex
	calls map[string]int
	fail  map[string]bool
}

func (b *observedBuilder) Build(p *platform.AIWorkload) ([]resource.Intent, error) {
	b.mu.Lock()
	b.calls[p.Name]++
	fail := b.fail[p.Name]
	b.mu.Unlock()
	if fail {
		return nil, errors.New("injected unexpected failure")
	}
	return fixtures.Plan(p)
}
func (b *observedBuilder) count(name string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls[name]
}
func (b *observedBuilder) failing(name string, value bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.fail[name] = value
}

func eventually(t *testing.T, description string, check func() error) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	var err error
	for time.Now().Before(deadline) {
		if err = check(); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s: %v", description, err)
}

func consistently(t *testing.T, description string, check func() error) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if err := check(); err != nil {
			t.Fatalf("%s: %v", description, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestReconciliationWithRealAPI(t *testing.T) {
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
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := platform.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	api, err := client.New(cfg, client.Options{Scheme: s})
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	for _, ns := range []string{"workloads", "system"} {
		if err := api.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}); err != nil {
			t.Fatal(err)
		}
	}
	b := &observedBuilder{calls: map[string]int{}, fail: map[string]bool{}}
	starts := 0
	start := func() func() {
		starts++
		mgr, err := manager.New(cfg, manager.Options{WatchNamespace: "workloads", ManagerNamespace: "system", ProbeAddress: "0", Builder: b, ControllerName: fmt.Sprintf("reconciliation-%d", starts)})
		if err != nil {
			t.Fatal(err)
		}
		runCtx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- mgr.Start(runCtx) }()
		stopped := false
		stop := func() {
			if stopped {
				return
			}
			stopped = true
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Error(err)
				}
			case <-time.After(15 * time.Second):
				t.Error("manager failed to stop")
			}
		}
		t.Cleanup(stop)
		return stop
	}
	stop := start()
	create := func(name string) *platform.AIWorkload {
		t.Helper()
		p := &platform.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "workloads"}, Spec: platform.AIWorkloadSpec{Image: "example.invalid/app:v1", Container: platform.ContainerSpec{Port: 8080}}}
		if name == "alpha" {
			p.Spec.SecretRefs = []platform.SecretReference{"referenced"}
		}
		if err := api.Create(ctx, p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	alpha, beta := create("alpha"), create("beta")
	objects := func(p *platform.AIWorkload) []client.Object {
		plan, err := fixtures.Plan(p)
		if err != nil {
			t.Fatal(err)
		}
		result := make([]client.Object, 0, len(plan))
		for _, i := range plan {
			result = append(result, i.Object)
		}
		return result
	}
	children := objects(alpha)
	otherChildren := objects(beta)
	ready := func(children []client.Object) {
		t.Helper()
		eventually(t, "all four children created", func() error {
			for _, o := range children {
				if err := api.Get(ctx, client.ObjectKeyFromObject(o), o); err != nil {
					return err
				}
				if o.GetLabels()["test.awcp/managed"] != "true" {
					return fmt.Errorf("%T not restored", o)
				}
			}
			return nil
		})
	}
	ready(children)
	ready(otherChildren)
	snapshots := func(children []client.Object) map[string]string {
		result := map[string]string{}
		for _, o := range children {
			if err := api.Get(ctx, client.ObjectKeyFromObject(o), o); err != nil {
				t.Fatal(err)
			}
			result[fmt.Sprintf("%T", o)] = string(o.GetUID()) + "/" + o.GetResourceVersion()
		}
		return result
	}
	assertSnapshot := func(children []client.Object, want map[string]string) {
		t.Helper()
		got := snapshots(children)
		for key, value := range want {
			if got[key] != value {
				t.Fatalf("unexpected child mutation %s: %s -> %s", key, value, got[key])
			}
		}
	}
	// Wait for initial child events; stable API RVs prove defaulting does not cause patches.
	time.Sleep(300 * time.Millisecond)
	initial := snapshots(children)
	otherInitial := snapshots(otherChildren)
	baseCount := b.count(alpha.Name)
	time.Sleep(500 * time.Millisecond)
	assertSnapshot(children, initial)
	if b.count(alpha.Name) != baseCount {
		t.Fatal("reconciliation did not settle")
	}

	t.Run("all four watches repair drift and deletion only for their parent", func(t *testing.T) {
		for _, o := range children {
			t.Run(fmt.Sprintf("%T", o), func(t *testing.T) {
				if err := api.Get(ctx, client.ObjectKeyFromObject(o), o); err != nil {
					t.Fatal(err)
				}
				before := o.DeepCopyObject().(client.Object)
				labels := o.GetLabels()
				labels["test.awcp/managed"] = "drift"
				labels["user.example/keep"] = "yes"
				o.SetLabels(labels)
				if err := api.Patch(ctx, o, client.MergeFrom(before)); err != nil {
					t.Fatal(err)
				}
				eventually(t, "managed metadata repaired", func() error {
					if err := api.Get(ctx, client.ObjectKeyFromObject(o), o); err != nil {
						return err
					}
					if o.GetLabels()["test.awcp/managed"] != "true" || o.GetLabels()["user.example/keep"] != "yes" {
						return errors.New("managed or user label wrong")
					}
					return nil
				})
				uid := o.GetUID()
				if err := api.Delete(ctx, o); err != nil {
					t.Fatal(err)
				}
				eventually(t, "owned child recreated", func() error {
					if err := api.Get(ctx, client.ObjectKeyFromObject(o), o); err != nil {
						return err
					}
					if o.GetUID() == uid {
						return errors.New("old UID still exists")
					}
					return nil
				})
				owner := metav1.GetControllerOf(o)
				if owner == nil || owner.UID != alpha.UID || ptr.Deref(owner.BlockOwnerDeletion, true) {
					t.Fatal("incorrect parent ownership")
				}
				assertSnapshot(otherChildren, otherInitial)
			})
		}
	})
	t.Run("server defaults allocated IPs and injected sidecars survive no-op", func(t *testing.T) {
		key := client.ObjectKey{Namespace: alpha.Namespace, Name: resource.ChildName(alpha.Name)}
		var d appsv1.Deployment
		var svc corev1.Service
		if err := api.Get(ctx, key, &d); err != nil {
			t.Fatal(err)
		}
		d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, corev1.Container{Name: "injected", Image: "example.invalid/sidecar:v1"})
		if err := api.Update(ctx, &d); err != nil {
			t.Fatal(err)
		}
		if err := api.Get(ctx, key, &svc); err != nil {
			t.Fatal(err)
		}
		if svc.Spec.ClusterIP == "" {
			t.Fatal("API did not allocate ClusterIP")
		}
		ip := svc.Spec.ClusterIP
		count := b.count(alpha.Name)
		svc.Annotations = map[string]string{"user.example/note": "preserve"}
		if err := api.Update(ctx, &svc); err != nil {
			t.Fatal(err)
		}
		eventually(t, "service update enqueues", func() error {
			if b.count(alpha.Name) <= count {
				return errors.New("watch not delivered")
			}
			return nil
		})
		time.Sleep(200 * time.Millisecond)
		stable := snapshots(children)
		time.Sleep(300 * time.Millisecond)
		assertSnapshot(children, stable)
		if err := api.Get(ctx, key, &d); err != nil {
			t.Fatal(err)
		}
		if len(d.Spec.Template.Spec.Containers) != 2 || d.Spec.Strategy.Type == "" || d.Spec.Template.Spec.DNSPolicy == "" {
			t.Fatal("defaults or sidecar lost")
		}
		if err := api.Get(ctx, key, &svc); err != nil {
			t.Fatal(err)
		}
		if svc.Spec.ClusterIP != ip || svc.Annotations["user.example/note"] != "preserve" {
			t.Fatal("allocated IP or user metadata lost")
		}
		// Actual spec drift, not only metadata: replicas must return to desired value.
		d.Spec.Replicas = ptr.To(int32(7))
		if err := api.Update(ctx, &d); err != nil {
			t.Fatal(err)
		}
		eventually(t, "deployment spec drift repaired", func() error {
			if err := api.Get(ctx, key, &d); err != nil {
				return err
			}
			if ptr.Deref(d.Spec.Replicas, 0) != 1 {
				return errors.New("replicas not repaired")
			}
			return nil
		})
	})
	t.Run("deployment status and Secret deletion enqueue without parent edit", func(t *testing.T) {
		key := client.ObjectKey{Namespace: alpha.Namespace, Name: resource.ChildName(alpha.Name)}
		var d appsv1.Deployment
		if err := api.Get(ctx, key, &d); err != nil {
			t.Fatal(err)
		}
		time.Sleep(200 * time.Millisecond)
		count := b.count(alpha.Name)
		otherCount := b.count(beta.Name)
		d.Status.ReadyReplicas = 1
		d.Status.Replicas = 1
		if err := api.Status().Update(ctx, &d); err != nil {
			t.Fatal(err)
		}
		eventually(t, "deployment status watch", func() error {
			if b.count(alpha.Name) <= count {
				return errors.New("status event lost")
			}
			return nil
		})
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "referenced", Namespace: alpha.Namespace}, Data: map[string][]byte{"test": []byte("synthetic-not-a-real-secret")}}
		count = b.count(alpha.Name)
		if err := api.Create(ctx, secret); err != nil {
			t.Fatal(err)
		}
		eventually(t, "Secret create watch", func() error {
			if b.count(alpha.Name) <= count {
				return errors.New("Secret create not mapped")
			}
			return nil
		})
		count = b.count(alpha.Name)
		if err := api.Delete(ctx, secret); err != nil {
			t.Fatal(err)
		}
		eventually(t, "Secret delete watch", func() error {
			if b.count(alpha.Name) <= count {
				return errors.New("Secret deletion not mapped")
			}
			return nil
		})
		if b.count(beta.Name) != otherCount {
			t.Fatal("unrelated parent enqueued")
		}
		count = b.count(alpha.Name)
		for attempt := 0; attempt < 5; attempt++ {
			var statusOnly platform.AIWorkload
			if err := api.Get(ctx, client.ObjectKeyFromObject(alpha), &statusOnly); err != nil {
				t.Fatal(err)
			}
			statusOnly.Status.ReadyReplicas = 9
			if err := api.Status().Update(ctx, &statusOnly); err == nil {
				break
			} else if !apierrors.IsConflict(err) {
				t.Fatal(err)
			} else if attempt == 4 {
				t.Fatal("controller status patch kept conflicting with status-only test update")
			}
		}
		time.Sleep(300 * time.Millisecond)
		if b.count(alpha.Name) != count {
			t.Fatal("parent status update enqueued itself")
		}
	})
	t.Run("ownership conflict visible and unrelated errors isolated", func(t *testing.T) {
		name := "collision"
		foreign := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: resource.ChildName(name), Namespace: "workloads"}, Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 80}}}}
		if err := api.Create(ctx, foreign); err != nil {
			t.Fatal(err)
		}
		rv := foreign.ResourceVersion
		p := create(name)
		eventually(t, "foreign ownership condition", func() error {
			if err := api.Get(ctx, client.ObjectKeyFromObject(p), p); err != nil {
				return err
			}
			c := meta.FindStatusCondition(p.Status.Conditions, "Degraded")
			if c == nil || c.Reason != "ResourceOwnershipConflict" {
				return errors.New("condition missing")
			}
			return nil
		})
		if err := api.Get(ctx, client.ObjectKeyFromObject(foreign), foreign); err != nil {
			t.Fatal(err)
		}
		if foreign.ResourceVersion != rv || len(foreign.OwnerReferences) != 0 {
			t.Fatal("foreign resource adopted or modified")
		}
		eventually(t, "warning event", func() error {
			var list eventsv1.EventList
			if err := api.List(ctx, &list, client.InNamespace("workloads")); err != nil {
				return err
			}
			for _, e := range list.Items {
				if e.Regarding.UID == p.UID && e.Reason == "ResourceOwnershipConflict" {
					return nil
				}
			}
			return errors.New("warning Event missing")
		})
		time.Sleep(200 * time.Millisecond)
		count := b.count(name)
		time.Sleep(300 * time.Millisecond)
		if b.count(name) != count {
			t.Fatal("ownership conflict hot-loop")
		}
		b.failing("broken", true)
		broken := create("broken")
		healthy := create("healthy")
		ready(objects(healthy))
		eventually(t, "unexpected failure condition", func() error {
			if err := api.Get(ctx, client.ObjectKeyFromObject(broken), broken); err != nil {
				return err
			}
			if !meta.IsStatusConditionTrue(broken.Status.Conditions, "Degraded") {
				return errors.New("failure not reported")
			}
			return nil
		})
		b.failing("broken", false)
		ready(objects(broken))
		// Explicit test-admin resolution is not automatic adoption by the controller.
		if err := api.Delete(ctx, foreign); err != nil {
			t.Fatal(err)
		}
		p.Spec.Image = "example.invalid/app:v2"
		if err := api.Update(ctx, p); err != nil {
			t.Fatal(err)
		}
		ready(objects(p))
		eventually(t, "ownership recovery", func() error {
			if err := api.Get(ctx, client.ObjectKeyFromObject(p), p); err != nil {
				return err
			}
			if !meta.IsStatusConditionFalse(p.Status.Conditions, "Degraded") || meta.IsStatusConditionTrue(p.Status.Conditions, "Ready") {
				return errors.New("recovery status wrong")
			}
			return nil
		})
	})
	t.Run("deleting parent never recreates an owned child", func(t *testing.T) {
		deleting := create("deleting")
		deletingChildren := objects(deleting)
		ready(deletingChildren)
		key := client.ObjectKeyFromObject(deleting)
		for attempt := 0; attempt < 5; attempt++ {
			var current platform.AIWorkload
			if err := api.Get(ctx, key, &current); err != nil {
				t.Fatal(err)
			}
			if len(current.Finalizers) != 0 {
				t.Fatalf("AWCP must not add a finalizer: %v", current.Finalizers)
			}
			current.Finalizers = []string{"test.example/hold"}
			if err := api.Update(ctx, &current); err == nil {
				break
			} else if !apierrors.IsConflict(err) {
				t.Fatal(err)
			} else if attempt == 4 {
				t.Fatal("could not add fixture finalizer")
			}
		}
		if err := api.Delete(ctx, deleting); err != nil {
			t.Fatal(err)
		}
		eventually(t, "parent reaches deletion state", func() error {
			var current platform.AIWorkload
			if err := api.Get(ctx, key, &current); err != nil {
				return err
			}
			if current.DeletionTimestamp.IsZero() || len(current.Finalizers) != 1 || current.Finalizers[0] != "test.example/hold" {
				return errors.New("parent deletion guard is not observable")
			}
			return nil
		})
		var deployment appsv1.Deployment
		deploymentKey := client.ObjectKeyFromObject(deletingChildren[1])
		if err := api.Get(ctx, deploymentKey, &deployment); err != nil {
			t.Fatal(err)
		}
		if err := api.Delete(ctx, &deployment); err != nil {
			t.Fatal(err)
		}
		consistently(t, "deleting parent must not recreate Deployment", func() error {
			err := api.Get(ctx, deploymentKey, &appsv1.Deployment{})
			if apierrors.IsNotFound(err) {
				return nil
			}
			if err != nil {
				return err
			}
			return errors.New("Deployment was recreated while parent is deleting")
		})
		// Envtest does not run the garbage collector. Remove only the test fixture
		// finalizer so the API object can disappear; kind proves cascade cleanup.
		var current platform.AIWorkload
		if err := api.Get(ctx, key, &current); err != nil {
			t.Fatal(err)
		}
		current.Finalizers = nil
		if err := api.Update(ctx, &current); err != nil {
			t.Fatal(err)
		}
		eventually(t, "parent finalizes", func() error {
			err := api.Get(ctx, key, &platform.AIWorkload{})
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		})
	})
	t.Run("restart reconstructs from API without database", func(t *testing.T) {
		stop()
		oldUIDs := map[string]types.UID{}
		for _, o := range children {
			if err := api.Get(ctx, client.ObjectKeyFromObject(o), o); err != nil {
				t.Fatal(err)
			}
			oldUIDs[fmt.Sprintf("%T", o)] = o.GetUID()
			if err := api.Delete(ctx, o); err != nil {
				t.Fatal(err)
			}
		}
		stop = start()
		ready(children)
		for _, o := range children {
			if oldUIDs[fmt.Sprintf("%T", o)] == o.GetUID() {
				t.Fatal("child not recreated after restart")
			}
		}
		time.Sleep(300 * time.Millisecond)
		stable := snapshots(children)
		time.Sleep(300 * time.Millisecond)
		assertSnapshot(children, stable)
		assertSnapshot(otherChildren, otherInitial)
	})
	stop()
}
