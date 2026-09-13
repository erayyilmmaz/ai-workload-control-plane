# NetworkPolicy generation and boundary — AWCP-9

When `spec.network.enabled` is omitted, empty or true, AWCP creates one current-UID-
owned standard `networking.k8s.io/v1` NetworkPolicy. It selects only the generated
workload labels and permits one ingress path: TCP to the named pod port `http` from
any Pod in the same namespace. A `podSelector: {}` peer without a namespace selector
is namespace-local; Service IPs are never used as source identity.

`enabled: false` issues the same guarded optional delete used for Service lifecycle:
only the current parent UID's policy is removed. Foreign/stale-owner objects and
unrelated user NetworkPolicies remain untouched. AWCP owns the entire generated policy
spec, so drifted selectors, ingress rules, Egress rules or policy types are repaired;
unrelated labels/annotations are preserved.

## Explicit non-claims

The generated policy contains `policyTypes: [Ingress]` and no `egress` rules or
`Egress` policy type. It does not block DNS, Kubernetes API access, external model/API
calls or other outbound connections. Kubernetes NetworkPolicies are additive, so
other policies can further restrict or allow traffic. Enforcement depends on the
cluster's network plugin: the default kind smoke validates the API object only, not
allow/deny traffic behavior. No vendor-specific Cilium/Calico resource or CNI profile
is a V0 dependency.

## Inspection

```bash
kubectl -n <namespace> get networkpolicy <child-name> -o yaml
kubectl -n <namespace> describe networkpolicy <child-name>
```

See the [Kubernetes NetworkPolicy concepts](https://kubernetes.io/docs/concepts/services-networking/network-policies/),
[architecture contract](architecture.md), and [AWCP-9 evidence](verification/AWCP-9.md).
