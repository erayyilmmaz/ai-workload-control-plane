# V0 scope

Decision: AWCP-2, accepted 2026-09-12. Implementation is tracked separately.

## Problem and user

A developer with namespace access should describe one application rather than manually keeping four Kubernetes resources consistent. The operator owns the derived resource configuration and exposes a useful failure state when it cannot satisfy that description.

`AIWorkload` is a namespaced desired-state definition for one stateless, long-running HTTP application. It is not a batch job, an inference engine, or a resource scheduler. Its image may call external AI APIs, but the demo makes no external AI calls and needs no paid account.

## Supported use cases

| Developer action | Intended result | Evidence owner |
| --- | --- | --- |
| Create a valid CR | Dedicated identity, Deployment and enabled networking resources are reconciled | AWCP-4 through AWCP-10 |
| Change image or pod configuration | Deployment template changes and a rollout starts | AWCP-6 |
| Change replicas, including zero | Deployment scales; zero has an explicit non-degraded status | AWCP-6, AWCP-10 |
| Delete or edit an owned resource | Controller repairs managed configuration | AWCP-5 through AWCP-9 |
| Reference a missing Secret | Actionable `SecretNotFound`; recovery after Secret creation without editing the CR | AWCP-8, AWCP-10 |
| Enable/disable Service or NetworkPolicy | Only that CR's currently owned optional child is created/deleted | AWCP-7, AWCP-9 |
| Delete the CR | Kubernetes garbage collection removes the owned subtree; user Secrets survive | AWCP-12, AWCP-14 |
| Inspect or demonstrate the system | Conditions, Events, controller metrics and repeatable local examples | AWCP-10, AWCP-11, AWCP-17 |

## Included deliverables

- Go, Kubebuilder and controller-runtime; `platform.example.io/v1alpha1`, kind `AIWorkload`.
- Reconciliation of Deployment, ClusterIP Service, ServiceAccount and standard `networking.k8s.io/v1` NetworkPolicy.
- Public/non-root compatible workload images, CPU/memory requests and limits, HTTP readiness/liveness probes and Secret `envFrom` references.
- Observed generation, replica counts, service endpoint, `Ready`/`Progressing`/`Degraded` conditions and Kubernetes Events.
- Structured logs, controller metrics, Prometheus scrape configuration, Grafana dashboard and an optional-to-run OpenTelemetry Collector example.
- Unit tests, envtest API tests, real kind E2E tests and local Docker images for the manager and demo app.
- GitHub Actions quality gates, exact tool pins, Kustomize install/release bundle, README, ADRs and a real demo.

## Explicit optional decisions

| Topic | V0 decision | Closure expectation |
| --- | --- | --- |
| Helm | Deferred; Kustomize is the canonical package | AWCP-15 documents this choice; no chart is required |
| Trace spans | Deferred; controller metrics and structured logs are sufficient | AWCP-11 still supplies the Collector example |
| NetworkPolicy enforcement profile | Optional CNI test profile | Without it, claim policy generation only |
| Manager replicas | One manager process in V0; leader election enabled | No HA or multi-replica failover claim |
| Secret rotation rollout | Deferred | Updating Secret data does not promise to refresh an existing process environment |
| Startup probe | Deferred | Slow-start applications must work within the documented HTTP probe contract |
| Image registry authentication | Deferred | No V0 `imagePullSecrets` field; examples use public or locally loaded images |

## Non-goals

Frontend; REST management API; hosted SaaS; organizations/users; billing; GPU scheduling; inference server; LLM gateway; model routing; agent execution runtime; vector database; PostgreSQL; Redis; RabbitMQ/Kafka; multi-cluster; cloud-specific identity; Terraform; Argo CD; HPA; admission webhook; full multi-tenancy.

There is also no arbitrary PodSpec escape hatch, privileged workload mode, automatic Secret revocation, persistent volume orchestration or external-resource cleanup in V0. These would change the trust or lifecycle boundary and need a separate decision.

## What Ready promises

The current spec was evaluated, required referenced objects are present, owned resources reflect desired managed fields and the latest Deployment rollout has the requested available/ready replicas. It does not certify an external AI provider, model quality, credential validity, public connectivity, or enterprise isolation.

Kubernetes API is the source of truth. Workqueues, informer caches and telemetry are disposable observations. No database or broker is introduced for controller state.

## Milestone completion

A story closes against its stated evidence layer. Design acceptance is distinct from a successful build; local tests are distinct from hosted CI; workflow checks are distinct from an enabled merge ruleset; release preparation is distinct from a published image/tag. [Traceability](traceability.md) assigns the eventual runtime evidence to later stories.
