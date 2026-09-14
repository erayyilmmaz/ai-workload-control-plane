# ADR-010 — Optional platform capabilities are feature-scoped

Status: Accepted. Date: 2026-09-14. Tracking: AWCP-19.

## Context

V1 integrates optional platform components such as Gateway API, External Secrets
Operator, Argo Rollouts, Prometheus-compatible monitoring and external metrics.
Treating any of them as a manager startup prerequisite would make V0 workloads
unavailable in otherwise valid clusters.

## Decision

Maintain a small, testable API capability catalog. A feature reconciler performs
capability discovery only when its corresponding API field requests the feature.
Missing APIs are an expected `Unavailable` result; discovery transport or
authorization errors are `Unknown` and follow bounded retry. Neither result is
checked during manager startup.

The existing `Ready`, `Progressing` and `Degraded` condition set remains the
baseline. A feature maps a missing dependency to a precise degraded reason such as
`DependencyUnavailable`, without exposing raw provider errors or credentials.

## Consequences and boundaries

The running manager remains scoped to its configured namespace and does not gain
cluster-wide ownership of optional platforms. Argo CD is a GitOps delivery system,
not an AWCP runtime child; OpenCost/OTel endpoint health is not guessed from CRD
presence. Each integration story owns its exact RBAC, discovery adapter, status
mapping and runtime evidence.

## Alternatives considered

Failing manager startup on a missing optional CRD would turn a feature opt-in into
a platform outage. Treating discovery errors as absent APIs hides authorization or
network failures. Installing/configuring dependencies from AWCP would violate the
platform-admin ownership boundary.

## Validation and revisit trigger

AWCP-19 unit-tests catalog copying and Available/Unavailable/Unknown separation.
Feature stories must add real discovery and missing-dependency integration tests.
Revisit before adding a dependency with unavoidable manager-wide startup impact.
