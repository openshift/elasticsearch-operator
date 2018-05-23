package k8shandler

import (
	"fmt"

	"github.com/operator-framework/operator-sdk/pkg/sdk/query"
	v1alpha1 "github.com/t0ffel/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	apps "k8s.io/api/apps/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func isNodeConfigured(sSet apps.StatefulSet, dpl *v1alpha1.Elasticsearch) bool {
	label := sSet.ObjectMeta.Labels["es-node-role"]
	for _, cmpNode := range dpl.Spec.Nodes {
		if cmpNode.NodeRole == label {
			return true
		}
	}
	return false
}

func listStatefulSets(dpl *v1alpha1.Elasticsearch) (*apps.StatefulSetList, error) {
	list := ssList()
	labelSelector := labels.SelectorFromSet(LabelsForESCluster(dpl.Name)).String()
	listOps := &metav1.ListOptions{LabelSelector: labelSelector}
	err := query.List(dpl.Namespace, list, query.WithListOptions(listOps))
	if err != nil {
		return list, fmt.Errorf("Unable to list StatefulSets: %v", err)
	}

	return list, nil
}

func ssList() *apps.StatefulSetList {
	return &apps.StatefulSetList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
	}
}

func (cfg *elasticsearchNode) isDifferent(sset *apps.StatefulSet) (bool, error) {
	// Check replicas number
	if cfg.getReplicas() != *sset.Spec.Replicas {
		return true, nil
	}

	// Check if the Variables are the desired ones

	return false, nil
}
