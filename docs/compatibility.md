# Technical baseline and compatibility

Decision date: 2026-09-12. Machine-readable source: [toolchain.lock.json](../toolchain.lock.json).

**AWCP-2 evidence was source/metadata-only. AWCP-3 adds local byte verification,
resolved modules and executable checks; exact outcomes are in
[AWCP-3 evidence](verification/AWCP-3.md). Unexecuted platforms remain unverified.**

## Selected versions

| Component | Exact selection | Reason / evidence |
| --- | --- | --- |
| Go language | 1.26.0 | Minimum in controller-runtime and selected scaffold go.mod |
| Go toolchain | 1.26.8 | Go release metadata includes darwin/arm64 and linux/amd64 archives |
| Kubebuilder | v4.15.0, go/v4 plugin | Tagged scaffold uses controller-runtime v0.24.1 and Kubernetes client minor v0.36 |
| controller-runtime | v0.24.1 | Runtime source minimum Go 1.26.0; client minor v0.36 |
| Kubernetes release-family client modules | v0.36.4 | Patch update within the selected client minor; compare the expected delta to scaffold v0.36.0 |
| controller-tools / controller-gen | v0.21.0 | Selected scaffold pin; generator module targets Kubernetes v0.36 and Go 1.26 |
| setup-envtest | 3be3f1bf2b2fcc6b5c9510d55c6a9972294653d0 | Commit behind controller-runtime v0.24.1; nested module exists at that revision |
| envtest API binaries | 1.36.2 | Exact published release has darwin-arm64 and linux-amd64 tarballs |
| kind | v0.33.0 | Published native CLI assets and selected pre-built node image |
| kind Kubernetes server | 1.36.4 | Digest-pinned pre-built node image below |
| kubectl | v1.36.4 | Official release checksum endpoints verified for both host platforms |
| Kustomize | v5.8.1 | Scaffold pin; tool's Go minimum is 1.24.0 |
| golangci-lint | v2.12.2 | Scaffold pin; source's Go minimum is 1.25.0; build using selected Go 1.26.8 |
| Ginkgo / Gomega | v2.28.0 / v1.39.1 | Selected scaffold test dependency versions |

The v0.36.4 family is `k8s.io/api`, `apimachinery`, `client-go`, `apiextensions-apiserver`, `apiserver`, `component-base` and `streaming`. Do not assign that version to independent `k8s.io/*` modules. Their resolved pins remain klog/v2 v2.140.0, utils v0.0.0-20260210185600-b8788abfbbc2 and kube-openapi v0.0.0-20260317180543-43fb72c5454a. AWCP-3 resolved all seven family modules to v0.36.4 and verified the module checksums. `make tidy` preserves family pins even for lazy-loaded members not compiled by the bootstrap.

The patch resolution also selected newer transitive x/net v0.56.0, x/sys v0.46.0,
x/text v0.39.0, x/tools v0.47.0 and structured-merge-diff/v6 v6.3.3. The full graph
is recorded in go.mod/go.sum; generator/linter dependencies are separately built
and do not override the runtime graph. Actual v4.15.0 CLI templates initially
resolved Ginkgo v2.27.4/Gomega v1.39.0 (different from the tagged sample inspected
in AWCP-2); the project explicitly normalizes them to v2.28.0/v1.39.1.

The lock is a version decision and published-artifact record, not a substitute for Go's module checksum database or a claim that binaries have been downloaded and verified locally. Build tools in their own dependency context; do not force generator dependencies into the operator's runtime module graph.

## Kubernetes node image

```text
kindest/node:v1.36.4@sha256:099e049362a1526b2db71494e1947aae99bd16290d7c895f2b7ea312e3cbfaed
```

The kind v0.33.0 release explicitly lists this image and states that its node images support linux/amd64 and linux/arm64. A macOS ARM64 host uses Linux ARM64 nodes in Docker's Linux VM; there is no Darwin Kubernetes node image. Runtime architecture support still needs actual E2E execution on each claimed platform.

The release introduction says “defaults to Kubernetes 1.36.1”, while its detailed default/pre-built image list points to 1.37.0 and includes 1.36.4. The backlog initially repeated that introduction. We avoid the inconsistent default statement by always supplying the explicit 1.36.4 digest above. Do not use kind's implicit default image.

## Artifact availability

| Artifact | macOS ARM64 developer host | Linux AMD64 CI host | Verification |
| --- | --- | --- | --- |
| Go 1.26.8 | darwin-arm64 archive listed | linux-amd64 archive listed | Published SHA-256 in official Go JSON |
| Kubebuilder v4.15.0 | Native binary listed | Native binary listed | GitHub release asset digest |
| kind v0.33.0 | Native binary listed | Native binary listed | GitHub release asset digest |
| kubectl v1.36.4 | Official checksum endpoint returns digest | Official checksum endpoint returns digest | Official dl.k8s.io SHA-256 |
| envtest v1.36.2 | Native tarball listed | Native tarball listed | controller-tools release asset digest |
| kind node v1.36.4 | Linux ARM64 image via Docker | Linux AMD64 image | Upstream release lists multi-architecture image digest |

All exact asset URLs and SHA-256 values are in the lock. AWCP-3 downloaded all five
darwin-arm64 assets, verified their bytes and executed native version/process
checks. Linux AMD64 archive entries are still published metadata only; Linux
artifacts cannot be runtime-tested directly by a Darwin process.

