# Local development — reconciliation foundation stage

This checkout contains a real, buildable Kubebuilder scaffold. It is **not yet a
workload operator**: API validation and the reconciliation engine work, but the
shipped manager has no production builder yet. Samples produce no children or
workload status. Tests inject a minimal four-kind plan to prove ownership, no-op,
drift, watches and restart. Never use contract samples as an application deployment.
AWCP-6 begins production field mappings.

## Prerequisites and first setup

Supported bootstrap hosts: macOS ARM64 and Linux AMD64. Install Git, Bash, Make,
curl, jq, tar and `shasum` (Perl Digest::SHA on Linux). A C compiler is needed for
`make test-race` (Xcode Command Line Tools on macOS; a C build toolchain on Linux).
Docker is needed only for image build and the isolated kind smoke test.
No globally installed Go, Kubebuilder, kubectl, kind or Kubernetes cluster is
required for unit/envtest checks. Docker Desktop is a prerequisite on macOS;
the scripts never launch a GUI or modify an existing kubeconfig.

```bash
git clone https://github.com/erayyilmmaz/ai-workload-control-plane.git
cd ai-workload-control-plane
make bootstrap
make verify
make test-race
```

`make bootstrap` downloads Go 1.26.8, Kubebuilder 4.15.0 and envtest 1.36.2,
verifies each archive's SHA-256 against `toolchain.lock.json` **before execution**,
and installs into ignored `.tools/`. It builds pinned controller-gen, Kustomize,
golangci-lint and setup-envtest in separate tool dependency contexts. Go modules
are verified using `go.sum` and the Go checksum database. No `curl | bash`,
global installation, `latest` selector or version fallback is used.

The first setup requires network access to the upstream releases and Go module
proxy/checksum database. Prepared unit/envtest runs can use `GOPROXY=off`; they
start only local API-server/etcd processes, use no existing cluster and contact
no AI service. This offline claim does not cover first image pulls or first
compilation of an uncached cross-platform dependency.

Go's initial tool installation can query module deprecation metadata even with
cached module archives, so **bootstrap itself is not an offline workflow**.

## Commands

| Command | Purpose |
| --- | --- |
| `make generate` | Generate API DeepCopy methods from authored types |
| `make manifests` | Generate structural CRD and bounded namespaced controller Role |
| `make verify-generated` | Regenerate and compare CRD, Role and DeepCopy byte-for-byte |
| `make build` | Compile `bin/manager`, without contacting a Kubernetes cluster |
| `make vet` / `make lint` | Go vet, selected static linters and formatting checks |
| `make fmt` / `make lint-fix` | Explicit formatting/fix commands (mutate source) |
| `make test-unit` | Fast `-short` tests; envtest is explicitly skipped |
| `make test-envtest` | Manager integration, API contract and reconciliation/watch suites |
| `make test` | Full `go test -count=1 ./...`, including envtest |
| `make test-race` | Full tests with Go race detector |
| `make render` | Validate Kustomize references and write ignored `dist/install.yaml` |
| `make verify` | Generation, build, vet, lint, tests, generation stability and rendering |
| `make tidy` | Tidy modules, reapply exact release-family pins, download checksums |
| `make docker-build` | Local image only; never pushes an image |
| `make smoke` | Isolated kind manager/container smoke and automatic cleanup |

Use `make tidy` rather than plain `go mod tidy`: lazy loading can otherwise
remove explicit patch pins for Kubernetes modules not yet compiled by this
bootstrap. All seven selected release-family modules must remain v0.36.4.
Independent k8s.io modules retain their own version lines. Tools do not add their
dependencies to the manager module.

For direct Go commands or an IDE, set these in the checkout root:

```bash
export PATH="$PWD/.tools/go/bin:$PATH"
export GOPATH="$PWD/.tools/gopath"
export GOCACHE="$PWD/.tools/cache/go-build"
export GOTOOLCHAIN=local
export KUBEBUILDER_ASSETS="$PWD/.tools/envtest"
go test ./...
go vet ./...
go mod verify
```

Missing `KUBEBUILDER_ASSETS` fails the integration suite with setup instructions;
it never silently skips required coverage. The pinned setup-envtest utility is
available in `.tools/bin/setup-envtest-<revision>/setup-envtest`. The default
asset installer uses the locked release archive directly so its bytes, not just
its version selector, are checked. Envtest teardown calls `Stop` once and reports
failure rather than retrying a failed shutdown away.

## What the tests prove

- Controller unit cases: existing/deleting objects remain unchanged, missing
  objects succeed, out-of-scope requests cause no reads, missing configuration
  fails closed, and Forbidden/transient API errors are preserved for runtime retry.
- Manager unit cases: empty/multiple/malformed namespaces are rejected; readiness
  is false before startup and after shutdown and requires leader-scoped startup.
