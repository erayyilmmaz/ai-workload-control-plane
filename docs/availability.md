# Availability, PodDisruptionBudget and leader election — AWCP-28

`spec.availability.enabled: true` creates one current-UID-owned `policy/v1`
PodDisruptionBudget (PDB) for the workload's generated Deployment. It selects
the exact stable workload identity labels, never a user-supplied selector or a
different namespace. A PDB limits **voluntary** disruptions such as a drain; it
does not add replicas, reserve nodes, guarantee capacity, or protect against an
involuntary node failure.

Exactly one integer budget shape is required when availability is enabled:

```yaml
spec:
  replicas: 3
  availability:
    enabled: true
    minAvailable: 2
```

`minAvailable` is 1..20 and `maxUnavailable` is 0..19. Percentages are not in
the workload API, because integer counts keep the small, bounded 0..20 replica
contract predictable. AWCP rejects missing or contradictory budget shapes,
`minAvailable` above the static replica count or HPA `minReplicas`, and
`maxUnavailable` at or above that same lower bound. Removing or disabling the
block deletes only the current-UID-owned PDB.

`AvailabilityReady=True/PDBActive` means AWCP observed its PDB after applying
it. `AvailabilityReady=False/PDBReconciling` is a transient API observation.
`SingleReplicaDisruptionBlocked` is an intentional warning: a single replica
with a valid PDB cannot remain available while it is voluntarily disrupted, so
the PDB blocks eviction rather than pretending that one Pod is highly available.

The manager itself runs two replicas. Controller-runtime Lease leader election
permits only the active leader to reconcile; the standby waits for leadership.
The `awcp-controller-manager` PDB requires one available leader. The manager
uses a 15-second lease, 10-second renew deadline, 2-second retry period, and
releases its Lease on normal shutdown. This proves a bounded control-plane
failover configuration, not zone diversity, multi-cluster continuity or an
application availability SLO.

References: [Kubernetes PDB guidance](https://kubernetes.io/docs/tasks/run-application/configure-pdb/)
and [controller-runtime manager options](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/manager#Options).
