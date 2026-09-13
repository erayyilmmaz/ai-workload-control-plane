# AWCP-12 — Deletion, garbage collection and lifecycle edges: execution evidence

Date: 2026-09-13

## Delivered scope

- `AIWorkload` relies on same-namespace controller ownerReferences and Kubernetes
  garbage collection for its Deployment, ServiceAccount, optional Service and
  optional NetworkPolicy. V0 adds no finalizer.
- A missing or deleting parent cannot build or write child resources. Repeated
  deletion reconciliation is a no-op; a current-parent UID never adopts or deletes
  an old same-name parent's child.
- Optional-child delete paths retain UID and resourceVersion preconditions. A
  replacement created during a delete race is not removed.
- User-owned Secrets are neither owned, adopted nor deleted by AWCP.

## Validation

| Command | Result | Coverage |
| --- | --- | --- |
| `go test ./internal/controller` | Passed | Deletion guard, repeated reconciliation, user-Secret preservation, stale-owner and delete-race boundaries |
| `make verify` | Passed | Full build/lint/unit/envtest/generated/render validation |
| `make test-race` | Passed | Full race detector validation |
| `make docker-build IMG=awcp-manager:awcp-12` | Passed | Linux ARM64 manager image built from `72b299d` |
| `make smoke IMG=awcp-manager:awcp-12` | Passed | Fresh kind parent deletion; four children and ReplicaSet/Pod subtree GC, user Secret preserved and cleanup succeeded |

## Limits

The kind check proves Kubernetes garbage collection for the current four-child V0
tree. It does not prove cleanup of external systems, CRD removal behavior, or a
production CNI/traffic policy. Any future external resource or ordered teardown
requirement must add a specific finalizer design before implementation.

See [ADR-008](../adr/ADR-008-deletion-and-finalizers.md) and
[reconciliation behavior](../reconciliation.md).
