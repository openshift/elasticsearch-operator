package k8shandler

import (
	"fmt"
	"reflect"

	"github.com/openshift/elasticsearch-operator/pkg/utils"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/retry"

	api "github.com/openshift/elasticsearch-operator/pkg/apis/elasticsearch/v1"
)

var wrongConfig bool
var nodes map[string][]NodeTypeInterface

func FlushNodes(clusterName, namespace string) {
	nodes[nodeMapKey(clusterName, namespace)] = []NodeTypeInterface{}
}

func nodeMapKey(clusterName, namespace string) string {
	return fmt.Sprintf("%v-%v", clusterName, namespace)
}

// CreateOrUpdateElasticsearchCluster creates an Elasticsearch deployment
func CreateOrUpdateElasticsearchCluster(cluster *api.Elasticsearch) error {

	// Verify that we didn't scale up too many masters
	err := isValidConf(cluster)
	if err != nil {
		// if wrongConfig=true then we've already print out error message
		// don't flood the stderr of the operator with the same message
		if wrongConfig {
			return nil
		}
		wrongConfig = true
		return err
	}
	wrongConfig = false

	getNodes(cluster)

	progressUnshedulableNodes(cluster)

	// if there is a node currently being upgraded, work on that first
	upgradeInProgressNode := getNodeUpgradeInProgress(cluster)
	scheduledUpgradeNodes := getScheduledUpgradeNodes(cluster)
	if upgradeInProgressNode != nil {

		clusterStatus := cluster.Status.DeepCopy()
		index, nodeStatus := getNodeStatus(upgradeInProgressNode.name(), clusterStatus)

		if _, ok := containsNodeTypeInterface(upgradeInProgressNode, scheduledUpgradeNodes); ok {
			logrus.Debugf("Continuing update for %v", upgradeInProgressNode.name())
			upgradeInProgressNode.update(nodeStatus)
		} else {
			logrus.Debugf("Continuing restart for %v", upgradeInProgressNode.name())
			upgradeInProgressNode.restart(nodeStatus)
		}

		nodeState := upgradeInProgressNode.state()

		nodeStatus.UpgradeStatus.ScheduledForUpgrade = nodeState.UpgradeStatus.ScheduledForUpgrade
		nodeStatus.UpgradeStatus.ScheduledForRedeploy = nodeState.UpgradeStatus.ScheduledForRedeploy

		if index == NOT_FOUND_INDEX {
			clusterStatus.Nodes = append(clusterStatus.Nodes, *nodeStatus)
		} else {
			clusterStatus.Nodes[index] = *nodeStatus
		}

		updateNodeStatus(cluster, *clusterStatus)

	} else {

		if len(scheduledUpgradeNodes) > 0 {
			for _, node := range scheduledUpgradeNodes {
				logrus.Debugf("Perform a update for %v", node.name())
				clusterStatus := cluster.Status.DeepCopy()
				index, nodeStatus := getNodeStatus(node.name(), clusterStatus)

				err := node.update(nodeStatus)
				nodeState := node.state()

				nodeStatus.UpgradeStatus.ScheduledForUpgrade = nodeState.UpgradeStatus.ScheduledForUpgrade
				nodeStatus.UpgradeStatus.ScheduledForRedeploy = nodeState.UpgradeStatus.ScheduledForRedeploy

				if index == NOT_FOUND_INDEX {
					clusterStatus.Nodes = append(clusterStatus.Nodes, *nodeStatus)
				} else {
					clusterStatus.Nodes[index] = *nodeStatus
				}

				updateNodeStatus(cluster, *clusterStatus)

				if err != nil {
					logrus.Warnf("Error occurred while updating node %v: %v", node.name(), err)
				}
			}

		} else {

			scheduledRedeployNodes := getScheduledRedeployOnlyNodes(cluster)
			if len(scheduledRedeployNodes) > 0 {
				// get all nodes that need only a rollout
				// TODO: ready cluster for a pod restart first
				for _, node := range scheduledRedeployNodes {
					logrus.Debugf("Perform a redeploy for %v", node.name())
					clusterStatus := cluster.Status.DeepCopy()
					index, nodeStatus := getNodeStatus(node.name(), clusterStatus)

					node.restart(nodeStatus)
					nodeState := node.state()

					nodeStatus.UpgradeStatus.ScheduledForUpgrade = nodeState.UpgradeStatus.ScheduledForUpgrade
					nodeStatus.UpgradeStatus.ScheduledForRedeploy = nodeState.UpgradeStatus.ScheduledForRedeploy

					if index == NOT_FOUND_INDEX {
						clusterStatus.Nodes = append(clusterStatus.Nodes, *nodeStatus)
					} else {
						clusterStatus.Nodes[index] = *nodeStatus
					}

					updateNodeStatus(cluster, *clusterStatus)
				}

			} else {

				for _, node := range nodes[nodeMapKey(cluster.Name, cluster.Namespace)] {
					clusterStatus := cluster.Status.DeepCopy()
					index, nodeStatus := getNodeStatus(node.name(), clusterStatus)

					// Verify that we didn't scale up too many masters
					if err := isValidConf(cluster); err != nil {
						// if wrongConfig=true then we've already print out error message
						// don't flood the stderr of the operator with the same message
						if wrongConfig {
							return nil
						}
						wrongConfig = true
						return err
					}

					if err := node.create(); err != nil {
						return err
					}
					nodeState := node.state()

					nodeStatus.UpgradeStatus.ScheduledForUpgrade = nodeState.UpgradeStatus.ScheduledForUpgrade
					nodeStatus.UpgradeStatus.ScheduledForRedeploy = nodeState.UpgradeStatus.ScheduledForRedeploy

					if index == NOT_FOUND_INDEX {
						// this is a new status, just append
						nodeStatus.DeploymentName = nodeState.DeploymentName
						nodeStatus.StatefulSetName = nodeState.StatefulSetName
						clusterStatus.Nodes = append(clusterStatus.Nodes, *nodeStatus)
					} else {
						// this is an existing status, update in place
						clusterStatus.Nodes[index] = *nodeStatus
					}

					// update status here
					updateNodeStatus(cluster, *clusterStatus)

					updateMinMasters(cluster)
				}

			}
		}
	}

	// Scrape cluster health from elasticsearch every time
	return UpdateClusterStatus(cluster)
}

