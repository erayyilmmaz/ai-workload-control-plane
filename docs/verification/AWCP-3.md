# AWCP-3 — Bootstrap delivery evidence

Date: 2026-09-12. Jira: [AWCP-3](https://erayyilmmaz.atlassian.net/browse/AWCP-3).
Scope: repository/Kubebuilder foundation, not workload implementation.

## Delivered

- Kubebuilder v4.15.0/go-v4 `PROJECT`, actual GitHub module, namespaced
  `platform.example.io/v1alpha1` AIWorkload API scaffold and generated CRD/DeepCopy.
- Read-only primary controller with scope/error guards; testable manager wiring;
  required WATCH_NAMESPACE/MANAGER_NAMESPACE, one namespace cache, leader election
  enabled by default, process liveness and cache/leader startup readiness.
- Exact Go/runtime/test/tool dependencies, project-local checksum-verified native
  tools, isolated generator/linter builds, normalized Make commands and module pins.
- Minimal namespaced bootstrap Role, manager Lease/Event Role, two-namespace
  Kustomize layout, restricted Pod, sample explicitly marked bootstrap-only.
- Multi-stage digest-pinned image, UID/GID 65532, OCI labels, allowlisted Docker
  context; Apache-2.0 LICENSE, editor/lint/ignore configuration and development guide.
- Controller/manager/manifest unit tests, real API/etcd integration and an isolated
  kind manager smoke with diagnostics and cleanup on failure.

## Environment and executed checks

Host: macOS ARM64, Go 1.26.8, Docker server 29.2.1. Envtest Kubernetes 1.36.2.
Kind v0.33.0 with the locked Kubernetes 1.36.4 node image (Linux ARM64).
Tools and native release archives were byte-checked before execution; Linux AMD64
host artifacts were not executed. Full versions and hashes are in the lock.

| Check | Observed result |
| --- | --- |
| `make bootstrap` on a clean temporary source export | Passed; fresh extracted native tools and fresh tool binaries; download/module/build caches were reused and verified, not claimed as a cold-cache test |
| `make generate` / `make manifests` | Passed; structural namespaced CRD, namespaced read-only Role and DeepCopy generated |
| `make build` / `go vet ./...` | Passed |
| `make lint` | Passed, 0 issues; formatting diff empty |
| `make test-unit` | Passed; `-short` explicitly skips integration |
| `make test` / direct `go test ./...` | Passed, including real API/etcd integration |
| `make test-race` | Passed on darwin/arm64, including integration |
| Prepared `GOPROXY=off go test ./...`, `go vet ./...`, `go mod verify` | Passed; no module downloads; all module checksums verified |
| `make verify-generated` | Second generation identical for CRD/Role/DeepCopy |
| `make render` | Passed; Kustomize resolves the complete bootstrap bundle |
| `make docker-build` | Passed, local Linux ARM64 manager image, non-root image user |
| `make smoke` | Passed: rollout, actual container UID 65532, restricted security, healthz/readyz, Lease and scoped positive/negative authorization |
| Empty namespace process startup | Exit 1 with configuration error before Kubernetes config loading |
| Shell syntax / whitespace / targeted credential-pattern review | Passed; no known private-key/token patterns found in implementation files; not a full security audit |

The clean-source run used `/private/tmp/awcp-clean.FbsWD2`, without source-tree
build outputs or installed tool binaries. It shared previously downloaded archives,
Go module cache and compiler cache explicitly. Bootstrap was run with normal
network availability, followed by all `make verify` gates. An intentionally offline
bootstrap attempt failed on Go's deprecation metadata lookup; this is why the guide
distinguishes first preparation from prepared offline tests.

## Behavior-level proof

`TestBootstrapReconcile` covers seven cases: existing read-only, missing success,
deleting read-only, out-of-scope no read, empty scope error, Forbidden preservation,
transient-error preservation. Write interceptors and object comparisons guard
against accidentally implementing mutation in this bootstrap.

`TestNamespaceValidation` rejects empty, whitespace, comma-separated, uppercase,
path-like, trailing-space and dotted namespace values for both settings.
`TestReadinessRequiresStartup` checks the default/unready, ready and shutdown
states and requires leader-scoped startup.

`TestManagerSecurity`, `TestGeneratedRBACAndCRD` and `TestDockerfileMatchesLock`
cover the deployment/CRD/Role/image baseline. The Ginkgo bootstrap envtest spec
creates CRs through the real API, round-trips the status subresource, verifies
cache visibility only in the selected namespace and observes a non-empty Lease
holder in the manager namespace. Manager cancellation and envtest Stop are checked;
Stop is not retried.

The smoke script created `awcp-bootstrap-awcp-smoke-xqrkav`, used a temporary
kubeconfig, then deleted that cluster. A subsequent `kind get clusters` returned
no clusters. Kubelet-reported container UID was 65532; `/healthz` and `/readyz`
returned `ok`. The runtime identity could read AIWorkloads only in awcp-workloads;
it could not read Secrets, create Deployments, update primary AIWorkloads or read
AIWorkloads in default. No global kubeconfig or existing cluster was used.

The initially tested image was local `awcp-manager:awcp-3`, image index
`sha256:70ecb50f65cfb207f286bc5d446e8be8dadccd6319fd531bdca3e53c2c3c3db1`.
Its OCI revision label recorded the parent HEAD because the source was not yet
committed. This is pre-commit local execution evidence, not a published release
artifact or a claim of a commit-attributed registry image.

## Issues found and resolved during implementation

1. Generator `paths=./...` entered the project-local tools' nested modules. It now
   scans only API/controller source paths, excluding tools and unrelated modules.
2. macOS Make's direct executable lookup did not honor the recipe PATH as expected.
   Commands now use the explicit project-local Go binary; paths with spaces work.
3. The initial image build preceded successful DeepCopy generation and failed.
   Generation was completed, then the image was rebuilt and passed real smoke.
4. The upstream header's YEAR placeholder produced an empty year with the selected
   generator invocation. The project boilerplate now explicitly records 2026;
   generated files are regenerated from that source, not hand-edited.

## Limits and next step

`spec` remains empty, and reconciliation does not create children or publish
workload conditions. Envtest is not a scheduler/kubelet/GC test. The kind test is
manager smoke, not AWCP-14's application lifecycle E2E. No multi-manager failover,
Linux AMD64 execution, CNI enforcement, hosted CI, security scan/pentest, registry
publication or release is claimed. Metrics/webhook servers are deliberately absent.

Next: AWCP-4 defines the typed AIWorkload contract, defaults, validation and status
schema with API tests. Delivery Git commit/push evidence is attached to Jira; the
commit containing this file is the source milestone. Original backlog snapshots
are retained unchanged.
