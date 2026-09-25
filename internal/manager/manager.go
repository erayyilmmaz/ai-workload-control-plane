// SPDX-License-Identifier: Apache-2.0
// Package manager wires the scoped cache, controller, election and probes.
package manager

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/discovery"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	platformv1alpha1 "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/capability"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/controller"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/resource"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/telemetry"
)

// Options are explicit so out-of-cluster runs cannot silently expand scope.
type Options struct {
	// WatchNamespace is the V0 single-namespace spelling. It remains supported
	// for manifests and tests that have not opted into the explicit list.
	WatchNamespace string
	// WatchNamespaces is a bounded, explicitly configured tenant namespace list.
	// It must not be combined with WatchNamespace.
	WatchNamespaces  []string
	ManagerNamespace string
	ProbeAddress     string
	// MetricsBindAddress is disabled for library/tests when empty; cmd/main enables :8443.
	MetricsBindAddress string
	LeaderElection     bool
	// LeaseDuration, RenewDeadline and RetryPeriod make voluntary leader
	// handoff and unplanned failover timing explicit. Zero selects the
	// controller-runtime defaults (15s, 10s, 2s) for library callers.
	LeaseDuration time.Duration
	RenewDeadline time.Duration
	RetryPeriod   time.Duration
	// ControllerName defaults to aiworkload; tests use unique names for sequential managers.
	ControllerName string
	// Builder overrides the production plan in tests; nil uses WorkloadBuilder.
	Builder resource.Builder
}

// EffectiveWatchNamespaces returns the configured bounded cache scope.
func (o Options) EffectiveWatchNamespaces() []string {
	if len(o.WatchNamespaces) != 0 {
		return append([]string(nil), o.WatchNamespaces...)
	}
	return []string{o.WatchNamespace}
}

// ParseWatchNamespaces accepts a comma-separated environment value without
// silently trimming malformed configuration.
func ParseWatchNamespaces(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ",")
}

// Validate requires one or more distinct DNS-label workload namespaces and a
// single DNS-label manager namespace. A list is deliberately explicit: AWCP
// never changes its cache scope from an AIWorkload object.
func (o Options) Validate() error {
	if o.WatchNamespace != "" && len(o.WatchNamespaces) != 0 {
		return errors.New("set either WATCH_NAMESPACE or WATCH_NAMESPACES, not both")
	}
	seen := map[string]bool{}
	for _, value := range o.EffectiveWatchNamespaces() {
		if problems := validation.IsDNS1123Label(value); len(problems) != 0 {
			return fmt.Errorf("WATCH_NAMESPACES entries must be non-empty DNS-label namespaces")
		}
		if seen[value] {
			return fmt.Errorf("WATCH_NAMESPACES contains duplicate namespace %q", value)
		}
		seen[value] = true
	}
	if problems := validation.IsDNS1123Label(o.ManagerNamespace); len(problems) != 0 {
		return fmt.Errorf("MANAGER_NAMESPACE must be one non-empty DNS-label namespace")
	}
	leaseDuration, renewDeadline, retryPeriod := o.effectiveLeaderElectionTiming()
	if leaseDuration <= 0 || renewDeadline <= 0 || retryPeriod <= 0 || renewDeadline >= leaseDuration || retryPeriod >= renewDeadline {
		return errors.New("leader election timing requires 0 < retry period < renew deadline < lease duration")
	}
	return nil
}

func (o Options) effectiveLeaderElectionTiming() (time.Duration, time.Duration, time.Duration) {
	leaseDuration, renewDeadline, retryPeriod := o.LeaseDuration, o.RenewDeadline, o.RetryPeriod
	if leaseDuration == 0 {
		leaseDuration = 15 * time.Second
	}
	if renewDeadline == 0 {
		renewDeadline = 10 * time.Second
	}
	if retryPeriod == 0 {
		retryPeriod = 2 * time.Second
	}
	return leaseDuration, renewDeadline, retryPeriod
}

// New constructs the manager without starting processes or modifying the API.
func New(cfg *rest.Config, options Options) (ctrl.Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if options.Builder == nil {
		options.Builder = resource.WorkloadBuilder{}
	}
	watchNamespaces := options.EffectiveWatchNamespaces()
	cacheNamespaces := make(map[string]cache.Config, len(watchNamespaces))
	for _, namespace := range watchNamespaces {
		cacheNamespaces[namespace] = cache.Config{}
	}
	metricsAddress := options.MetricsBindAddress
	if metricsAddress == "" {
		metricsAddress = "0"
	}
	leaseDuration, renewDeadline, retryPeriod := options.effectiveLeaderElectionTiming()
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
		Cache:                         cache.Options{DefaultNamespaces: cacheNamespaces},
		Metrics:                       metricsserver.Options{BindAddress: metricsAddress, SecureServing: metricsAddress != "0", FilterProvider: filters.WithAuthenticationAndAuthorization},
		HealthProbeBindAddress:        options.ProbeAddress,
		LeaderElection:                options.LeaderElection,
		LeaderElectionID:              "awcp-controller.platform.example.io",
		LeaderElectionNamespace:       options.ManagerNamespace,
		LeaderElectionResourceLock:    "leases",
		LeaderElectionReleaseOnCancel: true,
		LeaseDuration:                 &leaseDuration,
		RenewDeadline:                 &renewDeadline,
		RetryPeriod:                   &retryPeriod,
	})
	if err != nil {
		return nil, err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}
	if err = (&controller.AIWorkloadReconciler{
		Client: mgr.GetClient(), TenantReader: mgr.GetAPIReader(), ExternalSecretsReader: mgr.GetAPIReader(), GatewayReader: mgr.GetAPIReader(), CapabilityLookup: capability.DiscoveryLookup{Discovery: discoveryClient}, WatchNamespaces: watchNamespaces, Builder: options.Builder, ControllerName: options.ControllerName, Telemetry: telemetry.Default(),
	}).SetupWithManager(mgr); err != nil {
		return nil, err
	}
	// +kubebuilder:scaffold:builder
	ready := &startupReadiness{cache: mgr.GetCache()}
	if err = mgr.Add(ready); err != nil {
		return nil, err
	}
	if err = mgr.Add(&telemetry.GaugeRefresher{Reader: mgr.GetCache(), Namespaces: watchNamespaces, Metrics: telemetry.Default()}); err != nil {
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
