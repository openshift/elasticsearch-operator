package k8shandler

import (
	"context"
	"fmt"

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
func (er *ElasticsearchRequest) CreateOrUpdateElasticsearchCluster() error {
	esClient := er.esClient

	// Verify that we didn't scale up too many masters
	err := er.isValidConf()
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

	er.populateNodes()

	//clearing transient setting because of a bug in earlier releases which
	//may leave the shard allocation in an undesirable state
	er.tryEnsureNoTransitiveShardAllocations()

	if err := er.progressUnschedulableNodes(); err != nil {
		logrus.Warnf("unable to progress unschedulable nodes for %v in namespace %v", er.cluster.Name, er.cluster.Namespace)
		return er.UpdateClusterStatus()
	}

	certRestartNodes := er.getScheduledCertRedeployNodes()
	stillRecovering := containsClusterCondition(api.Recovering, v1.ConditionTrue, &er.cluster.Status)
	if len(certRestartNodes) > 0 || stillRecovering {
		if err := er.PerformFullClusterCertRestart(certRestartNodes); err != nil {
			logrus.Warnf("unable to complete full cluster restart for %v in namespace %v", er.cluster.Name, er.cluster.Namespace)
			return er.UpdateClusterStatus()
		}

		er.UpdateClusterStatus()
	}

	// if there is a node currently being upgraded, work on that first
	inProgressNode := er.getNodeUpgradeInProgress()
	scheduledNodes := er.getScheduledUpgradeNodes()

	// Check if we have a node that was in the progress -- if so, continue updating it
	if inProgressNode != nil {
		// Check to see if the inProgressNode was being updated or restarted
		if _, ok := containsNodeTypeInterface(inProgressNode, scheduledNodes); ok {
			logrus.Debugf("Continuing update for %v", inProgressNode.name())
			if err := er.PerformNodeUpdate(inProgressNode); err != nil {
				logrus.Warnf("unable to update node. E: %s", err.Error())
				return er.UpdateClusterStatus()
			}

			// update scheduled nodes since we were able to complete upgrade for inProgressNode
			scheduledNodes = er.getScheduledUpgradeNodes()
		} else {
			logrus.Debugf("Continuing restart for %v", inProgressNode.name())
			if err := er.PerformNodeRestart(inProgressNode); err != nil {
				logrus.Warnf("unable to restart node %q: %s", inProgressNode.name(), err.Error())
				return er.UpdateClusterStatus()
			}
		}

		er.UpdateClusterStatus()
	}

	// We didn't have any in progress, but we have ones scheduled to be updated
	if len(scheduledNodes) > 0 {

		// get the current ES version
		version, err := esClient.GetLowestClusterVersion()
		if err != nil {
			// this can be because we couldn't get a valid response from ES
			logrus.Warnf("when trying to get LowestClusterVersion: %s", err.Error())
			return er.UpdateClusterStatus()
		}

		logrus.Debugf("Found current cluster version to be %q", version)
		comparison := comparators.CompareVersions(version, expectedMinVersion)

		// if it is < what we expect (6.0) then do full cluster update:
		if comparison > 0 {
			// perform a full cluster update
			if err := er.PerformFullClusterUpdate(scheduledNodes); err != nil {
				logrus.Warnf("when trying to perform full cluster update: %s", err.Error())
				return er.UpdateClusterStatus()
			}

		} else {

			if err := er.PerformRollingUpdate(scheduledNodes); err != nil {
				logrus.Warnf("when trying to perform rolling update: %s", err.Error())
				return er.UpdateClusterStatus()
			}
		}

		er.UpdateClusterStatus()
	}

	if er.getNodeUpgradeInProgress() == nil {
		// We have no updates or restarts in progress
		// create any nodes we are missing and perform any required operations to ensure state
		for _, node := range nodes[nodeMapKey(er.cluster.Name, er.cluster.Namespace)] {
			clusterStatus := er.cluster.Status.DeepCopy()
			_, nodeStatus := getNodeStatus(node.name(), clusterStatus)

			if err := node.create(); err != nil {
				return err
			}

			addNodeState(node, nodeStatus)
			if err := er.setNodeStatus(node, nodeStatus, clusterStatus); err != nil {
				logrus.Warnf("unable to set status for node %q: %s", node.name(), err.Error())
			}
		}

		// ensure that MinMasters is (n / 2 + 1)
		er.updateMinMasters()

		// ensure we always have shard allocation to All if we aren't doing an update...
		er.tryEnsureAllShardAllocation()

		// we only want to update our replicas if we aren't in the middle up an update
		er.updateReplicas()

		// add alias to old indices if they exist and don't have one
		// this should be removed after one release...
		if er.ClusterReady() {
			if aliasNeededMap == nil {
				aliasNeededMap = make(map[string]bool)
			}
			if val, ok := aliasNeededMap[nodeMapKey(er.cluster.Name, er.cluster.Namespace)]; !ok || val {
				successful := esClient.AddAliasForOldIndices()

				if successful {
					aliasNeededMap[nodeMapKey(er.cluster.Name, er.cluster.Namespace)] = false
				}
			}
		}
	}

	// Scrape cluster health from elasticsearch every time
	return er.UpdateClusterStatus()
}

func (er *ElasticsearchRequest) getNodeUpgradeInProgress() NodeTypeInterface {
	cluster := er.cluster

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

func (er *ElasticsearchRequest) progressUnschedulableNodes() error {
	cluster := er.cluster

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
						return err
					}
				}
			}
		}
	}

	return nil
}

func (er *ElasticsearchRequest) setUUIDs() {
	cluster := er.cluster

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
				if getErr := er.client.Get(context.TODO(), types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster); getErr != nil {
					logrus.Debugf("Could not get Elasticsearch %v: %v", cluster.Name, getErr)
					return getErr
				}

				if cluster.Spec.Nodes[index].GenUUID != nil {
					return nil
				}

				cluster.Spec.Nodes[index].GenUUID = &uuid

				if updateErr := er.client.Update(context.TODO(), cluster); updateErr != nil {
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

func (er *ElasticsearchRequest) populateNodes() {
	er.setUUIDs()

	if nodes == nil {
		nodes = make(map[string][]NodeTypeInterface)
	}

	cluster := er.cluster
	currentNodes := []NodeTypeInterface{}

	// get list of client only nodes, and collapse node info into the node (self field) if needed
	for _, node := range cluster.Spec.Nodes {
		// build the NodeTypeInterface list
		for _, nodeTypeInterface := range er.GetNodeTypeInterface(*node.GenUUID, node) {

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
				if er.AnyNodeReady() {
					er.updateMinMasters()
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

func (er *ElasticsearchRequest) getScheduledUpgradeNodes() []NodeTypeInterface {
	cluster := er.cluster
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

func (er *ElasticsearchRequest) getScheduledCertRedeployNodes() []NodeTypeInterface {
	cluster := er.cluster
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
	nodeStatus.UpgradeStatus.ScheduledForCertRedeploy = nodeState.UpgradeStatus.ScheduledForCertRedeploy
	nodeStatus.DeploymentName = nodeState.DeploymentName
	nodeStatus.StatefulSetName = nodeState.StatefulSetName
}
