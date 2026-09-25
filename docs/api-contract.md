# AIWorkload API and status contract

The V0 `v1alpha1` surface in this document is frozen for V1 compatibility. See
[the AWCP-19 API evolution contract](v1-api-evolution.md) before adding a field,
changing a default or deciding a new served version.

Types, structural schema, defaults and admission tests were implemented in AWCP-4.
AWCP-7 implements the Service endpoint portion; AWCP-8 implements missing-Secret
prerequisites; AWCP-10 implements the generation-aware Deployment status reducer.

## API identity

`platform.example.io/v1alpha1`, kind `AIWorkload`, plural `aiworkloads`, namespaced scope. The Go module is `github.com/erayyilmmaz/ai-workload-control-plane`. `platform.example.io` is an intentional portfolio API group, not a claim to own a production domain. Changing the group later is a migration, not a silent rename.

One served/storage version and the `/status` subresource are implemented. No conversion/defaulting/validation webhook is introduced. Structural OpenAPI schema, defaults and CEL handle declarative validation. The manager does not write spec to apply defaults.

## Field decisions

| Path | V0 contract |
| --- | --- |
| spec.environment | Optional V1 logical DNS-label identity (1..63 characters), such as `dev`, `staging` or `prod`; it labels AWCP-owned children but never chooses namespace, cluster or permissions |
| spec.image | Required string, 1..2048 characters, no whitespace (including Unicode space); image availability/non-root execution are runtime concerns |
| spec.replicas | Integer, default 1, range 0..20; explicit 0 is preserved |
| spec.autoscaling | Optional bounded HPA contract; enabled mode requires min/max and one CPU, Memory, Pods or External metric; `minReplicas: 0` is rejected on the current 1.36 baseline |
| spec.availability | Optional PDB contract; enabled mode requires exactly one integer `minAvailable` (1..20) or `maxUnavailable` (0..19), constrained to the static/HPA lower replica bound; it protects voluntary disruption only |
| spec.container.port | Required integer 1..65535; named container port `http`, TCP |
| spec.resources | Optional requests/limits maps, only cpu/memory keys, quoted quantity strings of 1..64 characters; future builders convert to native ResourceRequirements |
| spec.health.readiness.path | Optional block; if present path is non-empty and starts with `/` |
| spec.health.liveness.path | Same rule; no probe when its block is omitted |
| spec.service.enabled | Default true; explicit false is preserved even with defaulted siblings |
| spec.service.port | Default 80, range 1..65535; ClusterIP TCP only, targetPort `http` |
| spec.exposure | Optional `ClusterLocal` or namespace-local `HTTPRoute` intent. HTTPRoute requires `gateway`, `sectionName`, exact DNS `hostname` and `/`-prefixed `path`; it never selects a namespace, Service type, TLS or arbitrary backend. |
| spec.secretRefs | Default empty ordered list, at most 32 unique Secret DNS-subdomain names of 1..253 characters, from the CR namespace |
| spec.externalSecrets | Optional ordered list of at most 16 unique `{externalSecret, targetSecret}` DNS-subdomain pairs. Both names are namespace-local ESO references; neither is a provider value or remote key. |
| spec.network.enabled | Default true; same-namespace ingress to container TCP port; no egress isolation |

Resource quantities must be parseable and non-negative. CEL `isQuantity`, `quantity`
and `compareTo` enforce these rules and request <= corresponding limit on Kubernetes
1.36. Tests include `900m <= 1`, `1024Mi == 1Gi`, exponent notation, zero and invalid
units/negative/over-limit values. Quantities are strings in this public API: quote
integer-looking YAML values. JSON numbers are rejected. CPU/memory keys are validated
as map keys, so unsupported resources are rejected rather than silently pruned.

This is not complete Pod admission validation: CPU precision, LimitRange defaults,
ResourceQuota and cluster policies remain child-API concerns. When child creation
is implemented, its validation rejection becomes `InvalidConfiguration` rather
than a retry storm. AWCP-6 maps these quantities into the Deployment without
writing defaults into the CR. Missing resource keys remain absent in the CR; no fabricated
requests or limits are inserted. Collection/string bounds constrain CEL validation
cost as well as API payload size.

