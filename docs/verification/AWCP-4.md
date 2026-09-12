# AWCP-4 — AIWorkload API and CRD contract evidence

Date: 2026-09-12. Scope provided directly by the user. No Jira access or update was
performed; Jira completion is user-managed. Controller workload behavior remains
unimplemented, and its read-only RBAC is unchanged.

## Delivered

- Typed spec: image, pointer replicas, container HTTP port, CPU/memory resources,
  independent readiness/liveness paths, optional Service, ordered Secret names,
  optional ingress policy configuration.
- Structural namespaced v1alpha1 CRD with server defaults and CEL validation.
  `/status` is retained; Ready/Replicas/Image/Age printer columns added.
- Typed observedGeneration, desiredReplicas, readyReplicas, endpoint and bounded
  map-by-type metav1.Condition status. Conditions must include observedGeneration.
- Minimal/full valid manifests, 29 standalone invalid manifests plus expected-field
  index, unknown-field typo fixture and synthetic status YAML.
- Go serialization/DeepCopy tests, controller-independent real-API contract suite,
  existing manager integration regression and expanded API/development docs.

## Acceptance mapping

| Requirement | Implemented proof |
| --- | --- |
| Namespaced CRD and active status subresource | Existing manifest test + independent envtest installation/namespace create and status writes |
| Required non-blank image and required container port | Invalid missing/empty/ASCII-whitespace/Unicode-whitespace image and missing container/port fixtures rejected |
| Replica range/default; explicit 0 | Defaults and zero tests; -1/21 create rejection; valid 20 and invalid update rejection |
| Ports 1..65535 | Container and Service 0/65536 rejection; both valid boundaries accepted |
| Nested defaults and explicit false | Omitted blocks, empty blocks, partial Service block and explicit false cases preserve intended values |
| Resources | Invalid formats, negatives, non-string quantities, unsupported keys and request > limit rejected; mixed-unit equality/order and zero accepted |
| Probes | Missing/empty/non-slash paths rejected; full sample round-trips both paths; minimal spec adds no probes |
| Secret references | Ordered list preserved; empty, invalid, repeated, overlong and excessive names rejected; dotted/253-character names accepted |
| Status schema and responsibility | Synthetic YAML round-trip; create ignores status; status update ignores spec/does not advance generation; spec update preserves status/advances generation |
| Conditions | Required generation and unique type enforced; negative ready count rejected |
| Unknown fields | Same typo rejected under Strict, accepted and pruned under Ignore; no silently stored unsupported PodSpec |
| Explain and printer columns | Bundled envtest kubectl executes explain for spec and image; server table includes NAME/READY/REPLICAS/IMAGE/AGE and expected image/Ready value |
| Independent CRD contract | `test/contract` starts only kube-apiserver/etcd, no AWCP manager/controller |
| Reproducible generated output | Second CRD/RBAC/DeepCopy generation produces no diff |

## Executed checks

Environment: macOS ARM64, Go 1.26.8, envtest Kubernetes 1.36.2 and its bundled
kubectl. Existing exact dependency/tool pins were retained; no additional package,
webhook, cluster permission or service was introduced.

| Command | Result |
| --- | --- |
| `make fmt generate manifests` | Passed; schema installed successfully by the real API server |
| `make test-envtest` | Passed: manager regression and independent API contract suite |
| `make fmt verify test-race` | Passed: build, vet, lint (0 issues), all tests, repeat generation, Kustomize rendering and race detector |
| `git diff --check` | Passed |

Named tests: `TestExplicitZeroFalseSerialization`, `TestDeepCopyIsolation`,
`TestAPIContract` (eight scenario groups including 29 negative fixtures), existing
`TestBootstrapIntegration`, controller/manager/manifest regression tests. The
negative fixture suite requires an API Invalid error at the expected field and
verifies the invalid resource was not stored. The unknown-field test separately
requires BadRequest from Strict validation.

During implementation the first Strict test caught controller-runtime's Raw option
being overwritten by the top-level FieldValidation option. The test now sends the
actual supported option and demonstrates the server-side rejection. Status fixture
decoding was also corrected to supply GVK to the unstructured decoder; the complete
suite passed afterward. These were test harness fixes, not relaxed schema rules.

## Deliberate boundaries and next action

Resources use quoted strings and Kubernetes quantity-aware CEL, not lexical
comparison. Bounds (32 Secret references, 64-character quantities, two resource
keys per map, 16 conditions) keep validation cost bounded and are documented as
part of this alpha API. CPU precision, image pull/reference/runtime failures and
cluster admission policies are not fully duplicated at the CR layer; future child
reconciliation must map child rejection to InvalidConfiguration. No such controller
behavior is claimed here.

The accepted V0 probe timing, named port, Secret envFrom precedence and NetworkPolicy
semantics are documented, not implemented as children. Synthetic status does not
prove current workload health. No kind lifecycle E2E, image rebuild, hosted CI,
deployment or published release was required or claimed for this API-only step.
Envtest control planes and temporary test kubeconfigs are cleaned up by the tests.

The AWCP-3 empty-spec scaffold is intentionally superseded: new creates/updates
require image and container.port. This is an alpha schema tightening, not a
production data migration. Anyone retaining an AWCP-3 demo CR must provide the new
required fields. Next: AWCP-5 reconciliation foundation/ownership, using this
validated contract. Git delivery is the commit containing this evidence file;
the verified pushed commit ID is returned in the handoff message.
