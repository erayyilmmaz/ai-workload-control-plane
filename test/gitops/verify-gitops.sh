#!/usr/bin/env bash
# Static GitOps contract guard. It renders only local sources; no cluster, token
# or Argo CD API is used by CI.
set -euo pipefail
cd "$(dirname "$0")/../.."

kustomize=.tools/bin/kustomize-"$(jq -r '.tools.kustomize' toolchain.lock.json)"/kustomize
test -x "$kustomize"

for file in gitops/README.md gitops/argocd/bootstrap.sh gitops/argocd/installation.lock.yaml gitops/argocd/values.yaml gitops/platform/base/project.yaml gitops/argocd/applications/awcp-platform.yaml gitops/argocd/applications/awcp-environments.yaml gitops/workloads/base/gitops-demo.yaml gitops/workloads/environments/dev/kustomization.yaml gitops/workloads/environments/staging/kustomization.yaml gitops/workloads/environments/prod/kustomization.yaml docs/gitops.md docs/adr/ADR-011-gitops-ownership-boundary.md docs/adr/ADR-012-environment-delivery-model.md; do
  test -s "$file"
done

"$kustomize" build gitops/platform/base | grep -Fq 'kind: AppProject'
applications="$($kustomize build gitops/argocd/applications)"
printf '%s\n' "$applications" | grep -Fq 'name: awcp-platform'
printf '%s\n' "$applications" | grep -Fq 'kind: ApplicationSet'
printf '%s\n' "$applications" | grep -Fq 'name: awcp-environments'
printf '%s\n' "$applications" | grep -Fq 'path: config/default'
printf '%s\n' "$applications" | grep -Fq 'path: gitops/workloads/environments/*'
printf '%s\n' "$applications" | grep -Fq 'prune: false'
printf '%s\n' "$applications" | grep -Fq 'prune: true'
printf '%s\n' "$applications" | grep -Fq 'allowEmpty: false'
printf '%s\n' "$applications" | grep -Fq 'selfHeal: true'
printf '%s\n' "$applications" | grep -Fq 'eq .path.basename "prod"'
for environment in dev staging prod; do
	rendered="$("$kustomize" build "gitops/workloads/environments/$environment")"
	printf '%s\n' "$rendered" | grep -Fq 'kind: AIWorkload'
	printf '%s\n' "$rendered" | grep -Fq "platform.example.io/environment: $environment"
	printf '%s\n' "$rendered" | grep -Fq "environment: $environment"
done
! rg -n '^kind: (Deployment|Service|ServiceAccount|NetworkPolicy)$' gitops/workloads
grep -Fq 'installManifestSHA256: 9a87f2b3e14c278f12501eb0ef5c3955b27cf05370ca425381c6a908cf85a5c5' gitops/argocd/installation.lock.yaml
grep -Fq 'version: 10.9.0' gitops/argocd/installation.lock.yaml
grep -Fq 'apply --server-side --force-conflicts' docs/gitops.md test/e2e/gitops-e2e.sh
grep -Fq 'create namespace argocd' test/e2e/gitops-e2e.sh
! rg -n -i '(password|token|clientsecret):\s*[^#[:space:]]' gitops/argocd
echo 'PASS: GitOps environment sources are renderable, parent-only, isolated by overlay and free of committed credentials'
