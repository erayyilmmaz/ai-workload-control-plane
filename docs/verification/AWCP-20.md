# AWCP-20 — GitOps repository model and Argo CD integration: execution evidence

## Scope delivered

- Added a GitOps source layout with a restricted `AppProject`, separate platform
  and workload Applications, and a parent-only `AIWorkload` source.
- Kept the canonical AWCP install package in `config/default`; no duplicate
  controller package is maintained under `gitops/`.
- Pinned the official Argo CD v3.5.2 install manifest SHA-256 and matching Helm
  chart 10.9.0. The included Helm values are a credential-free local/reference
  profile, not production TLS/SSO/HA configuration.
- Enabled automated sync and self-heal for platform and workloads; platform prune
  is deliberately disabled, while workload prune is enabled with
  `allowEmpty: false`.
- Added CI-safe static rendering/ownership/credential checks and an opt-in
  disposable kind + Argo CD acceptance test. CI receives no Argo CD API token,
  cluster credential or apply step.

## Real-cluster evidence

On 2026-09-16, `make gitops-e2e` ran against public commit
`9a1ca20d3a2ebe009d150397163144447c87197e` on local macOS ARM64 Docker/kind.
It created and later deleted an isolated `awcp-gitops-*` cluster. The run
checksum-verified and server-side applied the Argo CD v3.5.2 manifest, installed
the `awcp-platform` Project/Application, then let Argo CD fetch the pinned remote
commit and synchronize the AWCP platform package and `gitops-demo` parent.

The AWCP manager became Ready, generated its Deployment, Service, ServiceAccount
and NetworkPolicy, and each child had the `gitops-demo` owner reference with no
Argo CD tracking annotation. Patching the Git-managed parent to `replicas: 2`
was self-healed back to the Git value `1` without declaring those children in the
Argo source. The temporary kind cluster was deleted after the passing run.

The executable run does not make a temporary remote Git deletion commit, so it
does not claim a live prune demonstration. The committed Application and static
guard prove workload `prune: true` plus `allowEmpty: false`; the documented
runbook requires a reviewed Git deletion and confirmation that only the parent
and its UID-owned children disappear. Platform prune remains disabled.

## Compatibility fixes discovered by the real API

The first installer run exposed the Kubernetes client-side apply annotation limit
on Argo CD's large ApplicationSet CRD; AWCP now uses checksum-verified
server-side apply with an explicit `argocd` namespace for the manifest path.
The Argo CD 3.5.2 CRD also rejected the newer `retry.backoff.refresh` field, so it
was removed while preserving the supported bounded retry/backoff policy. These
are installation/schema compatibility fixes, not changes to AWCP workload
behavior.

## Local validation

- `helm template` rendered chart 10.9.0 with the committed values.
- `make test-unit`, `make test-envtest`, `make gitops-verify`,
  `make verify-docs` and `make verify-ci` passed.
- `make verify`, `make e2e`, `make gitops-verify`, `make verify-docs`,
  `make verify-ci`, `make release-bundle`, `make verify-package` and `make vuln`
  passed after the final dependency update. `govulncheck` reported zero reachable
  vulnerabilities (two imported-package findings had no call path).

Hosted CI, a production Argo installation, private repository credentials, Git
webhooks, SSO/TLS/HA, remote Git deletion/prune execution and production image
selection remain separate evidence states.