func updateMinMasters(cluster *api.Elasticsearch) {
	// do as best effort -- whenever we create a node update min masters (if required)

	currentMasterCount, err := GetMinMasterNodes(cluster.Name, cluster.Namespace)
	if err != nil {
		logrus.Debugf("Unable to get current min master count for cluster %v", cluster.Name)
	}

	desiredMasterCount := getMasterCount(cluster)/2 + 1
	currentNodeCount, err := GetClusterNodeCount(cluster.Name, cluster.Namespace)

	// check that we have the required number of master nodes in the cluster...
	if currentNodeCount >= desiredMasterCount {
		if currentMasterCount != desiredMasterCount {
			if _, setErr := SetMinMasterNodes(cluster.Name, cluster.Namespace, desiredMasterCount); setErr != nil {
				logrus.Debugf("Unable to set min master count to %d for cluster %v", desiredMasterCount, cluster.Name)
			}
		}
	}
}

func getNodeUpgradeInProgress(cluster *api.Elasticsearch) NodeTypeInterface {
	for _, node := range cluster.Status.Nodes {
		if node.UpgradeStatus.UnderUpgrade == v1.ConditionTrue {
			for _, nodeTypeInterface := range nodes[nodeMapKey(cluster.Name, cluster.Namespace)] {
				if node.DeploymentName == nodeTypeInterface.name() ||
					node.StatefulSetName == nodeTypeInterface.name() {
					return nodeTypeInterface
				}
			}
		}
	}

	return nil
}

func progressUnshedulableNodes(cluster *api.Elasticsearch) {
	for _, node := range cluster.Status.Nodes {
		if isPodUnschedulableConditionTrue(node.Conditions) {
			for _, nodeTypeInterface := range nodes[nodeMapKey(cluster.Name, cluster.Namespace)] {
				if node.DeploymentName == nodeTypeInterface.name() ||
					node.StatefulSetName == nodeTypeInterface.name() {
					logrus.Debugf("Node %s is unschedulable, trying to recover...", nodeTypeInterface.name())
					if err := nodeTypeInterface.progressUnshedulableNode(&node); err != nil {
						logrus.Warnf("Failed to progress update of unschedulable node '%s': %v", nodeTypeInterface.name(), err)
					}
				}
			}
		}
	}
}

func setUUIDs(cluster *api.Elasticsearch) {

	for index := 0; index < len(cluster.Spec.Nodes); index++ {
		if cluster.Spec.Nodes[index].GenUUID == nil {
			uuid, err := utils.RandStringBytes(8)
			if err != nil {
				continue
			}

			// update the node to set uuid
			cluster.Spec.Nodes[index].GenUUID = &uuid

			nretries := -1
			retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				nretries++
				if getErr := sdk.Get(cluster); getErr != nil {
					logrus.Debugf("Could not get Elasticsearch %v: %v", cluster.Name, getErr)
					return getErr
				}

				if cluster.Spec.Nodes[index].GenUUID != nil {
					return nil
				}

				cluster.Spec.Nodes[index].GenUUID = &uuid

				if updateErr := sdk.Update(cluster); updateErr != nil {
					logrus.Debugf("Failed to update Elasticsearch %s status. Reason: %v. Trying again...", cluster.Name, updateErr)
					return updateErr
				}
				return nil
			})

			if retryErr != nil {
				logrus.Errorf("Error: could not update status for Elasticsearch %v after %v retries: %v", cluster.Name, nretries, retryErr)
			}
			logrus.Debugf("Updated Elasticsearch %v after %v retries", cluster.Name, nretries)
		}
	}

}

