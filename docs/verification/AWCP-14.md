# AWCP-14 — kind end-to-end environment: execution evidence

## Scope delivered

- `test/e2e/lifecycle-e2e.sh` creates an explicit-kubeconfig, uniquely named kind cluster and always targets cleanup to that cluster alone.
- `examples/demo-app/` supplies digest-based non-root local `v1`/`v2` HTTP images.
- `make e2e` renders the install bundle, builds/loads all images and exercises create, traffic, rollout, scale, drift, Secret recovery, manager restart, authenticated metrics and garbage collection.
- Bounded waits and safe failure diagnostics are built in; neither Secret data nor bearer tokens are emitted.

## Execution record

On 2026-09-13, the completed local working tree was executed with:

```bash
DOCKER_CONFIG="$PWD/.tools/docker-public" \
DOCKER_HOST=unix:///Users/eray-refgen/.docker/run/docker.sock \
make e2e
```

Result: **PASS**. The harness completed create, Service traffic, v1-to-v2
rollout, scale, Deployment drift recreation, manager restart plus Service/
ServiceAccount/NetworkPolicy drift recreation, Secret failure/restore,
authenticated metrics and owner-reference garbage collection; it then deleted
its `awcp-e2e-*` kind cluster successfully.

| Environment | Observed value |
| --- | --- |
| Source subsequently committed | `e2131afce2c49fff6f78901f6d086c377c138db8` |
| Host | Darwin arm64 |
| Docker server | 29.2.1 |
| kind | v0.33.0 (darwin/arm64) |
| kubectl client | v1.36.4 |
| kind node | `kindest/node:v1.36.4@sha256:099e049362a1526b2db71494e1947aae99bd16290d7c895f2b7ea312e3cbfaed` |

`make verify` also passed before this E2E run: generation/manifests, build,
vet, lint, root unit/envtest/manifests/reconciliation tests, demo-app unit test,
generated-file check and render. This is local execution evidence only; hosted CI
and CNI enforcement remain separate evidence.

## Known boundaries

The suite verifies standard NetworkPolicy generation but not CNI traffic enforcement. Its local macOS ARM64 result does not establish a Linux AMD64 CI result. It does not validate production install packaging, published images, multi-manager failover or a full tenant boundary.
