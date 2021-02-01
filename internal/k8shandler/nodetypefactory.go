package k8shandler

import (
	"fmt"

	api "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/internal/elasticsearch"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NodeTypeInterface interace represents individual Elasticsearch node
type NodeTypeInterface interface {
	state() api.ElasticsearchNodeStatus // this will get the current -- used for status
	updateReference(node NodeTypeInterface)
	populateReference(nodeName string, node api.ElasticsearchNode, cluster *api.Elasticsearch, roleMap map[api.ElasticsearchNodeRole]bool, replicas int32, client client.Client, esClient elasticsearch.Client)

	create() error // this will create the node in the case where it is new
	isMissing() bool
	name() string
	delete() error
	getSecretHash() string

	refreshHashes()
	scaleDown() error
	scaleUp() error
	progressNodeChanges() error              // this function is used to tell the node to push out its changes
	waitForNodeRejoinCluster() (bool, error) // this function is used to determine if a node has rejoined the cluster
	waitForNodeLeaveCluster() (bool, error)  // this function is used to determine if a node has left the cluster
}

// NodeTypeFactory is a factory to construct either statefulset or deployment
type NodeTypeFactory func(name, namespace string) NodeTypeInterface

// this can potentially return a list if we have replicas > 1 for a data node
func (er *ElasticsearchRequest) GetNodeTypeInterface(uuid string, node api.ElasticsearchNode) []NodeTypeInterface {
	nodes := []NodeTypeInterface{}

	roleMap := getNodeRoleMap(node)

	// common spec => cluster.Spec.Spec
	nodeName := fmt.Sprintf("%s-%s", er.cluster.Name, getNodeSuffix(uuid, roleMap))

	// if we have a data node then we need to create one deployment per replica
	if isDataNode(node) {
		// for loop from 1 to replica as replicaIndex
		//   it is 1 instead of 0 because of legacy code
		for replicaIndex := int32(1); replicaIndex <= node.NodeCount; replicaIndex++ {
			dataNodeName := addDataNodeSuffix(nodeName, replicaIndex)
			node := newDeploymentNode(dataNodeName, node, er.cluster, roleMap, er.client, er.esClient)
			nodes = append(nodes, node)
		}
	} else {
		node := newStatefulSetNode(nodeName, node, er.cluster, roleMap, er.client, er.esClient)
		nodes = append(nodes, node)
	}

	return nodes
}

func getNodeSuffix(uuid string, roleMap map[api.ElasticsearchNodeRole]bool) string {
	suffix := ""
	if roleMap[api.ElasticsearchRoleClient] {
		suffix = fmt.Sprintf("%s%s", suffix, "c")
	}

	if roleMap[api.ElasticsearchRoleData] {
		suffix = fmt.Sprintf("%s%s", suffix, "d")
	}

	if roleMap[api.ElasticsearchRoleMaster] {
		suffix = fmt.Sprintf("%s%s", suffix, "m")
	}

	return fmt.Sprintf("%s-%s", suffix, uuid)
}

func addDataNodeSuffix(nodeName string, replicaNumber int32) string {
	return fmt.Sprintf("%s-%d", nodeName, replicaNumber)
}

// newDeploymentNode constructs deploymentNode struct for data nodes
func newDeploymentNode(nodeName string, node api.ElasticsearchNode, cluster *api.Elasticsearch, roleMap map[api.ElasticsearchNodeRole]bool, client client.Client, esClient elasticsearch.Client) NodeTypeInterface {
	deploymentNode := deploymentNode{}

	deploymentNode.populateReference(nodeName, node, cluster, roleMap, int32(1), client, esClient)

	return &deploymentNode
}

// newStatefulSetNode constructs statefulSetNode struct for non-data nodes
func newStatefulSetNode(nodeName string, node api.ElasticsearchNode, cluster *api.Elasticsearch, roleMap map[api.ElasticsearchNodeRole]bool, client client.Client, esClient elasticsearch.Client) NodeTypeInterface {
	statefulSetNode := statefulSetNode{}

	statefulSetNode.populateReference(nodeName, node, cluster, roleMap, node.NodeCount, client, esClient)

	return &statefulSetNode
}

func containsNodeTypeInterface(node NodeTypeInterface, list []NodeTypeInterface) (int, bool) {
	for index, nodeTypeInterface := range list {
		if nodeTypeInterface.name() == node.name() {
			return index, true
		}
	}

	return -1, false
}

func (er *ElasticsearchRequest) getNodeState(node NodeTypeInterface) *api.ElasticsearchNodeStatus {
	index, status := getNodeStatus(node.name(), &er.cluster.Status)

	if index == NotFoundIndex {
		state := node.state()
		status = &state
	}

	return status
}
