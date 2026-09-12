# AWCP-5 — Reconciliation foundation and ownership

Date: 2026-09-12. Baseline: AWCP-4 commit `89a0660` on `main`.
Task scope comes from the local dated backlog export; Jira was neither read nor
updated. The user owns Jira completion updates.

## Delivered

- `internal/resource/plan.go`: pure Builder/Intent boundary, explicit optional
  absence and stable bounded child naming.
- `internal/controller/engine.go`: whole-plan validation, deterministic dependency
  order, create/patch/delete/no-op, exact owner guard, semantic comparison,
  optimistic patches and UID/resourceVersion-protected optional deletion.
- Reconciler/watches/failures: namespace and lifecycle guards, contextual logs,
  four owned watches, Secret metadata/index mapping, sanitized retry categories,
  deduplicated failure conditions/events and readiness-neutral recovery.
- Manager composition point, generated bounded Role, exact permission tests and
  updated kind authorization checks.
- Unit/race tests, real-API watch/restart tests, implementation contract and handoff.

No new dependency or API schema change. Generated Role was regenerated from
markers, not edited by hand. Production resource mapping remains intentionally
disabled in the shipped manager; tests inject `test/fixtures.Plan`.

## Acceptance evidence

| Requirement | Test/evidence |
| --- | --- |
| Duplicate reconcile is idempotent | `TestEngineIdempotenceDriftAndDeletion`: four kinds, one create write, repeat no-op, one repair patch, recreate; envtest stable child resourceVersions |
| Four children recover from delete/drift | Same unit test plus `TestReconciliationWithRealAPI/all_four_watches...`: actual watch delivery, replacement UIDs, managed label repair and unrelated parent's unchanged UID/RV |
| Defaulting does not produce update storm | `server_defaults_allocated_IPs_and_injected_sidecars_survive_no-op`: real Service allocation, Deployment defaults, injected sidecar, stable RV; actual replica drift repair |
| Restart has no state database | `restart_reconstructs_from_API_without_database`: stop manager, remove four children, construct/start new manager, verify new UIDs and stable unrelated children |
| Other workloads progress despite error | `ownership_conflict_visible_and_unrelated_errors_isolated`: live manager with failing builder plus healthy parent; clear fault and recover |
| No foreign adoption/destructive takeover | `TestOwnershipNeverAdoptsOrDeletesForeignResources`: four kinds × six owner variants; live API foreign Service unchanged and ownership Warning Event |
| Concurrency is safe | `TestPatchConflictPreservesConcurrentChangesThenRecovers`, `TestCreateRaceDoesNotAdoptWinner`, `TestOptionalDeleteRaceCannotRemoveReplacement` |
| Optional deletion is explicit and guarded | `TestPlanBoundaryAndDeletePreconditions`: only Service/NetworkPolicy, exact UID+RV; absence is no-op |
| Termination guard | `TestDeletionGuardAndPrimaryPredicate`, `TestDeletingChildWaitsWithoutRecreation` |
| Errors are classified, visible and bounded | `TestFailureClassificationConditionsAndRecovery`: Conflict, AlreadyExists, Forbidden, timeout, throttling, unexpected, ownership and configuration; safe messages, one status/Event on repeated failure, recovery never Ready=True |
| Watch/predicate avoids missed status and self-loop | `deployment_status_and_Secret_deletion_enqueue_without_parent_edit`, primary predicate unit test; parent status-only write causes no self-reconcile |
| Secret watch is scoped and payload-free | `TestSecretIndexMapping`, metadata-only manager watch; real Secret create/delete enqueues only referencing parent |
| Invalid targets cannot be written | `TestPlanBoundaryAndDeletePreconditions`, `TestMutatorCannotChangeIdentity`; typed nil, unsupported kind, duplicate, wrong namespace/name, owner mutation |
| Stable bounded names | `TestChildNameIsBoundedStableAndUsesFullName`: long names, dots, shared prefixes, known hash fixture |
| Least-privilege Role remains bounded | `TestGeneratedRBACAndCRD`: exact eight-rule matrix; kind checks described below |

