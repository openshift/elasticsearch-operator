package k8shandler

import (
	"fmt"

	apps "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/ViaQ/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
)

// clusterState struct represents the state of the cluster
type clusterState struct {
	Nodes                []*nodeState
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
func CreateOrUpdateElasticsearchCluster(dpl *v1alpha1.Elasticsearch, configMapName, serviceAccountName string) error {

	cState, err := NewClusterState(dpl, configMapName, serviceAccountName)
	if err != nil {
		return err
	}

	action, err := cState.getRequiredAction()
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
	case action == v1alpha1.ElasticsearchActionRollingRestartNeeded:
		// TODO: change this to do the actual rolling restart
		err = cState.buildNewCluster(asOwner(dpl))
		if err != nil {
			return err
		}
	case action == v1alpha1.ElasticsearchActionNone:
		// No action is requested
		return nil
	default:
		return fmt.Errorf("Unknown cluster action requested: %v", action)
	}
	return nil
}

func NewClusterState(dpl *v1alpha1.Elasticsearch, configMapName, serviceAccountName string) (clusterState, error) {
	nodes := []*nodeState{}
	cState := clusterState{
		Nodes: nodes,
	}
	var i int32
	for nodeNum, node := range dpl.Spec.Nodes {

		for i = 1; i <= node.Replicas; i++ {
			nodeCfg, err := constructNodeSpec(dpl, node, configMapName, serviceAccountName, int32(nodeNum), i)
			if err != nil {
				return cState, fmt.Errorf("Unable to construct ES node config %v", err)
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
	return cState, nil
}

// getRequiredAction checks the desired state against what's present in current
// deployments/statefulsets/pods
func (cState *clusterState) getRequiredAction() (v1alpha1.ElasticsearchRequiredAction, error) {
	// TODO: Add condition that if an operation is currently in progress
	// not to try to queue another action. Instead return ElasticsearchActionInProgress which
	// is noop.

	// TODO: Handle failures. Maybe introduce some ElasticsearchCondition which says
	// what action was attempted last, when, how many tries and what the result is.

	// TODO: implement better logic to understand when new cluster is needed
	// maybe RequiredAction ElasticsearchActionNewClusterNeeded should be renamed to
	// ElasticsearchActionAddNewNodes - will blindly add new nodes to the cluster.
	for _, node := range cState.Nodes {
		if node.Deployment == nil {
			return v1alpha1.ElasticsearchActionNewClusterNeeded, nil
		}
	}

	// TODO: implement rolling restart action if any deployment/configmap actually deployed
	// is different from the desired.
	for _, node := range cState.Nodes {
		if node.Config.IsUpdateNeeded() {
			return v1alpha1.ElasticsearchActionRollingRestartNeeded, nil
		}
	}

	// If some deployments exist that are not specified in CR, they'll be in DanglingDeployments
	// we need to remove those to comply with the desired cluster structure.
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
		// the returned deployment doesn't have TypeMeta, so we're adding it.
		node.TypeMeta = metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		}
		err := sdk.Delete(&node)
		if err != nil {
			return fmt.Errorf("Unable to delete resource %v: ", err)
		}
	}
	return nil
}

func (node *nodeState) setDeployment(deployment apps.Deployment) {
	node.Deployment = &deployment
}
