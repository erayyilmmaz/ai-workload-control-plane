#!/usr/bin/env bash
# Real-Kubernetes acceptance test. It creates and deletes only its own kind cluster.
set -euo pipefail
cd "$(dirname "$0")/../.."
manager_image="${1:-awcp-manager:awcp-15}"
demo_v1="${2:-awcp-demo:v1}"
demo_v2="${3:-awcp-demo:v2}"

for tool in kind kubectl; do test -x ".tools/bin/$tool" || bash hack/bootstrap-tools.sh "$tool"; done
kind="$PWD/.tools/bin/kind"
kubectl="$PWD/.tools/bin/kubectl"
scratch="$(mktemp -d "${TMPDIR:-/tmp}/awcp-e2e.XXXXXX")"
cluster="awcp-e2e-$(basename "$scratch" | tr '[:upper:].' '[:lower:]-')"
export KUBECONFIG="$scratch/kubeconfig"
created=false
port_forward_pid=""

stop_port_forward() {
  if test -n "$port_forward_pid"; then kill "$port_forward_pid" 2>/dev/null || true; wait "$port_forward_pid" 2>/dev/null || true; port_forward_pid=""; fi
}
cleanup() {
  result=$?
  trap - EXIT INT TERM
  stop_port_forward
  if $created; then
    if test "$result" -ne 0; then
      echo "E2E failed; collecting safe diagnostics for $cluster" >&2
      "$kubectl" get pods -A || true
      "$kubectl" -n awcp-system get events --sort-by=.lastTimestamp || true
      "$kubectl" -n awcp-workloads get events --sort-by=.lastTimestamp || true
      "$kubectl" -n awcp-system logs deployment/awcp-controller-manager --all-containers || true
    fi
    "$kind" delete cluster --name "$cluster" || result=1
  fi
  rm -r "$scratch"
  exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

wait_workload_ready() {
  local replicas="$1" state attempt
  for attempt in $(seq 1 90); do
    state="$("$kubectl" -n awcp-workloads get aiworkload/lifecycle-demo -o json 2>/dev/null || true)"
    if test -n "$state" && jq -e --argjson replicas "$replicas" '
      .status.observedGeneration == .metadata.generation and .status.desiredReplicas == $replicas and .status.readyReplicas == $replicas and
      ([.status.conditions[]? | select(.type == "Ready" and .status == "True" and .reason == "WorkloadReady")] | length == 1) and
      ([.status.conditions[]? | select(.type == "Progressing" and .status == "False")] | length == 1) and
      ([.status.conditions[]? | select(.type == "Degraded" and .status == "False")] | length == 1)
    ' <<<"$state" >/dev/null; then return 0; fi
    sleep 1
  done
  echo "AIWorkload did not become Ready at $replicas replicas" >&2; return 1
}
wait_secret_failure() {
  local state attempt
  for attempt in $(seq 1 60); do
    state="$("$kubectl" -n awcp-workloads get aiworkload/lifecycle-demo -o json 2>/dev/null || true)"
    if test -n "$state" && jq -e '
      .status.observedGeneration == .metadata.generation and
      ([.status.conditions[]? | select(.type == "Ready" and .status == "False" and .reason == "SecretNotFound")] | length == 1) and
      ([.status.conditions[]? | select(.type == "Degraded" and .status == "True" and .reason == "SecretNotFound")] | length == 1)
    ' <<<"$state" >/dev/null; then return 0; fi
    sleep 1
  done
  echo 'Missing Secret did not produce SecretNotFound conditions' >&2; return 1
}
wait_new_uid() {
  local resource="$1" old_uid="$2" uid attempt
  for attempt in $(seq 1 90); do
    uid="$("$kubectl" -n awcp-workloads get "$resource" -o jsonpath='{.metadata.uid}' 2>/dev/null || true)"
    if test -n "$uid" && test "$uid" != "$old_uid"; then return 0; fi
    sleep 1
  done
  echo "$resource was not recreated" >&2; return 1
}
assert_version() {
  local expected="$1" port actual="" attempt
  : > "$scratch/port-forward.log"
  "$kubectl" -n awcp-workloads port-forward service/"$child" 0:80 >"$scratch/port-forward.log" 2>&1 & port_forward_pid=$!
  for attempt in $(seq 1 30); do
    port="$(sed -nE 's/.*127\.0\.0\.1:([0-9]+).*/\1/p' "$scratch/port-forward.log" | head -n 1)"
    if test -n "$port"; then actual="$(curl --fail --silent --show-error --max-time 5 "http://127.0.0.1:$port/version" || true)"; test "$actual" = "$expected" && break; fi
    kill -0 "$port_forward_pid" 2>/dev/null || { cat "$scratch/port-forward.log" >&2; return 1; }
    sleep 1
  done
  stop_port_forward
  test "$actual" = "$expected" || { echo "Service returned ${actual:-no response}, expected $expected" >&2; return 1; }
}
assert_metrics() {
  local manager_pod token port snapshot attempt
  manager_pod="$("$kubectl" -n awcp-system get pod -l app.kubernetes.io/name=awcp-controller-manager -o json | jq -er '[.items[] | select(.metadata.deletionTimestamp == null) | select(any(.status.conditions[]?; .type == "Ready" and .status == "True"))] | if length == 1 then .[0].metadata.name else error("expected one Ready manager Pod") end')"
  token="$("$kubectl" -n awcp-system create token awcp-e2e-metrics-reader --duration=10m)"
  : > "$scratch/metrics-port-forward.log"
  "$kubectl" -n awcp-system port-forward "pod/$manager_pod" 0:8443 >"$scratch/metrics-port-forward.log" 2>&1 & port_forward_pid=$!
  for attempt in $(seq 1 45); do
    port="$(sed -nE 's/.*127\.0\.0\.1:([0-9]+).*/\1/p' "$scratch/metrics-port-forward.log" | head -n 1)"
    if test -n "$port"; then
      snapshot="$(curl --insecure --fail --silent --show-error --max-time 5 -H "Authorization: Bearer $token" "https://127.0.0.1:$port/metrics" || true)"
      if awk '$1 == "awcp_controller_managed_workloads" && $2 == 1 { m=1 } $1 == "awcp_controller_ready_workloads" && $2 == 1 { r=1 } $1 == "awcp_controller_degraded_workloads" && $2 == 0 { d=1 } END { exit !(m && r && d) }' <<<"$snapshot"; then stop_port_forward; return 0; fi
    fi
    sleep 1
  done
  stop_port_forward; echo 'Authenticated metrics did not converge' >&2; return 1
}

if "$kind" get clusters | grep -Fxq "$cluster"; then echo 'Refusing to reuse an existing cluster' >&2; exit 1; fi
for image in "$manager_image" "$demo_v1" "$demo_v2"; do test "$(docker image inspect "$image" --format '{{.Config.User}}')" = '65532:65532'; done
created=true
"$kind" create cluster --name "$cluster" --kubeconfig "$KUBECONFIG" --config test/e2e/kind-config.yaml --image "$(jq -r '.kubernetes.kindNodeImage' toolchain.lock.json)" --wait 120s
"$kind" load docker-image "$manager_image" "$demo_v1" "$demo_v2" --name "$cluster"
make deploy DEPLOY_IMG="$manager_image"
"$kubectl" apply -f examples/observability/metrics-reader-clusterrole.yaml
"$kubectl" apply -f test/e2e/metrics-reader.yaml
"$kubectl" -n awcp-workloads create secret generic demo-settings --from-literal=marker=synthetic
"$kubectl" apply -f test/e2e/demo-workload.yaml
workload=lifecycle-demo
hash="$(printf '%s' "$workload" | shasum -a 256 | awk '{print substr($1, 1, 16)}')"
child="awcp-$workload-$hash"

# Create, rollout, scale, service traffic.
"$kubectl" -n awcp-workloads rollout status "deployment/$child" --timeout=180s
wait_workload_ready 1
for owned in "deployment/$child" "service/$child" "serviceaccount/$child" "networkpolicy/$child"; do "$kubectl" -n awcp-workloads get "$owned" >/dev/null; done
assert_version v1
"$kubectl" -n awcp-workloads patch aiworkload/lifecycle-demo --type=merge -p '{"spec":{"image":"awcp-demo:v2"}}'
"$kubectl" -n awcp-workloads rollout status "deployment/$child" --timeout=180s
wait_workload_ready 1
assert_version v2
"$kubectl" -n awcp-workloads patch aiworkload/lifecycle-demo --type=merge -p '{"spec":{"replicas":2}}'
"$kubectl" -n awcp-workloads rollout status "deployment/$child" --timeout=180s
wait_workload_ready 2
assert_version v2

# Child drift before and after manager restart.
deployment_uid="$("$kubectl" -n awcp-workloads get "deployment/$child" -o jsonpath='{.metadata.uid}')"
"$kubectl" -n awcp-workloads delete "deployment/$child"
wait_new_uid "deployment/$child" "$deployment_uid"
"$kubectl" -n awcp-workloads rollout status "deployment/$child" --timeout=180s
wait_workload_ready 2
"$kubectl" -n awcp-system rollout restart deployment/awcp-controller-manager
"$kubectl" -n awcp-system rollout status deployment/awcp-controller-manager --timeout=180s
for resource in "service/$child" "serviceaccount/$child" "networkpolicy/$child"; do old_uid="$("$kubectl" -n awcp-workloads get "$resource" -o jsonpath='{.metadata.uid}')"; "$kubectl" -n awcp-workloads delete "$resource"; wait_new_uid "$resource" "$old_uid"; done
wait_workload_ready 2
assert_version v2

# Secret payload is not queried or printed.
"$kubectl" -n awcp-workloads delete secret/demo-settings
wait_secret_failure
"$kubectl" -n awcp-workloads create secret generic demo-settings --from-literal=marker=synthetic
wait_workload_ready 2
assert_metrics

# Normal package removal preserves the CRD, workload tree, namespace and user Secret.
make undeploy
"$kubectl" get crd/aiworkloads.platform.example.io >/dev/null
"$kubectl" -n awcp-workloads get "aiworkload/$workload" "deployment/$child" "service/$child" "serviceaccount/$child" "networkpolicy/$child" secret/demo-settings >/dev/null
if "$kubectl" -n awcp-system get deployment/awcp-controller-manager >/dev/null 2>&1; then
  echo 'make undeploy left the manager Deployment behind' >&2
  exit 1
fi

# GC still removes owned objects after an explicit parent deletion.
"$kubectl" -n awcp-workloads delete aiworkload/lifecycle-demo --wait=false
for owned in "deployment/$child" "service/$child" "serviceaccount/$child" "networkpolicy/$child"; do "$kubectl" -n awcp-workloads wait --for=delete "$owned" --timeout=90s; done
"$kubectl" -n awcp-workloads wait --for=delete aiworkload/lifecycle-demo --timeout=90s
for kind_name in replicasets pods; do
  for attempt in $(seq 1 90); do remaining="$("$kubectl" -n awcp-workloads get "$kind_name" -l "app.kubernetes.io/instance=$child" -o name 2>/dev/null || true)"; test -z "$remaining" && break; sleep 1; done
  test -z "${remaining:-}" || { echo "$kind_name remained after parent deletion" >&2; exit 1; }
done
"$kubectl" -n awcp-workloads get secret/demo-settings -o json | jq -e '.metadata.ownerReferences == null and .data.marker != null' >/dev/null
echo 'PASS: package deploy/undeploy, traffic, v1-to-v2 rollout, scale, drift, Secret recovery, restart, metrics and garbage collection'
