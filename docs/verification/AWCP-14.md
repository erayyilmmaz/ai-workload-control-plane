# AWCP-14 — kind end-to-end environment: execution evidence

## Scope delivered

- `test/e2e/lifecycle-e2e.sh` creates an explicit-kubeconfig, uniquely named kind cluster and always targets cleanup to that cluster alone.
- `examples/demo-app/` supplies digest-based non-root local `v1`/`v2` HTTP images.
- `make e2e` renders the install bundle, builds/loads all images and exercises create, traffic, rollout, scale, drift, Secret recovery, manager restart, authenticated metrics and garbage collection.
- Bounded waits and safe failure diagnostics are built in; neither Secret data nor bearer tokens are emitted.

## Execution record

The exact local command, tool versions, result and commit will be recorded here after the full kind run. A successful source build or envtest run is not E2E evidence, and hosted CI / CNI enforcement remain separate evidence.

## Known boundaries

The suite verifies standard NetworkPolicy generation but not CNI traffic enforcement. Its local macOS ARM64 result does not establish a Linux AMD64 CI result. It does not validate production install packaging, published images, multi-manager failover or a full tenant boundary.
