# AWCP-9 — Network isolation and NetworkPolicy reconciliation: execution evidence

Date: 2026-09-13

## Delivered scope

- The production builder emits an optional standard `networking.k8s.io/v1`
  NetworkPolicy after ServiceAccount, Deployment and Service.
- Enabled/default policy selects only this CR's UID/child-name pods and allows only
  same-namespace Pod ingress to named TCP port `http`.
- The managed policy contains `policyTypes: [Ingress]`, no Egress type/rules and no
  vendor-specific resources. Its whole spec is a deliberate drift-repair boundary;
  unrelated metadata remains intact.
- `network.enabled: false` uses guarded deletion. Foreign/stale-owner and unrelated
  policies are not adopted, changed or removed by the controller.

## Validation

| Command | Result | Coverage |
| --- | --- | --- |
| `make test-unit` | Passed | Pure selector, peer, port, toggle and drift contract |
| `KUBEBUILDER_ASSETS=... go test -count=1 -v ./test/reconciliation` | Passed | Real API create, drift repair, disable/delete, re-enable and unrelated policy preservation |
| `make verify` | Passed | Full build/lint/unit/envtest/generated/render validation |
| `make test-race` | Passed | Full race detector validation |
| `DOCKER_CONFIG=... DOCKER_HOST=... make docker-build IMG=awcp-manager:awcp-9` | Passed | Local Linux ARM64 manager image for commit `244a64b` |
| `DOCKER_CONFIG=... DOCKER_HOST=... make smoke IMG=awcp-manager:awcp-9` | Passed | Fresh kind bootstrap: manager startup, identity, Deployment, Service and NetworkPolicy API contracts |

`TestProductionDeployment/NetworkPolicy_drift_toggle_and_unrelated_policy_lifecycle`
uses a real API server and validates API object lifecycle, not CNI dataplane behavior.

## Limits

Envtest and the default kind setup do not establish an enforcing CNI profile. Therefore
same-namespace allow, cross-namespace deny, Service traffic and Egress reachability
are not claimed as traffic-test results. The policy does not implement egress isolation
or an enterprise firewall; other NetworkPolicies remain additive. Workload readiness
and the full status reducer remain AWCP-10 work.

See [NetworkPolicy behavior](../network-policy.md) and the official
[Kubernetes NetworkPolicy documentation](https://kubernetes.io/docs/concepts/services-networking/network-policies/).
