package k8shandler

import (
	"context"
	"fmt"

	"github.com/openshift/elasticsearch-operator/pkg/log"
	"github.com/openshift/elasticsearch-operator/pkg/utils"
	"github.com/openshift/elasticsearch-operator/pkg/utils/comparators"

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
	ll := log.WithValues("cluster", er.cluster.Name, "namespace", er.cluster.Namespace)
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

	// Populate nodes from the custom resources spec.nodes
	if err := er.populateNodes(); err != nil {
		return err
	}

	//clearing transient setting because of a bug in earlier releases which
	//may leave the shard allocation in an undesirable state
	er.tryEnsureNoTransitiveShardAllocations()

	// Update the cluster status immediately to refresh status.nodes
	// before progressing with any unschedulable nodes.
	// Ensures that deleted nodes are removed from status.nodes.
	if err := er.UpdateClusterStatus(); err != nil {
		return err
	}
	if err := er.progressUnschedulableNodes(); err != nil {
		ll.Error(err, "unable to progress unschedulable nodes")
		return er.UpdateClusterStatus()
	}

	certRestartNodes := er.getScheduledCertRedeployNodes()
	stillRecovering := containsClusterCondition(api.Recovering, v1.ConditionTrue, &er.cluster.Status)
	if len(certRestartNodes) > 0 || stillRecovering {
		if err := er.PerformFullClusterCertRestart(certRestartNodes); err != nil {
			ll.Error(err, "unable to complete full cluster restart")
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
			if err := er.PerformNodeUpdate(inProgressNode); err != nil {
				ll.Error(err, "unable to update node")
				return er.UpdateClusterStatus()
			}

			// update scheduled nodes since we were able to complete upgrade for inProgressNode
			scheduledNodes = er.getScheduledUpgradeNodes()
		} else {
			if err := er.PerformNodeRestart(inProgressNode); err != nil {
				ll.Error(err, "unable to restart node", "node", inProgressNode.name())
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
			ll.Error(err, "failed to get LowestClusterVersion")
			return er.UpdateClusterStatus()
		}

		comparison := comparators.CompareVersions(version, expectedMinVersion)

		// if it is < what we expect (6.0) then do full cluster update:
		if comparison > 0 {
			// perform a full cluster update
			if err := er.PerformFullClusterUpdate(scheduledNodes); err != nil {
				log.Error(err, "failed to perform full cluster update")
				return er.UpdateClusterStatus()
			}

		} else {

			if err := er.PerformRollingUpdate(scheduledNodes); err != nil {
				log.Error(err, "failed to perform rolling update")
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
				log.Error(err, "unable to set status for node", "node", node.name())
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
	clusterNodes := nodes[nodeMapKey(cluster.GetName(), cluster.GetNamespace())]

	for _, nodeStatus := range cluster.Status.Nodes {
		if isPodUnschedulableConditionTrue(nodeStatus.Conditions) ||
			isPodImagePullBackOff(nodeStatus.Conditions) ||
			isPodCrashLoopBackOff(nodeStatus.Conditions) {

			for _, node := range clusterNodes {
				if nodeStatus.DeploymentName == node.name() || nodeStatus.StatefulSetName == node.name() {
					if node.isMissing() {
						log.Info("Unschedulable node does not have k8s resource, skipping", "node", node.name())
						continue
					}

					if err := node.progressNodeChanges(); err != nil {
						log.Error(err, "Failed to progress update of unschedulable node", "node", node.name())
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

			er.setUUID(index, uuid)
		}
	}
}

func (er *ElasticsearchRequest) setUUID(index int, uuid string) {

	ll := log.WithValues("cluster", er.cluster.Name, "namespace", er.cluster.Namespace)

	nretries := -1
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		nretries++
		if err := er.client.Get(context.TODO(), types.NamespacedName{Name: er.cluster.Name, Namespace: er.cluster.Namespace}, er.cluster); err != nil {
			// FIXME: return structured error
			ll.Info("Could not get Elasticsearch cluster", "error", err)
			return err
		}

		if er.cluster.Spec.Nodes[index].GenUUID != nil {
			return nil
		}

		er.cluster.Spec.Nodes[index].GenUUID = &uuid

		if updateErr := er.client.Update(context.TODO(), er.cluster); updateErr != nil {
			// FIXME: return structured error
			ll.Info("Failed to update Elasticsearch status. Trying again...", "error", updateErr)
			return updateErr
		}
		return nil
	})

	if err != nil {
		ll.Error(err, "Could not update CR for Elasticsearch", "retries", nretries)
	} else {
		ll.Info("Updated Elasticsearch", "retries", nretries)
	}
}

func (er *ElasticsearchRequest) populateNodes() error {
	if err := er.recoverOrphanedCluster(); err != nil {
		return err
	}
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
				log.Error(err, "unable to delete node")
			}

			// remove from status.Nodes
			if index, _ := getNodeStatus(node.name(), &cluster.Status); index != NOT_FOUND_INDEX {
				cluster.Status.Nodes = append(cluster.Status.Nodes[:index], cluster.Status.Nodes[index+1:]...)
			}
		}
	}

	nodes[nodeMapKey(cluster.Name, cluster.Namespace)] = currentNodes

	return nil
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
