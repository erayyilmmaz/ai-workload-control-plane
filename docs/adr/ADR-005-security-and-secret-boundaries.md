# ADR-005 — Security and Secret Boundaries

Status: Accepted design. Date: 2026-09-12. Tracking: AWCP-2.

## Context

Creating a pod can grant practical access to namespace Secrets even without direct Secret read access. The manager also needs permissions that must not leak into workload identity.

## Decision

Create a dedicated workload ServiceAccount without RoleBindings; set automountServiceAccountToken=false on both ServiceAccount and Pod. Default pod security is non-root, no privilege escalation, drop ALL capabilities, RuntimeDefault seccomp. Secret envFrom references stay in the same namespace and are never copied, mutated or deleted by the controller.

## Consequences and boundaries

Use Secret metadata-only observation/indexing to reduce payload handling, while acknowledging that Secret get/list/watch RBAC can authorize full data reads. Do not claim API-level Secret field redaction. A missing/returned Secret requeues referencing CRs. Existing process environment values are not revoked/refreshed by Secret deletion/update. API errors, Secret payloads and tokens must not enter log, status, Event, metric label or generated annotation.

## Runtime permissions

The planned manager RBAC is reviewed by resource, separately from installation privileges:

| Namespace / resource | Runtime verbs | Purpose |
| --- | --- | --- |
| Managed namespace: aiworkloads | get, list, watch | Observe desired state; no primary-spec writes |
| Managed namespace: aiworkloads/status | patch, update | Publish observations |
| Managed namespace: deployments, serviceaccounts | get, list, watch, create, patch, update | Reconcile owned children; deletion is native GC |
| Managed namespace: services, networkpolicies | get, list, watch, create, patch, update, delete | Includes optional-child disable |
| Managed namespace: secrets | get, list, watch | Metadata implementation; underlying RBAC still authorizes content reads |
| Managed namespace and manager namespace: events | create, patch, update | Reconcile and leadership events |
| Manager namespace: leases | get, create, update | Leader election for the one configured manager |

No wildcard resources/verbs, runtime cluster-admin, namespace mutation, Secret mutation, CRD mutation or workload RoleBinding creation is granted. Manager Pod needs its own API token, while workload Pod token automount remains disabled. Least-privilege tests must verify this distinction in AWCP-8; authorization to install CRDs is an administrator concern. Prometheus scrape authorization, when needed, is a separate narrowly scoped reader role in AWCP-11.

## Alternatives considered

Using default ServiceAccount obscures identity; granting workload cluster roles is unnecessary. A per-Secret Role with resourceNames does not trivially implement arbitrary dynamic namespace watch/filtering. Webhooks or full multi-tenancy require separate scope and are not silently added.

## Validation and revisit trigger

AWCP-8 verifies manager/workload permissions and Secret restore without editing the parent. Negative leakage tests use synthetic values; kind tests prove runtime identity. Ownership conflict and API Forbidden must not be mislabeled SecretNotFound.

## References

- [Upstream reference](https://kubernetes.io/docs/concepts/security/rbac-good-practices/)
- [Architecture](../architecture.md)
- [Compatibility](../compatibility.md)
