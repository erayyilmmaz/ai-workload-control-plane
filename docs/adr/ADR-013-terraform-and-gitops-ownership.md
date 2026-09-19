# ADR-013 — Terraform and GitOps ownership boundary

Status: Accepted. Date: 2026-09-16. Tracking: AWCP-22.

## Context

AWCP needs an infrastructure reference that teams can validate without a cloud
account, while AWCP-20/21 already establish Git and Argo CD as the delivery
source for the platform package and parent workload intent. Allowing both tools
to write the same Kubernetes objects would create competing desired states.

## Decision

Use one AWS/EKS reference implementation under `infra/terraform`. It declares
only network and EKS control-plane prerequisites: VPC/subnets/routes, optional
NAT, cluster IAM role, KMS Secret encryption and a private EKS API endpoint.
Terraform has no Kubernetes provider and no resource pointing at `gitops/`,
`config/` or application manifests.

Git + Argo CD remains the owner of the AWCP platform package and parent
`AIWorkload` overlays. AWCP remains the owner of generated workload children.
The Terraform local default backend is not a production state decision; an
approved remote state backend and access model are a required external handoff.

## Consequences and boundaries

`make infra-verify` pins the Terraform CLI, provider selection and Trivy binary,
then performs formatting, lockfile-readonly initialization, static validation and
high/critical IaC scanning. CI uses the same non-mutating command with no AWS
credential or cloud apply.

The checked-in configuration does not create node groups, user access entries,
OIDC providers, workload IAM roles, VPC endpoints, application Secrets, Argo CD,
AWCP or a state backend. It therefore is not a deployable production landing
zone. NAT is optional and disabled because it has cost and egress-policy impact.

## Alternatives considered

Using Terraform's Kubernetes provider for AWCP/Argo resources was rejected
because it would overlap the GitOps desired source. Providing all EKS/GKE/AKS
variants was rejected because one verified, documented reference is preferable to
three unvalidated cloud shapes. A generic kind-only Terraform example would not
exercise the cloud network, IAM or private-control-plane boundaries this story
needs to make explicit.

## Validation and revisit trigger

Revisit before adding a production state backend, user/network access model,
node pools, multi-account/multi-cluster layout, cloud provider alternative or any
Kubernetes provider. Such a change must specify which owner gains a resource and
why it cannot remain in the existing GitOps/AWCP boundary.
