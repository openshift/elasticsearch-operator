package k8shandler

import (
	api "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	"github.com/sirupsen/logrus"
)

// this function should be called before we try doing operations to make sure all our nodes are
// first ready
func (er *ElasticsearchRequest) ClusterReady() bool {
	// bypass using Status -- check all our pods first, make sure they're all 'Ready'
	podStates := er.GetCurrentPodStateMap()

	if len(podStates) == 0 {
		// we have no pods in the cluster -- it can't be ready
		return false
	}

	for _, stateMap := range podStates {
		if len(stateMap[api.PodStateTypeNotReady]) > 0 || len(stateMap[api.PodStateTypeFailed]) > 0 {
			return false
		}
	}

	return true
}

// this function should be called before we try doing operations to make sure any of our nodes
// are first ready
func (er *ElasticsearchRequest) AnyNodeReady() bool {
	podStates := er.GetCurrentPodStateMap()

	for _, stateMap := range podStates {
		if len(stateMap[api.PodStateTypeReady]) > 0 {
			return true
		}
	}

	return false
}

func (er *ElasticsearchRequest) updateMinMasters() {
	// do as best effort -- whenever we create a node update min masters (if required)
	if er.AnyNodeReady() {
		cluster := er.cluster
		esClient := er.esClient

		currentMasterCount, err := esClient.GetMinMasterNodes()
		if err != nil {
			logrus.Debugf("Unable to get current min master count for cluster %q in namespace %q", cluster.Name, cluster.Namespace)
		}

		desiredMasterCount := getMasterCount(cluster)/2 + 1
		currentNodeCount, err := esClient.GetClusterNodeCount()
		if err != nil {
			logrus.Warnf("Unable to get cluster node count for cluster %q in namespace %q: %s", cluster.Name, cluster.Namespace, err.Error())
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
}

func (er *ElasticsearchRequest) tryEnsureAllShardAllocation() {
	if er.AnyNodeReady() {
		if ok, err := er.esClient.SetShardAllocation(api.ShardAllocationAll); !ok {
			logrus.Warnf("Unable to enable shard allocation for cluster %q in namespace %q: %v", er.cluster.Name, er.cluster.Namespace, err)
		}
	}
}

func (er *ElasticsearchRequest) tryEnsureNoTransitiveShardAllocations() {
	if er.AnyNodeReady() {
		if success, err := er.esClient.ClearTransientShardAllocation(); !success {
			logrus.Warnf("Unable to clear transient shard allocation for cluster %q in namespace %q: %s", er.cluster.Namespace, er.cluster.Namespace, err.Error())
		}
	}
}

func (er *ElasticsearchRequest) updateReplicas() {
	if er.ClusterReady() {
		replicaCount := int32(calculateReplicaCount(er.cluster))
		if err := er.esClient.UpdateReplicaCount(replicaCount); err != nil {
			logrus.Warnf("Unable to update replica count for cluster %q in namespace %q: %v", er.cluster.Namespace, er.cluster.Namespace, err)
		}
	}
}
