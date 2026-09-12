#!/usr/bin/env bash
# Manager/container smoke only; workload lifecycle E2E belongs to AWCP-14.
set -euo pipefail
cd "$(dirname "$0")/../.."
image="${1:-awcp-manager:awcp-3}"
for tool in kind kubectl; do
  test -x ".tools/bin/$tool" || bash hack/bootstrap-tools.sh "$tool"
done
kind="$PWD/.tools/bin/kind"
kubectl="$PWD/.tools/bin/kubectl"
scratch="$(mktemp -d "${TMPDIR:-/tmp}/awcp-smoke.XXXXXX")"
cluster="awcp-bootstrap-$(basename "$scratch" | tr '[:upper:].' '[:lower:]-')"
export KUBECONFIG="$scratch/kubeconfig"
created=false
cleanup() {
  result=$?
  trap - EXIT INT TERM
  if $created; then
    if [ "$result" -ne 0 ]; then
      "$kubectl" get pods -A || true
      "$kubectl" -n awcp-system describe deployment awcp-controller-manager || true
      "$kubectl" -n awcp-system logs deployment/awcp-controller-manager --all-containers || true
    fi
    if ! "$kind" delete cluster --name "$cluster"; then result=1; fi
  fi
  rm -r "$scratch"
  exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
if "$kind" get clusters | grep -Fxq "$cluster"; then
  echo 'Refusing to reuse an existing cluster' >&2
  exit 1
fi
test "$(docker image inspect "$image" --format '{{.Config.User}}')" = '65532:65532'
created=true
"$kind" create cluster --name "$cluster" --kubeconfig "$KUBECONFIG" \
  --image "$(jq -r '.kubernetes.kindNodeImage' toolchain.lock.json)" --wait 120s
"$kind" load docker-image "$image" --name "$cluster"
"$kubectl" apply -f dist/install.yaml
"$kubectl" -n awcp-system set image deployment/awcp-controller-manager "manager=$image"
"$kubectl" -n awcp-system rollout status deployment/awcp-controller-manager --timeout=180s
pod="$("$kubectl" -n awcp-system get pod -l app.kubernetes.io/name=awcp-controller-manager -o jsonpath='{.items[0].metadata.name}')"
"$kubectl" -n awcp-system get pod "$pod" -o json | jq -e '
  .spec.securityContext.runAsUser == 65532 and
  .spec.securityContext.runAsNonRoot == true and
  .spec.securityContext.seccompProfile.type == "RuntimeDefault" and
  .spec.containers[0].securityContext.allowPrivilegeEscalation == false and
  .spec.containers[0].securityContext.readOnlyRootFilesystem == true and
  .spec.containers[0].securityContext.capabilities.drop == ["ALL"] and
  .status.containerStatuses[0].ready == true and
  .status.containerStatuses[0].user.linux.uid == 65532'
for endpoint in healthz readyz; do
  test "$("$kubectl" get --raw "/api/v1/namespaces/awcp-system/pods/$pod:8081/proxy/$endpoint")" = ok
done
"$kubectl" -n awcp-system get lease awcp-controller.platform.example.io -o json | jq -e '.spec.holderIdentity | length > 0'
"$kubectl" apply -f config/samples/platform_v1alpha1_aiworkload.yaml
identity=system:serviceaccount:awcp-system:awcp-controller-manager
test "$("$kubectl" auth can-i get aiworkloads.platform.example.io -n awcp-workloads --as="$identity")" = yes
for rule in 'get secrets' 'create deployments.apps' 'update aiworkloads.platform.example.io'; do
  # Word splitting intentionally supplies verb/resource from these fixed cases.
  answer="$("$kubectl" auth can-i $rule -n awcp-workloads --as="$identity" || true)"
  test "$answer" = no
done
answer="$("$kubectl" auth can-i get aiworkloads.platform.example.io -n default --as="$identity" || true)"
test "$answer" = no
"$kubectl" -n awcp-system logs deployment/awcp-controller-manager --tail=30
echo 'PASS: container UID, restricted security, health/readiness, Lease and bootstrap RBAC'
