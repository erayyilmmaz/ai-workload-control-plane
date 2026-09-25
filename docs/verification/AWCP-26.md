# AWCP-26 — Workload exposure lifecycle and Gateway API HTTPRoute

## Delivered boundary

- `spec.exposure` is optional. `ClusterLocal` keeps the V0 ClusterIP Service
  contract; `HTTPRoute` creates one current-UID-owned, namespace-local route.
- HTTPRoute requires an existing Programmed Gateway in the same namespace, one
  listener section, exact hostname and PathPrefix. AWCP never creates/updates a
  Gateway or changes Service type.
- HTTPRoute requires explicit `spec.network.enabled: false`; Gateway data-plane
  ingress policy remains a platform boundary rather than an inferred broad
  NetworkPolicy exception.
- `ExposureReady` reflects HTTPRoute `Accepted` and `ResolvedRefs` independently
  of Deployment rollout `Ready`. V0 workloads do not discover Gateway API.

## Local evidence

| Command | Result | Scope |
| --- | --- | --- |
| `make generate manifests fmt test-unit` | passed | API schema, route intent, Gateway readiness and capability boundary |
| `make test-envtest` | passed | Real CRD admission, V0 compatibility and existing controller regression suite |
| `bash -n test/e2e/lifecycle-e2e.sh` | passed | E2E shell syntax and checksum-pinned Gateway installation flow |
| `make e2e` | environment gate | Local Docker daemon is unavailable; no local kind Gateway traffic result is claimed |

Hosted CI and its disposable kind HTTPRoute traffic result are recorded after the
implementation commit is pushed.

The first hosted run identified a reference-environment issue rather than an AWCP
controller failure: Envoy Gateway's default `LoadBalancer` Service cannot receive
an address on plain kind, so the Gateway stayed `Programmed=False`. The E2E fixture
now uses the documented EnvoyProxy `ClusterIP` Service override and re-runs the
same Gateway/route/traffic proof.

## Hosted evidence

[GitHub Actions run 36112989601](https://github.com/erayyilmmaz/ai-workload-control-plane/actions/runs/36112989601)
passed all 12 jobs for commit `e9d706e`. Its Linux AMD64 kind E2E completed the
Gateway `Programmed` check, AWCP-owned HTTPRoute `Accepted`/`ResolvedRefs` checks,
Host-header traffic through the Envoy Service, and guarded HTTPRoute garbage
collection while the platform Gateway remained present.