envtest 1.36.2 and kind 1.36.4 intentionally share minor 1.36 but use different available patch artifacts. Envtest validates API behavior; kind validates real cluster behavior. No broader Kubernetes minor support is claimed. First preparation may need network access; prepared unit/envtest runs do not call external AI services.

## Scaffold comparison and required bootstrap adjustments

Tagged reference files were inspected at Kubebuilder commit `034c380389c00396878da8b388d42b17d55f8dd8`:

| Setting | v4.15.0 upstream scaffold | AWCP decision |
| --- | --- | --- |
| Go directive | 1.26.0 | Keep; run exact toolchain 1.26.8 |
| controller-runtime | v0.24.1 | Keep |
| Core k8s.io modules | v0.36.0 | Upgrade together to v0.36.4; validate resulting graph in AWCP-3 |
| controller-tools | v0.21.0 | Keep |
| Kustomize | v5.8.1 | Keep |
| golangci-lint | v2.12.2 | Keep |
| ENVTEST_VERSION | Derived from runtime version | Exact source commit in lock |
| ENVTEST_K8S_VERSION | Derived minor 1.36 | Exact 1.36.2, not a floating selector |
| kind create | No explicit image in sample Makefile | Supply pinned 1.36.4 node digest |
| Sample project dependencies | Includes webhook/cert-manager demonstrations | Do not copy these into AWCP: no webhook or cert-manager requirement |
| Image | Floating controller:latest in example | Local explicit dev version; release digest later |

Kubebuilder v4.16.0 was evaluated, not selected: its tagged scaffold uses controller-runtime v0.25.0 / k8s.io v0.37.0, which would move this project to a different baseline. v4.16.0 release fixes relevant to V0 must be carried into the later implementation: envtest Stop failure must be checked once (not retried away), cleanup must survive failed tests, and generated network rules must target the actual pod port. Webhook/Helm/autoupdate additions are outside this V0 scope.

AWCP-3 bootstrap entry point, after installing the exact tools, is `kubebuilder init --domain example.io --repo github.com/erayyilmmaz/ai-workload-control-plane --plugins go/v4`, followed by an API for group `platform`, version `v1alpha1`, kind `AIWorkload`. Run in a scratch directory first so the scaffold's README/Makefile/.gitignore do not overwrite these design documents. Move reviewed scaffold outputs into this existing repository. The actual API fields belong to AWCP-4.

## Change policy

Pin exact versions; no `latest`, `1.36.x`, floating branch or wildcard tool selectors in reproducible workflows. An update must change the lock, local/CI configuration and this matrix together, then run generation, build, relevant unit/envtest and kind tests. Library minimums, source compatibility, asset availability and executed runtime compatibility are separate claims.

The local host currently exposes Docker, kind, kubectl, Node and Python on PATH; Go, Kubebuilder, standalone Kustomize and golangci-lint were not found on PATH during AWCP-2. Installation and process execution belong to AWCP-3. No user's global toolchain, Docker cluster or kubeconfig was changed by this design milestone.

## AWCP-3 bootstrap adaptations

Go and all build tools are now project-local under `.tools/`. `Makefile` uses an
explicit Go executable path for compatibility with macOS Make 3.81 and checkout
paths containing spaces. Controller-gen scans only `api` and `internal/controller`,
not the nested tool module cache. Go's standard `./...` test/package commands
already exclude dot-prefixed directories.

The bootstrap is single-namespace and read-only; primary/status mutation and
child-resource permissions are not granted before those features exist. Both
namespace settings are mandatory, leader election defaults on, health is process
liveness, readiness waits for cache sync and leader-scoped startup. Metrics and
webhook servers are not exposed. Only one manager replica is supported.

The builder and runtime images are digest-pinned in Dockerfile and the lock:
Go 1.26.8 bookworm (`9fdc884a…`) and distroless static nonroot (`1c2c046b…`).
The full digests came from registry OCI indexes on 2026-09-12. Container tests do
not imply a published image, multi-architecture execution or a vulnerability audit.

## Sources

- [Tagged scaffold go.mod](https://github.com/kubernetes-sigs/kubebuilder/blob/v4.15.0/testdata/project-v4/go.mod)
- [Tagged scaffold Makefile](https://github.com/kubernetes-sigs/kubebuilder/blob/v4.15.0/testdata/project-v4/Makefile)
- [Newer scaffold comparison](https://github.com/kubernetes-sigs/kubebuilder/blob/v4.16.0/testdata/project-v4/go.mod)
- [controller-runtime go.mod](https://github.com/kubernetes-sigs/controller-runtime/blob/v0.24.1/go.mod)
- [setup-envtest source](https://github.com/kubernetes-sigs/controller-runtime/blob/v0.24.1/tools/setup-envtest/go.mod)
- [controller-tools module](https://github.com/kubernetes-sigs/controller-tools/blob/v0.21.0/go.mod)
- [Kustomize module](https://github.com/kubernetes-sigs/kustomize/blob/kustomize/v5.8.1/kustomize/go.mod)
- [golangci-lint module](https://github.com/golangci/golangci-lint/blob/v2.12.2/go.mod)
- [Kubernetes API patch module](https://github.com/kubernetes/api/blob/v0.36.4/go.mod)
- [Kubebuilder binary assets](https://github.com/kubernetes-sigs/kubebuilder/releases/tag/v4.15.0)
- [kind release and images](https://github.com/kubernetes-sigs/kind/releases/tag/v0.33.0)
- [envtest assets](https://github.com/kubernetes-sigs/controller-tools/releases/tag/envtest-v1.36.2)
- [Go release metadata](https://go.dev/dl/?mode=json&include=all)
