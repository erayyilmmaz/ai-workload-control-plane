// SPDX-License-Identifier: Apache-2.0
// Package manager wires the scoped cache, controller, election and probes.
package manager

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	platformv1alpha1 "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/controller"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/resource"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/telemetry"
)

// Options are explicit so out-of-cluster runs cannot silently expand scope.
type Options struct {
	WatchNamespace   string
	ManagerNamespace string
	ProbeAddress     string
	// MetricsBindAddress is disabled for library/tests when empty; cmd/main enables :8443.
	MetricsBindAddress string
	LeaderElection     bool
	// ControllerName defaults to aiworkload; tests use unique names for sequential managers.
	ControllerName string
	// Builder overrides the production plan in tests; nil uses WorkloadBuilder.
	Builder resource.Builder
}

// Validate requires exactly one DNS-label namespace for each purpose.
func (o Options) Validate() error {
	for name, value := range map[string]string{
		"WATCH_NAMESPACE":   o.WatchNamespace,
		"MANAGER_NAMESPACE": o.ManagerNamespace,
	} {
		if problems := validation.IsDNS1123Label(value); len(problems) != 0 {
			return fmt.Errorf("%s must be one non-empty DNS-label namespace", name)
		}
	}
	return nil
}

// New constructs the manager without starting processes or modifying the API.
func New(cfg *rest.Config, options Options) (ctrl.Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if options.Builder == nil {
		options.Builder = resource.WorkloadBuilder{}
	}
	metricsAddress := options.MetricsBindAddress
	if metricsAddress == "" {
		metricsAddress = "0"
	}
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := platformv1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	// +kubebuilder:scaffold:scheme
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                        scheme,
		Cache:                         cache.Options{DefaultNamespaces: map[string]cache.Config{options.WatchNamespace: {}}},
		Metrics:                       metricsserver.Options{BindAddress: metricsAddress, SecureServing: metricsAddress != "0", FilterProvider: filters.WithAuthenticationAndAuthorization},
		HealthProbeBindAddress:        options.ProbeAddress,
		LeaderElection:                options.LeaderElection,
		LeaderElectionID:              "awcp-controller.platform.example.io",
		LeaderElectionNamespace:       options.ManagerNamespace,
		LeaderElectionResourceLock:    "leases",
		LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		return nil, err
	}
	if err = (&controller.AIWorkloadReconciler{
		Client: mgr.GetClient(), WatchNamespace: options.WatchNamespace, Builder: options.Builder, ControllerName: options.ControllerName, Telemetry: telemetry.Default(),
	}).SetupWithManager(mgr); err != nil {
		return nil, err
	}
	// +kubebuilder:scaffold:builder
	ready := &startupReadiness{cache: mgr.GetCache()}
	if err = mgr.Add(ready); err != nil {
		return nil, err
	}
	if err = mgr.Add(&telemetry.GaugeRefresher{Reader: mgr.GetCache(), Namespace: options.WatchNamespace, Metrics: telemetry.Default()}); err != nil {
		return nil, err
	}
	if err = mgr.AddHealthzCheck("process", healthz.Ping); err != nil {
		return nil, err
	}
	if err = mgr.AddReadyzCheck("cache-and-leader", ready.check); err != nil {
		return nil, err
	}
	return mgr, nil
}

type startupReadiness struct {
	cache cache.Cache
	ready atomic.Bool
}

// The manager starts this only after leadership is acquired (or explicitly disabled).
func (*startupReadiness) NeedLeaderElection() bool { return true }

func (r *startupReadiness) Start(ctx context.Context) error {
	if !r.cache.WaitForCacheSync(ctx) {
		return errors.New("cache did not synchronize")
	}
	r.ready.Store(true)
	<-ctx.Done()
	r.ready.Store(false)
	return nil
}

func (r *startupReadiness) check(_ *http.Request) error {
	if !r.ready.Load() {
		return errors.New("manager startup is not complete")
	}
	return nil
}
