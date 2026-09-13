# AWCP-6 — Deployment reconciliation and workload lifecycle

Date: 2026-09-13. Baseline: AWCP-5 commit `c605d8a` on `main`.
The local dated backlog export defined this task. Jira was not read or updated;
the user owns Jira completion updates.

## Delivered

- `resource.WorkloadBuilder` is the default manager builder and emits exactly one
  Deployment intent. It maps image, replicas, CPU/memory quantities, named HTTP
  port, fixed probe timing, ordered non-optional Secret `envFrom`, future dedicated
  ServiceAccount name, token automount, Pod/container security and rollout fields.
- The AWCP-5 engine owns create/patch/no-op, UID owner references, drift repair,
  deletion recovery, optimistic patching and foreign-resource guards. No separate
  Deployment write path bypasses those protections.
- Stable two-label Deployment selectors, common metadata, bounded child naming and
  `blockOwnerDeletion=false` are exercised against a real API and kind cluster.
- An existing owned Deployment whose immutable selector differs fails visibly as
  `ResourceOwnershipConflict`; it is never mutated, deleted or recreated.
- Tests cover template-versus-scale behavior, server defaults, injected sidecars
  and metadata, resource parsing/defaulting, aliases, failure fixtures and
  manager restart/delete recovery inherited from the shared engine.
- Deployment mapping/diagnosis documentation, intentional failure fixtures and
  kind smoke coverage are added. The Makefile's local image default is now
  `awcp-manager:awcp-6`.

No CRD schema, module pin or runtime RBAC expansion was needed: Deployment
create/patch/watch authorization already exists in the bounded AWCP-5 Role.

## Acceptance evidence

| Requirement | Concrete proof |
| --- | --- |
| `image=demo:v1`, `replicas=2` map deterministically | `TestDeploymentMapping` checks name, selector, image, replicas, owner handoff and full owned mapping; real-API suite checks the manager's default builder |
| Image changes create rollout intent | Unit `TestDeploymentTemplateChangesAndRemoval`; envtest image tag/digest update proves template changes while selector/Deployment identity stay fixed |
| Scale is distinct from rollout intent | Unit and envtest `replicas 1 to 2 to 0` preserve template/selector exactly while only `.spec.replicas` changes |
| Resources and health propagate | Unit checks parsed CPU/memory and both named-port HTTP probes; envtest checks limit-only request normalization, fixed timing and removals |
| Secret mapping is ordered and safe | Unit/envtest assert ordered non-optional `envFrom.secretRef`, no payload I/O, removal clearing and no implicit restart claim |
| Security/identity binding | Unit/envtest/kind assert future child ServiceAccount name, token automount=false, non-root, no privilege escalation, drop ALL and RuntimeDefault seccomp; AWCP-8 creates the ServiceAccount object |
| Deleted Deployment returns | `TestProductionDeployment/deleted production Deployment is recreated` asserts a new Deployment UID and current API-derived template |
| Unrelated workloads/fields remain safe | Real API tests preserve user annotations, Pod annotations, injected sidecar/volume, server defaults and unrelated parent UID/resourceVersion |
| Immutable selector is explicit/safe | Unit and envtest `immutable selector is visible bounded and never recreated`; condition, no mutation and unrelated parent isolation |
| Intentional unavailable-image/capacity fixtures | Envtest parses CRD-valid YAML and preserves image/resources in child template; fixture comments and deployment guide distinguish this from kubelet/scheduler execution |
| No reconcile/update storm | Repeated Engine apply is `Unchanged`; real API child resourceVersion stays stable after no-op reconciliation |
| Container/RBAC installation path | Kind smoke creates the real sample Deployment and verifies owner reference, selector, workload container mapping, future identity binding and allowed/denied Role operations |

## Executed checks

Host: macOS ARM64. Go 1.26.8; Kubernetes client modules 0.36.4;
controller-runtime 0.24.1; envtest assets and kind node Kubernetes 1.36.4.
No tool/module lock changed.

| Command | Result |
| --- | --- |
| `make fmt lint-fix test` | PASS: zero lint issues and all unit/envtest packages, including production Deployment suite |
| `make verify test-race` | PASS: generated source, build, vet, lint, all tests, race detector, generated-file stability and Kustomize rendering |
| `git diff --check` and `bash -n test/e2e/bootstrap-smoke.sh` | PASS |
| `DOCKER_CONFIG="$PWD/.tools/docker-public" DOCKER_HOST=unix:///Users/eray-refgen/.docker/run/docker.sock make docker-build IMG=awcp-manager:awcp-6` | PASS: local Linux ARM64 image `awcp-manager:awcp-6` |
| `DOCKER_CONFIG="$PWD/.tools/docker-public" DOCKER_HOST=unix:///Users/eray-refgen/.docker/run/docker.sock make smoke IMG=awcp-manager:awcp-6` | PASS: isolated kind cluster, manager security/health/Lease, real Deployment contract and foundation RBAC |

The image build used the existing ignored public-registry Docker configuration so
the desktop credential helper was not involved. Image manifest list:
`sha256:925c694735c72ad0cdf586f8e7b629e6073c47a8df5f8639a3579da8f6f85875`.
It contains this pre-commit working tree; its revision label is the prior commit
`c605d8a`, not a published release image.

Kind created `awcp-bootstrap-awcp-smoke-9irgxd` and deleted it during cleanup.
Its temporary kubeconfig was deleted too; no existing user cluster/kubeconfig was
used. Build/tool caches and the local test image remain reusable.

One test-harness correction occurred during development: the failure fixtures use
the installation's `awcp-workloads` namespace whereas the isolated manager test
watches `workloads`; the test changes only the decoded in-memory namespace before
creating it. The fixture files retain their installation documentation and were
not altered to fit a test-only namespace.

## Boundaries and next handoff

- AWCP-6 creates the Deployment but intentionally does **not** create its dedicated
  ServiceAccount. In a real cluster the native Deployment/ReplicaSet path can show
  `FailedCreate` until AWCP-8; this is not bypassed with the default identity.
- Service lifecycle/endpoint is AWCP-7, workload identity/Secret prerequisite
  handling is AWCP-8, NetworkPolicy is AWCP-9 and complete observed status/events
  are AWCP-10. Existing AWCP-5 foundation failure status is not readiness.
- Envtest has no Deployment controller, scheduler, kubelet, image pull, CrashLoop,
  probe execution, quota enforcement, GC or CNI. It proves API object mapping and
  watches, not successful Pods, ImagePullBackOff, Unschedulable or HTTP behavior.
- Kind proves manager/Deployment API integration only. Because the dedicated
  ServiceAccount is intentionally absent and samples use placeholder images, it
  does not claim an available replica or application traffic.
- No hosted CI, image registry push, user-cluster deployment, Secret data read,
  Jira mutation or production release was performed.

Next: **AWCP-7 — Service discovery and exposure lifecycle**. Add only the
ClusterIP Service builder/lifecycle and endpoint foundations, preserving allocated
network fields and the Deployment identity/selector contract established here.
