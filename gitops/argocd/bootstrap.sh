#!/usr/bin/env bash
# Installs only the Argo CD control plane into the currently selected cluster.
# It does not apply AWCP Applications, write credentials or run in CI.
set -euo pipefail
cd "$(dirname "$0")/../.."

for tool in kubectl helm; do command -v "$tool" >/dev/null || { echo "$tool is required" >&2; exit 1; }; done
test "$(kubectl config current-context)" != "" || { echo 'select a Kubernetes context first' >&2; exit 1; }

chart_version="$(awk '$1 == "helm:" { in_helm = 1; next } in_helm && $1 == "version:" { print $2; exit }' gitops/argocd/installation.lock.yaml)"
test -n "$chart_version" || { echo 'Helm chart version is missing from installation.lock.yaml' >&2; exit 1; }
helm upgrade --install argocd argo-cd \
  --repo https://argoproj.github.io/argo-helm \
  --version "$chart_version" \
  --namespace argocd --create-namespace \
  --values gitops/argocd/values.yaml \
  --wait --timeout 10m

echo 'Argo CD is installed. Review docs/gitops.md, then apply the AWCP Project and Applications explicitly.'
