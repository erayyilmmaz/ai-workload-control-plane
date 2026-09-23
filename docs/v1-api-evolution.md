# V1 API evolution and compatibility contract

Status: accepted baseline for AWCP-19, 2026-09-14. This document designs the V1
surface; it does **not** claim that the listed features are implemented or that
their dependencies are installed.

## Decision

V1 keeps `platform.example.io/v1alpha1` as the single served and storage version
while the API is explicitly alpha. V1 feature stories may add only optional,
structural, backward-compatible fields. An absent V1 field must preserve the V0
behavior exactly. There is no conversion webhook, automatic spec rewrite or
manager-side default write in AWCP-19.

`v1beta1` is required before a breaking semantic change, a field rename/removal,
a different default for an existing field, an incompatible list/map ownership
change, or a production API-stability commitment. The triggering story must add a
versioned migration plan and conversion decision before serving a second version.

## Frozen V0 surface

The V0 `v1alpha1` fields and their defaults remain the contract in
[API contract](api-contract.md): `image`, `replicas`, `container`, `resources`,
`health`, `service`, `secretRefs` and `network`. In particular, omission remains
different from explicit `replicas: 0` or `enabled: false`; V1 must not normalize
those values into the primary spec.

The regression fixture
[`v0-compatible.yaml`](../test/fixtures/v1/v0-compatible.yaml) is submitted to a
real API server with strict field validation. It deliberately contains no V1
fields. Future V1 stories must keep that fixture valid and must add their own
presence/absence/defaulting cases.

## Planned V1 surface

None of these fields is served by AWCP-19. They are reserved designs, introduced
only in their owning story after schema, builder, status and lifecycle tests exist.

| Planned field | Owning story | Contract direction | Absent-field behavior |
| --- | --- | --- | --- |
| `spec.environment` | AWCP-21 | Implemented optional logical environment identity; it labels AWCP-owned children and never selects a namespace or cluster | Existing namespace/manifest behavior is unchanged |
| `spec.tenant` | AWCP-23 | Implemented optional tenant identity; it must match the namespace-local GitOps tenant profile before AWCP creates children | Omission preserves V0 behavior and does not read a tenant profile |
| `spec.autoscaling` | AWCP-27 | HPA intent, min/max/metrics/behavior | `replicas` remains controller-owned as in V0 |
| `spec.exposure` | AWCP-26 | ClusterLocal or external HTTPRoute intent | Existing ClusterIP Service behavior is unchanged |
| `spec.delivery` | AWCP-29 | Rolling, Canary or BlueGreen rollout intent | Existing Deployment rolling update is retained |
| `spec.externalSecrets` | AWCP-25 | Implemented namespace-local `{externalSecret,targetSecret}` ESO references without values; only Ready SecretStore/ExternalSecret targets are consumed | Existing direct `secretRefs` semantics are retained |
| `spec.availability` | AWCP-28 | Workload PDB intent | No PDB is created |
| `spec.policy` | AWCP-24 | Implemented optional `baseline` or `restricted` platform profile; never arbitrary CEL | Omission preserves V0 behavior outside a namespace selected by a policy binding |

`environment`, `tenant`, and `policy` remain declarative identity, not authority. GitOps
placement, namespace creation, AppProject/RBAC, quota and policy binding belong to
their platform stories. A workload cannot choose a broader namespace, Gateway,
SecretStore or permission by supplying one of these fields.

## Defaults, deprecation and migration

- New optional fields default to omission unless their owning feature has a
  server-side structural default with a documented V0-equivalent meaning.
- A field is deprecated only with a replacement, an explicit warning/migration
  window, examples, strict-validation fixtures and a release-note entry.
- V1 does not silently remove, rename or reinterpret V0 fields. A user keeps the
  old manifest valid until the separately versioned migration boundary.
- Unknown fields remain governed by Kubernetes structural-schema pruning and the
  client field-validation mode. Strict clients receive an error; this is not a
  compatibility escape hatch.

## Status and failure contract

V0 continues to publish only the `Ready`, `Progressing` and `Degraded` condition
types. A V1 feature with an absent optional dependency uses the existing condition
set with a precise reason such as `DependencyUnavailable`; it does not claim the
dependency is healthy or manufacture a new successful state. Discovery errors are
transient observations (`ReconcileFailed`/backoff), not proof that a dependency is
absent. Feature-specific status fields and condition types require their own API
story and bounded-size review.

AWCP-23 adds the bounded `TenantReady` condition only when `spec.tenant` is
present. `TenantConfigured=True` proves that the namespace-local profile matched;
`TenantNotConfigured`, `TenantMismatch`, and `TenantProfileInvalid` are terminal
configuration observations. A quota rejection is reported as the retryable,
sanitized `QuotaExceeded` reason on the existing operational conditions. Neither
condition exposes ConfigMap, Secret, or Kubernetes API error payloads.

Status never contains Secret values, credentials, raw provider errors, bearer
tokens or unbounded revision history.

