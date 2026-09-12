# ADR-004 — Reconciliation and Ownership Model

Status: Accepted design. Date: 2026-09-12. Tracking: AWCP-2.

## Context

The controller must repair only the configuration it owns, preserve unrelated user objects and recover from partial API failures.

## Decision

Use deterministic child naming, strict same-namespace owner-UID checks and managed-field merge patches as defined in ../architecture.md. Watch the primary, each owned kind and referenced Secret metadata. Do not globally filter child events by generation. Parent NotFound/deletionTimestamp stops child creation.

## Consequences and boundaries

One reconcile and repeated reconciles have the same desired result. Foreign or stale ownership raises ResourceOwnershipConflict without adoption/deletion. Mutable drift is repaired; incompatible immutable fields are surfaced. API failures use framework backoff; persistent failures use event/change recovery and a bounded 60-second recheck. Status writes occur only after semantic changes.

## Alternatives considered

Blind full-object Update would overwrite unrelated/admission-added fields. Force-adoption or delete/recreate would risk user data and service identity. Polling alone would delay Secret restore and create unnecessary API load. A transaction spanning all child resources is not available.

## Validation and revisit trigger

AWCP-5 verifies no-op/idempotence, all-child watches and collisions; AWCP-6..10 cover mappings and status; envtest proves API behavior; kind proves real rollout and recovery.

## References

- [Upstream reference](https://book.kubebuilder.io/reference/watching-resources/secondary-owned-resources.html)
- [Architecture](../architecture.md)
- [Compatibility](../compatibility.md)
