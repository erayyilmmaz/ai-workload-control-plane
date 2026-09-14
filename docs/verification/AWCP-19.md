# AWCP-19 — V1 baseline, API evolution and compatibility contract: execution evidence

## Scope delivered

- Frozen the existing `platform.example.io/v1alpha1` V0 surface as the V1
  compatibility boundary and added a strict real-API regression fixture for a
  V0-only manifest.
- Chosen additive, optional `v1alpha1` fields for V1 feature stories; a served
  `v1beta1` is reserved for an explicit breaking migration or stability boundary.
- Recorded the API decision and optional-platform capability decision in ADR-009
  and ADR-010, including defaults, deprecation, status, upgrade and rollback
  rules.
- Added a pure, on-demand optional-capability catalog. It does not perform
  manager-startup discovery and classifies a missing API separately from a
  discovery/authorization failure.
- Added envtest proof that the manager elects and continues to start when the
  optional Gateway API, External Secrets, Argo Rollouts, external metrics and
  Prometheus APIs are absent.

## Decisions and boundaries

`v1alpha1` remains the sole served/storage version in this step. AWCP-19 does
not add any V1 field to the CRD, change a V0 default, install an optional platform
dependency, enable a conversion webhook, or re-pin the Kubernetes/toolchain
baseline. Consequently, existing V0 manifests retain their behavior and V1
feature implementation/proof remains owned by AWCP-20 onward.

The current proven runtime baseline is still envtest 1.36.2 and kind node 1.36.4.
Gateway, External Secrets, Argo Rollouts, Prometheus/OpenCost/OTel and the
Kubernetes 1.37 external-metrics scale-to-zero path are documented compatibility
decisions, not runtime claims from this story.

## Local validation

On 2026-09-14, after the final code/docs changes:

- `make test-unit` passed, including the optional-capability catalog tests.
- `make verify-docs` passed, including the V1 contract/ADR structural guard.
- `make test-envtest` passed: bootstrap integration (2 specs), API contract,
  production deployment and reconciliation suites.

The standalone `go test ./test/integration/...` command intentionally requires
`KUBEBUILDER_ASSETS`; the project make target supplies its pinned envtest assets.
This is a test invocation boundary, not an API or manager failure.

Hosted CI, a release/tag/image, a V1 CRD migration and a Kubernetes 1.37
rebaseline are separate future evidence states.
