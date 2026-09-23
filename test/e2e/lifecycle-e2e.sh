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
      "$kubectl" -n awcp-tenant-alpha get events --sort-by=.lastTimestamp || true
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

demo_step() {
  printf '\n== AWCP portfolio demo %s/16: %s ==\n' "$1" "$2"
}

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
wait_tenant_secret_failure() {
  local state attempt
  for attempt in $(seq 1 60); do
    state="$("$kubectl" -n awcp-tenant-alpha get aiworkload/tenant-isolation -o json 2>/dev/null || true)"
    if test -n "$state" && jq -e '
      .status.observedGeneration == .metadata.generation and
      ([.status.conditions[]? | select(.type == "TenantReady" and .status == "True" and .reason == "TenantConfigured")] | length == 1) and
      ([.status.conditions[]? | select(.type == "Degraded" and .status == "True" and .reason == "SecretNotFound")] | length == 1)
    ' <<<"$state" >/dev/null; then return 0; fi
    sleep 1
  done
  echo 'Tenant workload did not prove local Secret isolation' >&2; return 1
}
wait_tenant_ready() {
  local state attempt
  for attempt in $(seq 1 90); do
    state="$("$kubectl" -n awcp-tenant-alpha get aiworkload/tenant-isolation -o json 2>/dev/null || true)"
    if test -n "$state" && jq -e '
      .status.observedGeneration == .metadata.generation and .status.readyReplicas == 1 and
      ([.status.conditions[]? | select(.type == "TenantReady" and .status == "True" and .reason == "TenantConfigured")] | length == 1) and
      ([.status.conditions[]? | select(.type == "Ready" and .status == "True" and .reason == "WorkloadReady")] | length == 1)
    ' <<<"$state" >/dev/null; then return 0; fi
    sleep 1
  done
  echo 'Tenant workload did not become Ready' >&2; return 1
}
wait_tenant_quota_released() {
  local pods quota attempt
  for attempt in $(seq 1 90); do
    pods="$("$kubectl" -n awcp-tenant-alpha get pods -o json 2>/dev/null || true)"
    quota="$("$kubectl" -n awcp-tenant-alpha get resourcequota/awcp-tenant-quota -o json 2>/dev/null || true)"
    if test -n "$pods" && test -n "$quota" && jq -e '
      [.items[] | select(.metadata.deletionTimestamp == null)] | length == 0
    ' <<<"$pods" >/dev/null && jq -e '
      (.status.used."requests.cpu" // "0") == "0" and
      (.status.used."limits.cpu" // "0") == "0" and
      (.status.used.pods // "0") == "0"
    ' <<<"$quota" >/dev/null; then return 0; fi
    sleep 1
  done
  echo 'Tenant workload quota usage was not released before quota admission test' >&2; return 1
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

install_external_secrets_operator() {
  local lock="test/e2e/external-secrets.lock.json" bundle expected actual url deployment
  bundle="$scratch/external-secrets.yaml"
  url="$(jq -er '.url' "$lock")"
  expected="$(jq -er '.sha256' "$lock")"
  curl --fail --silent --show-error --location "$url" -o "$bundle"
  actual="$(shasum -a 256 "$bundle" | awk '{print $1}')"
  test "$actual" = "$expected" || { echo 'External Secrets Operator manifest checksum mismatch' >&2; return 1; }
  "$kubectl" apply --server-side --force-conflicts -f "$bundle"
  for deployment in external-secrets external-secrets-webhook external-secrets-cert-controller; do
    "$kubectl" -n default rollout status "deployment/$deployment" --timeout=240s
  done
}

wait_external_secret_ready() {
  local attempt state
  for attempt in $(seq 1 90); do
    state="$("$kubectl" -n awcp-workloads get externalsecret/awcp-e2e-settings-sync -o json 2>/dev/null || true)"
    if test -n "$state" && jq -e '
      [.status.conditions[]? | select(.type == "Ready" and .status == "True")] | length == 1
    ' <<<"$state" >/dev/null; then return 0; fi
    sleep 1
  done
  echo 'ExternalSecret did not become Ready' >&2; return 1
}

wait_resource_version_change() {
  local resource="$1" old="$2" current attempt
  for attempt in $(seq 1 90); do
    current="$("$kubectl" -n awcp-workloads get "$resource" -o jsonpath='{.metadata.resourceVersion}' 2>/dev/null || true)"
    if test -n "$current" && test "$current" != "$old"; then return 0; fi
    sleep 1
  done
  echo "$resource resourceVersion did not change" >&2; return 1
}

wait_external_secret_rollout() {
  local deployment="$1" old="$2" current attempt
  for attempt in $(seq 1 90); do
    current="$("$kubectl" -n awcp-workloads get "deployment/$deployment" -o jsonpath='{.spec.template.metadata.annotations.platform\.example\.io/external-secret-revision}' 2>/dev/null || true)"
    if test -n "$current" && test "$current" != "$old"; then return 0; fi
    sleep 1
  done
  echo 'AWCP did not roll out after ExternalSecret target metadata changed' >&2; return 1
}

if "$kind" get clusters | grep -Fxq "$cluster"; then echo 'Refusing to reuse an existing cluster' >&2; exit 1; fi
for image in "$manager_image" "$demo_v1" "$demo_v2"; do test "$(docker image inspect "$image" --format '{{.Config.User}}')" = '65532:65532'; done
created=true
"$kind" create cluster --name "$cluster" --kubeconfig "$KUBECONFIG" --config test/e2e/kind-config.yaml --image "$(jq -r '.kubernetes.kindNodeImage' toolchain.lock.json)" --wait 120s
"$kind" load docker-image "$manager_image" "$demo_v1" "$demo_v2" --name "$cluster"
make deploy DEPLOY_IMG="$manager_image"
"$kubectl" apply -f examples/observability/metrics-reader-clusterrole.yaml
"$kubectl" apply -f test/e2e/metrics-reader.yaml

# AWCP-25: execute the checksum-verified official ESO manifest on this disposable
# kind cluster, then prove provider sync and target metadata rotation without
# printing or reading Secret data.
install_external_secrets_operator
"$kubectl" apply -f test/e2e/external-secret-store.yaml
"$kubectl" -n awcp-workloads wait --for=condition=Ready secretstore/awcp-e2e-fake-store --timeout=120s
"$kubectl" -n awcp-workloads wait --for=create secret/awcp-e2e-settings --timeout=120s
wait_external_secret_ready
"$kubectl" apply -f test/e2e/external-secret-workload.yaml
external_workload=external-secret-rotation
external_hash="$(printf '%s' "$external_workload" | shasum -a 256 | awk '{print substr($1, 1, 16)}')"
external_child="awcp-$external_workload-$external_hash"
"$kubectl" -n awcp-workloads rollout status "deployment/$external_child" --timeout=180s
external_secret_rv="$("$kubectl" -n awcp-workloads get secret/awcp-e2e-settings -o jsonpath='{.metadata.resourceVersion}')"
external_revision="$("$kubectl" -n awcp-workloads get "deployment/$external_child" -o jsonpath='{.spec.template.metadata.annotations.platform\.example\.io/external-secret-revision}')"
test -n "$external_revision"
"$kubectl" -n awcp-workloads patch externalsecret/awcp-e2e-settings-sync --type=json -p='[{"op":"replace","path":"/spec/data/0/remoteRef/version","value":"v2"}]'
wait_resource_version_change secret/awcp-e2e-settings "$external_secret_rv"
wait_external_secret_ready
wait_external_secret_rollout "$external_child" "$external_revision"
"$kubectl" -n awcp-workloads rollout status "deployment/$external_child" --timeout=180s
"$kubectl" -n awcp-workloads delete aiworkload/external-secret-rotation --wait=false
"$kubectl" -n awcp-workloads wait --for=delete aiworkload/external-secret-rotation --timeout=90s
"$kubectl" -n awcp-workloads wait --for=delete "deployment/$external_child" --timeout=90s
"$kubectl" -n awcp-workloads get externalsecret/awcp-e2e-settings-sync secret/awcp-e2e-settings >/dev/null

# Tenant acceptance: all resources are predeclared by config/default; AWCP only
# watches the fixed namespaces and creates namespace-local workload children.
for tenant in alpha bravo charlie; do
  namespace="awcp-tenant-$tenant"
  "$kubectl" -n "$namespace" get configmap/awcp-tenant-profile resourcequota/awcp-tenant-quota limitrange/awcp-tenant-limits >/dev/null
done
test "$("$kubectl" auth can-i create aiworkloads.platform.example.io --as=tenant-alpha --as-group=awcp:tenant-alpha-developers -n awcp-tenant-alpha)" = yes
test "$("$kubectl" auth can-i get secrets --as=tenant-alpha --as-group=awcp:tenant-alpha-developers -n awcp-tenant-bravo)" = no
test "$("$kubectl" auth can-i create aiworkloads.platform.example.io --as=tenant-alpha --as-group=awcp:tenant-alpha-developers -n awcp-tenant-bravo)" = no
if "$kubectl" apply -f test/e2e/tenant-policy-violation.yaml; then
  echo 'Restricted policy accepted a workload selecting baseline' >&2; exit 1
fi
"$kubectl" -n awcp-tenant-bravo create secret generic tenant-only-secret --from-literal=marker=bravo
"$kubectl" apply -f test/e2e/tenant-alpha-workload.yaml
wait_tenant_secret_failure
"$kubectl" -n awcp-tenant-alpha create secret generic tenant-only-secret --from-literal=marker=alpha
tenant_workload=tenant-isolation
tenant_hash="$(printf '%s' "$tenant_workload" | shasum -a 256 | awk '{print substr($1, 1, 16)}')"
tenant_child="awcp-$tenant_workload-$tenant_hash"
"$kubectl" -n awcp-tenant-alpha rollout status "deployment/$tenant_child" --timeout=180s
wait_tenant_ready
test "$("$kubectl" auth can-i get secrets --as=system:serviceaccount:awcp-tenant-alpha:"$tenant_child" -n awcp-tenant-alpha)" = no
"$kubectl" -n awcp-tenant-alpha delete aiworkload/tenant-isolation --wait=false
"$kubectl" -n awcp-tenant-alpha wait --for=delete aiworkload/tenant-isolation --timeout=90s
"$kubectl" -n awcp-tenant-alpha wait --for=delete "deployment/$tenant_child" --timeout=90s
wait_tenant_quota_released
if "$kubectl" apply -f test/e2e/tenant-limitrange-violation.yaml; then
  echo 'LimitRange accepted a Pod over the tenant maximum' >&2; exit 1
fi
"$kubectl" apply -f test/e2e/tenant-quota-pod-1.yaml
"$kubectl" apply -f test/e2e/tenant-quota-pod-2.yaml
if "$kubectl" apply -f test/e2e/tenant-quota-pod-3.yaml; then
  echo 'ResourceQuota accepted a third 1-CPU Pod over the small profile ceiling' >&2; exit 1
fi
"$kubectl" -n awcp-tenant-alpha delete pod/tenant-quota-pod-1 pod/tenant-quota-pod-2 --ignore-not-found
demo_step 1 'create AIWorkload'
"$kubectl" -n awcp-workloads create secret generic demo-settings --from-literal=marker=synthetic
"$kubectl" apply -f test/e2e/demo-workload.yaml
workload=lifecycle-demo
hash="$(printf '%s' "$workload" | shasum -a 256 | awk '{print substr($1, 1, 16)}')"
child="awcp-$workload-$hash"

# Create, rollout, scale, service traffic.
demo_step 2 'owned Deployment appears'
"$kubectl" -n awcp-workloads wait --for=create "deployment/$child" --timeout=90s
demo_step 3 'owned Service appears'
"$kubectl" -n awcp-workloads wait --for=create "service/$child" --timeout=90s
demo_step 4 'workload becomes Ready and serves v1'
"$kubectl" -n awcp-workloads rollout status "deployment/$child" --timeout=180s
wait_workload_ready 1
for owned in "deployment/$child" "service/$child" "serviceaccount/$child" "networkpolicy/$child"; do "$kubectl" -n awcp-workloads get "$owned" >/dev/null; done
assert_version v1
deployment_uid="$("$kubectl" -n awcp-workloads get "deployment/$child" -o jsonpath='{.metadata.uid}')"
demo_step 5 'manually delete the owned Deployment'
"$kubectl" -n awcp-workloads delete "deployment/$child"
demo_step 6 'wait for reconciliation to restore a new Deployment UID'
wait_new_uid "deployment/$child" "$deployment_uid"
"$kubectl" -n awcp-workloads rollout status "deployment/$child" --timeout=180s
wait_workload_ready 1
assert_version v1
demo_step 7 'change workload image from v1 to v2'
"$kubectl" -n awcp-workloads patch aiworkload/lifecycle-demo --type=merge -p '{"spec":{"image":"awcp-demo:v2"}}'
demo_step 8 'wait for rollout and verify Service HTTP v2'
"$kubectl" -n awcp-workloads rollout status "deployment/$child" --timeout=180s
wait_workload_ready 1
assert_version v2
"$kubectl" -n awcp-workloads patch aiworkload/lifecycle-demo --type=merge -p '{"spec":{"replicas":2}}'
"$kubectl" -n awcp-workloads rollout status "deployment/$child" --timeout=180s
wait_workload_ready 2
assert_version v2

# Child drift before and after manager restart.
"$kubectl" -n awcp-system rollout restart deployment/awcp-controller-manager
"$kubectl" -n awcp-system rollout status deployment/awcp-controller-manager --timeout=180s
for resource in "service/$child" "serviceaccount/$child" "networkpolicy/$child"; do old_uid="$("$kubectl" -n awcp-workloads get "$resource" -o jsonpath='{.metadata.uid}')"; "$kubectl" -n awcp-workloads delete "$resource"; wait_new_uid "$resource" "$old_uid"; done
wait_workload_ready 2
assert_version v2

# Secret payload is not queried or printed.
demo_step 9 'remove the required user-owned Secret'
"$kubectl" -n awcp-workloads delete secret/demo-settings
demo_step 10 'observe SecretNotFound degradation'
wait_secret_failure
demo_step 11 'restore the Secret without changing the AIWorkload'
"$kubectl" -n awcp-workloads create secret generic demo-settings --from-literal=marker=synthetic
demo_step 12 'wait for Ready recovery'
wait_workload_ready 2
demo_step 13 'optional Grafana view (dashboard import is documented; base install does not deploy Grafana)'
demo_step 14 'read authenticated controller metrics with least privilege'
assert_metrics

# Normal package removal preserves the CRD, workload tree, namespace and user Secret.
echo 'Additional safety check: normal package removal preserves workload data and the CRD.'
make undeploy
"$kubectl" get crd/aiworkloads.platform.example.io >/dev/null
"$kubectl" -n awcp-workloads get "aiworkload/$workload" "deployment/$child" "service/$child" "serviceaccount/$child" "networkpolicy/$child" secret/demo-settings >/dev/null
if "$kubectl" -n awcp-system get deployment/awcp-controller-manager >/dev/null 2>&1; then
  echo 'make undeploy left the manager Deployment behind' >&2
  exit 1
fi

# GC still removes owned objects after an explicit parent deletion.
demo_step 15 'delete AIWorkload'
"$kubectl" -n awcp-workloads delete aiworkload/lifecycle-demo --wait=false
demo_step 16 'wait for owned children to disappear and verify user Secret survives'
for owned in "deployment/$child" "service/$child" "serviceaccount/$child" "networkpolicy/$child"; do "$kubectl" -n awcp-workloads wait --for=delete "$owned" --timeout=90s; done
"$kubectl" -n awcp-workloads wait --for=delete aiworkload/lifecycle-demo --timeout=90s
for kind_name in replicasets pods; do
  for attempt in $(seq 1 90); do remaining="$("$kubectl" -n awcp-workloads get "$kind_name" -l "app.kubernetes.io/instance=$child" -o name 2>/dev/null || true)"; test -z "$remaining" && break; sleep 1; done
  test -z "${remaining:-}" || { echo "$kind_name remained after parent deletion" >&2; exit 1; }
done
"$kubectl" -n awcp-workloads get secret/demo-settings -o json | jq -e '.metadata.ownerReferences == null and .data.marker != null' >/dev/null
echo 'PASS: package deploy/undeploy, traffic, v1-to-v2 rollout, scale, drift, Secret recovery, restart, metrics and garbage collection'
