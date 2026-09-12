# ADR-007 — API Versioning Strategy

Status: Accepted design. Date: 2026-09-12. Tracking: AWCP-2.

## Context

An alpha API still needs explicit defaults, stable condition meanings and a documented path for breaking changes.

## Decision

Serve and store platform.example.io/v1alpha1, kind AIWorkload. Implement a structural schema with defaulting/validation and a status subresource. There is no admission or conversion webhook in V0. The field/default/condition contract is ../api-contract.md.

## Consequences and boundaries

Default false/zero handling must be explicit. Unknown-field pruning is distinct from strict request validation. Names, selectors and resource identities cannot change as incidental refactors. Alpha breaking changes require migration/recreate guidance plus schema, example and test updates. A release tag does not imply GA API stability.

## Alternatives considered

Unversioned configuration would conceal compatibility changes. An early conversion webhook adds certificates and operational dependencies before multiple served versions exist. Reusing an existing unrelated workload CRD would not exercise this project's intended platform API.

## Validation and revisit trigger

AWCP-4 validates schema/default/status contracts and kubectl explain; AWCP-17 records compatibility/release limitations. Add a new served version or real production domain only with a migration decision.

## References

- [Upstream reference](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/)
- [Architecture](../architecture.md)
- [Compatibility](../compatibility.md)
