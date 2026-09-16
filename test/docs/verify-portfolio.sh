#!/usr/bin/env bash
# Fast structural guard for the public starting path; runtime behavior belongs to E2E.
set -euo pipefail
cd "$(dirname "$0")/../.."

for file in examples/basic.yaml examples/with-secrets.yaml examples/network-policy.yaml demo/portfolio-demo.sh docs/demo.md docs/release.md docs/gitops.md docs/verification/AWCP-17.md docs/verification/AWCP-19.md docs/verification/AWCP-20.md docs/adr/ADR-009-v1-api-evolution.md docs/adr/ADR-010-optional-platform-capabilities.md docs/adr/ADR-011-gitops-ownership-boundary.md test/fixtures/v1/v0-compatible.yaml; do
  test -s "$file"
done
test -x demo/portfolio-demo.sh

for heading in 'Problem' 'Why this exists' 'Architecture' 'Quick Start' 'AIWorkload API' 'Reconciliation model' 'Drift recovery' 'Security model' 'Observability' 'Testing strategy' 'Architecture decisions' 'Limitations' 'Roadmap'; do
  grep -Fq "## $heading" README.md
done
for file in examples/basic.yaml examples/with-secrets.yaml examples/network-policy.yaml; do
  grep -Eq '^apiVersion: platform\.example\.io/v1alpha1$' "$file"
  grep -Eq '^kind: AIWorkload$' "$file"
  grep -Eq '^  namespace: awcp-workloads$' "$file"
  grep -Eq '^  image: awcp-demo:v1$' "$file"
done
grep -Fq 'demo-settings' examples/with-secrets.yaml
! grep -Eq '^(data|stringData):' examples/with-secrets.yaml
grep -Fq 'enabled: true' examples/network-policy.yaml
grep -Fq 'make quickstart' README.md
grep -Fq 'make portfolio-demo' README.md
grep -Fq 'v0.1.0' docs/release.md
grep -Fq 'published release' docs/release.md
for heading in 'Decision' 'Frozen V0 surface' 'Planned V1 surface' 'Optional dependency capability model' 'Compatibility matrix' 'CRD upgrade and rollback test plan' 'V1 non-goals'; do
  grep -Fq "## $heading" docs/v1-api-evolution.md
done
grep -Fq 'Kubernetes 1.37+' docs/v1-api-evolution.md
grep -Fq 'v0-compatible.yaml' docs/v1-api-evolution.md
grep -Fq 'V1 API Evolution and Migration Boundary' docs/adr/README.md
grep -Fq 'Optional Platform Capabilities' docs/adr/README.md
grep -Fq 'GitOps Ownership Boundary' docs/adr/README.md
for heading in 'Ownership model' 'Bootstrap and local demo' 'Sync, prune, self-heal and rollback' 'Observability and failure boundaries'; do
  grep -Fq "## $heading" docs/gitops.md
done
grep -Fq 'allowEmpty: false' docs/gitops.md
echo 'PASS: portfolio docs, V1 compatibility contract, examples, release preparation and demo entrypoint are present'
