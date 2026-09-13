# Service discovery and exposure lifecycle — AWCP-7

Each enabled `AIWorkload` receives one current-UID-owned, cluster-local Kubernetes
Service named with the deterministic child name from ADR-004. V0 supports exactly
one `ClusterIP` Service with one TCP port named `http`. The Service port defaults to
80 and targets the Deployment container's named `http` port; it never copies the
numeric container port into `targetPort`. This keeps a Service port update separate
from the workload's internal listener contract.

## Ownership boundary

| Field | Controller-owned | Preserved or rejected |
| --- | --- | --- |
| Identity and metadata | Deterministic name/namespace, current parent owner reference, common labels and workload-name annotation | Other labels and annotations |
| Selector | Exact CR UID and child-name labels | No selector broadening or adoption |
| Exposure | `type: ClusterIP`, one TCP `http` port, configured Service port, named targetPort `http` | NodePort, LoadBalancer, ExternalName and headless (`clusterIP: None`) are rejected |
| Cluster allocation | Never written | API-assigned `clusterIP`, `clusterIPs`, `ipFamilies`, `ipFamilyPolicy` remain intact |

Ports are an intentional whole-list ownership boundary. Extra or altered Service
ports are repaired on reconcile. An existing current-UID-owned Service that has an
incompatible type or headless allocation is not deleted or replaced: the controller
reports `ResourceOwnershipConflict` and requires an administrator to resolve it.
Foreign or stale-owner Services remain untouched under the AWCP-5 ownership guard.

## Enable, update and discovery semantics

`spec.service` omitted, `{}`, or `enabled: true` creates/repairs the Service.
`enabled: false` issues a guarded delete only for the Service currently owned by the
same parent UID, then clears `status.endpoint`. A foreign collision is preserved and
also leaves the endpoint empty. Changing `spec.service.port` patches the Service
while preserving its allocated cluster address.

After a successful enabled Service apply, the controller publishes:

```text
<child-name>.<namespace>.svc:<service-port>
```

This is in-cluster discovery data only. It does not hard-code a cluster domain,
does not prove DNS resolution, endpoints, readiness, HTTP success or network-policy
reachability. Kubernetes only routes Service traffic to ready endpoints; full
workload identity and a runnable, pinned demo image arrive in later milestones, so
AWCP-7 does not claim an application traffic E2E result.

## Diagnosis

Use authorized, read-only inspection first:

```bash
kubectl -n <namespace> get service <child-name> -o wide
kubectl -n <namespace> describe service <child-name>
kubectl -n <namespace> get aiworkload <name> -o jsonpath='{.status.endpoint}'
```

See [Kubernetes Service concepts](https://kubernetes.io/docs/concepts/services-networking/service/),
[reconciliation semantics](reconciliation.md), and
[AWCP-7 verification evidence](verification/AWCP-7.md).
