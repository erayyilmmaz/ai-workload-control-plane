# AWCP-17 — documentation, portfolio demo and V0 release preparation: execution evidence

## Scope delivered

- A clean-user README with problem, architecture, Quick Start, API, examples,
  reconciliation, drift, security, observability, test, limitation and roadmap
  boundaries.
- Runnable local `basic`, Secret-reference and NetworkPolicy examples without
  committed Secret payloads.
- A narrated 16-step portfolio entrypoint that reuses the real kind lifecycle
  harness rather than creating a separate happy-path demo.
- Release preparation: semantic versioning policy, candidate checklist, notes
  template and explicit distinction between preparation and publication.

## Execution record

On 2026-09-13, `make quickstart` created a new local `awcp-quickstart` kind
cluster, built and loaded the local manager and demo images, deployed the canonical
Kustomize package, applied `examples/basic.yaml`, and observed `basic-demo`
`Ready=True`. The same cluster then accepted `examples/with-secrets.yaml` after a
synthetic `demo-settings` Secret was created, and `examples/network-policy.yaml`.
Both became Ready; only the NetworkPolicy example produced a NetworkPolicy object.

`make verify`, `make verify-ci`, `make vuln`, `make release-bundle` and
`make verify-package` passed locally. `govulncheck` found zero reachable
vulnerabilities; it retained two imported-package findings with no call path.

`make portfolio-demo` passed on local macOS ARM64 Docker/kind. Its recorded
16-step output includes Deployment and Service creation, Ready/HTTP v1, Deployment
drift repair with a new UID, v1-to-v2 rollout/HTTP v2, SecretNotFound degradation,
Secret restore without a CR change, authenticated metrics, parent deletion and
owned-child garbage collection while the user Secret survived. Its random temporary
kind cluster was deleted after success.

Hosted CI for this documentation/demo change, GitHub tag/release, registry image,
merge ruleset and release artifacts are separate evidence states. No tag, release,
registry image or ruleset is implied by this local record.

## Known boundaries

Grafana remains optional infrastructure and is not installed by the base package.
The standard kind demo proves NetworkPolicy object generation, not CNI enforcement.
No `v0.1.0` tag, GitHub Release, production image, SBOM, signing or provenance is
claimed until an authorized action produces its own artifact evidence.