## Optional dependency capability model

The code-level catalog in `internal/capability` names the discoverable APIs needed
by future feature stories. Detection is **on demand in the feature reconciler**,
not manager startup. Therefore an installation without Gateway API, External
Secrets Operator, Argo Rollouts, Prometheus Operator or an external metrics API
still starts and continues reconciling V0 workloads.

| Dependency | Capability decision | Missing behavior |
| --- | --- | --- |
| Argo CD | GitOps delivery dependency, not an AWCP runtime API dependency | AWCP still reconciles a CR delivered by another mechanism; AWCP-20 owns GitOps demo/status integration |
| Gateway API | Discover `Gateway` and `HTTPRoute` v1 APIs when `exposure` is requested | Do not create a route; report actionable dependency failure |
| External Secrets Operator | Discover namespaced `SecretStore` and `ExternalSecret` v1 APIs when requested | Do not read/provider-fetch a Secret; report actionable dependency failure |
| Argo Rollouts | Discover `Rollout` when Canary/BlueGreen is requested | Preserve/follow declared Rolling fallback policy; do not create both a Deployment and Rollout |
| HPA external metrics | Discover external metrics API only when external metric or scale-to-zero is requested | HPA feature reports unavailable; ordinary static replicas continue |
| Prometheus/OpenCost/OTel | Health/configuration is owned by their integration stories, not inferred from a CRD alone | Telemetry or cost data may be unavailable; core reconcile continues |
| ValidatingAdmissionPolicy | Cluster-version/API check at policy installation time | Policy manifests are not installed on unsupported clusters; no custom webhook fallback is implied |

The catalog classifies a missing API as `Unavailable` and a discovery transport or
authorization failure as `Unknown`. The latter must use bounded retry and safe
diagnostics. Feature stories must sanitize provider errors before status/Event/log
publication.

## Compatibility matrix

| Surface | AWCP-19 evidence | Minimum / prerequisite | Scope boundary |
| --- | --- | --- | --- |
| V0 AIWorkload API | Real envtest and kind evidence on Kubernetes 1.36.x | Existing pinned 1.36.2 envtest / 1.36.4 kind node | The only runtime baseline proven by this story |
| ValidatingAdmissionPolicy | Design and manifest compatibility decision | Kubernetes 1.30+; stable API | AWCP-24 installs/tests policy later |
| Gateway API HTTPRoute | Design only | Gateway API v1 CRDs plus a platform Gateway implementation | AWCP-26 owns route implementation and traffic proof |
| External Secrets | Namespaced ESO v1 contract and kind proof | ESO v2.11.0 test install; Ready SecretStore/ExternalSecret and target Secret metadata | AWCP-25 proves target rotation rollout, not provider credential validity |
| Argo Rollouts | Design only | Rollouts CRD/controller | AWCP-29 owns progressive delivery proof |
| HPA scale-to-zero | Design only | Kubernetes 1.37+, object or external metric; 1.36 baseline cannot claim it | AWCP-27 owns rebaseline and E2E proof |
| Prometheus, OTel, OpenCost | Optional integration design only | Independently installed/configured platform dependency | AWCP-31/32 own dashboard, alert and cost evidence |

The current V0 tool lock is intentionally not re-pinned in AWCP-19. Changing the
Kubernetes/client/envtest/kind minor is a later explicit baseline change that must
update the lock, generated manifests, local validation and hosted CI together.

## CRD upgrade and rollback test plan

1. Install the current single-version CRD and create the V0 compatibility fixture
   with strict field validation.
2. Upgrade the CRD with an additive V1 field; re-read the existing object and
   prove existing V0 values/defaults/status remain unchanged.
3. Create both absent-field and explicit-field V1 fixtures; verify structural
   validation and controller behavior independently.
4. Roll back only when no persisted object relies on the added field. Otherwise
   reject the rollback and use the documented forward migration.
5. Before any served `v1beta1`, add conversion/migration fixtures for both
   directions and test a version-skewed manager/API-server sequence.

No CRD downgrade, cluster restore or production migration is performed by
AWCP-19. Those executable procedures belong to AWCP-34.

## V1 non-goals

V1 does not introduce multi-cluster federation, a custom Kubernetes/GPU scheduler,
a custom secrets manager, custom gateway/ingress or autoscaler, service mesh,
billing, frontend portal, full Backstage/IDP, model-serving runtime, LLM gateway,
vector database or agent orchestration engine.

## References

- [Kubernetes CRD versioning](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/)
- [ValidatingAdmissionPolicy](https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/)
- [Horizontal Pod Autoscaler scale-to-zero](https://kubernetes.io/docs/concepts/workloads/autoscaling/horizontal-pod-autoscale/)
- [Gateway API HTTPRoute](https://gateway-api.sigs.k8s.io/reference/api-types/httproute/)
- [External Secrets SecretStore](https://external-secrets.io/main/api/secretstore/)
