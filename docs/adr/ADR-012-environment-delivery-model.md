# ADR-012 — Environment delivery model

Status: Accepted. Date: 2026-09-16. Tracking: AWCP-21.

## Context

AWCP-20 demonstrated one Git-sourced parent `AIWorkload`. V1 needs controlled
dev, staging and prod variations without committing controller-generated children
or creating a separate delivery pipeline for every environment.

## Decision

Use a Kustomize common parent definition and one directory per environment under
`gitops/workloads/environments`. An Argo CD `ApplicationSet` Git directory
generator produces one Application per environment directory. Overlay changes are
limited to parent metadata, the optional `spec.environment` identity and explicit
workload values such as replicas.

The public local/reference profile targets all generated Applications to the
existing `awcp-workloads` namespace because the current AWCP manager deliberately
watches exactly one namespace. Unique parent names (`gitops-demo-dev`,
`gitops-demo-staging`, `gitops-demo-prod`) keep the managed child subtrees
independent. This is delivery separation, not namespace or tenant isolation.

Dev and staging use automated sync/self-heal. The prod generated Application has
automated sync disabled and needs an explicit sync after a reviewed Git change.
Production promotion must additionally use a protected promotion branch, signed
tag or immutable commit/image digest; this reference repository does not claim to
configure those external Git or registry controls.

## Consequences and boundaries

Adding an environment directory produces an Application without a new pipeline.
An environment overlay cannot change another overlay's path, parent name or
replica value. Git remains the desired state for each parent; AWCP alone owns the
children and propagates a bounded environment label when the optional identity is
present.

The documented future namespace convention is `awcp-<environment>-workloads`.
Using that convention requires an explicit manager watch/RBAC/tenant-profile
implementation; an `AIWorkload.spec.environment` value never selects it. No
multi-cluster, tenant authorization, namespace provisioning, branch-protection,
secret credential, webhook, image-signature or production promotion evidence is
created by this decision.

## Validation and revisit trigger

Static validation renders each overlay, checks ApplicationSet parent-only sources
and rejects committed credentials. The disposable kind test verifies that Argo CD
generates dev/staging/prod Applications from the pushed Git revision, requires an
explicit prod sync, preserves overlay values, self-heals dev drift and leaves all
generated children solely AWCP-owned. Revisit when adding multi-namespace manager
watching, tenant governance, a private repository or a second destination cluster.
