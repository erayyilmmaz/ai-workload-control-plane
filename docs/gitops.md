# GitOps and Argo CD — AWCP-20 / AWCP-21

AWCP uses Git as the desired-state source for the operator package and the parent
`AIWorkload` objects. This is a delivery boundary, not a second reconciliation
engine: Kubernetes remains the source of truth for observed state and the AWCP
controller remains the sole writer of its owned children.

## Ownership model

| Owner | Resources / responsibility | Explicitly not owned |
| --- | --- | --- |
| Git + Argo CD platform Application | `config/default`: namespaces, tenant profiles/quotas/limits, admission policy/binding, CRD, AWCP manager, bounded RBAC and metrics Service; optional namespace-local ESO SecretStore/ExternalSecret manifests after ESO bootstrap | Argo CD/ESO installation, provider credentials and destructive platform prune |
| Git + Argo CD ApplicationSet | Parent `AIWorkload` objects rendered from `gitops/workloads/environments/*` | Generated Deployment, Service, ServiceAccount and NetworkPolicy |
| AWCP controller | Current-parent UID-owned workload children and status/events | Argo CD `Application`/`AppProject`, user Secrets, namespaces and shared infrastructure |
| Cluster administrator | Argo CD bootstrap, repository policy, credential/SSO configuration, production image selection and CRD/namespace removal | Application content changes made by a workload developer |

The controller-generated children are intentionally absent from Git. A child has a
Kubernetes owner reference to its `AIWorkload`; it is lifecycle-reconciled by AWCP,
not declared in an Argo source. This prevents GitOps and operator reconciliation
from overwriting one another.

`AppProject/awcp-platform` restricts the source to this public repository and the
explicit platform, shared-workload, and three reference tenant namespaces. Its
cluster-scoped whitelist is limited to Namespace, CRD, metrics-auth RBAC, and the
single reviewed ValidatingAdmissionPolicy/Binding kinds required for AWCP-24 and
the namespace-scoped ESO SecretStore/ExternalSecret kinds consumed by AWCP-25;
tenant resources remain namespace-scoped. It is a starting boundary, not a
replacement for cluster admission/RBAC policy. AWCP-23 keeps the manager's runtime
scope bounded to the exact same namespace list rather than granting it a
ClusterRole.

## Repository model

The checked-in [GitOps layout](../gitops/README.md) has three independent pieces:

- `gitops/platform/base`: the `AppProject` governance boundary. The actual
  platform source is `config/default`, kept canonical to avoid a copied operator
  package drifting from the supported Kustomize install path.
- `gitops/argocd/applications`: a platform `Application` and the workload
  `ApplicationSet`. Bootstrap applies the Project first, then the platform
  Application, waits for it, and only then applies the ApplicationSet.
- `gitops/workloads/base`: one common parent-only definition;
  `gitops/workloads/environments/{dev,staging,prod}` provide isolated Kustomize
  overlays. The Git directory generator creates `awcp-dev`, `awcp-staging` and
  `awcp-prod` Applications from these paths.

The Applications track `main` only for the public local demo. Dev/staging sync
automatically; prod has automated sync disabled, so an explicit operator sync is
required after review. Production must use a protected promotion branch, signed
tag or immutable commit SHA and an immutable image digest; it must not reuse the
local `awcp-demo:*` images.

`spec.environment` is a logical, optional identity which creates the bounded
`platform.example.io/environment` label on AWCP-owned children. It never chooses
a namespace or destination cluster. The local/reference profile intentionally
uses the pre-existing single watched namespace (`awcp-workloads`) and unique
parent names per overlay. The future isolated namespace convention is
`awcp-<environment>-workloads`; adopting it requires explicit manager scope/RBAC
work and is not implied by an environment label.

## Bootstrap and local demo

Argo CD bootstraps itself before it can reconcile Git. The committed installer lock
pins the official v3.5.2 install manifest SHA-256 and matching `argo-cd` Helm chart
10.9.0. The Helm values are an optional single-node local/reference profile: they
contain no repository credentials or fixed admin secret, use a ClusterIP Service,
and set `server.insecure` only for local port-forward use. They are not a
production SSO/TLS/HA/security baseline.

Select the intended cluster first, then run the explicit control-plane bootstrap:

```bash
make gitops-bootstrap
```

This command uses the pinned Helm chart and changes the selected cluster; it is
never run by CI. To use the official pinned manifest instead, first verify the
SHA-256 in `gitops/argocd/installation.lock.yaml`, then create the `argocd`
namespace and apply the downloaded file with
`kubectl apply --server-side --force-conflicts -n argocd -f <verified-file>` to
the deliberately selected context. Server-side apply avoids the Kubernetes
client-side annotation limit on Argo CD's large ApplicationSet CRD.

After Argo CD is healthy, bootstrap the AWCP Project and platform Application.
Do not use `kubectl apply` for `AIWorkload` manifests after this one-time setup:

