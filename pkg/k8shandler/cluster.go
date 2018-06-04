package k8shandler

import (
	"fmt"
	apps "k8s.io/api/apps/v1beta2"
	"k8s.io/api/core/v1"

	"github.com/sirupsen/logrus"
	v1alpha1 "github.com/t0ffel/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
)

type clusterState struct {
	Nodes []nodeState
	DanglingStatefulSets apps.StatefulSetList
	DanglingDeployments  apps.DeploymentList
}

type nodeState struct {
	Config      elasticsearchNode
	StatefulSet apps.StatefulSet
	Deployment  apps.Deployment
	Pods        v1.Pod
}

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

func NewClusterState(dpl *v1alpha1.Elasticsearch) clusterState {
	nodes := []nodeState{}
	cState := clusterState{
		Nodes: nodes,
	}
	var i int32
	for nodeNum, node := range dpl.Spec.Nodes {

		for i = 1; i <= node.Replicas; i++ {
			nodeCfg, err := constructNodeConfig(dpl, node, int32(nodeNum), i)
			if err != nil {
				logrus.Errorf("Unable to construct ES node config %v", err)
			}

			node := nodeState{
				Config: nodeCfg,
			}
			cState.Nodes = append(cState.Nodes, node)
		}
	}

	cState.amendDeployments(dpl)
	return cState
}

// getClusterState checks the desired state against what's present in current
// deployments/statefulsets/pods
func (cState *clusterState) getClusterRequiredAction() (v1alpha1.ElasticsearchK8sHealth, error) {
	nodeList, err := listNodes(dpl)
	if err != nil {
		return v1alpha1.ElasticsearchK8sInterventionNeeded, fmt.Errorf("Unable to list Elasticsearch's nodes: %v", err)
	}

	if len(nodeList) == 0 {
		return v1alpha1.ElasticsearchK8sNewClusterNeeded, nil
	}

	//podList, err := listPods(dpl)
	//if err != nil {
	//	return v1alpha1.ElasticsearchK8sInterventionNeeded, fmt.Errorf("Unable to list Elasticsearch pods: %v", err)
	//}

	var i int32
	for nodeNum, node := range dpl.Spec.Nodes {

		for i = 1; i <= node.Replicas; i++ {
			nodeCfg, err := constructNodeConfig(dpl, node, int32(nodeNum), i)
			if err != nil {
				return v1alpha1.ElasticsearchK8sInterventionNeeded, fmt.Errorf("Unable to construct ES node config %v", err)
			}

			updateNeeded := nodeCfg.IsUpdateNeeded(dpl)
			if updateNeeded {
				logrus.Infof("Node %s requires cluster update")
				return v1alpha1.ElasticsearchK8sRollingRestartNeeded, nil
			}
		}
	}
	// Scale down
	return v1alpha1.ElasticsearchK8sOK, nil
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
