package k8shandler

import (
	v1alpha1 "github.com/t0ffel/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	apps "k8s.io/api/apps/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type NodeTypeInterface interface {
	getResource() runtime.Object
	isNodeConfigured(dpl *v1alpha1.Elasticsearch) bool
	isDifferent(cfg *elasticsearchNode) (bool, error)
	constructNodeResource(cfg *elasticsearchNode) (runtime.Object, error)
	addOwnerRefToObject(r metav1.OwnerReference)
	delete() error
	query() error
}

type NodeTypeFactory func(name, namespace string) NodeTypeInterface

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
