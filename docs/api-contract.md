# AIWorkload API and status contract

Accepted design for AWCP-2. Types/schema are implemented in AWCP-4; controller behavior arrives in later stories.

## API identity

`platform.example.io/v1alpha1`, kind `AIWorkload`, plural `aiworkloads`, namespaced scope. The Go module is `github.com/erayyilmmaz/ai-workload-control-plane`. `platform.example.io` is an intentional portfolio API group, not a claim to own a production domain. Changing the group later is a migration, not a silent rename.

One served/storage version is planned, with a `/status` subresource. No conversion/defaulting/validation webhook is introduced. Structural OpenAPI schema, defaults and CEL handle the supported declarative validation. The manager does not write spec to apply defaults.

## Field decisions

| Path | V0 contract |
| --- | --- |
| spec.image | Required non-blank image reference; public or locally available non-root image |
| spec.replicas | Integer, default 1, range 0..20; explicit 0 is preserved |
| spec.container.port | Required integer 1..65535; named container port `http`, TCP |
| spec.resources | Optional CPU/memory requests and limits; map to Kubernetes ResourceRequirements |
| spec.health.readiness.path | Optional block; if present path is non-empty and starts with `/` |
| spec.health.liveness.path | Same rule; no probe when its block is omitted |
| spec.service.enabled | Default true; explicit false is preserved even with defaulted siblings |
| spec.service.port | Default 80, range 1..65535; ClusterIP TCP only, targetPort `http` |
| spec.secretRefs | Default empty ordered list of unique, valid Secret names from the CR namespace |
| spec.network.enabled | Default true; same-namespace ingress to container TCP port; no egress isolation |

Resource quantities must be parseable and non-negative. If a request and its limit are both present, request must not exceed limit. Use schema/CEL where the selected Kubernetes version can validate quantities correctly; do not compare quantity strings lexicographically. If a rule cannot be expressed reliably at the CR API, detect it before child creation or map the child API rejection to `InvalidConfiguration` without retry storms. Such an API validation boundary must be documented and covered in AWCP-4.

Readiness defaults: initialDelaySeconds 0, periodSeconds 5, timeoutSeconds 1, failureThreshold 3, successThreshold 1. Liveness defaults: initialDelaySeconds 10, periodSeconds 10, timeoutSeconds 1, failureThreshold 3, successThreshold 1. Both use HTTP and the named `http` container port. These timing knobs are fixed implementation defaults in V0, not extra API fields.

Rolling update uses Deployment RollingUpdate with maxUnavailable=0, maxSurge=1, minReadySeconds=0 and progressDeadlineSeconds=120. A rollout temporarily needs capacity for the surge pod. Demo images use distinct version tags and explicit IfNotPresent pull policy; replacing the bytes behind an unchanged tag is not an update mechanism. Manager and released images use pinned versions/digests.

The pod runs under its generated ServiceAccount with automountServiceAccountToken=false, runAsNonRoot=true, allowPrivilegeEscalation=false, all capabilities dropped and seccomp RuntimeDefault. No privileged or arbitrary PodSpec fields are accepted.

## Secret and network semantics

Secret references map to non-optional `envFrom.secretRef`. Preserve list order; later references take precedence for overlapping keys. Key compatibility and whether values are valid for the application remain application concerns. The controller does not copy Secret data, hash payloads into annotations, or create a Secret.

Deleting a Secret makes the desired contract unmet even if existing pods still hold environment values. Recreating it triggers a new observation through the reference index/watch. Updating data does not automatically restart or refresh an existing process. V0 is not a credential revocation mechanism.

The generated NetworkPolicy has policyTypes=[Ingress], selects only the CR UID/name labels, and permits same-namespace pods (`podSelector: {}` within an ingress peer) to the container's TCP port. It contains no Egress policy type. Other Kubernetes policies remain additive; disabling this policy does not remove other restrictions. Cluster CNI enforcement is an external prerequisite for traffic isolation claims.

## Status

| Field | Meaning |
| --- | --- |
| observedGeneration | CR generation evaluated by the controller; not proof of successful rollout |
| desiredReplicas | Current desired replicas from spec |
| readyReplicas | Observed Deployment readyReplicas, zero if Deployment is missing |
| endpoint | `<child>.<namespace>.svc:<service-port>` if enabled and present; otherwise empty |
| conditions | `metav1.Condition` list, map keyed by type |

Every condition includes type, status, reason, message, observedGeneration and lastTransitionTime. Unknown observations use status `Unknown`; do not manufacture a healthy state before inspection. lastTransitionTime changes only when the condition's status value changes. A reason/message/generation change can require a status patch without resetting transition time.

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

Unknown fields are pruned under the structural schema by default. Strict field validation requests must reject unknown fields. Document both behaviors; do not treat default pruning as strict typo rejection.

Required fixtures: valid minimal spec; all-fields spec; explicit zero/false; blank image; replicas -1 and 21; ports 0 and 65536; invalid quantity/request-limit relation; invalid health path; duplicate/malformed Secret references; unknown fields in both validation modes; status writes separated from spec writes. Examples will be checked against the generated CRD in AWCP-4, not represented as already validated here.

No breaking alpha schema change is silent: update schema, examples, compatibility notes and fixtures together. Later API conversion or production group migration is a separate decision.

## References

- [Custom resource validation and status](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/)
- [Deployment progression](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/)
- [HTTP probes](https://kubernetes.io/docs/concepts/workloads/pods/probes/)
- [Secrets](https://kubernetes.io/docs/concepts/configuration/secret/)
- [NetworkPolicy](https://kubernetes.io/docs/concepts/services-networking/network-policies/)
