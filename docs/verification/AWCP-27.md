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
