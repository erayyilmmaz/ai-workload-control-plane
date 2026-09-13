# Deployment mapping and lifecycle — AWCP-6

## Current behavior and prerequisites

The shipped manager now uses `resource.WorkloadBuilder` by default and emits one
Deployment intent per AIWorkload. The AWCP-5 engine provides ownership checks,
create/patch/no-op, drift repair and deletion recovery. There is no separate
Deployment reconciler with a different safety policy.

The dedicated ServiceAccount **name is bound but its object is not created yet**
(AWCP-8). Native ReplicaSet/Pod creation can therefore report `FailedCreate` until
that identity exists. Secret names are mapped but metadata prerequisites/status
arrive in AWCP-8. AWCP-7 adds the optional cluster-local Service described in
[service.md](service.md); NetworkPolicy arrives in AWCP-9. Do not work around this
staged implementation by switching to the default ServiceAccount or granting
workload RBAC. This is not full application readiness or a production release.

## Field ownership

| Target | Controller-owned values | Preserved values |
| --- | --- | --- |
| Deployment identity | ADR-004 bounded child name, parent namespace; ownerReference via engine | No adoption or identity rewrite |
| Deployment and Pod-template metadata | Five common/selector labels from architecture; `platform.example.io/workload-name` annotation | Other labels/annotations, including deployment revision and injected metadata |
| Selector | Exact two-label CR UID/child-name identity, no expressions | Mismatch on an existing object is a conflict; selector is never changed |
| Deployment rollout | replicas (nil→1, explicit 0 retained), RollingUpdate, maxUnavailable=0, maxSurge=1, minReadySeconds=0, progressDeadlineSeconds=120, paused=false | Server-defaulted or explicitly configured revisionHistoryLimit |
| Pod identity | serviceAccountName and legacy alias = child name, token automount=false | No ServiceAccount/RBAC creation in this step |
| Pod security | runAsNonRoot=true, seccomp RuntimeDefault | Other context fields, e.g. fsGroup and a chosen non-root UID |
| Container `workload` | Image, explicit IfNotPresent, named TCP port `http`, CPU/memory resources, readiness/liveness, entire ordered envFrom list | Named-container lookup preserves position, other containers, command/args/env, mounts, unrelated ports and API defaults |
| `http` port entry | Name, container port, TCP; no hostPort/hostIP | Other named port entries |
| Workload container security | non-root, privileged=false, allowPrivilegeEscalation=false, capabilities drop ALL with no additions, RuntimeDefault seccomp | Non-managed context fields such as readOnlyRootFilesystem and a non-root runAsUser |
| Resource maps | CPU/memory requests and limits; removed keys are removed | Non-API resource keys and resource claims injected externally |
| Probes | Exact HTTP handlers/timing below; absent block removes its probe | No startup probe is generated; unrelated fields outside the owned probes remain untouched |

Renaming the `workload` container out-of-band does not transfer ownership to its
new name: the named managed container is restored, and other containers are not
deleted. The same named-entry rule applies to `http`. The limited AIWorkload API
does not expose arbitrary PodSpec fields or promise image compatibility.

## Mapping/defaulting details

The builder snapshots its input, returns fresh maps/pointers and does no API or
Secret payload I/O. Repeated mutation with the same spec is deterministic. No
generation, timestamp, random token, image hash or Secret-content checksum is
added to Pod-template metadata.

Requests/limits are parsed as Kubernetes quantities, never floating-point values.
Unsupported, malformed, negative or request-over-limit values fail with the safe
`InvalidConfiguration` category. If a limit is present without its matching
request, the Deployment explicitly requests that limit, matching Kubernetes'
ordinary limit-only default. This does not write defaults into the AIWorkload CR.
Explicit zero requests are preserved. Pod admission, LimitRange, quotas and
cluster-specific resource validation remain external boundaries. See the
[Kubernetes resource contract](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/).

