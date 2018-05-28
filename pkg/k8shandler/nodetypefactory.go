package k8shandler

import (
	v1alpha1 "github.com/t0ffel/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	apps "k8s.io/api/apps/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NodeTypeInterface interace represents individual Elasticsearch node
type NodeTypeInterface interface {
	getResource() runtime.Object
	isNodeConfigured(dpl *v1alpha1.Elasticsearch) bool
	isDifferent(cfg *elasticsearchNode) (bool, error)
	constructNodeResource(cfg *elasticsearchNode, owner metav1.OwnerReference) (runtime.Object, error)
	delete() error
	query() error
}

// NodeTypeFactory is a factory to construct either statefulset or deployment
type NodeTypeFactory func(name, namespace string) NodeTypeInterface

// NewDeploymentNode constructs deploymentNode struct for data nodes
func NewDeploymentNode(name, namespace string) NodeTypeInterface {
	depl := apps.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	node := deploymentNode{resource: depl}
	return &node
}

// NewStatefulSetNode constructs statefulSetNode struct for non-data nodes
func NewStatefulSetNode(name, namespace string) NodeTypeInterface {
	depl := apps.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	ss := statefulSetNode{resource: depl}
	return &ss
}
