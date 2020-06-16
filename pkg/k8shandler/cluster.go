package k8shandler

import (
	"context"
	"fmt"
	"reflect"

	"github.com/openshift/elasticsearch-operator/pkg/utils"
	"github.com/openshift/elasticsearch-operator/pkg/utils/comparators"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	api "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
)

const expectedMinVersion = "6.0"

var wrongConfig bool
var nodes map[string][]NodeTypeInterface

var aliasNeededMap map[string]bool

func FlushNodes(clusterName, namespace string) {
	nodes[nodeMapKey(clusterName, namespace)] = []NodeTypeInterface{}
}

func nodeMapKey(clusterName, namespace string) string {
	return fmt.Sprintf("%v-%v", clusterName, namespace)
}

// CreateOrUpdateElasticsearchCluster creates an Elasticsearch deployment
func (elasticsearchRequest *ElasticsearchRequest) CreateOrUpdateElasticsearchCluster() error {
	esClient := elasticsearchRequest.esClient

	// Verify that we didn't scale up too many masters
	err := elasticsearchRequest.isValidConf()
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

	elasticsearchRequest.getNodes()

	//clearing transient setting because of a bug in earlier releases which
	//may leave the shard allocation in an undesirable state
	if elasticsearchRequest.AnyNodeReady() {
		if success, err := esClient.ClearTransientShardAllocation(); !success {
			logrus.Warnf("Unable to clear transient shard allocation for cluster %q in namespace %q: %s", elasticsearchRequest.cluster.Namespace, elasticsearchRequest.cluster.Namespace, err.Error())
		}
	}

	progressUnschedulableNodes(elasticsearchRequest.cluster)

	certRestartNodes := getScheduledCertRedeployNodes(elasticsearchRequest.cluster)
	stillRecovering := containsClusterCondition(api.Recovering, v1.ConditionTrue, &elasticsearchRequest.cluster.Status)
	if len(certRestartNodes) > 0 || stillRecovering {
		err = elasticsearchRequest.PerformFullClusterCertRestart(certRestartNodes)
		if err != nil {
			return elasticsearchRequest.UpdateClusterStatus()
		}
	}

	// if there is a node currently being upgraded, work on that first
	inProgressNode := getNodeUpgradeInProgress(elasticsearchRequest.cluster)
	scheduledNodes := getScheduledUpgradeNodes(elasticsearchRequest.cluster)

	// Check if we have a node that was in the progress of an update
	// if so, continue updating it
	if inProgressNode != nil {
		// currently no way to distinguish between the two -- restart node and update node
		//  however we don't currently have anything that will trigger just a rolling restart of nodes
		//if _, ok := containsNodeTypeInterface(inProgressNode, scheduledUpgradeNodes); ok {
		logrus.Debugf("Continuing update for %v", inProgressNode.name())
		if err := elasticsearchRequest.PerformNodeUpdate(inProgressNode); err != nil {
			logrus.Warnf("unable to update node. E: %s", err.Error())
		}
		/*} else {
			logrus.Debugf("Continuing restart for %v", inProgressNode.name())
			if err := elasticsearchRequest.PerformNodeRestart(inProgressNode); err != nil {
				logrus.Warnf("unable to restart node %q: %s", inProgressNode.name(), err.Error())
			}
		}*/

	} else {

		// We didn't have any in progress, but we have ones scheduled to be updated
		if len(scheduledNodes) > 0 {

			// get the current ES version
			version, err := esClient.GetLowestClusterVersion()
			if err != nil {
				// this can be because we couldn't get a valid response from ES
				logrus.Warnf("when trying to get LowestClusterVersion: %s", err.Error())
			} else {

				logrus.Debugf("Found current cluster version to be %q", version)
				comparison := comparators.CompareVersions(version, expectedMinVersion)

				// if it is < what we expect (6.0) then do full cluster update:
				if comparison > 0 {
					// perform a full cluster update
					if err := elasticsearchRequest.PerformFullClusterUpdate(scheduledNodes); err != nil {
						logrus.Warnf("when trying to perform full cluster update: %s", err.Error())
					}

				} else {

					if err := elasticsearchRequest.PerformRollingUpdate(scheduledNodes); err != nil {
						logrus.Warnf("when trying to perform rolling update: %s", err.Error())
					}
				}
			}

		} else {
			// FIXME: either add logic that will do just restarts of nodes or remove this code
			// Check if we have any nodes scheduled for a restart
			scheduledRestartNodes := getScheduledRedeployOnlyNodes(elasticsearchRequest.cluster)
			if len(scheduledRestartNodes) > 0 {
				// get all nodes that need only a restart

				if err := elasticsearchRequest.PerformRollingRestart(scheduledRestartNodes); err != nil {
					logrus.Warnf("when trying to perform rolling restart: %v", err)
				}
			} else {

				// We have no updates or restarts scheduled
				// create any nodes we are missing and perform any required operations to ensure state
				for _, node := range nodes[nodeMapKey(elasticsearchRequest.cluster.Name, elasticsearchRequest.cluster.Namespace)] {
					clusterStatus := elasticsearchRequest.cluster.Status.DeepCopy()
					_, nodeStatus := getNodeStatus(node.name(), clusterStatus)

					// Verify that we didn't scale up too many masters
					if err := elasticsearchRequest.isValidConf(); err != nil {
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

					addNodeState(node, nodeStatus)
					if err := elasticsearchRequest.setNodeStatus(node, nodeStatus, clusterStatus); err != nil {
						logrus.Warnf("unable to set status for node %q: %s", node.name(), err.Error())
					}

					// if we created a node ensure that MinMasters is (n / 2 + 1)
					if elasticsearchRequest.AnyNodeReady() {
						elasticsearchRequest.updateMinMasters()
					}
				}

				// ensure we always have shard allocation to All if we aren't doing an update...
				if elasticsearchRequest.AnyNodeReady() {
					if ok, err := esClient.SetShardAllocation(api.ShardAllocationAll); !ok {
						logrus.Warnf("Unable to enable shard allocation for cluster %q in namespace %q: %v", elasticsearchRequest.cluster.Name, elasticsearchRequest.cluster.Namespace, err)
					}
				}

				// we only want to update our replicas if we aren't in the middle up an update
				if elasticsearchRequest.ClusterReady() {
					if err := esClient.UpdateReplicaCount(int32(calculateReplicaCount(elasticsearchRequest.cluster))); err != nil {
						logrus.Error(err)
					}
					if aliasNeededMap == nil {
						aliasNeededMap = make(map[string]bool)
					}
					if val, ok := aliasNeededMap[nodeMapKey(elasticsearchRequest.cluster.Name, elasticsearchRequest.cluster.Namespace)]; !ok || val {
						// add alias to old indices if they exist and don't have one
						// this should be removed after one release...
						successful := esClient.AddAliasForOldIndices()

						if successful {
							aliasNeededMap[nodeMapKey(elasticsearchRequest.cluster.Name, elasticsearchRequest.cluster.Namespace)] = false
						}
					}
				}
			}
		}
	}

	// Scrape cluster health from elasticsearch every time
	return elasticsearchRequest.UpdateClusterStatus()
}

func (elasticsearchRequest *ElasticsearchRequest) updateMinMasters() {
	// do as best effort -- whenever we create a node update min masters (if required)

	cluster := elasticsearchRequest.cluster
	esClient := elasticsearchRequest.esClient

	currentMasterCount, err := esClient.GetMinMasterNodes()
	if err != nil {
		logrus.Debugf("Unable to get current min master count for cluster %q in namespace %q", cluster.Name, cluster.Namespace)
	}

	desiredMasterCount := getMasterCount(cluster)/2 + 1
	currentNodeCount, err := esClient.GetClusterNodeCount()
	if err != nil {
		logrus.Warnf("unable to get cluster node count for cluster %q in namespace %q: %s", cluster.Name, cluster.Namespace, err.Error())
	}

	// check that we have the required number of master nodes in the cluster...
	if currentNodeCount >= desiredMasterCount {
		if currentMasterCount != desiredMasterCount {
			if _, setErr := esClient.SetMinMasterNodes(desiredMasterCount); setErr != nil {
				logrus.Debugf("Unable to set min master count to %d for cluster %q in namespace %q", desiredMasterCount, cluster.Name, cluster.Namespace)
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

func progressUnschedulableNodes(cluster *api.Elasticsearch) {
	for _, node := range cluster.Status.Nodes {
		if isPodUnschedulableConditionTrue(node.Conditions) ||
			isPodImagePullBackOff(node.Conditions) ||
			isPodCrashLoopBackOff(node.Conditions) {
			for _, nodeTypeInterface := range nodes[nodeMapKey(cluster.Name, cluster.Namespace)] {
				if node.DeploymentName == nodeTypeInterface.name() ||
					node.StatefulSetName == nodeTypeInterface.name() {
					logrus.Debugf("Node %s is unschedulable, trying to recover...", nodeTypeInterface.name())
					if err := nodeTypeInterface.progressNodeChanges(); err != nil {
						logrus.Warnf("Failed to progress update of unschedulable node '%s': %v", nodeTypeInterface.name(), err)
					}
				}
			}
		}
	}
}

func (elasticsearchRequest *ElasticsearchRequest) setUUIDs() {

	cluster := elasticsearchRequest.cluster

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
				if getErr := elasticsearchRequest.client.Get(context.TODO(), types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster); getErr != nil {
					logrus.Debugf("Could not get Elasticsearch %v: %v", cluster.Name, getErr)
					return getErr
				}

				if cluster.Spec.Nodes[index].GenUUID != nil {
					return nil
				}

				cluster.Spec.Nodes[index].GenUUID = &uuid

				if updateErr := elasticsearchRequest.client.Update(context.TODO(), cluster); updateErr != nil {
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

func (elasticsearchRequest *ElasticsearchRequest) getNodes() {

	elasticsearchRequest.setUUIDs()

	if nodes == nil {
		nodes = make(map[string][]NodeTypeInterface)
	}

	cluster := elasticsearchRequest.cluster
	currentNodes := []NodeTypeInterface{}

	// get list of client only nodes, and collapse node info into the node (self field) if needed
	for _, node := range cluster.Spec.Nodes {

		// build the NodeTypeInterface list
		for _, nodeTypeInterface := range elasticsearchRequest.GetNodeTypeInterface(*node.GenUUID, node) {

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
				if elasticsearchRequest.AnyNodeReady() {
					elasticsearchRequest.updateMinMasters()
					minMasterUpdated = true
				}
			}
			if err := node.delete(); err != nil {
				logrus.Warnf("unable to delete node. E: %s\r\n", err.Error())
			}

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

func getScheduledCertRedeployNodes(cluster *api.Elasticsearch) []NodeTypeInterface {
	redeployCertNodes := []NodeTypeInterface{}
	dataNodes := []NodeTypeInterface{}

	for _, node := range cluster.Status.Nodes {
		if node.UpgradeStatus.ScheduledForCertRedeploy == v1.ConditionTrue {
			for _, nodeTypeInterface := range nodes[nodeMapKey(cluster.Name, cluster.Namespace)] {
				if node.DeploymentName == nodeTypeInterface.name() {
					dataNodes = append(dataNodes, nodeTypeInterface)
				}

				if node.StatefulSetName == nodeTypeInterface.name() {
					redeployCertNodes = append(redeployCertNodes, nodeTypeInterface)
				}
			}
		}
	}

	redeployCertNodes = append(redeployCertNodes, dataNodes...)

	return redeployCertNodes
}

func addNodeState(node NodeTypeInterface, nodeStatus *api.ElasticsearchNodeStatus) {

	nodeState := node.state()

	nodeStatus.UpgradeStatus.ScheduledForUpgrade = nodeState.UpgradeStatus.ScheduledForUpgrade
	nodeStatus.UpgradeStatus.ScheduledForRedeploy = nodeState.UpgradeStatus.ScheduledForRedeploy
	nodeStatus.UpgradeStatus.ScheduledForCertRedeploy = nodeState.UpgradeStatus.ScheduledForCertRedeploy
	nodeStatus.DeploymentName = nodeState.DeploymentName
	nodeStatus.StatefulSetName = nodeState.StatefulSetName
}

func (elasticsearchRequest *ElasticsearchRequest) setNodeStatus(node NodeTypeInterface, nodeStatus *api.ElasticsearchNodeStatus, clusterStatus *api.ElasticsearchStatus) error {

	index, _ := getNodeStatus(node.name(), clusterStatus)

	if index == NOT_FOUND_INDEX {
		clusterStatus.Nodes = append(clusterStatus.Nodes, *nodeStatus)
	} else {
		clusterStatus.Nodes[index] = *nodeStatus
	}

	return elasticsearchRequest.updateNodeStatus(*clusterStatus)
}

func (elasticsearchRequest *ElasticsearchRequest) updateNodeStatus(status api.ElasticsearchStatus) error {

	cluster := elasticsearchRequest.cluster
	// if there is nothing to update, don't
	if reflect.DeepEqual(cluster.Status, status) {
		return nil
	}

	nretries := -1
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		nretries++
		if getErr := elasticsearchRequest.client.Get(context.TODO(), types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster); getErr != nil {
			logrus.Debugf("Could not get Elasticsearch %v: %v", cluster.Name, getErr)
			return getErr
		}

		cluster.Status = status

		if updateErr := elasticsearchRequest.client.Update(context.TODO(), cluster); updateErr != nil {
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

// this function should be called before we try doing operations to make sure all our nodes are
// first ready
func (elasticsearchRequest *ElasticsearchRequest) ClusterReady() bool {

	// bypass using Status -- check all our pods first, make sure they're all 'Ready'
	podStates := elasticsearchRequest.GetCurrentPodStateMap()

	for _, stateMap := range podStates {
		if len(stateMap[api.PodStateTypeNotReady]) > 0 || len(stateMap[api.PodStateTypeFailed]) > 0 {
			return false
		}
	}

	return true
}

// this function should be called before we try doing operations to make sure any of our nodes
// are first ready
func (elasticsearchRequest *ElasticsearchRequest) AnyNodeReady() bool {

	podStates := elasticsearchRequest.GetCurrentPodStateMap()

	for _, stateMap := range podStates {
		if len(stateMap[api.PodStateTypeReady]) > 0 {
			return true
		}
	}

	return false
}
