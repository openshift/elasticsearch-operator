package k8shandler

import (
	"fmt"

	"github.com/operator-framework/operator-sdk/pkg/sdk/action"
	"github.com/sirupsen/logrus"
	v1alpha1 "github.com/t0ffel/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// createOrUpdateElasticsearchCluster creates an Elasticsearch deployment
func createOrUpdateElasticsearchCluster(dpl *v1alpha1.Elasticsearch) error {
	for _, node := range dpl.Spec.Nodes {

		nodeCfg, err := constructNodeConfig(dpl, node)
		if err != nil {
			return fmt.Errorf("Unable to construct ES node config %v", err)
		}
		err = nodeCfg.CreateOrUpdateNode(dpl)
		if err != nil {
			return fmt.Errorf("Unable to create Elasticsearch node: %v", err)
		}
	}

	err := removeStaleNodes(dpl)
	if err != nil {
		return fmt.Errorf("Unable to remove some stale nodes: %v", err)
	}
	return nil
}

// list existing StatefulSets and delete those unmanaged by the operator
func removeStaleNodes(dpl *v1alpha1.Elasticsearch) error {
	sSetList, err := listStatefulSets(dpl)
	if err != nil {
		return fmt.Errorf("Unable to list Elasticsearch's StatefulSets: %v", err)
	}
	for _, sSet := range sSetList.Items {
		logrus.Infof("found statefulset: %v", sSet.ObjectMeta.Name)
		if !isNodeConfigured(sSet, dpl) {
			// the returned statefulset doesn't have TypeMeta, so we're adding it.
			sSet.TypeMeta = metav1.TypeMeta{
				Kind:       "StatefulSet",
				APIVersion: "apps/v1",
			}
			err = action.Delete(&sSet)
			if err != nil {
				return fmt.Errorf("Unable to delete StatefulSet %v: ", err)
			}
		}
	}
	return nil
}