Optional nested service/network blocks default to `{}` so child defaults also apply
when the entire block is omitted. Pointer fields in Go distinguish omission from
explicit `replicas: 0` or `enabled: false`; those values survive serialization and
admission. Optional non-nullable fields supplied as null follow Kubernetes pruning/
defaulting rules, not a third boolean state. Container and container.port are required.
Health blocks have no defaults; each present probe requires a path of 1..2048
characters starting with `/`. Empty `health: {}` enables neither probe.

Readiness defaults: initialDelaySeconds 0, periodSeconds 5, timeoutSeconds 1, failureThreshold 3, successThreshold 1. Liveness defaults: initialDelaySeconds 10, periodSeconds 10, timeoutSeconds 1, failureThreshold 3, successThreshold 1. Both use HTTP and the named `http` container port. These timing knobs are fixed implementation defaults in V0, not extra API fields.

Rolling update uses Deployment RollingUpdate with maxUnavailable=0, maxSurge=1, minReadySeconds=0 and progressDeadlineSeconds=120. A rollout temporarily needs capacity for the surge pod. Demo images use distinct version tags and explicit IfNotPresent pull policy; replacing the bytes behind an unchanged tag is not an update mechanism. Manager and released images use pinned versions/digests.

The pod runs under its generated ServiceAccount with automountServiceAccountToken=false, runAsNonRoot=true, allowPrivilegeEscalation=false, all capabilities dropped and seccomp RuntimeDefault. No privileged or arbitrary PodSpec fields are accepted.

## Secret and network semantics

Direct Secret references map to non-optional `envFrom.secretRef`. `externalSecrets` appends each validated ESO target after direct references, preserving its declared order. The controller requires ESO v1, a Ready namespaced `SecretStore`, a Ready `ExternalSecret`, an exact target-name match and target Secret metadata; it never creates these ESO resources, reads provider data, copies Secret data or accepts `ClusterSecretStore`. Key compatibility and whether values are valid for the application remain application concerns.

Deleting a Secret makes the desired contract unmet even if existing pods still hold environment values. Recreating it triggers a new observation through the reference index/watch. Direct `secretRefs` updates do not restart a process. For an `externalSecrets` target only, a changed Secret metadata resourceVersion is hashed into the owned Pod-template annotation and therefore triggers a normal Deployment rollout without hashing or exposing Secret payload. This detects ESO target updates, not credential validity or revocation.

The generated NetworkPolicy has policyTypes=[Ingress], selects only the CR UID/name labels, and permits same-namespace pods (`podSelector: {}` within an ingress peer) to the container's TCP port. It contains no Egress policy type. Other Kubernetes policies remain additive; disabling this policy does not remove other restrictions. Cluster CNI enforcement is an external prerequisite for traffic isolation claims.

An HTTPRoute exposure must explicitly set `spec.network.enabled: false`, because
an implementation's data plane can run outside the workload namespace and AWCP
does not infer or grant its ingress identity. Gateway API status is published as
the separate `ExposureReady` condition; Deployment `Ready` remains a rollout
observation. See [HTTPRoute exposure](exposure.md).

When `spec.availability.enabled=true`, AWCP owns a same-namespace `policy/v1`
PodDisruptionBudget using only the generated workload identity labels. The
budget accepts exactly one integer `minAvailable` or `maxUnavailable`; percent
budgets, custom selectors and cross-workload disruption policy are deliberately
outside the API. `AvailabilityReady` reports PDB observation independently from
Deployment rollout readiness. See [availability behavior](availability.md).

## Status

| Field | Meaning |
| --- | --- |
| observedGeneration | CR generation evaluated by the controller; not proof of successful rollout |
| desiredReplicas | Current desired replicas from spec |
| readyReplicas | Observed Deployment readyReplicas, zero if Deployment is missing |
| endpoint | `<child>.<namespace>.svc:<service-port>` if enabled and present; otherwise empty |
| conditions | At most 16 `metav1.Condition` entries, map keyed by type; duplicate types rejected |

Every condition includes type, status, reason, message, observedGeneration and lastTransitionTime. Unknown observations use status `Unknown`; do not manufacture a healthy state before inspection. lastTransitionTime changes only when the condition's status value changes. A reason/message/generation change can require a status patch without resetting transition time.

