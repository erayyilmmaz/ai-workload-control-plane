# ADR-009 — V1 API evolution and migration boundary

Status: Accepted. Date: 2026-09-14. Tracking: AWCP-19.

## Context

V0 publishes a single alpha API version, `platform.example.io/v1alpha1`. V1 needs
new declarative feature areas without breaking manifests, default semantics,
status behavior or the existing controller's least-privilege boundary.

## Decision

Keep `v1alpha1` served and storage version throughout V1. Add only optional,
structural, backward-compatible fields in the feature story that implements them.
Absent V1 fields must preserve V0 behavior. Do not add a conversion webhook or
manager-side spec-default write in this baseline story.

Create `v1beta1` only for a breaking semantic/API decision or a production
stability commitment. That decision must include conversion/migration design,
fixtures, upgrade/rollback boundaries and user-facing migration documentation
before a second served version exists.

## Consequences and boundaries

AWCP-19 does not add unimplemented JSON fields merely to reserve names. The
design table in [V1 API evolution](../v1-api-evolution.md) is the reservation;
each feature later earns its schema by delivering its builder, lifecycle, status,
validation and tests together. The V0 compatibility fixture is a permanent
regression check.

An alpha version does not make undocumented breakage acceptable. New defaults,
list/map semantics, field removals and interpretation changes are compatibility
events and require the same migration discipline.

## Alternatives considered

Creating `v1beta1` immediately would require version conversion and operational
certificate/webhook work before a second implementation exists. Adding every V1
field now would expose unsupported behavior and make omission/default semantics
ambiguous. Leaving evolution undocumented would make future breaking changes
accidental.

## Validation and revisit trigger

AWCP-19 validates the existing fixture against envtest and documents the staged
CRD upgrade plan. Revisit this ADR before a breaking change, second served version,
production API stability claim or Kubernetes baseline minor update.
