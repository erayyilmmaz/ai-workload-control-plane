#!/usr/bin/env bash
# Manager/container plus identity/Deployment/Service-contract smoke; full ready-pod traffic E2E belongs to AWCP-14.
set -euo pipefail
cd "$(dirname "$0")/../.."
image="${1:-awcp-manager:awcp-8}"
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
# A completed rollout may still list the old terminating Pod. Select the Ready
# Pod for the requested image, never the first item returned by the API.
pod="$("$kubectl" -n awcp-system get pod -l app.kubernetes.io/name=awcp-controller-manager -o json | jq -er --arg image "$image" '
  [.items[] | select(.metadata.deletionTimestamp == null)
   | select(any(.spec.containers[]; .name == "manager" and .image == $image))
   | select(any(.status.conditions[]?; .type == "Ready" and .status == "True"))]
  | if length == 1 then .[0].metadata.name else error("expected one Ready manager Pod for the requested image") end')"
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
workload="bootstrap-sample"
hash="$(printf '%s' "$workload" | shasum -a 256 | awk '{print substr($1, 1, 16)}')"
child="awcp-$workload-$hash"
"$kubectl" -n awcp-workloads wait --for=create "deployment/$child" --timeout=30s
"$kubectl" -n awcp-workloads wait --for=create "service/$child" --timeout=30s
workload_uid="$("$kubectl" -n awcp-workloads get "aiworkload/$workload" -o jsonpath='{.metadata.uid}')"
"$kubectl" -n awcp-workloads get "deployment/$child" -o json | jq -e --arg child "$child" --arg uid "$workload_uid" '
  .metadata.ownerReferences == [{apiVersion:"platform.example.io/v1alpha1", kind:"AIWorkload", name:"bootstrap-sample", uid:$uid, controller:true, blockOwnerDeletion:false}] and
  .spec.replicas == 1 and
  .spec.selector.matchLabels == {"app.kubernetes.io/instance":$child, "platform.example.io/workload-uid":$uid} and
  .spec.template.metadata.labels == (.spec.template.metadata.labels | . + {"app.kubernetes.io/instance":$child, "platform.example.io/workload-uid":$uid}) and
  .spec.template.spec.serviceAccountName == $child and
  .spec.template.spec.automountServiceAccountToken == false and
  ([.spec.template.spec.containers[] | select(.name == "workload")] | length == 1) and
  ([.spec.template.spec.containers[] | select(.name == "workload")][0] |
    .image == "ghcr.io/example/demo-agent:v1" and .imagePullPolicy == "IfNotPresent" and
    .ports == [{name:"http", containerPort:8080, protocol:"TCP"}] and
    .securityContext.runAsNonRoot == true and
    .securityContext.allowPrivilegeEscalation == false and
    .securityContext.capabilities.drop == ["ALL"] and
    .securityContext.seccompProfile.type == "RuntimeDefault")'
"$kubectl" -n awcp-workloads get "service/$child" -o json | jq -e --arg child "$child" --arg uid "$workload_uid" '
  .metadata.ownerReferences == [{apiVersion:"platform.example.io/v1alpha1", kind:"AIWorkload", name:"bootstrap-sample", uid:$uid, controller:true, blockOwnerDeletion:false}] and
  .spec.type == "ClusterIP" and
  .spec.clusterIP != "" and .spec.clusterIP != "None" and
  .spec.selector == {"app.kubernetes.io/instance":$child, "platform.example.io/workload-uid":$uid} and
  .spec.ports == [{name:"http", protocol:"TCP", port:80, targetPort:"http"}]'
endpoint="$child.awcp-workloads.svc:80"
i=0
while test "$i" -lt 30; do
  actual_endpoint="$("$kubectl" -n awcp-workloads get "aiworkload/$workload" -o jsonpath='{.status.endpoint}')"
  test "$actual_endpoint" = "$endpoint" && break
  i=$((i + 1))
  sleep 1
done
test "${actual_endpoint:-}" = "$endpoint"
"$kubectl" -n awcp-workloads get "serviceaccount/$child" -o json | jq -e --arg child "$child" --arg uid "$workload_uid" '
  .metadata.ownerReferences == [{apiVersion:"platform.example.io/v1alpha1", kind:"AIWorkload", name:"bootstrap-sample", uid:$uid, controller:true, blockOwnerDeletion:false}] and
  .automountServiceAccountToken == false and
  .secrets == null and .imagePullSecrets == null'
if "$kubectl" -n awcp-workloads get rolebinding -o json | jq -e --arg child "$child" '[.items[] | select(any(.subjects[]?; .kind == "ServiceAccount" and .name == $child))] | length == 0' >/dev/null; then :; else
  echo 'AWCP-8 must not grant workload ServiceAccounts through RoleBindings' >&2
  exit 1
fi
identity=system:serviceaccount:awcp-system:awcp-controller-manager
test "$("$kubectl" auth can-i get aiworkloads.platform.example.io -n awcp-workloads --as="$identity")" = yes
for rule in 'get secrets' 'create deployments.apps' 'create events.events.k8s.io'; do
  # Intentional splitting of resource/verb pairs.
  test "$("$kubectl" auth can-i $rule -n awcp-workloads --as="$identity")" = yes
done
test "$("$kubectl" auth can-i patch aiworkloads.platform.example.io --subresource=status -n awcp-workloads --as="$identity")" = yes
for rule in 'create secrets' 'delete deployments.apps' 'delete serviceaccounts' 'update aiworkloads.platform.example.io'; do
  # Word splitting intentionally supplies verb/resource from these fixed cases.
  answer="$("$kubectl" auth can-i $rule -n awcp-workloads --as="$identity" || true)"
  test "$answer" = no
done
workload_identity="system:serviceaccount:awcp-workloads:$child"
for rule in 'get secrets' 'get pods' 'create pods'; do
  answer="$("$kubectl" auth can-i $rule -n awcp-workloads --as="$workload_identity" || true)"
  test "$answer" = no
done
answer="$("$kubectl" auth can-i get aiworkloads.platform.example.io -n default --as="$identity" || true)"
test "$answer" = no
"$kubectl" -n awcp-system logs deployment/awcp-controller-manager --tail=30
echo 'PASS: manager security, health/readiness, Lease, identity/Deployment/Service contracts, endpoint and least-privilege RBAC'
