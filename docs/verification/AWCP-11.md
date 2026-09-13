# AWCP-11 — Controller observability and metrics: execution evidence

Date: 2026-09-13

## Delivered scope

- The manager exposes authenticated HTTPS metrics on `:8443`, behind controller-runtime
  Kubernetes authentication/authorization filtering. Its serving certificate has a
  dedicated writable volume; the rest of the manager filesystem remains read-only.
- AWCP adds only bounded child-operation/fixed-failure metrics and cache-derived
  managed/Ready/Degraded gauges. Controller-runtime remains the source for reconcile
  duration, result and error totals.
- Gauge refresh is a leader-scoped background runnable. It logs a refresh error but
  never returns it to a reconcile request or gates health/readiness.
- The repository contains versioned Prometheus, Grafana and optional OTel Collector
  examples. Neither scraper nor Collector is part of `config/default`.

## Validation

| Command | Result | Coverage |
| --- | --- | --- |
| `go test ./internal/telemetry ./internal/manager ./internal/controller` | Passed | Bounded labels, cache-derived gauges, manager/controller wiring |
| `go test ./test/manifests` | Passed | HTTPS metrics port/certificate volume, metrics Service and exact auth RBAC |
| `kustomize build config/default` and `jq empty examples/observability/grafana-awcp-dashboard.json` | Passed | Base render and dashboard JSON syntax |
| `make verify` | Passed | Full build/lint/unit/envtest/generated/render validation |
| `make test-race` | Passed | Full race detector validation |
| `make docker-build IMG=awcp-manager:awcp-11` | Passed | Linux ARM64 manager image built from `23051a9` |
| `make smoke IMG=awcp-manager:awcp-11` | Passed | Fresh kind bootstrap; HTTPS metrics server, Service and auth-RBAC contract |

## Limits

The base certificate is self-signed. Prometheus/Collector examples intentionally use
`insecure_skip_verify` for local demonstration only and must be replaced by trusted
certificate management in production. No Prometheus/Grafana/Collector backend is
installed or tested as an external service, no trace spans are emitted, and telemetry
unavailability is not a workload-health signal.

See [telemetry behavior](../telemetry.md) and [ADR-006](../adr/ADR-006-observability-model.md).
