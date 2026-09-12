# ADR-006 — Observability Model

Status: Accepted design. Date: 2026-09-12. Tracking: AWCP-2.

## Context

Developers need both a per-workload explanation and controller-wide signals without making telemetry a prerequisite for reconciliation.

## Decision

Expose standard metav1.Conditions and transition Events; retain structured logs; inventory controller-runtime metrics before adding custom collectors. Canonical demo path is /metrics → Prometheus → Grafana. Include an OpenTelemetry Collector example as an alternate optional-to-run pipeline; traces are deferred.

## Consequences and boundaries

Only bounded metric dimensions such as controller, kind, operation, result and fixed reason codes are allowed. No workload name, namespace, UID, image, URL, Secret or free-form error labels. Derive gauges from current cache state so restart and deletion do not corrupt counts. Do not double-count the same scrape via direct and Collector paths. Health/readiness do not depend on the telemetry backend.

## Alternatives considered

A telemetry database as controller state adds a failure dependency. Per-workload metric labels grow without bound. Logging full objects for troubleshooting can disclose referenced or embedded sensitive data.

## Validation and revisit trigger

AWCP-10 tests status/no-op/generation; AWCP-11 tests metric changes, restart and telemetry outage. An authenticated/TLS metrics scrape is configured using the selected scaffold; ServiceMonitor belongs only in an optional Prometheus Operator overlay.

## References

- [Upstream reference](https://book.kubebuilder.io/reference/metrics.html)
- [Architecture](../architecture.md)
- [Compatibility](../compatibility.md)
