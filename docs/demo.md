# Portfolio demo — AWCP-17

`make portfolio-demo` is the presentation entrypoint. It delegates to the same
real-Kubernetes lifecycle harness used by `make e2e` and hosted CI, so the demo is
not a separate, untested happy path. It builds local non-production images, creates
a random `awcp-e2e-*` kind cluster with a temporary kubeconfig, and removes it on
success or failure. It does not use the caller's current kube context.

## Before presenting

Docker must be available. The first run needs network access for pinned kind and
kubectl downloads plus the kind node image. It needs no registry, cloud, paid AI
account or real Secret value.

```bash
make bootstrap
make portfolio-demo
```

The command deletes its temporary cluster. For a slower live explanation, use the
same 16 steps below with `make quickstart` and the checked-in examples; do not claim
that manually changing a Secret value refreshes an existing container environment.

## The 16 visible steps

| # | Demonstration | Evidence produced by the harness |
| --- | --- | --- |
| 1 | Create `AIWorkload` | Applies `lifecycle-demo` and its synthetic user Secret |
| 2 | Deployment appears | Gets the deterministic owned Deployment |
| 3 | Service appears | Gets the owned ClusterIP Service |
| 4 | Workload becomes Ready | Waits for generation-aware `Ready=True` and HTTP `v1` |
| 5 | Delete Deployment | Explicitly deletes the owned Deployment |
| 6 | Drift is repaired | Waits for a different Deployment UID and Ready state |
| 7 | Change image | Patches `awcp-demo:v1` to `awcp-demo:v2` |
| 8 | Rolling update | Waits for rollout and verifies Service HTTP `v2` |
| 9 | Remove required Secret | Deletes only the user-owned synthetic Secret |
| 10 | Workload becomes Degraded | Requires `Ready=False`, `Degraded=True`, `SecretNotFound` |
| 11 | Restore Secret | Recreates the Secret without changing the CR |
| 12 | Workload recovers | Waits for `Ready=True` again |
| 13 | Open Grafana (optional) | Import the checked-in dashboard into an existing Grafana/Prometheus setup; AWCP does not deploy Grafana |
| 14 | Show reconcile metrics | Uses a temporary least-privilege token to read authenticated gauges |
| 15 | Delete `AIWorkload` | Deletes the parent object after package-removal safety is checked |
| 16 | Owned resources disappear | Waits for Kubernetes garbage collection; verifies the user Secret survives |

The harness also verifies scaling, manager restart and recreation of Service,
ServiceAccount and NetworkPolicy. Those strengthen the demo but are not substituted
for any numbered step.

## Grafana boundary

The base package installs only the authenticated metrics Service. To show step 13,
an operator must deliberately run Prometheus and Grafana, grant only the documented
metrics-reader permission, use
[`prometheus-scrape.yaml`](../examples/observability/prometheus-scrape.yaml), then
import [`grafana-awcp-dashboard.json`](../examples/observability/grafana-awcp-dashboard.json).
The base self-signed certificate requires the demo scrape setting shown there; a
production installation needs a trusted serving certificate. Do not run the OTel
Collector example and direct Prometheus scrape as duplicate pipelines for the same
series.

NetworkPolicy object generation is demonstrated, but kind's default networking is
not CNI enforcement evidence. See [E2E boundaries](e2e.md) and
[telemetry boundaries](telemetry.md).
