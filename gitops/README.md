# AWCP GitOps layout

This directory is the declarative delivery boundary for AWCP. It intentionally
does not duplicate the operator package under `config/default`: the platform
Application points to that canonical Kustomize source so Git has one definition
of the CRD, controller RBAC, manager and metrics Service.

```text
gitops/
├── argocd/
│   ├── applications/       # Application CRs, applied once after the Project
│   ├── bootstrap.sh        # explicit cluster-admin bootstrap; never CI
│   ├── installation.lock.yaml
│   └── values.yaml         # optional Helm installer values, without credentials
├── platform/
│   └── base/               # AppProject governance boundary
└── workloads/
    ├── base/               # common parent-only AIWorkload definition
    └── environments/       # dev, staging and prod Kustomize overlays
```

Each `gitops/workloads/environments/<environment>` overlay deliberately contains
only the parent `AIWorkload`. The ApplicationSet directory generator creates one
Argo Application per directory. The
operator owns its generated Deployment, Service, ServiceAccount and NetworkPolicy
through Kubernetes owner references; Argo CD must not add those child objects to
Git or a separate Application.

Read [the GitOps guide](../docs/gitops.md) before using this directory. The
bootstrap process changes a cluster and is intentionally not a CI target.
