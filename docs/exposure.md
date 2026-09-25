# HTTPRoute exposure lifecycle — AWCP-26

AWCP keeps the V0 Service contract unchanged: the generated Service is always
`ClusterIP`. External reachability is opt-in and is represented by one owned
Gateway API `HTTPRoute`; AWCP does not create a `Gateway`, `GatewayClass`,
load balancer, DNS record or TLS material.

## API contract

```yaml
spec:
  network:
    enabled: false
  exposure:
    mode: HTTPRoute
    gateway: platform-gateway
    sectionName: http
    hostname: api.example.test
    path: /v1
```

`ClusterLocal` is explicit Service-only intent and carries no Gateway fields.
`HTTPRoute` requires all four routing fields. Gateway and route are restricted
to the `AIWorkload` namespace; there is no namespace field, cross-namespace
reference, listener protocol, TLS, redirect/rewrite/filter, weight, timeout or
arbitrary backend escape hatch. The generated route has one exact hostname,
one `PathPrefix`, one named Gateway listener and one backend: AWCP's current
UID-owned ClusterIP Service at its configured Service port.

## Security and lifecycle boundary

The generated V0 NetworkPolicy admits only same-namespace Pod ingress. A
Gateway implementation may run its data plane in another namespace, so an
`HTTPRoute` requires explicit `spec.network.enabled: false`. The platform must
then enforce the Gateway-to-workload ingress policy outside AWCP. This prevents
a workload from silently broadening the default NetworkPolicy with an arbitrary
namespace selector.

When the feature is requested, AWCP discovers Gateway API v1 and reads only the
same-namespace Gateway. It requires `Programmed=True`, then creates/repairs the
route with its ordinary owner-reference and no Gateway write permission. Missing
API/Gateway, a disabled Service or enabled AWCP NetworkPolicy is a sanitized,
bounded configuration failure. A route is deleted only when it is still owned
by the current parent UID; removing `exposure` does not delete the platform
Gateway or its infrastructure.

`ExposureReady=True` means a matching Gateway status entry reports both
`Accepted=True` and `ResolvedRefs=True`. It is distinct from `Ready`, which
continues to describe the Deployment rollout. AWCP performs a bounded one-minute
recheck for opted-in routes rather than registering a startup-time HTTPRoute
watch: clusters without Gateway API must still start and reconcile V0 workloads.

## Runtime proof

The disposable kind E2E verifies the SHA-256-pinned Envoy Gateway v1.9.1
manifest, waits for the platform Gateway to be Programmed, creates an AWCP route,
waits for `Accepted` and `ResolvedRefs`, sends HTTP traffic through the Envoy
Gateway Service with the configured Host header, and confirms that deleting the
AIWorkload garbage-collects only its HTTPRoute while preserving the Gateway.
It is a local/reference proof, not production DNS, TLS, load-balancer or
platform-network-policy evidence.

References: [Gateway API HTTPRoute](https://gateway-api.sigs.k8s.io/reference/api-types/httproute/),
[Gateway API versioning](https://gateway-api.sigs.k8s.io/docs/concepts/versioning/),
and [Envoy Gateway Kubernetes-YAML installation](https://gateway.envoyproxy.io/latest/install/install-yaml/).