func getNodes(cluster *api.Elasticsearch) {

	setUUIDs(cluster)

	if nodes == nil {
		nodes = make(map[string][]NodeTypeInterface)
	}

	currentNodes := []NodeTypeInterface{}

	// get list of client only nodes, and collapse node info into the node (self field) if needed
	for _, node := range cluster.Spec.Nodes {

		// build the NodeTypeInterface list
		for _, nodeTypeInterface := range GetNodeTypeInterface(*node.GenUUID, node, cluster) {

			nodeIndex, ok := containsNodeTypeInterface(nodeTypeInterface, nodes[nodeMapKey(cluster.Name, cluster.Namespace)])
			if !ok {
				currentNodes = append(currentNodes, nodeTypeInterface)
			} else {
				nodes[nodeMapKey(cluster.Name, cluster.Namespace)][nodeIndex].updateReference(nodeTypeInterface)
				currentNodes = append(currentNodes, nodes[nodeMapKey(cluster.Name, cluster.Namespace)][nodeIndex])
			}

		}
	}

	minMasterUpdated := false

	// we want to only keep nodes that were generated and purge/delete any other ones...
	for _, node := range nodes[nodeMapKey(cluster.Name, cluster.Namespace)] {
		if _, ok := containsNodeTypeInterface(node, currentNodes); !ok {
			if !minMasterUpdated {
				// if we're removing a node make sure we set a lower min masters to keep cluster functional
				updateMinMasters(cluster)
				minMasterUpdated = true
			}
			node.delete()

			// remove from status.Nodes
			if index, _ := getNodeStatus(node.name(), &cluster.Status); index != NOT_FOUND_INDEX {
				cluster.Status.Nodes = append(cluster.Status.Nodes[:index], cluster.Status.Nodes[index+1:]...)
			}
		}
	}

	nodes[nodeMapKey(cluster.Name, cluster.Namespace)] = currentNodes
}

func getScheduledUpgradeNodes(cluster *api.Elasticsearch) []NodeTypeInterface {
	upgradeNodes := []NodeTypeInterface{}

	for _, node := range cluster.Status.Nodes {
		if node.UpgradeStatus.ScheduledForUpgrade == v1.ConditionTrue {
			for _, nodeTypeInterface := range nodes[nodeMapKey(cluster.Name, cluster.Namespace)] {
				if node.DeploymentName == nodeTypeInterface.name() ||
					node.StatefulSetName == nodeTypeInterface.name() {
					upgradeNodes = append(upgradeNodes, nodeTypeInterface)
				}
			}
		}
	}

	return upgradeNodes
}

func getScheduledRedeployOnlyNodes(cluster *api.Elasticsearch) []NodeTypeInterface {
	redeployNodes := []NodeTypeInterface{}

	for _, node := range cluster.Status.Nodes {
		if node.UpgradeStatus.ScheduledForRedeploy == v1.ConditionTrue &&
			(node.UpgradeStatus.ScheduledForUpgrade == v1.ConditionFalse ||
				node.UpgradeStatus.ScheduledForUpgrade == "") {
			for _, nodeTypeInterface := range nodes[nodeMapKey(cluster.Name, cluster.Namespace)] {
				if node.DeploymentName == nodeTypeInterface.name() ||
					node.StatefulSetName == nodeTypeInterface.name() {
					redeployNodes = append(redeployNodes, nodeTypeInterface)
				}
			}
		}
	}

	return redeployNodes
}

func updateNodeStatus(cluster *api.Elasticsearch, status api.ElasticsearchStatus) error {
	// if there is nothing to update, don't
	if reflect.DeepEqual(cluster.Status, status) {
		return nil
	}

	nretries := -1
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		nretries++
		if getErr := sdk.Get(cluster); getErr != nil {
			logrus.Debugf("Could not get Elasticsearch %v: %v", cluster.Name, getErr)
			return getErr
		}

		cluster.Status = status

		if updateErr := sdk.Update(cluster); updateErr != nil {
			logrus.Debugf("Failed to update Elasticsearch %s status. Reason: %v. Trying again...", cluster.Name, updateErr)
			return updateErr
		}

		logrus.Debugf("Updated Elasticsearch %v after %v retries", cluster.Name, nretries)
		return nil
	})

	if retryErr != nil {
		return fmt.Errorf("Error: could not update status for Elasticsearch %v after %v retries: %v", cluster.Name, nretries, retryErr)
	}

	return nil
}