```bash
.tools/bin/kubectl apply -k gitops/platform/base
.tools/bin/kubectl -n argocd apply -f gitops/argocd/applications/awcp-platform.yaml
.tools/bin/kubectl -n argocd wait --for=jsonpath='{.status.sync.status}'=Synced application/awcp-platform --timeout=5m
.tools/bin/kubectl -n argocd apply -f gitops/argocd/applications/awcp-environments.yaml
.tools/bin/kubectl -n argocd wait --for=jsonpath='{.status.sync.status}'=Synced application/awcp-dev --timeout=5m
.tools/bin/kubectl -n argocd wait --for=jsonpath='{.status.sync.status}'=Synced application/awcp-staging --timeout=5m
# Production intentionally needs an approved, explicit Argo CD sync.
```

For a disposable real-cluster check after the Git commit containing the manifests
is pushed, run `make gitops-e2e`. It creates a uniquely named kind cluster,
installs the checksum-verified Argo CD manifest, loads local demo images, proves
Git-sourced platform/workload reconciliation and self-healing, then deletes only
that cluster. It requires Docker, network access to the public repository and no
Argo CD API token. It does not test a remote Git deletion/prune commit; that
controlled operator runbook is below.

## Sync, prune, self-heal and rollback

The two Applications deliberately have different deletion policy:

| Application | Automated sync | Self-heal | Prune | Reason |
| --- | --- | --- | --- | --- |
| `awcp-platform` | enabled | enabled | disabled | A Git mistake must not automatically remove a CRD, RBAC or namespace. Platform removal is an administrator decision. |
| `awcp-dev`, `awcp-staging` | enabled | enabled | enabled, `allowEmpty: false` | Each Application owns exactly its Git directory's parent. A wrong overlay cannot alter another environment's source path or parent name. |
| `awcp-prod` | disabled; explicit approved sync | disabled | enabled, `allowEmpty: false` | Production Git changes are review/promotion actions, not automatic delivery from this local reference branch. |

Both Applications retry a failed sync at most five times using a bounded
exponential backoff (5 seconds, factor 2, maximum 3 minutes). The pinned Argo CD
3.5.2 API does not expose a retry-refresh field, so a new Git revision is the
explicit next reconciliation input. Manual mutation of a parent `AIWorkload` is
reverted by self-heal; manual mutation/deletion of an owned child is AWCP's drift
reconciliation responsibility.

For a deliberate workload deletion, delete only its parent YAML from one
environment overlay, review the pull request, merge/push it, and wait for that
environment Application to synchronize. Confirm that the named `AIWorkload` and its
UID-owned children disappear while user Secrets remain. Never enable
`allowEmpty: true` for this application without a separate approved bulk-removal
procedure.

Automated sync prevents an Argo CD UI/API rollback from remaining in effect: a
rollback must be a Git revert (or a promotion to a previously approved immutable
revision), followed by normal sync. Platform rollback needs an administrator
review because platform prune is disabled and CRD downgrade is not automatic.

## Environment promotion boundary

The executable local demo proves directory discovery, independent overlay values,
dev self-heal and an explicit prod synchronization operation. It deliberately does
not make a Git commit that promotes an image between environments, configure Git
branch rules, verify image signatures, or demonstrate remote deletion/prune. Those
are separate Git provider, registry and production-cluster evidence states.

Promotion is a Git change: update the reviewed environment overlay with an
immutable image digest and/or approved commit revision, merge into the protected
promotion source, then explicitly sync prod and verify both Argo Application state
and the AWCP `Ready` condition. Roll back through a Git revert or previously
approved immutable revision; never edit a generated child directly.

## Observability and failure boundaries

Argo CD Application sync/health status is the deployment signal; AWCP workload
conditions remain the runtime lifecycle signal. They must be read together:
`Synced` does not mean the image is Ready, and `Ready=True` does not prove that
Git is currently the desired source. Future AWCP-31 observability work may ingest
Argo signals, but AWCP-20 neither adds Argo credentials nor changes AWCP status.

A private repository, Git webhook, Argo API token, SSO provider, TLS certificate,
production RBAC policy, HA topology and image-signature enforcement are outside
this local/reference story. CI statically renders the GitOps manifests and has no
Argo CD token, cluster credential or apply step.

## References

- [Argo CD ApplicationSet and directory generators](https://argo-cd.readthedocs.io/en/stable/operator-manual/applicationset/Generators-Git/)
- [Argo CD automated sync, prune, self-heal and retry](https://argo-cd.readthedocs.io/en/stable/user-guide/auto_sync/)
- [Argo CD Projects](https://argo-cd.readthedocs.io/en/stable/user-guide/projects/)
- [Argo CD installation](https://argo-cd.readthedocs.io/en/stable/operator-manual/installation/)
- [Argo CD Helm chart 10.9.0](https://artifacthub.io/packages/helm/argo/argo-cd/10.9.0)
