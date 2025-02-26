/*
Copyright 2025.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LlamaStackSpec defines the desired state of LlamaStack
type LlamaStackSpec struct {
	Replicas     *int32         `json:"replicas,omitempty"`
	Distribution string         `json:"distribution"` // e.g., "ollama", "vllm-gpu"; mutually exclusive
	Image        string         `json:"image"`        // Custom Image, mutually exclusive with Distribution
	Template     corev1.PodSpec `json:"template,omitempty"`
}

// LlamaStackStatus defines the observed state of LlamaStack
type LlamaStackStatus struct {
	Distribution string `json:"distribution"`
	Image        string `json:"image,omitempty"`
	Ready        bool   `json:"ready"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Distribution",type="string",JSONPath=".status.distribution"
//+kubebuilder:printcolumn:name="Image",type="string",JSONPath=".status.image"
//+kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
// LlamaStack is the Schema for the llamastacks API

type LlamaStack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LlamaStackSpec   `json:"spec,omitempty"`
	Status LlamaStackStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LlamaStackList contains a list of LlamaStack
type LlamaStackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LlamaStack `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LlamaStack{}, &LlamaStackList{})
}
