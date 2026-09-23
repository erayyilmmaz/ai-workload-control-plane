# Workload identity and Secret prerequisite boundary — AWCP-8

Each `AIWorkload` has one deterministic, current-UID-owned ServiceAccount in the
workload namespace. It shares the child name used by the Deployment and Service.
The controller creates it before the Deployment, sets
`automountServiceAccountToken: false`, and preserves unrelated ServiceAccount
metadata and server-maintained fields. The Pod independently sets the same token
policy, so it cannot silently fall back to the namespace `default` identity.

No workload Role, ClusterRole, RoleBinding or ClusterRoleBinding is generated. The
manager identity remains separate and has the namespaced Role documented in
[ADR-005](adr/ADR-005-security-and-secret-boundaries.md). A workload creator can
still run images and choose same-namespace Secret references; this is not complete
multi-tenant isolation.

## Secret behavior

`spec.secretRefs` remains an ordered `envFrom.secretRef` list in the Deployment. On
each reconcile, AWCP requests only a `PartialObjectMetadata` representation of each
referenced Secret. It never copies, updates, deletes, hashes, logs or publishes
Secret data. Kubernetes Secret `get/list/watch` RBAC nevertheless authorizes payload
access at the API boundary; metadata-oriented use reduces handling in this process,
not the permission's power.

If one or more referenced Secrets are absent, children (including the dedicated
identity) may exist but the controller publishes `Ready=False`, `Degraded=True`,
reason `SecretNotFound`, with a generic message that includes neither a Secret name
nor data. A Secret create/delete event maps only to parents in the manager's watched
namespace via `spec.secretRefs`; restoring the same name re-evaluates the condition
without changing the parent. Forbidden, timeout and other API errors remain their
own retry categories and are never mislabeled as `SecretNotFound`.

Secret deletion does not revoke values already loaded into a process environment.
Direct `secretRefs` data updates do not restart existing Pods. AWCP-25 adds a
separate, opt-in ESO path: `spec.externalSecrets` references a Ready namespaced
`SecretStore`/`ExternalSecret` pair and target Secret, then uses only the target
metadata resourceVersion to roll the owned Deployment. AWCP does not create ESO
objects, read provider credentials or Secret values, and rejects ClusterSecretStore.
Endpoint discovery is independent of Secret readiness; it is not an application
health claim.

## Operator checks

Use only authorized metadata/status inspection:

```bash
kubectl -n <namespace> get serviceaccount <child-name>
kubectl -n <namespace> get aiworkload <name> -o jsonpath='{.status.conditions}'
kubectl auth can-i get secrets --as=system:serviceaccount:<namespace>:<child-name> -n <namespace>
```

The last command should return `no` for a generated workload identity. It does not
assess what an image creator or a namespace administrator may otherwise do.

References: [Kubernetes ServiceAccounts](https://kubernetes.io/docs/concepts/security/service-accounts/),
[Secrets](https://kubernetes.io/docs/concepts/configuration/secret/), and
[RBAC good practices](https://kubernetes.io/docs/concepts/security/rbac-good-practices/).
