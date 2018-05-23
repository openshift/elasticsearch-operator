package v1alpha1

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ElasticsearchList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Elasticsearch `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Elasticsearch struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              ElasticsearchSpec   `json:"spec"`
	Status            ElasticsearchStatus `json:"status,omitempty"`
}

type ElasticsearchNode struct {
	NodeRole  string                  `json:"nodeRole"`
	Replicas  int32                   `json:"replicas"`
	Resources v1.ResourceRequirements `json:"resources"`
}

type ElasticsearchNodeStatus struct {
	PodName string `json:"podName"`
	Status  string `json:"status"`
}

type ElasticsearchSpec struct {
	// Fill me
	Nodes  []ElasticsearchNode `json:"nodes"`
	Secure ElasticsearchSecure `json:"secure"`
}

type ElasticsearchSecure struct {
	Enabled bool `json:"enabled"`
}

type ElasticsearchStatus struct {
	// Fill me
	Nodes []ElasticsearchNodeStatus `json:"nodes"`
}
