package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Specification of the desired behavior of the Kibana
//
// +k8s:openapi-gen=true
type KibanaSpec struct {
	// Indicator if the resource is 'Managed' or 'Unmanaged' by the operator
	//
	ManagementState ManagementState `json:"managementState"`

	// The resource requirements for Kibana
	//
	// +nullable
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources"`

	// Define which Nodes the Pods are scheduled on.
	NodeSelector map[string]string   `json:"nodeSelector,omitempty"`
	Tolerations  []corev1.Toleration `json:"tolerations,omitempty"`

	// Number of instances to deploy for a Kibana deployment
	//
	// +optional
	Replicas int32 `json:"replicas"`

	// Specification of the Kibana Proxy component
	//
	// +optional
	ProxySpec `json:"proxy,omitempty"`
}

type ProxySpec struct {
	// The resource requirements for Kibana proxy
	//
	// +nullable
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources"`
}

// KibanaStatus defines the observed state of Kibana
// +k8s:openapi-gen=true
type KibanaStatus struct {
	// +optional
	Replicas int32 `json:"replicas"`
	// +optional
	Deployment string `json:"deployment"`
	// +optional
	ReplicaSets []string `json:"replicaSets,omitempty"`
	// +optional
	Pods PodStateMap `json:"pods,omitempty"`
	// +optional
	Conditions map[string]ClusterConditions `json:"clusterCondition,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Kibana is the Schema for the kibanas API
// +k8s:openapi-gen=true
// +kubebuilder:resource:path=kibanas,categories=logging,scope=Namespaced
// +kubebuilder:printcolumn:name="Management State",JSONPath=".spec.managementState",type=string
// +kubebuilder:printcolumn:name="Replicas",JSONPath=".spec.replicas",type=integer
type Kibana struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KibanaSpec     `json:"spec,omitempty"`
	Status []KibanaStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KibanaList contains a list of Kibana
type KibanaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Kibana `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Kibana{}, &KibanaList{})
}
