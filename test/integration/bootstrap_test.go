// SPDX-License-Identifier: Apache-2.0
package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	platformv1alpha1 "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/manager"
)

func TestBootstrapIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("envtest excluded by -short; run make test for real API coverage")
	}
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Fatal("KUBEBUILDER_ASSETS is required; run make test or see docs/development.md")
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bootstrap envtest suite")
}

var _ = Describe("Bootstrap with a real API and etcd", func() {
	var api client.Client
	var mgr ctrl.Manager
	var ctx context.Context

	BeforeEach(func() {
		ctrl.SetLogger(zap.New(zap.WriteTo(GinkgoWriter)))
		environment := &envtest.Environment{
			CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
			ErrorIfCRDPathMissing: true,
		}
		cfg, err := environment.Start()
		Expect(err).NotTo(HaveOccurred())
		// Stop exactly once; retrying Stop can hide teardown failures.
		DeferCleanup(func() { Expect(environment.Stop()).To(Succeed()) })
		scheme := runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(platformv1alpha1.AddToScheme(scheme)).To(Succeed())
		api, err = client.New(cfg, client.Options{Scheme: scheme})
		Expect(err).NotTo(HaveOccurred())
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(context.Background())
		for _, ns := range []string{"awcp-system", "awcp-workloads", "outside"} {
			Expect(api.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})).To(Succeed())
		}
		mgr, err = manager.New(cfg, manager.Options{
			WatchNamespace: "awcp-workloads", ManagerNamespace: "awcp-system", ProbeAddress: "0", LeaderElection: true,
		})
		Expect(err).NotTo(HaveOccurred())
		done := make(chan error, 1)
		go func() { done <- mgr.Start(ctx) }()
		DeferCleanup(func() {
			cancel()
			Eventually(done, 15*time.Second).Should(Receive(BeNil()))
		})
		Eventually(mgr.Elected(), 20*time.Second).Should(BeClosed())
	})

	It("registers the namespaced API, syncs only its namespace and preserves status", func() {
		workload := &platformv1alpha1.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: "bootstrap", Namespace: "awcp-workloads"}}
		Expect(api.Create(ctx, workload)).To(Succeed())
		Expect(workload.UID).NotTo(BeEmpty())
		key := client.ObjectKeyFromObject(workload)
		Eventually(func() error {
			return mgr.GetCache().Get(ctx, key, &platformv1alpha1.AIWorkload{})
		}, 10*time.Second).Should(Succeed())
		workload.Status.Conditions = []metav1.Condition{{
			Type: "BootstrapTest", Status: metav1.ConditionTrue, Reason: "TestFixture",
			Message: "Synthetic schema round-trip only", LastTransitionTime: metav1.Now(),
		}}
		Expect(api.Status().Update(ctx, workload)).To(Succeed())
		Eventually(func(g Gomega) {
			var cached platformv1alpha1.AIWorkload
			g.Expect(mgr.GetCache().Get(ctx, key, &cached)).To(Succeed())
			g.Expect(cached.Status.Conditions).To(HaveLen(1))
			g.Expect(cached.Status.Conditions[0].Reason).To(Equal("TestFixture"))
		}, 10*time.Second).Should(Succeed())
		outside := &platformv1alpha1.AIWorkload{ObjectMeta: metav1.ObjectMeta{Name: "outside", Namespace: "outside"}}
		Expect(api.Create(ctx, outside)).To(Succeed())
		Expect(mgr.GetCache().Get(ctx, client.ObjectKeyFromObject(outside), &platformv1alpha1.AIWorkload{})).NotTo(Succeed())
		var list platformv1alpha1.AIWorkloadList
		Expect(mgr.GetCache().List(ctx, &list)).To(Succeed())
		Expect(list.Items).To(HaveLen(1))
		var lease coordinationv1.Lease
		Expect(api.Get(ctx, client.ObjectKey{Namespace: "awcp-system", Name: "awcp-controller.platform.example.io"}, &lease)).To(Succeed())
		Expect(lease.Spec.HolderIdentity).NotTo(BeNil())
		Expect(*lease.Spec.HolderIdentity).NotTo(BeEmpty())
	})
})
