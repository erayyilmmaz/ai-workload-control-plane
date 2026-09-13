#!/usr/bin/env bash
# Fast structural guard for the public starting path; runtime behavior belongs to E2E.
set -euo pipefail
cd "$(dirname "$0")/../.."

for file in examples/basic.yaml examples/with-secrets.yaml examples/network-policy.yaml demo/portfolio-demo.sh docs/demo.md docs/release.md docs/verification/AWCP-17.md; do
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
echo 'PASS: portfolio docs, examples, release preparation and demo entrypoint are present'
