# V0 release preparation — AWCP-17

This document prepares a release; it does not create a Git tag, GitHub Release or
registry image. Those are external, publishable actions and require explicit
maintainer authorization plus the actual evidence recorded below.

## Versioning policy

Tags use `vMAJOR.MINOR.PATCH` under [Semantic Versioning 2.0.0](https://semver.org/).
While the public API is `v1alpha1`, compatibility is intentionally conservative:

- `v0.MINOR.0`: may add or change alpha API behavior; document migrations and
  regenerate CRDs/examples.
- `v0.MINOR.PATCH`: bug/security/documentation correction without an intended API
  contract change.
- `v1.0.0`: only after an explicit compatibility and support-policy decision; it is
  not implied by this repository state.

Pre-release tags such as `v0.1.0-rc.1` are allowed for review. A GitHub Release is
attached to a tag, while image deployment must use the verified immutable image
digest, not a mutable tag.

## Candidate `v0.1.0` checklist

1. Start from a clean, reviewed commit on `main`; record its full SHA.
2. Confirm the hosted CI run for that SHA is green and required merge rules, if
   adopted by repository owners, are visibly active.
3. Run `make verify`, `make verify-ci`, `make vuln`, `make e2e`,
   `make release-bundle` and `make verify-package`; record command output, platform
   and completion status. Do not represent local output as hosted evidence.
4. Build the manager with `VERSION=v0.1.0` and the exact revision; publish it only
   to an authorized registry and record the resulting `repo@sha256:...` digest.
5. Review release notes, supported Kubernetes baseline, V0 limitations, upgrade and
   safe-removal instructions. Do not claim SBOM, signing, provenance or a container
   scan unless their actual artifacts are attached.
6. Create an annotated `v0.1.0` tag at the verified SHA, then create a **draft
   release**. Attach `awcp-crds.yaml`, `awcp-operator.yaml`,
   `awcp-operator-uninstall.yaml` and `SHA256SUMS` from `dist/release/`.
7. Recheck every attachment checksum, the tag target, release notes and immutable
   image digest. Publish only after a maintainer approves all of them.
8. After publishing, add the actual release URL, tag SHA, image digest, workflow
   URL and artifact checksums to the release evidence. Without these, this remains
   release preparation rather than a published release.

GitHub releases package a tag with release notes and assets; drafts let maintainers
attach and review assets before publication. See GitHub's
[release guidance](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository).

## Release-note template

```markdown
# AI Workload Control Plane v0.1.0

## Scope

- Namespaced `AIWorkload` controller for Deployment, Service, ServiceAccount and
  ingress-only NetworkPolicy reconciliation.

## Install

Deploy the manager using `<registry>@sha256:<digest>` and the attached Kustomize
bundle. See the installation guide.

## Verification

- Commit: `<full SHA>`
- Hosted CI: `<workflow URL>`
- Kubernetes baseline: `<exact supported version>`
- Manager image: `<repo@sha256:digest>`
- Bundle checksums: attached `SHA256SUMS`

## Limitations

- Alpha API; no tenant isolation, egress isolation, Secret rotation or CNI
  enforcement guarantee.
- No claim of SBOM, signing, provenance or container scan unless linked here.
```

## Explicit non-actions

This repository has no published release at AWCP-17 completion. Do not run a tag,
`gh release create`, image push or destructive CRD cleanup as a checklist shortcut.
Normal `make undeploy` is safe manager removal; CRD deletion is an independent,
destructive administrator decision described in [installation](installation.md).
