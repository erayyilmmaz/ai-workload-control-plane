# Installation, upgrade and removal — AWCP-15

Kustomize is the canonical V0 package. Helm is deliberately deferred: this
single-manager operator has no chart-specific value yet, and a Helm chart would
need the same rendered/install/upgrade/uninstall evidence before it could be
offered. No Helm command or chart is part of this release.

## Before installing

You need cluster-admin-equivalent permission to install the CRD and the bounded
controller RBAC, plus a `kubectl` context deliberately selected by you. Go is not
required to consume a release bundle. The controller image must already be
available to every target node; use an immutable digest from a registry you operate
or trust. This repository currently publishes **no** production controller image,
so its local `awcp-manager:*` tags are suitable only when you have built and loaded
them into a local cluster.

The manager watches only `awcp-workloads` and runs in `awcp-system`. The default
bundle creates both namespaces and installs the `AIWorkload` CRD, controller
ServiceAccount/RBAC, manager Deployment and authenticated metrics Service.

## Install and deploy from this checkout

`make install` applies only the CRD. `make deploy` applies the full Kustomize
bundle and requires an explicit accessible image, so a missing public release can
never look like a working quick start.

```bash
make install
make deploy DEPLOY_IMG=registry.example.com/awcp/manager@sha256:<immutable-digest>
kubectl -n awcp-system rollout status deployment/awcp-controller-manager --timeout=180s
```

`make deploy` renders a temporary copy of the package with the supplied image; it
does not modify tracked manifests. It waits for the CRD and manager rollout. For a
new image digest, rerun the same command. The manager is intentionally a singleton
with leader election; rolling updates remove the old leader before requiring the
replacement to become Ready.

Once the controller is Ready, apply an `AIWorkload` whose image and referenced
Secrets are reachable in `awcp-workloads`. The ready-time objective starts only
after cluster access, the controller image and the workload image are available;
first Docker/image downloads are excluded. The local `make e2e` path is the
executed demo evidence; it builds and loads its non-production v1/v2 demo images.

## Release bundles and verification

Maintainers create portable YAML artifacts without a cluster:

```bash
make release-bundle
make verify-package
```

This produces ignored `dist/release/` files:

- `awcp-crds.yaml`: CRD only, installed first;
- `awcp-operator.yaml`: full Kustomize install bundle;
- `awcp-operator-uninstall.yaml`: manager/RBAC/metrics only;
- `SHA256SUMS`: checksums for those three YAML files.

A future GitHub Release must attach these exact files and their checksums, record
the controller image digest, and state the supported Kubernetes baseline. Creating
the bundle does not publish an image or a release. Image tags are human-readable
release aliases (`vX.Y.Z`); deployment should use the corresponding immutable
`repo@sha256:...` reference. Development images retain explicit local tags such as
`awcp-manager:awcp-15` and are never release evidence.

## Safe removal and explicit destructive actions

Normal operator removal is safe for workload data:

```bash
make undeploy
```

It removes only the manager Deployment, its ServiceAccount/RBAC and the metrics
Service. It intentionally does **not** delete the `AIWorkload` CRD, either
namespace, any `AIWorkload`, owned workload children or user Secrets. Existing
AIWorkloads stop being reconciled until the operator is deployed again.

Deleting a CRD is a separate, destructive administrator action. Kubernetes removes
all custom objects stored for that CRD; do this only after intentionally deleting
or preserving each workload and understanding its owned-child consequences.

```bash
# Destructive: removes all AIWorkload objects from the cluster.
kubectl delete crd aiworkloads.platform.example.io
```

Namespaces are likewise never removed by the package. If an administrator later
chooses to delete `awcp-workloads`, Kubernetes will delete all namespaced objects
inside it. There is intentionally no shortcut Make target for either destructive
operation.
