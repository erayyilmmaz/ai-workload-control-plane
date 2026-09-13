# AWCP-10 — Status conditions, failure model and Kubernetes Events: execution evidence

Date: 2026-09-13

## Delivered scope

- A single status reducer observes the current owned Deployment after successful
  child and Secret prerequisite reconciliation. It updates observed generation,
  desired and ready replicas, Service endpoint and the Ready/Progressing/Degraded
  condition triple only when their semantic value changes.
- `WorkloadReady` requires a current Deployment generation, `Available=True`, and
  updated, total, ready and available replica counts equal to desired replicas.
  An old successful rollout cannot report a new CR generation as Ready.
- The reducer distinguishes pending/unavailable rollouts, progress-deadline failure
  and scale-to-zero. Existing safe Secret, ownership, configuration and transient
  API failure classification remains in force.
- A Normal `WorkloadReady` Event is emitted only on a transition into Ready; existing
  Warning failure Events remain deduplicated by semantic status change. Messages do
  not contain Secret payloads/names or arbitrary API error bodies.

## Validation

| Command | Result | Coverage |
| --- | --- | --- |
| `go test -count=1 ./internal/controller` | Passed | Truth table, generation gate, transition timestamp, status no-op and Event deduplication |
| `KUBEBUILDER_ASSETS=... go test -count=1 -v ./test/reconciliation` | Passed | Real API current/stale rollout, Secret recovery, scale-to-zero, status no-op and existing ownership lifecycle |
| `make verify` | Passed | Full build/lint/unit/envtest/generated/render validation |
| `make test-race` | Passed | Full race detector validation |
| `DOCKER_CONFIG=... DOCKER_HOST=... make docker-build IMG=awcp-manager:awcp-10` | Passed | Local Linux ARM64 manager image for commit `f870677` |
| `DOCKER_CONFIG=... DOCKER_HOST=... make smoke IMG=awcp-manager:awcp-10` | Passed | Fresh kind bootstrap: manager, child resource, non-ready current status triple and least-privilege RBAC contracts |

The real-API status fixture sets Deployment status directly; envtest has no scheduler,
kubelet, image puller, DNS or Service traffic. It proves the controller's interpretation
of Kubernetes API status, not a live application's health.

## Limits

`Ready=True` is an observed Deployment rollout condition, not an application-level
HTTP, credential, DNS, egress or NetworkPolicy enforcement guarantee. The isolated
kind smoke checks the placeholder sample remains explicitly non-ready with a complete,
current status triple; real application rollout and traffic scenarios remain AWCP-14
work. Metrics and dashboards remain AWCP-11 work.

See [status behavior](../status.md), [API contract](../api-contract.md) and
[reconciliation behavior](../reconciliation.md).
