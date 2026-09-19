# Policy profiles and ValidatingAdmissionPolicy — AWCP-24

AWCP policy profiles are fixed platform vocabulary, not user-supplied CEL and not
controller-owned resources. `AIWorkload.spec.policy` accepts only `baseline` or
`restricted`; omitting it preserves the V0 API behavior in namespaces with no
matching admission binding.

## Reference profile model

| Profile | Namespace selection | Required workload fields | Enforcement |
| --- | --- | --- | --- |
| baseline | No AWCP binding | Existing CRD contract only | V0-compatible; no added admission rule |
| restricted | `platform.example.io/policy-profile: restricted` | `policy: restricted`, `tenant`, CPU/memory requests and limits, `network.enabled: true` | API request is denied before reconciliation |

The reference tenant namespaces are labeled `restricted` by GitOps. The policy
matches only `platform.example.io/v1alpha1` `AIWorkload` CREATE/UPDATE requests;
it does not mutate Deployments, read Secrets, configure CNI behavior, or grant the
manager extra permission. It has no parameters, avoiding a parameter-resource
read-authorization dependency. The binding uses `Deny` and the policy uses
`failurePolicy: Fail`: a malformed policy is safe-by-default for selected tenant
namespaces.

The manager does not create, modify, watch, or need RBAC for the
ValidatingAdmissionPolicy or its binding. GitOps owns both cluster-scoped objects,
and `make undeploy` deliberately removes them with the manager package so an
uninstalled AWCP package does not leave a hidden admission boundary behind.

## Developer result

In a restricted tenant namespace, a compliant parent starts as follows:

```yaml
spec:
  tenant: alpha
  policy: restricted
  resources:
    requests: {cpu: 100m, memory: 128Mi}
    limits: {cpu: 500m, memory: 512Mi}
  network:
    enabled: true
```

An omitted/`baseline` policy, missing tenant, partial compute resources, or
`network.enabled: false` is rejected by the API server with a fixed actionable
message. No AIWorkload status/Event can exist for a rejected object because the
controller never receives it.

Kubernetes requires both a policy and binding for a validating policy to have an
effect; a binding’s `Deny` action rejects a failed validation. [Kubernetes
ValidatingAdmissionPolicy](https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/)

## Verification boundary

`TestRestrictedAdmissionPolicy` uses a real envtest API server to install the
checked-in policy and binding, then proves deny/allow and unselected V0 behavior.
The kind E2E additionally attempts a `baseline` workload in tenant-alpha and
expects admission rejection. This proves Kubernetes API admission, not a
production policy rollout, audit sink, managed-cluster feature configuration, or
arbitrary organization policy language.
