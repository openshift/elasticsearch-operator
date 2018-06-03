package v1alpha1

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ElasticsearchList struct represents list of Elasticsearch objects
type ElasticsearchList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Elasticsearch `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Elasticsearch struct represents Elasticsearch cluster CRD
type Elasticsearch struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              ElasticsearchSpec   `json:"spec"`
	Status            ElasticsearchStatus `json:"status,omitempty"`
}

// ElasticsearchNode struct represents individual node in Elasticsearch cluster
type ElasticsearchNode struct {
	NodeRole     string                         `json:"nodeRole"`
	Replicas     int32                          `json:"replicas"`
	Resources    v1.ResourceRequirements        `json:"resources"`
	Config       ElasticsearchConfig            `json:"elasticsearchConfig"`
	NodeSelector map[string]string              `json:"nodeSelector,omitempty"`
	Storage      ElasticsearchNodeStorageSource `json:"storage"`
}

type ElasticsearchNodeStorageSource struct {
//	StorageType                 string                                   `json:storageType`
	HostPath                    *v1.HostPathVolumeSource                 `json:"hostPath,omitempty"`
	EmptyDir                    *v1.EmptyDirVolumeSource                 `json:"emptyDir,omitempty"`
	PersistentVolumeClaim       *v1.PersistentVolumeClaimVolumeSource    `json:"persistentVolumeClaim,omitempty"`
	PersistentVolumeClaimPrefix *PersistentVolumeClaimPrefixVolumeSource `json:"persistentVolumeClaimPrefix,omitempty"`
}

type PersistentVolumeClaimPrefixVolumeSource struct {
	ClaimPrefixName string `json:"prefixName"`
}

// ElasticsearchNodeStatus represents the status of individual Elasticsearch node
type ElasticsearchNodeStatus struct {
	PodName string `json:"podName"`
	Status  string `json:"status"`
}

// ElasticsearchSpec struct represents the Spec of Elasticsearch cluster CRD
type ElasticsearchSpec struct {
	// Fill me
	Nodes  []ElasticsearchNode `json:"nodes"`
	Config ElasticsearchConfig `json:"elasticsearchConfig"`
	Secure ElasticsearchSecure `json:"securityConfig"`
}

// ElasticsearchConfig represents configuration of an individual Elasticsearch node
type ElasticsearchConfig struct {
	Image string `json:"image,omitempty"`
}

// ElasticsearchSecure struct represents security configuration of the cluster
// whether SearchGuard is enabled along with oauth-proxy sidecar
type ElasticsearchSecure struct {
	Enabled bool   `json:"enabled"`
	Image   string `json:"image,omitempty"`
}

// ElasticsearchStatus represents the status of Elasticsearch cluster
type ElasticsearchStatus struct {
	// Fill me
	Nodes []ElasticsearchNodeStatus `json:"nodes"`
}