- AWCP-5 controller cases: four-kind create/patch/no-op, write counts, foreign/stale
  owners, optional deletes, concurrent patch/create/delete races, safe error
  categories, bounded failure status/events, parent predicate and Secret index.
- Manifest cases: namespaced CRD/status subresource, exact bounded Role,
  single-replica manager, restricted security, probes, namespace settings, pinned images.
- Envtest: actual CRD registration, namespaced create/status round-trip, cache
  visibility and exclusion, a Lease in the manager namespace, graceful shutdown.
- AWCP-4 contract envtest (no AWCP manager): defaults, zero/false preservation,
  quantity CEL, valid/invalid fixtures, Strict vs pruning, status/spec isolation,
  generation behavior, kubectl explain and server printer columns.
- AWCP-5 reconciliation envtest: real server defaults, Service IP and injected
  sidecar preservation, four-kind drift/delete watches, status/Secret events,
  stable resourceVersions, failure isolation, Events and manager restart.
- Kind smoke: image UID 65532, restricted runtime security, real health/readiness,
  manager rollout, leadership and positive/negative foundation authorization.

Envtest uses an administrator test client; it does not prove runtime RBAC.
Kind smoke checks the foundation Role with the shipped manager (nil builder),
not the test-only plans. Neither layer proves application rollout, garbage
collection, complete Secret dependency recovery, NetworkPolicy enforcement,
multi-replica leader failover or telemetry. Those belong to later stories.

## Container and isolated smoke

```bash
make docker-build
make smoke
```

The Dockerfile pins both the Go builder and distroless runtime by digest, includes
OCI source/license/version/revision labels, copies only the manager binary and
runs as `65532:65532`. The Pod drops every capability, disables privilege
escalation, uses RuntimeDefault seccomp and a read-only root filesystem; the
manager needs no writable mount. Its ServiceAccount token remains mounted because
it must contact Kubernetes. Future workload identities are a separate boundary.

`IMG` defaults to `awcp-manager:awcp-3`; override it consistently for build/smoke.
`REVISION` defaults to the current Git HEAD; before a source commit it identifies
the parent, not the uncommitted source. Rebuild with the committed revision when
producing an attributable image. The local image is not a published release.

The smoke script installs pinned kind/kubectl if absent, creates a random uniquely
named `awcp-bootstrap-*` cluster with the locked node digest and a temporary
kubeconfig, loads the local image and applies the bootstrap manifests. It refuses
an existing name. It prints diagnostics on failure and removes only its own
cluster/kubeconfig on exit, including interrupted runs. Images and tool/build
caches remain reusable. It never runs against the current user cluster.

Do **not** use `kubectl delete -f dist/install.yaml` as ordinary undeploy: the
bootstrap bundle includes Namespaces and a CRD, and deleting those destroys CRs
and other contents. There is intentionally no generic uninstall target here.
Safe packaging/install/uninstall workflows are AWCP-15's deliverable.

## Generated versus owned source

| Files | Ownership and editing rule |
| --- | --- |
| `api/v1alpha1/zz_generated.deepcopy.go`, `config/crd/bases/*`, `config/rbac/role.yaml` | Generated; edit types/markers and rerun Make |
| `PROJECT` | Kubebuilder CLI metadata; retain actual module/API identity |
| `api/v1alpha1/*_types.go`, `groupversion_info.go` | Initially scaffolded, now project-owned source |
| `cmd`, `internal`, `test`, other `config` files, Dockerfile/Makefile | Project-owned bootstrap adaptations with tests |
| `internal/resource` | Builder/Intent contract and naming; production mappings pending |
| `internal/telemetry` | Reserved boundary; not an implemented feature |
| `docs/backlog/*` | Original dated planning snapshot, not a live status board |

Scaffold provenance: checksum-verified Kubebuilder v4.15.0, go/v4, generated in a
scratch directory with `init --domain example.io --repo
github.com/erayyilmmaz/ai-workload-control-plane --project-name
ai-workload-control-plane --namespaced`, then `create api --group platform
--version v1alpha1 --kind AIWorkload --resource --controller --namespaced`.
Generation was deferred until exact dependencies were pinned. Existing architecture
documents were preserved. The tagged sample and CLI template differ in Ginkgo/
Gomega patch versions; both are explicitly normalized to the accepted lock.

The generated generic README, AGENTS guide, CI/devcontainer, webhook/cert-manager,
Prometheus and cluster-wide permission examples were not copied. The manager
factory lives in `internal/manager` for lifecycle tests. Authenticated metrics are
deferred to AWCP-11, not exposed anonymously by the bootstrap.

## References

- [Kubebuilder project and API generation](https://book.kubebuilder.io/quick-start.html)
- [Distroless runtime images](https://github.com/GoogleContainerTools/distroless)
- [Version/asset evidence](compatibility.md)
- [AWCP-3 execution evidence](verification/AWCP-3.md)
