# AWCP-15 — packaging and developer installation: execution evidence

## Scope delivered

- Kustomize is the canonical V0 installation package; Helm is explicitly deferred.
- Separate CRD-only, full operator and safe-uninstall Kustomizations are available.
- `make deploy` requires an explicit accessible immutable controller image and
  waits for CRD establishment and manager readiness without changing source files.
- `make release-bundle` emits three YAML artifacts and SHA-256 checksums; `make
  verify-package` asserts their separation, especially that uninstall excludes
  CRDs and Namespaces.
- Installation, upgrade/tagging, release-artifact and destructive-cleanup limits
  are documented in [installation.md](../installation.md).

## Execution record

On 2026-09-13, the completed local working tree passed:

```bash
make verify
make verify-package
DOCKER_CONFIG="$PWD/.tools/docker-public" \
DOCKER_HOST=unix:///Users/eray-refgen/.docker/run/docker.sock \
make e2e
```

`make verify` passed generation, build, vet, lint, root unit/envtest/manifests/
reconciliation tests, demo-app unit test, generated-file comparison and render.
`make verify-package` emitted and checked the CRD-only, full operator and safe
uninstall bundles plus their SHA-256 file. The kind E2E passed on Darwin ARM64
with Docker 29.2.1, kind v0.33.0, kubectl v1.36.4 and the pinned
`kindest/node:v1.36.4@sha256:099e049362a1526b2db71494e1947aae99bd16290d7c895f2b7ea312e3cbfaed` node.

During that E2E, `make deploy DEPLOY_IMG=awcp-manager:awcp-15` installed the
temporary image-patched full package and reached manager readiness. After workload
lifecycle checks, `make undeploy` removed the manager/RBAC/metrics while the CRD,
AIWorkload, owned child tree and user Secret remained. Explicit parent deletion
then completed Kubernetes garbage collection.

This is local execution evidence only. A local E2E manager image is not a
published production image, and a local package render is not hosted-release
evidence.

## Known boundaries

No Helm chart, published registry image, GitHub Release artifact, hosted CI run or
Linux AMD64 package execution is claimed by this story. The package deliberately
does not automatically remove CRDs, namespaces or workload objects.
