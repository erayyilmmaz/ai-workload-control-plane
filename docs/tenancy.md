# Tenant model, namespace boundaries, quotas and limits

AWCP-23 provides a reference multi-tenant Kubernetes layout for one cluster. It
does not add a cluster-wide AWCP controller, a tenant CRD, an identity provider,
or a billing system. Tenant onboarding is a reviewed Git change applied by the
existing platform Argo CD Application.

## Boundary and ownership

Each tenant owns one namespace: `awcp-tenant-alpha`, `awcp-tenant-bravo`, or
`awcp-tenant-charlie`. Namespace labels identify the immutable GitOps intent:
`platform.example.io/tenant` and `platform.example.io/quota-profile`. The
namespace-local `awcp-tenant-profile` ConfigMap binds the human-readable tenant
name to one of `small`, `medium`, or `large`.

An `AIWorkload.spec.tenant` is optional for V0 compatibility. When it is present,
AWCP reads only that fixed ConfigMap name in the parent namespace and requires an
exact tenant match before creating its ServiceAccount, Deployment, Service, or
NetworkPolicy. A workload cannot select a namespace, look up a ConfigMap in a
second namespace, or use a cross-namespace Secret reference; `secretRefs` remains
a list of local Secret names.

GitOps owns the namespace, profile ConfigMap, ResourceQuota, LimitRange,
tenant-developer Role/RoleBinding, and default-deny-ingress NetworkPolicy. AWCP
owns only UID-bound workload children and their status/events. The controller
receives separate same-named `Role`/`RoleBinding` pairs in the four explicit
watched namespaces. It receives no ClusterRole for workloads, Secrets, ConfigMaps,
or quotas. Its ConfigMap permission is a `get` constrained to
`awcp-tenant-profile`; it cannot list arbitrary ConfigMaps.

## Profiles

| Profile | Reference tenant | CPU requests / limits | Memory requests / limits | Object ceiling |
| --- | --- | --- | --- | --- |
| small | alpha | 2 / 4 | 4Gi / 8Gi | 10 Pods, 10 Services, 20 ConfigMaps and Secrets |
| medium | bravo | 8 / 16 | 16Gi / 32Gi | 40 Pods, 30 Services, 60 ConfigMaps and Secrets |
| large | charlie | 16 / 32 | 32Gi / 64Gi | 100 Pods, 60 Services, 120 ConfigMaps and Secrets |

The matching LimitRange supplies conservative CPU/memory defaults, establishes a
minimum of `10m` CPU and `32Mi` memory, and caps one container at 1 CPU/1Gi,
2 CPU/4Gi, or 4 CPU/8Gi respectively. ResourceQuota is the fairness ceiling;
LimitRange is the per-container admission policy. Kubernetes documents that
ResourceQuota is namespace-scoped and enforced by admission, while LimitRange
constrains resource requests/limits per object. [ResourceQuota](https://kubernetes.io/docs/concepts/policy/resource-quotas/)
and [LimitRange](https://kubernetes.io/docs/concepts/policy/limit-range/).

## Access model and operational signals

The `awcp:tenant-<name>-developers` group receives a namespace-local Role that
can manage parent `AIWorkload` objects only. It cannot read Secrets, write status,
or access another tenant namespace. Generated workload ServiceAccounts receive no
RoleBinding and have token automount disabled, so they do not inherit controller
or developer permissions. This uses Kubernetes' namespace-scoped Role/RoleBinding
model rather than a broad ClusterRole. [Kubernetes RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
and [multi-tenancy guidance](https://kubernetes.io/docs/concepts/security/multi-tenancy/).

`TenantReady=True/TenantConfigured` confirms the `spec.tenant` to profile match.
`TenantNotConfigured`, `TenantMismatch`, and `TenantProfileInvalid` set
`TenantReady=False` and prevent child creation. A Kubernetes forbidden error that
reports quota exhaustion becomes the safe `QuotaExceeded` condition/event reason;
the reconciliation remains retryable and the manager process continues.

## Verify locally

```bash
make render test-unit
.tools/bin/kubectl auth can-i create aiworkloads.platform.example.io \
  --as=tenant-alpha --as-group=awcp:tenant-alpha-developers \
  -n awcp-tenant-alpha
.tools/bin/kubectl auth can-i get secrets \
  --as=tenant-alpha --as-group=awcp:tenant-alpha-developers \
  -n awcp-tenant-bravo
```

Expected results are `yes` for the first command and `no` for the second. On a
disposable kind cluster, apply the package and create a Pod exceeding the selected
LimitRange maximum to observe admission rejection; do not perform quota stress on
a shared cluster. The repository's static and controller tests prove the bounded
RBAC/namespace configuration and mismatch/QuotaExceeded paths; a live CNI is
still required to prove NetworkPolicy packet enforcement.
