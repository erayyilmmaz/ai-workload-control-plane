# Controller telemetry — AWCP-11

The manager serves authenticated HTTPS `/metrics` on port `8443`. The base
installation creates `awcp-controller-metrics` only; it does not install Prometheus,
Grafana or an OpenTelemetry Collector. Health (`:8081`) and readiness remain separate
from telemetry and never depend on a scraper or exporter.

## Metric catalog and labels

Controller-runtime already provides `controller_runtime_reconcile_total`,
`controller_runtime_reconcile_errors_total` and
`controller_runtime_reconcile_time_seconds`. AWCP deliberately reuses those for
reconcile rate, errors and latency rather than duplicating them.

| AWCP metric | Labels | Source |
| --- | --- | --- |
| `awcp_controller_resource_reconciliations_total` | `kind`, `operation` | Child create/patch/delete/no-op/error outcomes |
| `awcp_controller_reconcile_failures_total` | `reason` | Fixed safe failure reason |
| `awcp_controller_managed_workloads` | none | Current watched-namespace cache list |
| `awcp_controller_ready_workloads` | none | Current `Ready=True` conditions |
| `awcp_controller_degraded_workloads` | none | Current `Degraded=True` conditions |

The controller refreshes gauges from cache state after leadership and periodically
thereafter. It never uses increment/decrement bookkeeping, so restart and deletion do
not create a count drift. Metric labels must never contain a workload name, namespace,
UID, image, URL, Secret, or arbitrary error text.

## Access and optional pipelines

The manager uses controller-runtime's Kubernetes authentication/authorization filter
and a self-signed serving certificate stored in its dedicated writable `emptyDir`.
The narrowly scoped `awcp-metrics-auth` ClusterRole lets only the manager create
TokenReviews and SubjectAccessReviews. A scraper needs the separate
`/metrics` non-resource permission in
[metrics-reader-clusterrole.yaml](../examples/observability/metrics-reader-clusterrole.yaml).

[prometheus-scrape.yaml](../examples/observability/prometheus-scrape.yaml) is a
fragment for an existing Prometheus deployment. Its certificate verification bypass is
only for the base self-signed demo; provide a trusted serving certificate before
production use. The [Grafana dashboard](../examples/observability/grafana-awcp-dashboard.json)
is version-controlled and uses the canonical direct Prometheus series.

[otel-collector.yaml](../examples/observability/otel-collector.yaml) is an optional,
explicitly applied alternative scrape pipeline. Do not run it alongside direct
Prometheus scraping for the same series. It exports to its `debug` exporter by default;
an external backend is a deployment-specific choice. Traces are not created by AWCP.

## Structured logs

Controller logs include bounded reconcile context (`namespace`, `name`, generation),
child kind and operation outcome. Status/Event/error messages use fixed generic text.
Do not log full objects, Secret names/data, token values, environment dumps or arbitrary
Kubernetes API response bodies.
