# ADR-003 — Why Namespaced AIWorkload

Status: Accepted design. Date: 2026-09-12. Tracking: AWCP-2.

## Context

Each application, its four generated resources and its Secret references should have an explicit local ownership scope.

## Decision

AIWorkload is Namespaced. All children and references remain in the parent namespace. The V0 manager watches exactly one explicitly configured namespace, with matching namespace RBAC; an empty WATCH_NAMESPACE fails startup. The manager namespace and managed namespace may differ.

## Consequences and boundaries

An AIWorkload cannot name another namespace's Secret or create a Namespace. Stable child selectors contain parent UID to isolate two resource incarnations. Namespace boundaries simplify ownership but do not alone establish tenant safety: users able to create workloads can consume accessible namespace resources.

## Alternatives considered

Cluster-scoped AIWorkload plus a target namespace field would require cross-namespace authorization and ownership rules. Cluster-wide watches would increase the blast radius without serving the initial use case.

## Validation and revisit trigger

AWCP-4 validates the schema scope; AWCP-8 verifies Secret indexes/RBAC do not cross namespaces. Expand to multi-namespace or tenancy only under a new ADR.

## References

- [Upstream reference](https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/)
- [Architecture](../architecture.md)
- [Compatibility](../compatibility.md)
