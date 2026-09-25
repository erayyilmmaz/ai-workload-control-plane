/*
Copyright 2026 AI Workload Control Plane contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// AIWorkloadSpec describes one stateless HTTP application, not an arbitrary PodSpec.
type AIWorkloadSpec struct {
	// Environment is an optional logical delivery identity such as dev, staging or
	// prod. It labels AWCP-owned children but never selects a namespace, cluster or
	// permission boundary.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern="^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"
	// +optional
	Environment string `json:"environment,omitempty"`

	// Tenant is an optional logical tenant identity. When present, AWCP verifies it
	// against the GitOps-managed tenant profile in this workload's namespace. It
	// never selects another namespace or grants Kubernetes permissions.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern="^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"
	// +optional
	Tenant string `json:"tenant,omitempty"`

	// Policy selects a platform-defined admission profile. It is declarative
	// intent, never a CEL expression or a way to bypass namespace policy. The
	// restricted profile is enforced only in namespaces selected by the GitOps
	// ValidatingAdmissionPolicyBinding; omission preserves V0 behavior elsewhere.
	// +kubebuilder:validation:Enum=baseline;restricted
	// +optional
	Policy string `json:"policy,omitempty"`

	// Image is the container image reference. Whitespace is forbidden; registry
	// availability and full OCI reference validity are checked at workload runtime.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:Pattern="^[^[:space:]\\p{Z}\\x{0085}]+$"
	// +required
	Image string `json:"image"`

	// Replicas is the desired count; zero intentionally scales the application down.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=20
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Autoscaling optionally delegates Deployment replica changes to one owned
	// autoscaling/v2 HorizontalPodAutoscaler. When enabled, replicas is an initial
	// bootstrap value only and AWCP never fights the HPA scale subresource.
	// +optional
	Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`

	// Availability optionally creates one policy/v1 PodDisruptionBudget for this
	// workload. It limits voluntary disruptions only; it neither guarantees
	// capacity nor protects against involuntary failures such as node loss.
	// +optional
	Availability *AvailabilitySpec `json:"availability,omitempty"`

	// Container declares the required HTTP TCP port, named http in generated Pods.
	// +required
	Container ContainerSpec `json:"container"`

	// Resources supports only CPU and memory requests/limits, as quoted quantities.
	// +optional
	Resources *ResourceRequirements `json:"resources,omitempty"`

	// Health enables only explicitly present readiness/liveness HTTP probe blocks.
	// +optional
	Health *HealthSpec `json:"health,omitempty"`

	// Service configures an optional ClusterIP Service targeting the named http port.
	// +kubebuilder:default={}
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`

	// Exposure optionally selects a cluster-local endpoint or one namespace-local
	// Gateway API HTTPRoute. It never creates a Gateway, changes the Service type,
	// or accepts cross-namespace routing authority.
	// +optional
	Exposure *ExposureSpec `json:"exposure,omitempty"`

	// SecretRefs contains ordered, unique Secret names in this CR's namespace.
	// Later envFrom entries win on overlapping keys. Values are never part of this API.
	// +kubebuilder:default={}
	// +kubebuilder:validation:MaxItems=32
	// +kubebuilder:validation:XValidation:rule="self.all(x, self.filter(y, y == x).size() == 1)",message="secretRefs must not contain duplicate names"
	// +listType=atomic
	// +optional
	SecretRefs []SecretReference `json:"secretRefs,omitempty"`

	// ExternalSecrets names namespace-local External Secrets Operator resources and
	// their expected target Secrets. AWCP never receives provider credentials,
	// remote values or a cross-namespace reference. The referenced ExternalSecret
	// must use a namespaced SecretStore and report Ready before AWCP starts Pods.
	// A target Secret metadata revision triggers an AWCP rollout without reading
	// Secret data.
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:XValidation:rule="self.all(x, self.filter(y, y.externalSecret == x.externalSecret).size() == 1)",message="externalSecrets must not contain duplicate externalSecret names"
	// +kubebuilder:validation:XValidation:rule="self.all(x, self.filter(y, y.targetSecret == x.targetSecret).size() == 1)",message="externalSecrets must not contain duplicate targetSecret names"
	// +listType=atomic
	// +optional
	ExternalSecrets []ExternalSecretReference `json:"externalSecrets,omitempty"`

	// Network enables same-namespace ingress policy generation; it does not isolate egress.
	// +kubebuilder:default={}
	// +optional
	Network *NetworkSpec `json:"network,omitempty"`
}

// ContainerSpec is the application HTTP listener configuration.
type ContainerSpec struct {
	// Port is required and becomes the TCP container port named http.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +required
	Port int32 `json:"port"`
}

// ResourceQuantity is a quoted Kubernetes quantity, e.g. 100m or 128Mi.
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=64
// +kubebuilder:validation:XValidation:rule="isQuantity(self) && !quantity(self).isLessThan(quantity('0'))",message="quantity must be parseable and non-negative"
type ResourceQuantity string

// ResourceQuantities restricts resource names to CPU and memory; extended resources are not V0.
// +kubebuilder:validation:MaxProperties=2
// +kubebuilder:validation:XValidation:rule="self.all(k, k in ['cpu', 'memory'])",message="only cpu and memory resources are supported"
type ResourceQuantities map[string]ResourceQuantity

// ResourceRequirements expresses optional requests and limits without Pod-level claims.
// +kubebuilder:validation:XValidation:rule="!has(self.requests) || !has(self.limits) || self.requests.all(k, !(k in self.limits) || !isQuantity(self.requests[k]) || !isQuantity(self.limits[k]) || quantity(self.requests[k]).compareTo(quantity(self.limits[k])) <= 0)",message="each request must not exceed its corresponding limit"
type ResourceRequirements struct {
	// Requests reserves CPU/memory. Omitted keys remain absent at this API layer.
	// +optional
	Requests ResourceQuantities `json:"requests,omitempty"`
	// Limits caps CPU/memory. A matching request cannot exceed the limit.
	// +optional
	Limits ResourceQuantities `json:"limits,omitempty"`
}

// HealthSpec enables independent HTTP probes with fixed V0 timing defaults.
type HealthSpec struct {
	// Readiness gates traffic; omitted means no readiness probe.
	// +optional
	Readiness *HTTPProbeSpec `json:"readiness,omitempty"`
	// Liveness detects an unhealthy process; omitted means no liveness probe.
	// +optional
	Liveness *HTTPProbeSpec `json:"liveness,omitempty"`
}

// HTTPProbeSpec selects a path on the named http container port.
type HTTPProbeSpec struct {
	// Path must start with /. V0 timing is fixed and documented in docs/api-contract.md.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:Pattern="^/"
	// +required
	Path string `json:"path"`
}

// ServiceSpec describes ClusterIP exposure within the cluster, not external ingress.
type ServiceSpec struct {
	// Enabled defaults to true; explicit false suppresses the Service.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// Port is the Service TCP port; it targets container port http.
	// +kubebuilder:default=80
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port *int32 `json:"port,omitempty"`
}

// AutoscalingMetricType bounds supported HPA metric sources to resource, Pods,
// or external metrics. Arbitrary adapter URLs and cross-namespace references are
// intentionally outside the workload API.
// +kubebuilder:validation:Enum=CPU;Memory;Pods;External
type AutoscalingMetricType string

const (
	AutoscalingCPU      AutoscalingMetricType = "CPU"
	AutoscalingMemory   AutoscalingMetricType = "Memory"
	AutoscalingPods     AutoscalingMetricType = "Pods"
	AutoscalingExternal AutoscalingMetricType = "External"
)

// AutoscalingSpec is a deliberately bounded autoscaling/v2 HPA contract. The
// current Kubernetes 1.36 runtime baseline does not claim native scale-to-zero,
// so minReplicas is intentionally at least one in this V1 story.
// +kubebuilder:validation:XValidation:rule="!has(self.enabled) || !self.enabled || (has(self.minReplicas) && has(self.maxReplicas) && self.metrics.size() > 0)",message="enabled autoscaling requires minReplicas, maxReplicas and at least one metric"
// +kubebuilder:validation:XValidation:rule="!has(self.minReplicas) || !has(self.maxReplicas) || self.minReplicas <= self.maxReplicas",message="minReplicas must not exceed maxReplicas"
type AutoscalingSpec struct {
	// Enabled defaults to false; omission preserves V0 static replica behavior.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// MinReplicas is the HPA lower bound. Zero is deferred until a Kubernetes 1.37
	// feature-gate and object/external-metric capability story is proven.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=20
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`
	// MaxReplicas is the HPA upper bound.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=20
	// +optional
	MaxReplicas *int32 `json:"maxReplicas,omitempty"`
	// Metrics is an atomic, bounded list because HPA evaluates all configured
	// metrics and uses the largest recommendation.
	// +kubebuilder:validation:MaxItems=4
	// +listType=atomic
	// +optional
	Metrics []AutoscalingMetricSpec `json:"metrics,omitempty"`
	// ScaleDownStabilizationSeconds optionally sets the HPA scale-down window.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=3600
	// +optional
	ScaleDownStabilizationSeconds *int32 `json:"scaleDownStabilizationSeconds,omitempty"`
}

// AutoscalingMetricSpec represents exactly one safe HPA metric declaration.
// CPU and Memory use targetUtilization; Pods and External use metricName and a
// non-negative targetValue. The adapter itself remains platform-managed.
// +kubebuilder:validation:XValidation:rule="(self.type == 'CPU' || self.type == 'Memory') ? has(self.targetUtilization) && !has(self.metricName) && !has(self.targetValue) : has(self.metricName) && has(self.targetValue) && !has(self.targetUtilization)",message="CPU/Memory require targetUtilization; Pods/External require metricName and targetValue"
type AutoscalingMetricSpec struct {
	// +required
	Type AutoscalingMetricType `json:"type"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +optional
	TargetUtilization *int32 `json:"targetUtilization,omitempty"`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern="^[a-zA-Z][a-zA-Z0-9_.-]*$"
	// +optional
	MetricName string `json:"metricName,omitempty"`
	// +optional
	TargetValue *ResourceQuantity `json:"targetValue,omitempty"`
}

// AvailabilitySpec is a bounded PodDisruptionBudget contract. Exactly one
// budget shape is required when it is enabled so an AIWorkload cannot express
// contradictory eviction policy. Percentages, selectors and cross-workload
// budgets remain platform-owned concerns.
// +kubebuilder:validation:XValidation:rule="!has(self.enabled) || !self.enabled || (has(self.minAvailable) != has(self.maxUnavailable))",message="enabled availability requires exactly one of minAvailable or maxUnavailable"
type AvailabilitySpec struct {
	// Enabled defaults to false. Omission preserves V0 behavior and does not
	// create a PodDisruptionBudget.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// MinAvailable is the integer count of Pods that must remain available during
	// voluntary disruption. It cannot exceed the workload's configured lower
	// replica bound; AWCP validates that consistency before applying a PDB.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=20
	// +optional
	MinAvailable *int32 `json:"minAvailable,omitempty"`
	// MaxUnavailable is the integer count of Pods that may be voluntarily
	// disrupted. Zero deliberately blocks voluntary eviction and is surfaced as
	// an operational warning for a single-replica workload.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=19
	// +optional
	MaxUnavailable *int32 `json:"maxUnavailable,omitempty"`
}

// ExposureMode bounds the V1 traffic surface to the existing ClusterIP Service
// or a single HTTPRoute attached to a platform-managed Gateway.
// +kubebuilder:validation:Enum=ClusterLocal;HTTPRoute
type ExposureMode string

const (
	ExposureClusterLocal ExposureMode = "ClusterLocal"
	ExposureHTTPRoute    ExposureMode = "HTTPRoute"
)

// ExposureSpec describes an optional HTTPRoute without delegating Gateway
// selection or listener configuration to the workload. Gateway, hostname and
// path are all same-namespace, one-route values; TLS, filters, weights and
// cross-namespace references remain platform-owned concerns.
// +kubebuilder:validation:XValidation:rule="self.mode != 'HTTPRoute' || (has(self.gateway) && has(self.sectionName) && has(self.hostname) && has(self.path))",message="HTTPRoute exposure requires gateway, sectionName, hostname and path"
// +kubebuilder:validation:XValidation:rule="self.mode != 'ClusterLocal' || (!has(self.gateway) && !has(self.sectionName) && !has(self.hostname) && !has(self.path))",message="ClusterLocal exposure must not configure Gateway API routing fields"
type ExposureSpec struct {
	// Mode either preserves cluster-local Service discovery or enables one HTTPRoute.
	// +required
	Mode ExposureMode `json:"mode"`
	// Gateway is an existing Gateway API Gateway in this workload's namespace.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern="^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$"
	// +optional
	Gateway string `json:"gateway,omitempty"`
	// SectionName selects one named HTTP listener on the namespace-local Gateway.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern="^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"
	// +optional
	SectionName string `json:"sectionName,omitempty"`
	// Hostname is the exact DNS hostname matched by the HTTPRoute.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern="^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$"
	// +optional
	Hostname string `json:"hostname,omitempty"`
	// Path is a PathPrefix HTTPRoute match and must begin with /.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:Pattern="^/"
	// +optional
	Path string `json:"path,omitempty"`
}

// SecretReference is a Kubernetes DNS-subdomain Secret name, not a payload or cross-namespace path.
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=253
// +kubebuilder:validation:Pattern="^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$"
type SecretReference string

// ExternalSecretReference names an existing External Secrets Operator
// ExternalSecret and the Secret it is expected to maintain in this namespace.
// Neither field contains a provider value, credential or remote secret key.
type ExternalSecretReference struct {
	// ExternalSecret is the namespace-local ESO ExternalSecret resource name.
	// +required
	ExternalSecret SecretReference `json:"externalSecret"`
	// TargetSecret is the namespace-local Kubernetes Secret expected from that
	// ExternalSecret. It is injected after direct secretRefs in deterministic order.
	// +required
	TargetSecret SecretReference `json:"targetSecret"`
}

// NetworkSpec controls only the generated ingress policy. CNI enforcement is external.
type NetworkSpec struct {
	// Enabled defaults to true; false disables only this workload's generated policy.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// AIWorkloadStatus defines the observed state of AIWorkload.
type AIWorkloadStatus struct {
	// ObservedGeneration is the CR generation evaluated, not proof of rollout success.
	// +kubebuilder:validation:Minimum=0
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// DesiredReplicas is the desired count observed in spec.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=20
	// +optional
	DesiredReplicas int32 `json:"desiredReplicas,omitempty"`
	// ReadyReplicas is the observed Deployment ready count (including possible surge).
	// +kubebuilder:validation:Minimum=0
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
	// Endpoint is service discovery (<child>.<namespace>.svc:<port>), not health proof.
	// +kubebuilder:validation:MaxLength=512
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
	// Conditions are controller observations keyed by type. No Ready value is defaulted.
	// Every written condition must identify the generation it evaluated.
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:XValidation:rule="self.all(c, has(c.observedGeneration))",message="every condition must include observedGeneration"
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=".spec.image"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// AIWorkload is the Schema for the aiworkloads API
type AIWorkload struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of AIWorkload
	// +required
	Spec AIWorkloadSpec `json:"spec"`

	// status defines the observed state of AIWorkload
	// +optional
	Status AIWorkloadStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// AIWorkloadList contains a list of AIWorkload
type AIWorkloadList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []AIWorkload `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &AIWorkload{}, &AIWorkloadList{})
		return nil
	})
}