## Executed checks

Host: macOS ARM64. Go 1.26.8; Kubernetes client modules 0.36.4;
controller-runtime 0.24.1; envtest API server/etcd assets 1.36.2.
The repository version lock was not changed.

| Command | Result |
| --- | --- |
| `make fmt lint-fix test-unit` | PASS, zero lint issues |
| `make test-envtest` | PASS: bootstrap, contract and new reconciliation suite |
| `make fmt lint-fix verify test-race` | PASS: build/vet/lint, all packages, real API suites, race detector, generated-file byte stability and Kustomize render |
| `git diff --check` and `bash -n test/e2e/bootstrap-smoke.sh` | PASS |
| Container build with isolated public-registry Docker configuration | PASS: `awcp-manager:awcp-5`, Linux ARM64; exact command below |
| `DOCKER_CONFIG="$PWD/.tools/docker-public" DOCKER_HOST=unix:///Users/eray-refgen/.docker/run/docker.sock make smoke IMG=awcp-manager:awcp-5` | PASS: kind 0.33.0 / node 1.36.4, UID 65532, restricted security, health/readiness, Lease, permitted/denied RBAC including status subresource and outside-namespace denial |

Initial real-API test development caught two fixture/harness errors: Deployment
readyReplicas must not exceed status.replicas, and controller-runtime rejects
duplicate controller names within one process even after shutdown. Fixtures now
set coherent status; sequential test managers use distinct names without turning
off runtime validation. These failures were fixed before the passing full runs.

The first `make docker-build IMG=awcp-manager:awcp-5` attempt stalled in
`docker-credential-desktop get`. That task's build/helper processes were stopped;
personal Docker credentials/settings were not changed. A separate ignored
`.tools/docker-public/config.json` only configured the local CLI plugin directory
and no credential store. The same pinned Dockerfile then built successfully:

```bash
docker --config .tools/docker-public \
  --host unix:///Users/eray-refgen/.docker/run/docker.sock build \
  --build-arg REVISION=89a0660a79c614d0a50ae160e31a176da6a385d9 \
  -t awcp-manager:awcp-5 .
```

Image ID: `sha256:35b3b4d50d52bb2512b7fa4f06a3f23a64af9c8ddef3f8dacc3d1d87d750eca0`.
It contains this milestone's working-tree source; the revision label records its
pre-commit parent, **not** a published/committed release image.

The first smoke run exposed an existing script race: the first listed Pod could
be the old terminating replica after rollout. The script now selects exactly one
non-terminating Ready Pod with the requested image. The rerun passed all checks.
Both runs removed their own temporary clusters/kubeconfigs; `kind get clusters`
confirmed none remained. Local image/tool caches remain reusable. No existing
user cluster or kubeconfig was changed.

## Boundaries and next handoff

- This is the reusable reconciliation engine, not a complete application operator.
  The default manager has a nil builder. AWCP-6 activates real Deployment mapping;
  Service, identity/Secret and NetworkPolicy mappings follow in AWCP-7/8/9.
- Test plans deliberately omit production security, probes, resources and Secret
  mapping. Do not copy them as the production contract.
- Foundation failure conditions are not the AWCP-10 observed-readiness reducer.
  No application Ready=True or endpoint is manufactured.
- Envtest has no kubelet, Deployment controller, garbage collector or enforcing
  CNI. It proves API/watch behavior, not rollout, Pods, GC or traffic enforcement.
- Fake delete-race test explicitly emulates the server's UID rejection; it does
  not claim fake client implements real UID delete admission.
- Manager restart is stop/reconstruct/start inside the test process with a fresh
  manager/cache. It is not a container crash or multi-replica failover test.
- No hosted CI, image publication, user-cluster deployment or Jira mutation.

Next: **AWCP-6 — Deployment reconciliation and workload lifecycle**. Supply a
production builder through the existing composition point, preserve the exact
ownership/retry boundary, and add real field-mapping/defaulting tests rather than
expanding the test-only fixtures into an undocumented production API.
