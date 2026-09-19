# Acceptance and implementation traceability

AWCP-2 defines the design baseline. The original source IDs AWC-1..16 are sequence identifiers; actual Jira IDs are AWCP-2..17. The dated [backlog export](backlog/awcp-v0.md) preserves the original complete task descriptions. Current contracts and [compatibility decisions](compatibility.md) supersede provisional source wording.

## AWCP-2 acceptance mapping

| Acceptance requirement | Concrete deliverable | Verification in this milestone |
| --- | --- | --- |
| Define domain and supported use cases | [Scope](scope.md), README | Application type and each lifecycle action are explicit |
| Declare V0/non-goals and optional decisions | [Scope](scope.md), ADR-001/006/007 | Exclusions visible; Helm/traces deferred; Collector example retained |
| Name resource source of truth | [Architecture](architecture.md), ADR-001 | Kubernetes spec/observed status; no external DB/broker |
| Decide ownership for every generated resource | Architecture responsibility/field tables, ADR-004 | Four direct children and Pod/ReplicaSet subtree distinguished |
| Protect user Secret/Namespace/shared resources | Architecture matrix, ADR-003/005/008 | No controller create/mutate/delete/ownerReference for these objects |
| Define naming, labels and annotations | Architecture naming section | Bounded deterministic child name, UID selector, no sensitive metadata |
| Define deletion and garbage collection | ADR-008 and architecture guards | No finalizer; owner UID and delete race policy specified |
| Define status/conditions | [API contract](api-contract.md) | Generation, truth table, reasons, endpoint and no-op semantics |
| Pin Kubernetes/controller-runtime and tools | [Compatibility](compatibility.md), [lock](../toolchain.lock.json) | Tagged scaffold compared; exact pins/asset URLs/checksums recorded |
| Check development/test platform availability | Lock platform sections | Published darwin/arm64 and linux/amd64 assets, Linux node architecture statement |
| Document security boundary and no webhook | ADR-005/007, scope | Runtime RBAC, workload identity, Secret capability and alpha validation boundary |
| ADR directory and API versioning | [Eight ADRs](adr/README.md) | All accepted with context, decisions, tradeoffs and validation ownership |
| Map all 16 implementation steps | Table below | Each step has a deliverable, test/evidence layer and dependency |

These are design/source checks. A statement such as “the controller never mutates a Secret” is an implementation requirement until AWCP-8 tests prove it. This distinction prevents documentation completion from being confused with a working operator.

## Story sequence and test ownership

AWCP-3's repository foundation is now implemented; see its
[acceptance and execution evidence](verification/AWCP-3.md) and
[development guide](development.md).
[AWCP-4](verification/AWCP-4.md) now implements the API schema and contract tests;
[AWCP-5](verification/AWCP-5.md) implements the reconciliation/ownership engine and
watch wiring. [AWCP-6](verification/AWCP-6.md) activates the production Deployment
mapping; [AWCP-7](verification/AWCP-7.md) activates optional ClusterIP Service
mapping and the discovery endpoint; [AWCP-8](verification/AWCP-8.md) activates
dedicated identity and Secret missing/restore decisions; [AWCP-9](verification/AWCP-9.md)
activates standard ingress-only NetworkPolicy generation; [AWCP-10](verification/AWCP-10.md)
activates generation-aware conditions, readiness and bounded Events. [AWCP-17](verification/AWCP-17.md)
now provides the clean-user Quick Start, three public examples, narrated portfolio
demo and release-preparation boundary. [AWCP-19](verification/AWCP-19.md) starts
the V1 compatibility contract while preserving the V0 `v1alpha1` runtime surface.
[AWCP-20](verification/AWCP-20.md) establishes the GitOps/Argo CD ownership
boundary without changing the operator's resource ownership model.
[AWCP-21](verification/AWCP-21.md) adds independently rendered dev/staging/prod
parent delivery, keeping V0 behavior intact whenever `spec.environment` is absent.
[AWCP-22](verification/AWCP-22.md) adds a provider-locked AWS/EKS infrastructure
reference while preserving GitOps ownership of platform and workload manifests.

