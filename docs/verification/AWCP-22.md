# AWCP-22 — Terraform reference infrastructure: execution evidence

## Scope delivered

- Added a single AWS/EKS reference under `infra/terraform`, with pinned
  Terraform `1.16.2` and AWS provider `6.64.0`.
- Declared a VPC, public/private subnet and route baseline, private EKS endpoint,
  least-privilege EKS cluster role, Kubernetes Secret KMS envelope encryption,
  control-plane logs and optional disabled-by-default NAT egress.
- Added a committed provider lock, pinned Trivy `0.74.0`, a safe
  `make infra-verify` command, and a read-only `infra-validate` GitHub Actions
  job. It has no cloud credential, backend, plan or apply step.
- Recorded the Terraform/GitOps/AWCP ownership boundary in ADR-013 and supplied
  a no-credential deployment handoff.

## Local validation

On 2026-09-16, macOS ARM64 validation used checksum-verified project-local
Terraform and Trivy binaries:

```text
terraform fmt -check -recursive                 PASS
terraform init -backend=false -lockfile=readonly PASS
terraform validate                              PASS
trivy config HIGH,CRITICAL                      PASS (0 findings)
```

The committed provider lock includes the supported macOS ARM64 and Linux AMD64
package checksums; subsequent verification is lockfile-readonly. This evidence
does not claim AWS credentials, a remote backend, `terraform plan`,
`terraform apply`, EKS creation, Argo bootstrap or a production deployment.

## Boundaries and next handoff

This reference does not manage Kubernetes manifests or AWCP workloads. Terraform
owns infrastructure; Git/Argo owns parent manifests; AWCP owns child resources.
Production state, network path to a private API, EKS access entries, node pools,
workload identity and cloud-account cost approval remain external, authorized
steps.

## Hosted CI evidence

GitHub Actions run
[35441831583](https://github.com/erayyilmmaz/ai-workload-control-plane/actions/runs/35441831583)
completed successfully for commit
`ce4c668d27388c2ee2fdded211f876be291d5399` on 2026-09-19. All 12 jobs passed,
including Linux AMD64 `infra-validate`, envtest and the disposable kind E2E job.
This is hosted CI evidence only; it does not create or alter cloud infrastructure.
