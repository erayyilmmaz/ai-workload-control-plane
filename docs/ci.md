# CI quality gates and supply-chain hygiene — AWCP-16

`.github/workflows/ci.yml` runs on pull requests and pushes to `main`. It is a
test-only workflow: `permissions` is `contents: read`, credentials are not
persisted, no environment or secret is referenced, and `pull_request_target` is
not used. This lets fork pull requests run the same checks without a publish
credential or paid cloud account.

## Stable checks

The following job names are intentionally stable. They are the candidates for
required status checks after a repository ruleset is configured:

| Job | Gate |
| --- | --- |
| `format` | Formatting diff check |
| `lint` / `vet` | Static analysis |
| `generate-check` / `manifest-check` | Generated DeepCopy and CRD/RBAC drift, including untracked files |
| `unit-test` / `envtest` | Fast/public-documentation guard and API-server-backed suites |
| `build` / `docker-build` | Binary and digest-pinned container build |
| `supply-chain` | Module tidiness, govulncheck, release-package and workflow-boundary checks |
| `e2e` | Full disposable kind lifecycle, package deploy and safe undeploy |

Every job has a timeout. Concurrency cancels stale runs for the same pull request
or ref, but never combines different PRs. The composite bootstrap action restores
only dependency/tool caches whose key is derived from `toolchain.lock.json` and
`go.sum`; every run still verifies downloaded binary checksums before use.

## Required-check configuration

Workflow presence does not itself block a merge. A repository administrator must
configure a GitHub ruleset/branch protection rule for `main` and select at least:

```text
format, lint, vet, generate-check, manifest-check,
unit-test, envtest, build, docker-build, supply-chain, e2e
```

Require the branch to be up to date if that is the repository policy. Do not claim
that merges are blocked until the ruleset is visibly active and one hosted PR run
has produced these check names. Keep administrative bypass policy outside this
repository and review it with repository owners.

## Dependency and action policy

- Go runtime pins, generator/linter versions, kind/node image and container base
  image digests remain in `toolchain.lock.json`, `go.sum` and Dockerfiles. Update
  them together with compatibility notes and relevant local/CI tests.
- Dependabot proposes weekly `gomod` and GitHub Action updates. It does not merge
  or publish automatically.
- Third-party GitHub Actions must use a full immutable commit SHA. The workflow
  currently pins checkout and cache; changing an action requires checking the tag
  to commit mapping and updating this document's evidence if the policy changes.
- `make vuln` pins `govulncheck` independently and queries the Go vulnerability
  database. It needs network access and reports vulnerabilities reachable from
  this module's code; it is not an SBOM, container scan, pentest or guarantee that
  an indirect dependency is safe in every deployment.
- Docker images are built and labelled in CI but never pushed. Publishing, signing,
  provenance attestation and a release are separate authorized operations.

## Local CI-equivalent commands

```bash
make verify
make verify-package
make verify-ci
make vuln
make e2e
```

Passing these commands is not hosted-CI evidence. The workflow was subsequently
observed passing on GitHub-hosted Linux AMD64 for commit `a6dbc76`, including the
kind E2E job; see [AWCP-16 execution evidence](verification/AWCP-16.md). This
does not configure required checks or block merges.
