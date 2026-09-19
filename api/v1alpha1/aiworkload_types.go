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

	// SecretRefs contains ordered, unique Secret names in this CR's namespace.
	// Later envFrom entries win on overlapping keys. Values are never part of this API.
	// +kubebuilder:default={}
	// +kubebuilder:validation:MaxItems=32
	// +kubebuilder:validation:XValidation:rule="self.all(x, self.filter(y, y == x).size() == 1)",message="secretRefs must not contain duplicate names"
	// +listType=atomic
	// +optional
	SecretRefs []SecretReference `json:"secretRefs,omitempty"`

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

// SecretReference is a Kubernetes DNS-subdomain Secret name, not a payload or cross-namespace path.
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=253
// +kubebuilder:validation:Pattern="^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$"
type SecretReference string

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
