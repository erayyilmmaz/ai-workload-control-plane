# ADR-011 — GitOps ownership boundary

Status: Accepted. Date: 2026-09-16. Tracking: AWCP-20.

## Context

V1 needs Git-driven delivery without causing Argo CD reconciliation to fight the
AWCP controller. The controller already owns generated workload children through
the parent `AIWorkload` UID, while GitOps needs a small, auditable source and
destination boundary.

## Decision

Argo CD manages only the AWCP platform package (`config/default`) and parent
`AIWorkload` manifests committed under `gitops/workloads`. It does not manage
AWCP-generated Deployments, Services, ServiceAccounts or NetworkPolicies.

Use a single restricted `AppProject` for the present public repository and the
two AWCP namespaces. Enable automated sync and self-heal for both Applications.
Disable platform prune; enable workload prune with `allowEmpty: false`. Treat
Argo CD installation as an explicit administrator bootstrap, outside both Argo
Applications and CI. A Git revert is the rollback source for an automated-sync
Application.

## Consequences and boundaries

Git is the desired source for parent manifests, but Kubernetes is still the
source of observed status. Argo sync/health and AWCP conditions are separate
signals. The local reference chart values contain no Git credential or admin
secret and do not claim production TLS, SSO, HA or network hardening.

The platform package remains in `config/default` rather than being copied under
`gitops`, preventing divergence from the supported Kustomize install package.
AWCP-21 owns environments/ApplicationSet; AWCP-31 owns broader GitOps status
observability.

## Alternatives considered

Committing generated child resources would create competing desired states.
Enabling platform prune would make a Git deletion capable of removing CRD/RBAC
infrastructure. Disabling workload prune would leave removed Git workloads running
indefinitely. Letting CI call the Argo API would require a token and turn a Git
commit delivery model into credentialed imperative deployment.

## Validation and revisit trigger

AWCP-20 renders Project/Application/workload sources, verifies the no-child
source rule and provides a disposable kind/Argo CD demo. Revisit before a private
repository, multi-cluster destination, ApplicationSet, production bootstrap or
any change that would make an optional GitOps component a manager-startup
dependency.
