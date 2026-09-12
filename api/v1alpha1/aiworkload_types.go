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

// AIWorkloadSpec is intentionally empty in the AWCP-3 bootstrap.
// The workload contract and validation are implemented in AWCP-4.
type AIWorkloadSpec struct{}

// AIWorkloadStatus defines the observed state of AIWorkload.
type AIWorkloadStatus struct {
	// Conditions retains the scaffold's standard status shape. The bootstrap
	// controller does not publish conditions or claim workload readiness.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced

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
