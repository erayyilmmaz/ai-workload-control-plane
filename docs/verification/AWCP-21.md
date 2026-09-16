# AWCP-21 — Multi-environment GitOps delivery and ApplicationSet: execution evidence

## Scope delivered

- Replaced the single workload Application with an Argo CD `ApplicationSet` using
  a Git directory generator over `gitops/workloads/environments/*`.
- Added parent-only Kustomize overlays for `dev`, `staging` and `prod`. Their
  generated Applications have independent source paths, parent names and replica
  values: 1, 2 and 3 respectively.
- Added optional `AIWorkload.spec.environment`: a bounded logical identity, not a
  namespace, cluster or permission selector. When present, AWCP places the same
  bounded label on its owned children; when absent, no environment label remains.
- Kept the local/reference profile in the existing single watched namespace with
  unique parent names. The documented `awcp-<environment>-workloads` convention
  is deferred until manager scope/RBAC and tenancy work exists.
- Dev/staging use automated sync and self-heal. Prod disables both and requires
  an explicit approved sync. Production Git/registry promotion controls are
  documented, not claimed as configured in this public reference repository.

## Real-cluster evidence

On 2026-09-16, `make gitops-e2e` ran against public commit
`199e9994b369e1b42b786d3bc9546edd8fffd57f` on local macOS ARM64 Docker/kind.
It created and deleted an isolated `awcp-gitops-*` cluster, checksum-verified and
server-side applied the pinned Argo CD v3.5.2 manifest, then synchronized the
AWCP platform from the pinned remote commit.

The ApplicationSet generated `awcp-dev`, `awcp-staging` and `awcp-prod` from the
three Git directories. Dev and staging synchronized automatically. The test
confirmed prod's automated/self-heal settings were false, then submitted a single
explicit Argo Application sync operation. All three parent workloads became Ready
with their expected environment and replica values. Each generated Deployment,
Service, ServiceAccount and NetworkPolicy had the matching parent owner reference,
the environment label and no Argo tracking annotation. Patching dev replicas to 4
was self-healed to the Git value 1. The temporary cluster was removed after pass.

The test does not create a remote Git promotion or deletion commit. It therefore
does not prove protected branches, signed tags, immutable image digest availability,
webhooks, remote prune, separate environment namespaces, tenancy policy or a
production Argo installation.

## Local validation

- `make generate manifests`, `make test-unit` and `make test-envtest` passed.
- `make gitops-verify`, `make verify-docs`, `helm template` for pinned chart
  10.9.0, shell syntax checks and whitespace checks passed.
- `make gitops-e2e` passed after commit/push; CI remains credential-free and does
  not contact an Argo API or cluster.
