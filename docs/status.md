# Observed workload status — AWCP-10

The controller reports only what it has observed for the current `AIWorkload`
generation. `observedGeneration` means that generation was evaluated; it is not a
shortcut for a successful application request.

## Condition contract

`Ready`, `Progressing` and `Degraded` are always written together after a successful
child/dependency reconciliation and Deployment read. Each condition has the same
reason, safe human-readable message and observed generation. The controller uses the
standard condition helper, so `lastTransitionTime` changes only when a condition's
status changes.

| State | Ready | Progressing | Degraded | Reason |
| --- | --- | --- | --- | --- |
| Current rollout complete | True | False | False | `WorkloadReady` |
| Rollout pending or stale | False | True | False | `Reconciling` |
| Deployment explicitly unavailable | False | True | False | `DeploymentUnavailable` |
| Deployment progress deadline exceeded | False | False | True | `ProgressDeadlineExceeded` |
| Desired replicas is zero | False | False | False | `ScaledToZero` |
| Secret/ownership/configuration failure | False | False | True | Existing safe failure reason |
| Transient API failure | Unknown | Unknown | True | `ReconcileFailed` |

`WorkloadReady` additionally requires `Deployment.status.observedGeneration` to
cover the Deployment generation, `Available=True`, and updated, total, ready and
available replica counts to equal desired replicas. An old Available condition cannot
make a newly changed image Ready.

`desiredReplicas` follows `spec.replicas` (default 1), `readyReplicas` is the observed
Deployment value, and `endpoint` remains service discovery only. It does not certify
DNS, application protocol health, credentials, NetworkPolicy enforcement or egress.

## Events and failure safety

The controller emits one Normal `WorkloadReady` Event for a false/unknown-to-true Ready
transition. It does not emit another success Event or status patch for an unchanged
reconcile. Failure conditions emit one Warning Event only when their semantic status
changes. Event, status and log messages use fixed generic text; Secret names, Secret
data and arbitrary API error bodies are excluded.

See the [API contract](api-contract.md), [reconciliation behavior](reconciliation.md)
and [AWCP-10 execution evidence](verification/AWCP-10.md).
