# AWCP-8 — Workload identity, Secret references and least privilege: execution evidence

Date: 2026-09-13

## Delivered scope

- `WorkloadBuilder` emits a dedicated ServiceAccount before Deployment and Service.
  It is current-UID-owned, has deterministic identity metadata and
  `automountServiceAccountToken: false`; no workload RBAC binding is created.
- Deployment remains explicitly bound to this identity and independently disables
  its token mount.
- Secret prerequisite validation requests `PartialObjectMetadata` only. Missing
  references produce generic `SecretNotFound` conditions; API errors retain their
  real retry category. Secret names/data are not added to status, Events or errors.
- The existing namespace-scoped Secret metadata watch/index now drives missing,
  restore, delete and second restore reconciliation without a parent spec change.
- Manager RBAC remains namespaced and contains no wildcard or Secret write/delete
  verbs. The smoke contract checks that the workload identity has no Secret/Pod
  permissions and no generated RoleBinding.

## Validation

| Command | Result | Coverage |
| --- | --- | --- |
| `make test-unit` | Passed | Pure identity mapping, metadata-only Secret reads, safe errors and API-error distinction |
| `KUBEBUILDER_ASSETS=... go test -count=1 -v ./test/reconciliation` | Passed | Real API ServiceAccount creation and Secret missing → create → delete → restore lifecycle |
| `make verify` | Passed | Full build/lint/unit/envtest/generated/render validation |
| `make test-race` | Passed | Full race detector validation |
| `DOCKER_CONFIG=... DOCKER_HOST=... make docker-build IMG=awcp-manager:awcp-8` | Passed | Local Linux ARM64 manager image for commit `80b1e7a` |

The real-API `TestProductionDeployment/Secret_missing_restore_delete_and_restore_update_only_safe_status` test uses a synthetic sentinel Secret value and asserts that it is absent from serialized parent status. `TestSecretValidationUsesMetadataAndSafeMissingError` asserts a `PartialObjectMetadata` request rather than a typed Secret object. `TestSecretValidationPreservesAPIErrors` proves Forbidden is not reclassified as missing.

The isolated kind smoke was updated to check the live ServiceAccount contract, the
absence of generated workload RoleBindings, and denied Secret/Pod permissions for the
workload identity. Its command was launched with the AWCP-8 image, but this execution
environment closed its tool session before returning the final PASS/FAIL line; it is
therefore not recorded as passing evidence. No kind cluster remained afterward.

## Limits

`PartialObjectMetadata` reduces controller payload handling but does not make
Kubernetes Secret RBAC field-scoped: `get/list/watch` still authorizes data access at
the API boundary. Envtest has no kubelet, scheduler, Service proxy, DNS, EndpointSlice
controller or credential-revocation semantics. No claim is made that a running process
loses an already loaded environment value after Secret deletion, that Secret data
updates restart Pods, or that a workload is Ready. NetworkPolicy is AWCP-9; full
readiness and status reduction are AWCP-10.

See [identity/security behavior](../identity-security.md),
[ADR-005](../adr/ADR-005-security-and-secret-boundaries.md), and the official
[ServiceAccount](https://kubernetes.io/docs/concepts/security/service-accounts/),
[Secret](https://kubernetes.io/docs/concepts/configuration/secret/) and
[RBAC](https://kubernetes.io/docs/concepts/security/rbac-good-practices/) guidance.
