# Architecture decision records

ADR-001..008 are accepted V0 design decisions for AWCP-2. ADR-009..010 establish
the AWCP-19 V1 compatibility baseline; ADR-011/012 establish the AWCP-20/21
GitOps delivery boundary; ADR-013 establishes AWCP-22 Terraform/IaC ownership.
Acceptance does not imply runtime implementation. Changes
should explain what supersedes the previous decision and update affected
contract/tests together.

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
| [ADR-009](ADR-009-v1-api-evolution.md) | V1 API Evolution and Migration Boundary |
| [ADR-010](ADR-010-optional-platform-capabilities.md) | Optional Platform Capabilities |
| [ADR-011](ADR-011-gitops-ownership-boundary.md) | GitOps Ownership Boundary |
| [ADR-012](ADR-012-environment-delivery-model.md) | Environment Delivery Model |
| [ADR-013](ADR-013-terraform-and-gitops-ownership.md) | Terraform and GitOps Ownership Boundary |

See [traceability](../traceability.md) for the story and test layer responsible for implementing each decision.
