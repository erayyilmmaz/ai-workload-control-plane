# Test strategy and evidence boundaries — AWCP-13

The project uses three layers. A passing later layer does not replace the evidence
provided by an earlier one, and an earlier layer does not imply a real workload
rollout.

| Layer | Command | Proves | Does not prove |
| --- | --- | --- | --- |
| Unit | `make test-unit` | Pure builders, names, status reduction, ownership guards and races | API-server defaulting, watches or Kubernetes garbage collection |
| Envtest | `make test-envtest` | CRD/default/CEL behavior, API reads/writes, watches, manager lifecycle and deletion guard | Scheduler, kubelet, traffic, CNI enforcement or garbage collection |
| Isolated kind | `make docker-build && make smoke` | Container/runtime contract, manager startup, RBAC, current owned-tree garbage collection and Secret survival | Application readiness, HTTP traffic, production CNI behavior or HA failover |

## Regression commands

```bash
make test-unit
make test-envtest
make coverage
make test-race
make verify
```

`make coverage` starts the same local envtest control plane as `make test`, then
writes ignored `dist/coverage.out` and `dist/coverage.txt`. It instruments only
packages in this module; third-party and standard-library code are deliberately
excluded. The report is a trend and gap-discovery artifact, not a substitute for a
behavioral acceptance test or a numeric release threshold.

## Determinism and safety

Integration assertions are bounded and clean up their manager/API-server processes.
They use no existing kubeconfig, cloud service, LLM credential or external AI API.
The first tool and envtest-asset download can require network access; prepared runs
use the project-local pinned binaries. Envtest has no built-in Kubernetes controllers,
so owner-reference and deletion-guard assertions remain there while real garbage
collection stays in the isolated kind test.

See [development commands](development.md), [traceability](traceability.md) and
[AWCP-13 evidence](verification/AWCP-13.md).
