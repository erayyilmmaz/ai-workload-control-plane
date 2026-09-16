#!/usr/bin/env bash
# Disposable real-cluster GitOps acceptance test. It consumes a public remote
# commit and never changes the caller's kubeconfig or Git repository.
set -euo pipefail
cd "$(dirname "$0")/../.."

repo_url="${1:?pass the public Git repository URL}"
revision="${2:?pass the pushed 40-character commit SHA}"
manager_image="${3:-awcp-manager:awcp-15}"
demo_image="${4:-awcp-demo:v1}"
if ! [[ "$revision" =~ ^[0-9a-f]{40}$ ]]; then
  echo 'revision must be a 40-character lowercase commit SHA' >&2
  exit 1
fi

for tool in kind kubectl; do test -x ".tools/bin/$tool" || bash hack/bootstrap-tools.sh "$tool"; done
kind="$PWD/.tools/bin/kind"
kubectl="$PWD/.tools/bin/kubectl"
scratch="$(mktemp -d "${TMPDIR:-/tmp}/awcp-gitops-e2e.XXXXXX")"
cluster="awcp-gitops-$(basename "$scratch" | tr '[:upper:].' '[:lower:]-')"
export KUBECONFIG="$scratch/kubeconfig"
created=false

cleanup() {
  result=$?
  trap - EXIT INT TERM
  if $created; then
    if test "$result" -ne 0; then
      echo "GitOps E2E failed; collecting safe diagnostics for $cluster" >&2
      "$kubectl" get pods -A || true
      "$kubectl" -n argocd get applications.argoproj.io || true
      "$kubectl" -n argocd get events --sort-by=.lastTimestamp || true
    fi
    "$kind" delete cluster --name "$cluster" || result=1
  fi
  rm -r "$scratch"
  exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

wait_application_sync() {
  local app="$1" attempt state
  for attempt in $(seq 1 300); do
    state="$("$kubectl" -n argocd get "application/$app" -o json 2>/dev/null || true)"
    if test -n "$state" && jq -e '.status.sync.status == "Synced"' <<<"$state" >/dev/null; then return 0; fi
    sleep 1
  done
  echo "Application $app did not become Synced" >&2
  return 1
}

wait_workload_ready() {
	local name="$1" replicas="$2" attempt state
	for attempt in $(seq 1 180); do
		state="$("$kubectl" -n awcp-workloads get "aiworkload/$name" -o json 2>/dev/null || true)"
		if test -n "$state" && jq -e --argjson replicas "$replicas" '(.spec.replicas == $replicas) and ([.status.conditions[]? | select(.type == "Ready" and .status == "True" and .reason == "WorkloadReady")] | length == 1)' <<<"$state" >/dev/null; then return 0; fi
		sleep 1
	done
	echo "Git-sourced AIWorkload $name did not become Ready" >&2
  return 1
}

if "$kind" get clusters | grep -Fxq "$cluster"; then echo 'refusing to reuse an existing cluster' >&2; exit 1; fi
for image in "$manager_image" "$demo_image"; do docker image inspect "$image" >/dev/null; done
created=true
"$kind" create cluster --name "$cluster" --kubeconfig "$KUBECONFIG" --config test/e2e/kind-config.yaml --image "$(jq -r '.kubernetes.kindNodeImage' toolchain.lock.json)" --wait 120s
"$kind" load docker-image "$manager_image" "$demo_image" --name "$cluster"

manifest="$scratch/argocd-install.yaml"
curl --fail --location --silent --show-error https://raw.githubusercontent.com/argoproj/argo-cd/v3.5.2/manifests/install.yaml > "$manifest"
test "$(shasum -a 256 "$manifest" | awk '{print $1}')" = '9a87f2b3e14c278f12501eb0ef5c3955b27cf05370ca425381c6a908cf85a5c5'
# The ApplicationSet CRD exceeds the client-side apply annotation limit on this
# Kubernetes baseline; this is the documented server-side CRD install path.
"$kubectl" create namespace argocd
"$kubectl" apply --server-side --force-conflicts -n argocd -f "$manifest"
"$kubectl" -n argocd rollout status deployment/argocd-server --timeout=300s
"$kubectl" -n argocd rollout status deployment/argocd-repo-server --timeout=300s
"$kubectl" -n argocd rollout status deployment/argocd-applicationset-controller --timeout=300s

kustomize=.tools/bin/kustomize-"$(jq -r '.tools.kustomize' toolchain.lock.json)"/kustomize
"$kustomize" build gitops/platform/base | "$kubectl" apply -f -
sed "s|repoURL: https://github.com/erayyilmmaz/ai-workload-control-plane.git|repoURL: $repo_url|; s|targetRevision: main|targetRevision: $revision|" gitops/argocd/applications/awcp-platform.yaml | "$kubectl" apply -f -
wait_application_sync awcp-platform
"$kubectl" -n awcp-system rollout status deployment/awcp-controller-manager --timeout=300s
sed -e "s|repoURL: https://github.com/erayyilmmaz/ai-workload-control-plane.git|repoURL: $repo_url|g" -e "s|revision: main|revision: $revision|g" -e "s|targetRevision: main|targetRevision: $revision|g" gitops/argocd/applications/awcp-environments.yaml | "$kubectl" apply -f -
for app in awcp-dev awcp-staging; do wait_application_sync "$app"; done
"$kubectl" -n argocd get application/awcp-prod -o json | jq -e '.spec.syncPolicy.automated.enabled == false' >/dev/null
# Production is intentionally not auto-synced. This is the explicit, auditable
# promotion action for the disposable reference cluster; production uses a reviewed Git revision.
"$kubectl" -n argocd patch application/awcp-prod --type=merge -p '{"operation":{"sync":{}}}'
wait_application_sync awcp-prod

for entry in 'dev 1' 'staging 2' 'prod 3'; do
	read -r environment replicas <<<"$entry"
	parent="gitops-demo-$environment"
	wait_workload_ready "$parent" "$replicas"
	"$kubectl" -n awcp-workloads get "aiworkload/$parent" -o json | jq -e --arg environment "$environment" '.spec.environment == $environment and .metadata.labels["platform.example.io/environment"] == $environment' >/dev/null
	child="awcp-$parent-$(printf '%s' "$parent" | shasum -a 256 | awk '{print substr($1, 1, 16)}')"
	for resource in "deployment/$child" "service/$child" "serviceaccount/$child" "networkpolicy/$child"; do
		"$kubectl" -n awcp-workloads get "$resource" -o json | jq -e --arg parent "$parent" --arg environment "$environment" '
      any(.metadata.ownerReferences[]?; .apiVersion == "platform.example.io/v1alpha1" and .kind == "AIWorkload" and .name == $parent) and
      ((.metadata.annotations["argocd.argoproj.io/tracking-id"] // "") == "") and
      .metadata.labels["platform.example.io/environment"] == $environment
    ' >/dev/null
	done
done
"$kubectl" -n awcp-workloads patch aiworkload/gitops-demo-dev --type=merge -p '{"spec":{"replicas":4}}'
wait_workload_ready gitops-demo-dev 1
"$kubectl" -n awcp-workloads get aiworkload/gitops-demo-dev -o json | jq -e '.spec.replicas == 1' >/dev/null
echo 'PASS: Argo CD rendered dev/staging/prod Applications from Git directories, required explicit prod promotion, and AWCP owned each environment child subtree'
