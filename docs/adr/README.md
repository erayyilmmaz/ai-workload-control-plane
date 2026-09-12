# Architecture decision records

All eight records are accepted design decisions for AWCP-2, dated 2026-09-12. Acceptance does not imply runtime implementation. Changes should explain what supersedes the previous decision and update the affected contract/tests together.

| Record | Decision |
| --- | --- |
| [ADR-001](ADR-001-why-kubernetes-operator.md) | Why Kubernetes Operator |
| [ADR-002](ADR-002-why-go.md) | Why Go |
| [ADR-003](ADR-003-why-namespaced-aiworkload.md) | Why Namespaced AIWorkload |
| [ADR-004](ADR-004-reconciliation-and-ownership.md) | Reconciliation and Ownership Model |
| [ADR-005](ADR-005-security-and-secret-boundaries.md) | Security and Secret Boundaries |
| [ADR-006](ADR-006-observability-model.md) | Observability Model |
| [ADR-007](ADR-007-api-versioning-strategy.md) | API Versioning Strategy |
| [ADR-008](ADR-008-deletion-and-finalizers.md) | Deletion and Finalizers |

See [traceability](../traceability.md) for the story and test layer responsible for implementing each decision.
