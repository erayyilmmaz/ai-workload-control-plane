# Autoscaling and HPA lifecycle — AWCP-27

`spec.autoscaling.enabled: true` creates one current-UID-owned
`autoscaling/v2` HorizontalPodAutoscaler for AWCP's Deployment. CPU and memory
metrics use utilization targets; Pods and External metrics use a named metric and
quantity target. The HPA has no adapter URL, credentials, cross-namespace target,
or arbitrary scale target: it can scale only the workload's owned Deployment.

When autoscaling is off or omitted, AWCP preserves the V0 `spec.replicas`
contract. When it is on, `spec.replicas` only initializes a new Deployment and
AWCP preserves its live replica value thereafter. HPA is therefore the sole
writer of the Deployment scale subresource. Removing autoscaling performs an
owner-safe HPA delete, then static replicas resume on later reconciliation.

The current pinned Kubernetes 1.36 baseline rejects `minReplicas: 0`. Native HPA
scale-to-zero becomes enabled by default in Kubernetes 1.37 and additionally
requires an Object or External metric; it is intentionally deferred rather than
claimed through CPU or memory metrics. HTTP requests also need a buffering layer
when no Pods are ready. `scaleDownStabilizationSeconds` is available to bound
downscale churn.

`AutoscalingReady=True/HPAActive` means the HPA reports `ScalingActive=True`.
`MetricUnavailable` mirrors an HPA `ScalingActive=False` observation; AWCP does
not install Metrics Server or a custom/external metric adapter, and it never
reads metric payloads. Missing metric infrastructure must not crash the manager.

References: [HPA v2 API](https://kubernetes.io/docs/reference/kubernetes-api/autoscaling/horizontal-pod-autoscaler-v2/),
[HPA concepts](https://kubernetes.io/docs/concepts/workloads/autoscaling/horizontal-pod-autoscale/),
and [Kubernetes 1.37 scale-to-zero](https://kubernetes.io/blog/2026/09/02/kubernetes-v1-37-hpa-scale-to-zero-beta/).
