# ADR-001 — Why Kubernetes Operator

Status: Accepted design. Date: 2026-09-12. Tracking: AWCP-2.

## Context

Users need a declarative application resource whose generated objects remain consistent after edits, deletion and controller restarts.

## Decision

Use a namespaced CRD and a controller-runtime reconciliation loop. Kubernetes API is the durable source of desired and observed state. Use native Deployment, Service, ServiceAccount and NetworkPolicy objects rather than a second provisioning database.

## Consequences and boundaries

Repeated reconcile calls converge; no command execution or external AI API call is part of the control loop. Partial creation is recoverable, not transactional. Eventual consistency means status must identify the generation it describes.

## Alternatives considered

A REST service that shells out to kubectl adds a second lifecycle/control surface; a database plus message broker duplicates Kubernetes state; a static manifest generator does not repair drift. None is needed for V0.

## Validation and revisit trigger

Ownership/watch tests in AWCP-5; real restart/drift recovery in AWCP-14. Revisit only for a separately scoped product API or non-Kubernetes control plane.

## References

- [Upstream reference](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
- [Architecture](../architecture.md)
- [Compatibility](../compatibility.md)