The schema requires each condition's observedGeneration in addition to the standard
metav1.Condition fields. Observed/ready/desired counts cannot be negative; desired
replicas is at most 20. readyReplicas has no 20 ceiling because a rolling update may
temporarily include a surge replica. Endpoint is bounded to 512 characters. No status
field is defaulted. Controllers own its semantic accuracy; admission does not compare
observedGeneration to live workload state. The [synthetic status fixture](../test/fixtures/status.yaml)
is round-tripped via `/status` by tests; it is not a real readiness report.

Writes to `/status` cannot modify spec or advance generation. Main-resource creation
discards supplied status, and ordinary spec updates preserve existing status and
advance generation for spec changes. These are real API-server tests, with no AWCP
controller running. The controller has status permission. AWCP-10 writes the endpoint
after a successful Service decision, desired and ready replica observations, and all
three conditions in one semantic status reducer. See [status behavior](status.md).

| Evaluated state | Ready | Progressing | Degraded | Primary reason |
| --- | --- | --- | --- | --- |
| Dependencies healthy; rollout converging | False | True | False | Reconciling |
| Current rollout and required children healthy | True | False | False | WorkloadReady |
| Required Secret missing | False | False | True | SecretNotFound |
| Foreign/old owner or incompatible immutable child | False | False | True | ResourceOwnershipConflict |
| Invalid desired configuration / child API validation | False | False | True | InvalidConfiguration |
| Latest rollout exceeds progress deadline | False | False | True | ProgressDeadlineExceeded |
| API observation/write fails | Unknown | Unknown | True | ReconcileFailed |
| replicas=0, with dependencies otherwise healthy | False | False | False | ScaledToZero |

Missing Secret, ownership or API failure takes precedence over the zero-replica state. For transient API errors, return framework backoff after best-effort status/event reporting; failure to write status may leave old observations in place and must be visible in logs/metrics.

`Ready=True` requires all of the following, from the latest observation:

1. The current CR generation has been evaluated and all required references are present.
2. All required children exist, are controlled by the current CR UID and match desired managed fields.
3. Deployment status.observedGeneration covers its current metadata.generation.
4. For desired replicas N > 0: updatedReplicas, replicas, readyReplicas and availableReplicas are all N; no older rollout remains in the counted replicas and Available=True.

The endpoint is service discovery information, not a health certificate. The `.svc` form avoids hard-coding `cluster.local`; DNS resolution is still performed inside the cluster. Workload status must not imply readiness of external providers or the truth of their credentials.

## Compatibility and validation fixtures

Unknown fields are pruned under the structural schema with Ignore (or Warn, which
also reports warnings). `fieldValidation=Strict` rejects them with BadRequest; clients
must actually send that option. In controller-runtime v0.24.1 use
`client.CreateOptions{FieldValidation: "Strict"}` rather than setting only Raw, which
is overwritten. Tests submit the same `spec.contaner` typo in both modes, proving
rejection versus pruning. There is no preserve-unknown-fields escape hatch for PodSpec.

The [minimal sample](../config/samples/platform_v1alpha1_aiworkload.yaml),
[full sample](../config/samples/aiworkload_full.yaml), 29 standalone invalid manifests
and [validation field index](../test/fixtures/invalid/index.json) are exercised by
`make test-envtest`. The full sample's image and Secrets are placeholders, not
downloaded credentials or a runnable demo. The default sample Kustomization includes
only the minimal object; the full sample can be applied explicitly.

`kubectl explain aiworkload.spec` and its image field return authored descriptions.
Server-generated printer columns are NAME (Kubernetes metadata), READY, REPLICAS
(desired spec count), IMAGE and AGE. Missing Ready status is deliberately blank.

No breaking alpha schema change is silent: update schema, examples, compatibility notes and fixtures together. `spec.environment` is the first V1 optional field: omitting it preserves the V0 rendered objects, while a present value is copied only into the bounded `platform.example.io/environment` label on AWCP-owned children. Later API conversion or production group migration is a separate decision.

## References

- [Custom resource validation and status](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/)
- [CEL quantity library](https://kubernetes.io/docs/reference/using-api/cel/#kubernetes-quantity-library)
- [Deployment progression](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/)
- [HTTP probes](https://kubernetes.io/docs/concepts/workloads/pods/probes/)
- [Secrets](https://kubernetes.io/docs/concepts/configuration/secret/)
- [NetworkPolicy](https://kubernetes.io/docs/concepts/services-networking/network-policies/)
