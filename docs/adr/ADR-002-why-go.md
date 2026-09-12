# ADR-002 — Why Go

Status: Accepted design. Date: 2026-09-12. Tracking: AWCP-2.

## Context

The project requires typed Kubernetes APIs, informer/watch integration, explicit error handling and a small deployable manager process.

## Decision

Use Go with Kubebuilder go/v4 and controller-runtime. Module path is github.com/erayyilmmaz/ai-workload-control-plane. Use the exact versions and checksums in ../../toolchain.lock.json; generate Go module files during AWCP-3.

## Consequences and boundaries

Keep api/v1alpha1 for the public type contract, internal/resource for pure desired-object builders, internal/controller for API interaction and lifecycle, internal/telemetry for observability, and cmd for manager wiring. Builders cannot perform network reads. Controller errors remain observable and testable.

## Alternatives considered

Python/TypeScript operator frameworks are viable, but would depart from the selected Go/Kubernetes implementation and tooling objective. A home-built watch queue would duplicate controller-runtime.

## Validation and revisit trigger

Tagged upstream go.mod/Makefile comparison closes the design gate. Actual generation/build/vet/lint and resolved module-graph evidence belong to AWCP-3. Do not copy the upstream sample's cert-manager/webhook dependencies.

## References

- [Upstream reference](https://github.com/kubernetes-sigs/kubebuilder/blob/v4.15.0/testdata/project-v4/go.mod)
- [Architecture](../architecture.md)
- [Compatibility](../compatibility.md)
