# AWCP-28 verification — availability, PDB and leader election

## Delivered contract

- Optional `spec.availability` accepts one integer `minAvailable` or
  `maxUnavailable` value when explicitly enabled.
- The controller creates, watches and owner-safely removes a namespace-local
  `policy/v1` PDB with the exact generated Deployment selector.
- Invalid/unsafe lower-bound combinations fail before the first child write;
  the one-replica no-voluntary-disruption boundary is a visible condition.
- The manager uses two scheduled replicas, Lease leader election with explicit
  15s/10s/2s timing, `ReleaseOnCancel`, and a controller PDB requiring one
  available leader.

## Local checks

Run from the repository root:

```bash
make generate manifests fmt
make test-unit
make test-envtest
make verify-docs
git diff --check
```

The real API contract includes enabled PDB acceptance and invalid shape/bounds.
Unit tests cover exact selector mapping, metadata preservation, HPA lower-bound
consistency, PDB status conditions, manager PDB and leader-timing validation.

`make e2e` additionally creates a workload PDB, confirms its owner and exact
selector, verifies `AvailabilityReady=PDBActive`, deletes the active manager
Pod and waits for a different ready leader. It is a disposable kind proof of
controller failover; it does not run a node drain or prove a multi-node/zone SLO.

## Boundaries

A PDB controls eviction API behavior for voluntary disruption only. It cannot
prevent force deletion, node failure, resource exhaustion, image failure, or
the loss of every replica. The manager standby is not an additional active
reconciler. No production topology, anti-affinity, topology spread constraint,
regional availability target, or workload load test is claimed by this story.
