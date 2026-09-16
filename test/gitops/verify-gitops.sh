#!/usr/bin/env bash
# Static GitOps contract guard. It renders only local sources; no cluster, token
# or Argo CD API is used by CI.
set -euo pipefail
cd "$(dirname "$0")/../.."

kustomize=.tools/bin/kustomize-"$(jq -r '.tools.kustomize' toolchain.lock.json)"/kustomize
test -x "$kustomize"

for file in gitops/README.md gitops/argocd/bootstrap.sh gitops/argocd/installation.lock.yaml gitops/argocd/values.yaml gitops/platform/base/project.yaml gitops/argocd/applications/awcp-platform.yaml gitops/argocd/applications/awcp-workloads.yaml gitops/workloads/base/gitops-demo.yaml docs/gitops.md docs/adr/ADR-011-gitops-ownership-boundary.md; do
  test -s "$file"
done

"$kustomize" build gitops/platform/base | grep -Fq 'kind: AppProject'
applications="$($kustomize build gitops/argocd/applications)"
printf '%s\n' "$applications" | grep -Fq 'name: awcp-platform'
printf '%s\n' "$applications" | grep -Fq 'name: awcp-workloads'
printf '%s\n' "$applications" | grep -Fq 'path: config/default'
printf '%s\n' "$applications" | grep -Fq 'path: gitops/workloads/base'
printf '%s\n' "$applications" | grep -Fq 'prune: false'
printf '%s\n' "$applications" | grep -Fq 'prune: true'
printf '%s\n' "$applications" | grep -Fq 'allowEmpty: false'
printf '%s\n' "$applications" | grep -Fq 'selfHeal: true'
"$kustomize" build gitops/workloads/base | grep -Fq 'kind: AIWorkload'
! rg -n '^kind: (Deployment|Service|ServiceAccount|NetworkPolicy)$' gitops/workloads
grep -Fq 'installManifestSHA256: 9a87f2b3e14c278f12501eb0ef5c3955b27cf05370ca425381c6a908cf85a5c5' gitops/argocd/installation.lock.yaml
grep -Fq 'version: 10.9.0' gitops/argocd/installation.lock.yaml
grep -Fq 'apply --server-side --force-conflicts' docs/gitops.md test/e2e/gitops-e2e.sh
grep -Fq 'create namespace argocd' test/e2e/gitops-e2e.sh
! rg -n -i '(password|token|clientsecret):\s*[^#[:space:]]' gitops/argocd
echo 'PASS: GitOps sources are renderable, bounded to parent AIWorkloads and free of committed credentials'
