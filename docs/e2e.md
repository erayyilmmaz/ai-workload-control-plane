# kind end-to-end environment — AWCP-14

`make e2e` is the local, single-command acceptance environment. It builds the manager and two entirely local demo images, creates an isolated random-named kind cluster with an explicit temporary kubeconfig, loads those images into its node, deploys the rendered bundle, and removes that cluster on success or failure. It never reads, changes, or deletes the user's current kube context.

The same run also verifies the checked-in `small` tenant profile: a tenant-alpha
developer can create only its namespace-local parent, a Secret only in tenant-bravo
does not satisfy tenant-alpha, generated workload ServiceAccounts have no Secret
read permission, a LimitRange overage is rejected at admission, and a third 1-CPU
Pod is rejected by the 2-CPU ResourceQuota. These are kind API/RBAC/admission
checks; a CNI-specific packet test remains outside this reference environment.

```bash
make e2e
```

The kind node reference comes only from `toolchain.lock.json`; the selected Kubernetes 1.36.4 image is digest-pinned. The manager and demo images use `IfNotPresent`, so no registry pull or external AI/LLM credential is involved. The test assumes Docker, `curl`, and a usable local kind/Docker architecture. The project bootstrap downloads pinned `kind` and `kubectl` when absent.

## What it proves

The harness creates `lifecycle-demo` in `awcp-workloads`, with a user-owned Secret and a local HTTP demo application exposing `/health`, `/ready`, and `/version`. It verifies, with bounded waits:

1. create: Deployment, Service, ServiceAccount, NetworkPolicy, ready status and Service HTTP `v1`;
2. update: `v1` to `v2` image rollout and HTTP `v2` response;
3. scale: one to two Ready replicas;
4. drift: deleted Deployment, then every other owned child after manager restart, are recreated;
5. failure/recovery: deleting the referenced Secret yields `Ready=False` and `Degraded=True` with `SecretNotFound`; recreating it returns Ready without editing the CR;
6. observability: an ephemeral ServiceAccount bound only to the `/metrics` reader role can scrape converged gauges after restart;
7. deletion: Kubernetes garbage collection removes the owned tree while the user Secret remains.

Failure diagnostics include Pods, Events and manager logs. They intentionally do not query or print Secret data. The temporary metrics bearer token is likewise never written to output.

## Boundaries and platform notes

The standard profile asserts NetworkPolicy object generation only. kind's default networking is not a substitute for a pinned CNI enforcement profile, so deny/allow traffic enforcement is deliberately not claimed. The test is designed for macOS ARM64 local Docker/kind and Linux AMD64 CI, but each architecture needs its own executed result; no cross-platform success is inferred from the other.

The suite proves a single-manager lifecycle on a disposable local cluster. It does not prove production availability, multi-manager leader failover, external DNS, registry publication, hosted CI, or tenant isolation.
