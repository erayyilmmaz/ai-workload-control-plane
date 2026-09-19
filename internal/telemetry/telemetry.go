// SPDX-License-Identifier: Apache-2.0
// Package telemetry owns bounded, controller-wide Prometheus signals.
package telemetry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	platformv1alpha1 "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
)

// Recorder is deliberately small so telemetry never becomes a reconciliation dependency.
type Recorder interface {
	RecordResource(kind, operation string)
	RecordFailure(reason string)
}

// Metrics contains the AWCP-owned subset of the Prometheus registry. Reconcile
// duration/outcome/error remain controller-runtime built-ins to avoid duplication.
type Metrics struct {
	resources *prometheus.CounterVec
	failures  *prometheus.CounterVec
	active    prometheus.Gauge
	ready     prometheus.Gauge
	degraded  prometheus.Gauge
}

func New(registerer prometheus.Registerer) (*Metrics, error) {
	m := &Metrics{
		resources: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "awcp", Subsystem: "controller", Name: "resource_reconciliations_total",
			Help: "Number of AIWorkload child resource reconciliation outcomes.",
		}, []string{"kind", "operation"}),
		failures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "awcp", Subsystem: "controller", Name: "reconcile_failures_total",
			Help: "Number of failed reconciliations by fixed safe reason.",
		}, []string{"reason"}),
		active: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "awcp", Subsystem: "controller", Name: "managed_workloads", Help: "Current AIWorkloads across AWCP's explicitly watched namespaces.",
		}),
		ready: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "awcp", Subsystem: "controller", Name: "ready_workloads", Help: "Current AIWorkloads with Ready=True.",
		}),
		degraded: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "awcp", Subsystem: "controller", Name: "degraded_workloads", Help: "Current AIWorkloads with Degraded=True.",
		}),
	}
	for _, collector := range []prometheus.Collector{m.resources, m.failures, m.active, m.ready, m.degraded} {
		if err := registerer.Register(collector); err != nil {
			return nil, fmt.Errorf("register AWCP metric: %w", err)
		}
	}
	return m, nil
}

func (m *Metrics) RecordResource(kind, operation string) {
	m.resources.WithLabelValues(kind, operation).Inc()
}
func (m *Metrics) RecordFailure(reason string) { m.failures.WithLabelValues(reason).Inc() }

// Refresh derives gauges from cache state, so restart and deletion never depend on
// blind increment/decrement bookkeeping.
func (m *Metrics) Refresh(ctx context.Context, reader client.Reader, namespace string) error {
	return m.RefreshNamespaces(ctx, reader, []string{namespace})
}

// RefreshNamespaces aggregates only the manager's fixed cache namespaces. It
// intentionally adds no tenant metric label, avoiding unbounded cardinality.
func (m *Metrics) RefreshNamespaces(ctx context.Context, reader client.Reader, namespaces []string) error {
	ready, degraded := 0, 0
	active := 0
	for _, namespace := range namespaces {
		var workloads platformv1alpha1.AIWorkloadList
		if err := reader.List(ctx, &workloads, client.InNamespace(namespace)); err != nil {
			return err
		}
		active += len(workloads.Items)
		for i := range workloads.Items {
			if meta.IsStatusConditionTrue(workloads.Items[i].Status.Conditions, "Ready") {
				ready++
			}
			if meta.IsStatusConditionTrue(workloads.Items[i].Status.Conditions, "Degraded") {
				degraded++
			}
		}
	}
	m.active.Set(float64(active))
	m.ready.Set(float64(ready))
	m.degraded.Set(float64(degraded))
	return nil
}

// GaugeRefresher observes cache state out of band; its failure is logged and never
// returned to or blocks the controller's core reconciliation path.
type GaugeRefresher struct {
	Reader     client.Reader
	Namespace  string
	Namespaces []string
	Metrics    *Metrics
	Interval   time.Duration
}

func (r *GaugeRefresher) NeedLeaderElection() bool { return true }

func (r *GaugeRefresher) Start(ctx context.Context) error {
	interval := r.Interval
	if interval <= 0 {
		interval = 15 * time.Second
	}
	refresh := func() {
		namespaces := r.Namespaces
		if len(namespaces) == 0 {
			namespaces = []string{r.Namespace}
		}
		if err := r.Metrics.RefreshNamespaces(ctx, r.Reader, namespaces); err != nil {
			log.FromContext(ctx).Error(err, "Could not refresh workload telemetry gauges")
		}
	}
	refresh()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			refresh()
		}
	}
}

var (
	defaultOnce    sync.Once
	defaultMetrics *Metrics
)

// Default registers exactly once in controller-runtime's global registry.
func Default() *Metrics {
	defaultOnce.Do(func() {
		var err error
		defaultMetrics, err = New(ctrlmetrics.Registry)
		if err != nil {
			panic(err)
		}
	})
	return defaultMetrics
}
