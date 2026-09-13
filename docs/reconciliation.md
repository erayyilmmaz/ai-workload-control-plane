# Reconciliation foundation — AWCP-5

## Scope and composition

`internal/resource.Builder` builds a deterministic `[]Intent` from a copied
AIWorkload without API calls or external state. Each intent has a fresh, typed
child identity and a pure `Mutate` callback. The callback runs on a fresh object
or a deep copy of the current object and changes only owned fields, preserving
server defaults and unrelated fields. `Absent` denotes an explicitly disabled
optional Service/NetworkPolicy; omission from a plan does **not** mean deletion.

Before writes the engine validates every target: one per supported kind, same
namespace, exact deterministic name, no prepopulated UID/resourceVersion/owner/
finalizer. Order is ServiceAccount → Deployment → Service → NetworkPolicy.
Missing/deleting parent stops work. Writes are sequential, not atomic; a retry
resumes from API state. No finalizer, database, force update or adoption is added.

`manager.Options.Builder` is the internal composition point. If unset, the manager
uses `WorkloadBuilder`, which emits the AWCP-6 Deployment intent. `test/fixtures.Plan`
remains deliberately incomplete and only supports AWCP-5 engine tests. AWCP-7/8/9
add the remaining production children; AWCP-10 observes readiness. An optional controller name
allows tests to restart managers sequentially without disabling the runtime's
process-global name uniqueness validation; the shipped name stays `aiworkload`.

## Identity and mutation safety

`ChildName` follows ADR-004: `awcp-<bounded prefix>-<64-bit full-name hash>`.
Names are stable across retries/restarts. Parent UID remains the ownership
identity: same-name parent recreation is not permission to take old children.

Every patch/delete verifies namespace and controller owner's exact API version,
kind, name and UID. Missing owner, non-controller reference, foreign kind/group,
or stale UID yields `ResourceOwnershipConflict`. No adoption is attempted even
after an AlreadyExists race. New references set `blockOwnerDeletion=false`;
no primary-delete/finalizer permission is granted.

Mutators cannot change identity, owner references, finalizers or deletion state.
Semantic equality avoids writes; differences use merge patch with resourceVersion
optimistic locking. Concurrent edits cause Conflict and a fresh read next time.
Optional deletes require **both UID and resourceVersion** preconditions. A
replacement must never be deleted using the stale owner's identity. Terminating
children are awaited, not stripped of finalizers or forcibly recreated.

The engine does not know resource-specific defaulting rules. Future builders
must mutate managed fields narrowly and test normalization against a real API.
Current fixtures prove Service IP/default and Deployment sidecar preservation,
not every future production field mapping.

## Watches and Secret boundary

- Primary create/delete and generation/deletion-timestamp/UID changes enqueue.
  Status-only and unrelated primary metadata updates do not self-enqueue.
- `Owns` registers four kinds without a generation-only filter. Child metadata/
  spec changes, deletion and Deployment status updates reach the parent.
- `WatchesMetadata` and the namespace-local `spec.secretRefs` index map Secret
  events only to referencing parents, including deletion. No Secret payload is
  read by this foundation and no Secret is written. RBAC cannot distinguish
  metadata from payload reads; it is not a content-isolation boundary. AWCP-8
  implements actual prerequisite decisions.
- Out-of-scope reconcile and Secret requests are ignored. Cache scope remains
  exactly `WATCH_NAMESPACE`; Lease scope remains `MANAGER_NAMESPACE`.

## Results, errors and status

| Situation | Result and observable behavior |
| --- | --- |
| Missing/deleting parent or successful apply/no-op | Empty result; no periodic success polling |
| Owned child terminating | Normal two-second recheck; no destructive recreation |
| Conflict / AlreadyExists / Forbidden / timeout / throttling / unexpected error | Sanitized distinct error category; controller-runtime per-key retry/backoff |
| Invalid plan/API Invalid or foreign ownership | Failure conditions and Warning Event; normal 60-second retry plus watch events |
| Unchanged failure | No repeated status patch or explicit Event emission |
| Recovery after foundation failure | Degraded=False, Progressing=True, Ready=Unknown; no application-readiness claim |

Errors retain their cause through `Unwrap` without putting arbitrary API error
bodies in log/status/Event text. Failure status records observed generation and
desired replicas. Ready/Progressing are Unknown on transient failure and False
on configuration/ownership failure; Degraded is True. Existing endpoint and
readyReplicas are not newly observed here: consumers must check conditions and
observedGeneration. The complete reducer is AWCP-10. Status patches use optimistic
locking and transition timestamps remain stable for unchanged condition status.

If a status write is denied or conflicts, that error is returned for retry; a
condition cannot be promised when the API forbids writing it. One failing parent
does not stop the queue permanently. No global retry/concurrency overrides.

## Runtime permissions

The generated namespaced Role permits primary/Secret read/watch, primary status
patch/update, four-kind get/list/watch/create/patch/update, Service/NetworkPolicy
delete and events.k8s.io Event writes. There is no Secret mutation, primary spec
write/delete, finalizer permission, Deployment/ServiceAccount deletion, Namespace
write or RBAC management. Manifest tests assert the complete matrix; kind smoke
checks the manager identity's allowed/denied operations and namespace boundary.

See [execution evidence](verification/AWCP-5.md) for tests and limitations.
