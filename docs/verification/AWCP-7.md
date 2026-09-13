# AWCP-7 — Service discovery and exposure lifecycle: execution evidence

Date: 2026-09-13

## Delivered scope

- `WorkloadBuilder` now emits a Deployment and an optional Service intent.
- The Service is cluster-local `ClusterIP`, with one TCP port named `http`.
  `spec.service.port` defaults to 80 and targets the named container port `http`.
- The builder owns selector and the whole port list, preserves API-assigned cluster
  allocation fields, and rejects headless/external exposure modes without delete/
  recreate.
- `enabled: false` produces the guarded optional-delete intent. The generic engine
  preserves foreign/stale-owner children; on disable the controller clears a stale
  endpoint even if deletion then reports a guarded conflict.
- After a successful enabled Service apply, status publishes
  `<child>.<namespace>.svc:<port>`. This is discovery data, not a readiness claim.

## Validation run

| Command | Result | What it covers |
| --- | --- | --- |
| `make verify` | Passed | Generate, build, vet, lint, full unit/envtest suite, generated-file stability and Kustomize render |
| `make test-race` | Passed | Full suite with Go race detector, including the new concurrent endpoint/status path |
| `DOCKER_CONFIG=... DOCKER_HOST=... make docker-build IMG=awcp-manager:awcp-7` | Passed | Local Linux ARM64 manager image build |
| `git diff --check` | Passed | No whitespace errors |

Focused real-API subtest `TestProductionDeployment/Service_port_drift_toggle_and_endpoint_lifecycle` passed. It proves Service selector/port drift repair, ClusterIP preservation across a port change, disable deletion plus endpoint clearing, and re-enable plus endpoint republishing. `TestServiceDriftAndAllocatedFields` proves preservation of `clusterIP`, `clusterIPs`, `ipFamilies` and `ipFamilyPolicy`; `TestServiceRejectsImmutableExposureModes` proves non-destructive rejection of NodePort, LoadBalancer, ExternalName and headless Service states. Failure classification includes the immutable Service allocation case.

The isolated kind smoke was extended to assert the live Service contract and status
endpoint. The command was launched with the locally built AWCP-7 image; this execution
environment did not return its final PASS/FAIL line before its tool session closed, so
it is not recorded as passing evidence here. No kind cluster remained afterward.

## Deliberate limits

Envtest runs an API server only: it has no kubelet, Service proxy, DNS, EndpointSlice
controller, Deployment controller, garbage collector or NetworkPolicy enforcement.
Therefore these results do **not** prove ready endpoints, DNS resolution or HTTP
traffic. AWCP-7 also deliberately does not create the dedicated ServiceAccount;
until AWCP-8, the generated Deployment can remain `FailedCreate`. A full runnable
workload and in-cluster DNS/HTTP lifecycle result require the later identity/demo
image E2E work. No hosted CI, image publication, deployment to a user cluster or
Jira mutation is claimed.

See [Service behavior](../service.md), [API contract](../api-contract.md) and
[reconciliation semantics](../reconciliation.md).
