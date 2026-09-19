# AWS/EKS reference infrastructure — AWCP-22

This directory is a reproducible **reference** for the infrastructure layer of
AI Workload Control Plane (AWCP). It provisions no application manifest,
`AIWorkload`, Argo CD `Application`, Secret value, node group, add-on or tenant.
Those are deliberately separate ownership domains.

## What it declares

- an AWS VPC with two or more public and private subnets;
- a private-only EKS control-plane endpoint, control-plane audit logs and
  Kubernetes Secret envelope encryption using a rotating KMS key;
- only the IAM role required by the EKS control plane; and
- optional, explicitly billable NAT egress for a later approved worker-network
  design.

The reference has no worker nodes. A production node-pool, VPC endpoints,
egress proxy/NAT topology, IP allow-lists, flow-log retention, backup, OIDC
provider and workload IAM roles need their own reviewed design and evidence.

## Ownership boundary

| Layer | Owner | Examples |
| --- | --- | --- |
| Infrastructure | Terraform | VPC, subnets, EKS control plane, EKS cluster role, KMS key |
| Platform package | Git + Argo CD | AWCP CRD, manager deployment, bounded RBAC |
| Workload intent | Git + Argo CD ApplicationSet | Parent `AIWorkload` YAML overlays |
| Generated runtime children | AWCP controller | Deployment, Service, ServiceAccount, NetworkPolicy and status |

Terraform never reads or applies `gitops/`, `config/` or `examples/`; Argo CD
never creates Terraform resources. The independently versioned ownership choice
is recorded in [ADR-013](../../docs/adr/ADR-013-terraform-and-gitops-ownership.md).

## Local safe validation

The command below only downloads pinned tools, resolves the committed provider
lock, formats, validates and scans configuration. It does not select a backend,
authenticate to AWS, call `plan`, create a state file or apply anything.

```bash
make infra-verify
```

`terraform init -backend=false -lockfile=readonly` must succeed against the
committed `.terraform.lock.hcl`. Its AWS provider checksums cover the supported
macOS ARM64 developer and Linux AMD64 CI platforms. A provider/version or
supported-platform change is intentional only when the lock file changes in the
same review.

## Authorized apply handoff (not executed by this repository)

1. Create an approved, access-controlled Terraform state backend outside this
   repository. State can contain sensitive operational metadata; do not use the
   default local state for a shared or production environment.
2. Copy `terraform.tfvars.example` to an untracked `terraform.tfvars`, set only
   non-secret account/region/network values, and obtain temporary least-privilege
   AWS credentials through the organization-approved identity flow. The principal
   performing cluster creation needs the KMS permissions AWS documents for a
   customer-managed encryption key, including `kms:DescribeKey` and
   `kms:CreateGrant`; grant these outside this repository's EKS role.
3. Run `terraform plan` from a network path that can later reach the private EKS
   endpoint. Review the plan, cost impact (especially NAT), IAM policy ARN,
   regions/AZs, KMS deletion window and endpoint exposure before any apply.
4. After a separately authorized apply, grant named bootstrap principals through
   EKS access entries. Then bootstrap Argo CD and AWCP using the GitOps runbook;
   do not add Kubernetes or AWCP manifests to this Terraform directory.

No cloud account, backend, credential, Kubernetes context or apply operation is
used by local checks or GitHub Actions.

## Inputs and outputs

Required inputs are AWS region, cluster name, two or more availability zones,
and matching VPC/public/private CIDRs. `enable_nat_gateway` defaults to `false`
because it creates recurring cost. Outputs expose the cluster name, private API
endpoint, CA data, VPC/private subnet IDs and OIDC issuer; endpoint and CA data
are marked sensitive.

The baseline Kubernetes version follows the repository's `1.36` contract. AWS
region availability is checked only during an authorized plan/apply, so update
the input when AWS's supported EKS version set differs in the target region.

## References

- [Terraform version and provider requirements](https://developer.hashicorp.com/terraform/language/block/terraform)
- [Terraform validate](https://developer.hashicorp.com/terraform/cli/commands/validate)
- [EKS private API endpoint access](https://docs.aws.amazon.com/eks/latest/userguide/cluster-endpoint.html)
- [EKS access entries](https://docs.aws.amazon.com/eks/latest/userguide/access-entries.html)
- [EKS KMS envelope encryption](https://docs.aws.amazon.com/eks/latest/userguide/enable-kms.html)
