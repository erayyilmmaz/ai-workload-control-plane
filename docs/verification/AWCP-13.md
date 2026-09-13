# AWCP-13 — Unit and envtest integration suite: execution evidence

Date: 2026-09-13

## Delivered scope

- The repository has explicit unit, real-API envtest, isolated-kind and race-test
  boundaries, with each layer documenting what it cannot prove.
- A real envtest parent-deletion scenario verifies that AWCP adds no finalizer and
  does not recreate a deleted owned Deployment once the parent is terminating.
- `make coverage` generates an atomic, project-only coverage profile and function
  report under ignored `dist/`; dependencies and the Go standard library are excluded.

## Validation

| Command | Result | Coverage |
| --- | --- | --- |
| `make test-unit` | Passed | Fast pure-test regression suite |
| `make test-envtest` | Passed | Real API contract, manager lifecycle, watches and deletion guard |
| `make coverage` | Passed | Project-only atomic profile and function report |
| `make verify` | Passed | Full build/lint/unit/envtest/generated/render validation |
| `make test-race` | Passed | Race detector validation |
| `make docker-build IMG=awcp-manager:awcp-13` | Passed | Linux ARM64 manager image built from `9c71207` |
| `make smoke IMG=awcp-manager:awcp-13` | Passed | Isolated kind regression of manager, RBAC, metrics and owned-tree deletion contracts |

The executed project-only coverage report totals **83.5%** statements. This is a
recorded baseline rather than a release threshold; behavioral acceptance remains the
primary gate.

## Limits

Envtest does not run Kubernetes built-in controllers, so it cannot prove garbage
collection, scheduler/kubelet rollout, Service traffic or CNI enforcement. The
current owned-tree garbage-collection behavior is checked by the isolated kind smoke;
full application lifecycle E2E remains AWCP-14 work.

See [test strategy](../testing.md) and [traceability](../traceability.md).