| Step / source | Actual Jira | Deliverable / decision | Evidence layer | Direct prerequisites |
| --- | --- | --- | --- | --- |
| 01 / AWC-1 | AWCP-2 | Scope, architecture, exact baseline, ADR-001..008 | Document/source/asset review | None |
| 02 / AWC-2 | AWCP-3 | Repository, Go module, scaffold, tools, Dockerfile | Generate/build/vet/lint, bootstrap envtest | AWCP-2 |
| 03 / AWC-3 | AWCP-4 | Typed API, defaults, structural CRD, status | API schema/default validation in envtest | AWCP-3 |
| 04 / AWC-4 | AWCP-5 | Ownership, watch wiring, idempotent reconcile | Unit + envtest events/no-op/collision | AWCP-4 |
| 05 / AWC-5 | AWCP-6 | Deployment field mapping and rollout intent | Builder unit + envtest; real rollout later | AWCP-5 |
| 06 / AWC-6 | AWCP-7 | Service toggles, allocated field preservation, endpoint | Unit + envtest; real DNS/HTTP later | AWCP-6 |
| 07 / AWC-7 | AWCP-8 | Identity, metadata-only Secret prerequisite, no workload RBAC | Unit/envtest missing/restore; kind authorization smoke | AWCP-6 |
| 08 / AWC-8 | AWCP-9 | Ingress-only policy generation and toggles | Unit/envtest; CNI enforcement only with separate profile | AWCP-6 |
| 09 / AWC-9 | AWCP-10 | Status, generation gates, reason/event model | Truth-table unit + no-op/status envtest | AWCP-6, 7, 8, 9 |
| 10 / AWC-10 | AWCP-11 | Metrics, structured logs, dashboard, Collector example | Counter/gauge/outage checks and dashboard demo | AWCP-5, 10 |
| 11 / AWC-11 | AWCP-12 | Delete guards and ownership lifecycle | Guard/race unit tests; kind GC and Secret-survival smoke | AWCP-6, 7, 8, 9, 10 |
| 12 / AWC-12 | AWCP-13 | Consolidated regression/coverage suite | Unit + real API envtest, bounded deletion guard, project-only atomic coverage, race where supported | AWCP-6 through 12 |
| 13 / AWC-13 | AWCP-14 | Isolated kind harness, demo image, diagnostics/cleanup | Real rollout, scale, drift, Secret recovery, service traffic, GC | AWCP-13 |
| 14 / AWC-14 | AWCP-15 | Kustomize package, install/uninstall guide | Install/upgrade/undeploy + explicit cleanup checks | AWCP-14 |
| 15 / AWC-15 | AWCP-16 | GitHub Actions and supply-chain policy | Hosted checks; merge ruleset verified separately | AWCP-13, 14 |
| 16 / AWC-16 | AWCP-17 | README/examples, 16-step demo, release preparation | Clean-user kind demo; release publication evidence remains separate | AWCP-15, 16 |
| V1-01 | AWCP-19 | V1 API/versioning/capability baseline | V0 strict envtest fixture; unit capability detection; optional-API startup envtest | AWCP-17 |
| V1-02 | AWCP-20 | GitOps repository model and Argo CD integration | Kustomize/static GitOps validation; disposable kind Argo CD demo | AWCP-19 |
| V1-03 | AWCP-21 | Multi-environment GitOps delivery and ApplicationSet | Overlay rendering, environment API/child labels and disposable kind promotion demo | AWCP-20 |
| V1-04 | AWCP-22 | Terraform reference infrastructure | Pinned Terraform/provider lock, static validation and high/critical IaC scan without cloud credentials | AWCP-20, 21 |

Each behavior gets relevant tests while it is implemented. AWCP-13 consolidates regression coverage; it does not postpone all tests until the end. Status logic introduced early may use temporary fixtures until the full lifecycle is available.

## Cross-cutting acceptance scenarios

| Scenario | Expected boundary | Final proof |
| --- | --- | --- |
| Same spec reconciled repeatedly | No duplicate resources, status writes or rollout | AWCP-5/10 envtest; AWCP-14 runtime |
| Managed mutable field drift | Restore owned configuration without touching foreign metadata | AWCP-5..9 |
| Foreign owner or old UID | Preserve resource; report conflict | AWCP-5, AWCP-12 |
| Secret absent → created → deleted → restored | Observe without editing primary; never mutate Secret | AWCP-8/10, AWCP-14 |
| Old Available during new image rollout | Do not report new generation Ready prematurely | AWCP-10/14 |
| Zero replicas | ScaledToZero; not degraded when dependencies are healthy | AWCP-6/10/14 |
| Optional child disabled | Delete only current UID-owned child, clear stale endpoint | AWCP-7/9 |
| API Conflict/Forbidden/timeout | Specific failure, bounded retry, other workloads progress | AWCP-5/10/13 |
| Manager restart / telemetry outage | State rebuilt from API; reconcile independent of telemetry | AWCP-11/14 |
| Parent deletion / same-name recreation | GC old subtree; never adopt/delete another UID; Secret preserved | AWCP-12 |
| Default kind without enforcing CNI | Policy object evidence only | AWCP-9/14 documentation |
| Operator undeploy vs CRD removal | Preserve workload by default; explicit destructive cleanup separate | AWCP-15/17 |

## Handoff and completion protocol

For every development step: read the user-provided task description and current repository, implement only that step, run meaningful checks, record commands/outcomes/limitations, then commit and push. The user authorized commit + push after each step on 2026-09-12. Verify local HEAD equals the remote branch SHA before reporting a successful push. Do not use force-push or rewrite history as part of this routine.

Starting with AWCP-4 the user supplies the Jira description and updates Jira
personally. Do not read or update Jira unless explicitly requested again; return
the completion evidence here and in the repository.

Jira progress is not a replacement for Git/test evidence. No unexecuted check is recorded as passing. A later step may revise an accepted design only with a documented reason and updates to affected tests/contracts.