Readiness is HTTP on named port `http`: initial delay 0, period 5, timeout 1,
success threshold 1, failure threshold 3. Liveness uses initial delay 10 and
period 10 with the same timeout/thresholds. Missing blocks produce no probe;
removing a block removes the previous handler. V0 has no configurable timing or
generated startup probe. Slow-start images must account for the fixed liveness
window. Probe behavior is delegated to kubelet, not simulated by the controller.
[Kubernetes probes](https://kubernetes.io/docs/concepts/workloads/pods/probes/)
describe the runtime semantics.

Secret refs become ordered, non-optional `envFrom.secretRef` entries. Empty/removal
clears the managed list; later entries retain precedence. Direct injected `env`
entries are preserved and Kubernetes gives them precedence over envFrom. A Secret
data change alone does not change the template or refresh running environments.

`IfNotPresent` is always explicit, including for `latest` if supplied. Use distinct
version tags or digests; mutating an unchanged tag is not a supported update
mechanism. Public, non-root-compatible images are the V0 contract. The current
repository samples contain placeholder images, not published demo releases.

## Update and failure semantics

Image/tag/digest, port, resources, probe and Secret-list changes update the Pod
template. Replica-only changes do not. Unchanged reconciles preserve both
Deployment resourceVersion and template. Kubernetes creates rollout revisions
from template changes, not scaling; see [Deployment updates](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#updating-a-deployment).
Envtest proves the **template change**, not that a ReplicaSet or healthy Pod ran.

An owned Deployment with an incompatible selector returns `ErrImmutableSelector`
before metadata/spec mutation. It becomes `ResourceOwnershipConflict` with an
explicit immutable-selector message, Warning Event and 60-second normal recheck.
There is no silent delete/recreate, selector broadening or adoption. Ordinary API
Invalid is `InvalidConfiguration`; transient API failures retain AWCP-5 backoff.

Built-in API defaults, injected sidecars and unrelated metadata are tested.
Admission webhooks that continually rewrite explicitly owned fields are not an
interoperability guarantee: align their ownership policy before enabling AWCP in
that namespace. The operator is not a full Pod security/tenant-isolation enforcer.

## Runtime diagnosis and evidence limits

The manager does not add Pod/ReplicaSet read permissions or ingest container logs
in AWCP-6. Authorized humans can inspect these with their own credentials. Never
paste Secret data, env dumps or unredacted application logs into an issue.

| Symptom | Where an authorized operator checks | Interpretation / next action |
| --- | --- | --- |
| FailedCreate; ServiceAccount not found | Deployment/ReplicaSet events | Expected staged prerequisite until AWCP-8; do not substitute default identity |
| ErrImagePull / ImagePullBackOff | Pod container waiting reason and redacted image-pull events | Invalid/unavailable image or registry access; fix spec.image, no automatic image fallback |
| CrashLoopBackOff | Restart count, last termination reason/exit code; controlled log review if authorized | Application startup/crash; non-root/image compatibility and application config may be involved |
| Readiness failing | Pod Ready/container readiness and probe events | Pod may run but should not receive ready Service traffic; check port/path and app readiness |
| Liveness failing | Probe events and increasing restarts | Kubelet restarts that container; fixed timing may be too aggressive for a slow-start app |
| Quota exceeded | ReplicaSet FailedCreate event and namespace quota | Pod admission/capacity failure; fix resource demand or ask cluster administrator |
| Insufficient CPU/memory | PodScheduled=False, Unschedulable and scheduler event | Requests cannot be scheduled; account for the extra maxSurge=1 Pod |
| Progress deadline exceeded | Deployment Progressing=False / ProgressDeadlineExceeded | Native controller reports failure to progress; it does not automatically roll back |

Before inspecting Pods, start with the Deployment and its status/events. Example
read-only commands (substitute exact namespace/name; require your own permissions):

```bash
kubectl -n awcp-workloads get deployment <child-name> -o wide
kubectl -n awcp-workloads describe deployment <child-name>
kubectl -n awcp-workloads get pods -l app.kubernetes.io/instance=<child-name>
```

`test/fixtures/deployment/missing-image.yaml` and `insufficient-resources.yaml`
are intentional failure inputs, validated/mapped in envtest. Their comments state
identity and image prerequisites. Do not apply capacity-failure fixtures to shared
clusters. CrashLoop/readiness/quota/deadline rows above are diagnostic expectations,
**not executed lifecycle results**. Real runtime failures/rollout and healthy
recovery are later kind E2E work; AWCP-10 maps observed failures into full status.

See [AWCP-6 execution evidence](verification/AWCP-6.md).
