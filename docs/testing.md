# Test strategy and evidence boundaries — AWCP-14

The project uses three layers. A passing later layer does not replace the evidence
provided by an earlier one, and an earlier layer does not imply a real workload
rollout.

| Layer | Command | Proves | Does not prove |
| --- | --- | --- | --- |
| Unit | `make test-unit` | Pure builders, names, status reduction, ownership guards and races | API-server defaulting, watches or Kubernetes garbage collection |
| Envtest | `make test-envtest` | CRD/default/CEL behavior, real ValidatingAdmissionPolicy deny/allow behavior, API reads/writes, watches, manager lifecycle and deletion guard | Scheduler, kubelet, traffic, CNI enforcement or garbage collection |
| Kind smoke | `make docker-build && make smoke` | Container/runtime contract, manager startup, RBAC, foundational owned-tree garbage collection and Secret survival | Application readiness, HTTP traffic, CNI enforcement or HA failover |
| Kind E2E | `make e2e` | Real scheduler/kubelet rollout, Service HTTP traffic, image update, scale, all-child drift, Secret recovery, restart recovery, authenticated metrics, tenant Role isolation, namespace-local Secret lookup, LimitRange/ResourceQuota admission and restricted tenant-policy denial | Production CNI packet enforcement, published-image install or HA failover |

## Regression commands

```bash
make test-unit
make test-envtest
make coverage
make test-race
make verify
make e2e
```

`make coverage` starts the same local envtest control plane as `make test`, then
writes ignored `dist/coverage.out` and `dist/coverage.txt`. It instruments only
packages in this module; third-party and standard-library code are deliberately
excluded. The report is a trend and gap-discovery artifact, not a substitute for a
behavioral acceptance test or a numeric release threshold.

## Determinism and safety

Integration assertions are bounded and clean up their manager/API-server processes.
The kind E2E uses its own random cluster and temporary explicit kubeconfig; it does
not select the user's current context. The suite uses no cloud service, LLM
credential or external AI API.
The first tool and envtest-asset download can require network access; prepared runs
use the project-local pinned binaries. Envtest has no built-in Kubernetes controllers,
so owner-reference and deletion-guard assertions remain there while real garbage
collection stays in the isolated kind test.

The standard kind profile asserts NetworkPolicy generation only; it does not prove
CNI enforcement. See [development commands](development.md),
[traceability](traceability.md), [AWCP-13 evidence](verification/AWCP-13.md) and
[AWCP-14 evidence](verification/AWCP-14.md).
