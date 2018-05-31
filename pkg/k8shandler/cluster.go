package k8shandler

import (
	"fmt"

	v1alpha1 "github.com/t0ffel/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
)

// CreateOrUpdateElasticsearchCluster creates an Elasticsearch deployment
func CreateOrUpdateElasticsearchCluster(dpl *v1alpha1.Elasticsearch) error {
	var i int32
	for nodeNum, node := range dpl.Spec.Nodes {

		if node.Replicas < 1 {
			return fmt.Errorf("Incorrect number of replicas for node %v. Must be >= 1", node)
		}
		for i = 1; i <= node.Replicas; i++ {
			nodeCfg, err := constructNodeConfig(dpl, node, int32(nodeNum), i)
			if err != nil {
				return fmt.Errorf("Unable to construct ES node config %v", err)
			}
			err = nodeCfg.CreateOrUpdateNode(dpl)
			if err != nil {
				return fmt.Errorf("Unable to create Elasticsearch node: %v", err)
			}
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
	nodeList, err := listNodes(dpl)
	if err != nil {
		return fmt.Errorf("Unable to list Elasticsearch's nodes: %v", err)
	}
	for _, node := range nodeList {
		//logrus.Infof("found statefulset: %v", node.getResource().ObjectMeta.Name)
		if !node.isNodeConfigured(dpl) {
			// the returned statefulset doesn't have TypeMeta, so we're adding it.
			// node.TypeMeta = metav1.TypeMeta{
			// 	Kind:       "StatefulSet",
			// 	APIVersion: "apps/v1",
			// }
			err = node.delete()
			if err != nil {
				return fmt.Errorf("Unable to delete resource %v: ", err)
			}
		}
	}
	return nil
}
