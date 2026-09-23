# AWCP-25 — External Secrets integration and Secret rotation

## Delivered boundary

- Optional `spec.externalSecrets` carries only namespace-local ESO
  `externalSecret` and `targetSecret` names; no provider path, credential or
  Secret value enters AWCP's API.
- AWCP requires ESO v1 discovery, a Ready namespaced `SecretStore`, Ready
  `ExternalSecret`, matching target and target Secret metadata. It rejects
  `ClusterSecretStore` and does not create/mutate ESO resources.
- Target Secret metadata resourceVersion is digested into the owned Pod template,
  triggering a normal Deployment rollout without data reads. Direct `secretRefs`
  retain their V0 no-auto-rotation behavior.
- The manager receives only namespaced read access to ESO objects. ESO provider
  identity and GitOps ownership remain external to AWCP.

## Local evidence

| Command | Result | Scope |
| --- | --- | --- |
| `make generate manifests fmt test-unit` | passed | API schema, deepcopy, RBAC and payload-free metadata/rotation unit contract |
| `make test-envtest` | passed | Real CRD admission/defaulting, V0 compatibility and reconciliation regression |
| `make verify` + package checks | passed | Build, vet, lint, generated artifacts, docs, GitOps, bundle and Terraform static validation |
| `make e2e` | environment gate | Docker daemon socket (`~/.docker/run/docker.sock`) was unavailable, so no local kind or ESO runtime result is claimed |

Hosted CI and its checksum-verified ESO kind E2E result are recorded after the
implementation commit is pushed.
