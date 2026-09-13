# AWCP-16 — CI quality gates and supply-chain hygiene: execution evidence

## Scope delivered

- A secrets-free GitHub Actions PR/main workflow with read-only permissions,
  pinned external action commits, per-job timeouts and cancellation of stale runs.
- Stable jobs cover format, lint, vet, generated code/manifests, unit, envtest,
  build, Docker build, supply-chain policy and kind E2E.
- Dependency/tool caching is keyed by the exact lock and module checksum files;
  bootstrap checksum verification still executes after cache restoration.
- `govulncheck` is independently pinned and wired into the supply-chain job.
- Dependabot proposes, but does not merge, weekly Go module and action updates.
- Workflow guard and package/E2E checks catch malformed local CI boundaries before
  a remote run. Required-check/ruleset activation is separately documented.

## Execution record

On 2026-09-13, `make verify-ci` passed the local workflow boundary assertions:
pinned action SHAs, `contents: read`, no secret/pull-request-target path, stable
job names, timeouts, cache and concurrency. `make tidy` was then run as a clean
module-drift check.

The first `make vuln` found reachable `GO-2026-6061` through
`google.golang.org/grpc v1.79.3`; the fixed minimum was v1.82.1. The dependency
was upgraded to v1.82.1, which also moved compatible transitive OpenTelemetry,
genproto, oauth2 and gonum versions. A second `make vuln` completed with **zero
reachable vulnerabilities**. It did report two imported-package findings without
a call path; these are scanner observations, not a clean bill of health for every
possible deployment.

`make verify`, `make verify-package`, `make verify-ci`, `make vuln` and `make e2e`
then passed locally. The E2E rebuilt the manager with the updated dependency graph,
completed the package deploy/undeploy lifecycle, and deleted its isolated kind
cluster. Local success is not hosted GitHub Actions, branch-protection, image
publication or release evidence.

## Known boundaries

No hosted workflow run, active GitHub ruleset, SBOM, image-signing/provenance,
container vulnerability scan or Linux AMD64 execution is claimed until observed
with its own artifact/run evidence.
