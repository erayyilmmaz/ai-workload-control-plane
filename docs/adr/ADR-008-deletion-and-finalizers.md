# ADR-008 — Deletion and Finalizers

Status: Implemented for V0. Date: 2026-09-13. Tracking: AWCP-12.

## Context

Only Kubernetes-owned child objects exist in V0. A custom deletion protocol could leave parents stuck without providing necessary cleanup.

## Decision

Use same-namespace controller ownerReferences and background Kubernetes garbage collection, with blockOwnerDeletion=false. Do not add an AIWorkload finalizer. Optional child disable paths use owner checks and child-UID delete preconditions. Parent deletion must not delete a user-owned Secret or Namespace.

## Consequences and boundaries

Parent deletion is asynchronous; use bounded eventual assertions in kind. A parent missing/deleting guard prevents deliberate recreation, while a deletion race is left safely to GC. Operator undeploy preserves CRDs, workload namespace and CR instances. CRD removal and explicit workload cleanup are separate destructive operations.

## Alternatives considered

Manual deletion of each child duplicates native GC and adds partial-failure handling. A finalizer is justified only if future non-Kubernetes resources or required ordered cleanup exist; it would need retry, idempotence and recovery documentation.

## Validation and revisit trigger

AWCP-12 verifies guards/ownership, repeated deletion behavior, user-Secret
preservation and actual kind-based Deployment/ReplicaSet/Pod subtree garbage
collection. Envtest has no garbage collector, so its tests assert ownerReferences
and deletion guards rather than imaginary cleanup. Revisit this decision before
adding an external resource, ordered teardown dependency or any other cleanup that
Kubernetes ownerReferences cannot perform.

## References

- [Upstream reference](https://book.kubebuilder.io/reference/using-finalizers.html)
- [Architecture](../architecture.md)
- [Compatibility](../compatibility.md)
