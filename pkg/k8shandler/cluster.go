package k8shandler

import (
	"fmt"
	apps "k8s.io/api/apps/v1beta2"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/operator-framework/operator-sdk/pkg/sdk/action"

	"github.com/sirupsen/logrus"
	v1alpha1 "github.com/t0ffel/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
)

type clusterState struct {
	Nodes []*nodeState
	DanglingStatefulSets *apps.StatefulSetList
	DanglingDeployments  *apps.DeploymentList
}

type nodeState struct {
	Config      elasticsearchNode
	StatefulSet *apps.StatefulSet
	Deployment  *apps.Deployment
	Pods        *v1.Pod
}

// CreateOrUpdateElasticsearchCluster creates an Elasticsearch deployment
func CreateOrUpdateElasticsearchCluster(dpl *v1alpha1.Elasticsearch) error {

	//err := removeStaleNodes(dpl)
	//if err != nil {
	//	return fmt.Errorf("Unable to remove some stale nodes: %v", err)
	//}

	cState := NewClusterState(dpl)
	action, err := cState.getClusterRequiredAction()
	if err != nil {
		return err
	}
	logrus.Infof("cluster required action is: %v", action)

	switch {
	case action == v1alpha1.ElasticsearchActionNewClusterNeeded:
		err = cState.buildNewCluster(asOwner(dpl))
		if err != nil {
			return err
		}
	case action == v1alpha1.ElasticsearchActionScaleDownNeeded:
		err = cState.removeStaleNodes()
		if err != nil {
			return err
		}
	}
	return nil
}

func NewClusterState(dpl *v1alpha1.Elasticsearch) clusterState {
	nodes := []*nodeState{}
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
			cState.Nodes = append(cState.Nodes, &node)
		}
	}

	cState.amendDeployments(dpl)
	// TODO: add amendStatefulSets
	// TODO: add amendPods
	return cState
}

// getClusterRequiredAction checks the desired state against what's present in current
// deployments/statefulsets/pods
func (cState *clusterState) getClusterRequiredAction() (v1alpha1.ElasticsearchRequiredAction, error) {
	// TODO: implement better logic to understand when new cluster is needed
	for _, node := range cState.Nodes {
		if node.Deployment == nil {
			return v1alpha1.ElasticsearchActionNewClusterNeeded, nil
		}
	}

	if cState.DanglingDeployments != nil {
		return v1alpha1.ElasticsearchActionScaleDownNeeded, nil
	}
	//podList, err := listPods(dpl)
	//if err != nil {
	//	return v1alpha1.ElasticsearchK8sInterventionNeeded, fmt.Errorf("Unable to list Elasticsearch pods: %v", err)
	//}

	return v1alpha1.ElasticsearchActionNone, nil
}

func (cState *clusterState) buildNewCluster(owner metav1.OwnerReference) error {
	for _, node := range cState.Nodes {
		err := node.Config.CreateOrUpdateNode(owner)
		if err != nil {
			return fmt.Errorf("Unable to create Elasticsearch node: %v", err)
		}
	}
	return nil
}

// list existing StatefulSets and delete those unmanaged by the operator
func (cState *clusterState) removeStaleNodes() error {
	for _, node := range cState.DanglingDeployments.Items {
		//logrus.Infof("found statefulset: %v", node.getResource().ObjectMeta.Name)
			// the returned statefulset doesn't have TypeMeta, so we're adding it.
			// node.TypeMeta = metav1.TypeMeta{
			// 	Kind:       "StatefulSet",
			// 	APIVersion: "apps/v1",
			// }
			err := action.Delete(&node)
			if err != nil {
				return fmt.Errorf("Unable to delete resource %v: ", err)
			}
	}
	return nil
}

func (node *nodeState) setDeployment(deployment apps.Deployment) {
	node.Deployment = &deployment
}
