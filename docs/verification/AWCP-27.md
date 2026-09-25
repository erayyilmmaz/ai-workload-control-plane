# AWCP-27 — Autoscaling and HPA lifecycle

## Delivered boundary

- Optional `spec.autoscaling` renders one owned `autoscaling/v2` HPA targeted
  only at the workload's owned Deployment.
- CPU, memory, Pods and External metric contracts are typed and admission-tested;
  arbitrary adapter locations, credentials and scale targets are absent.
- HPA-enabled Deployment updates preserve the live replica value, so AWCP does
  not fight the HPA scale subresource. Omission/disabled mode retains V0 static
  `spec.replicas` behavior.
- `AutoscalingReady` surfaces active HPA status or a safe metric-unavailable
  observation. AWCP does not install/read a metrics provider.
- `minReplicas: 0` is deliberately rejected on Kubernetes 1.36. Scale-to-zero,
  Object metric and external-adapter E2E proof require the later 1.37 rebaseline.

## Local evidence

| Command | Result | Scope |
| --- | --- | --- |
| `make generate manifests fmt test-unit` | passed | API, HPA intent, ownership/RBAC and static behavior |
| `make test-envtest` | passed | Real CRD admission for valid/invalid autoscaling contracts and regression suite |
| `make e2e` | environment gate | Local Docker daemon is unavailable; no local kind HPA result is claimed |

Hosted CI evidence is added after the implementation commit is pushed.

## Hosted evidence

[GitHub Actions run 36134253689](https://github.com/erayyilmmaz/ai-workload-control-plane/actions/runs/36134253689)
passed all 12 jobs for commit `d14ec3d`, including Linux AMD64 kind E2E. The
hosted E2E protects the existing workload lifecycle while the HPA contract is
validated through generated-manifest, unit and envtest gates. It does not claim
metric-adapter-backed HPA scale-up/down or scale-to-zero, which remain explicit
platform/rebaseline gates.
