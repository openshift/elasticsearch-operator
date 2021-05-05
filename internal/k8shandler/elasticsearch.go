package k8shandler

import (
	"github.com/ViaQ/logerr/kverrors"
	api "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	estypes "github.com/openshift/elasticsearch-operator/internal/types/elasticsearch"
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
	if !er.AnyNodeReady() {
		return
	}

	currentMasterCount, err := er.esClient.GetMinMasterNodes()
	if err != nil {
		er.L().Info("Unable to get current min master count")
	}

	desiredMasterCount := getMasterCount(er.cluster)/2 + 1
	currentNodeCount, err := er.esClient.GetClusterNodeCount()
	if err != nil {
		er.L().Error(err, "Unable to get cluster node count")
	}

	// check that we have the required number of master nodes in the cluster...
	if currentNodeCount >= desiredMasterCount {
		if currentMasterCount != desiredMasterCount {
			if _, setErr := er.esClient.SetMinMasterNodes(desiredMasterCount); setErr != nil {
				er.L().Info("Unable to set min master count", "count", desiredMasterCount)
			}
		}
	}
}

func (er *ElasticsearchRequest) tryEnsureAllShardAllocation() {
	if !er.AnyNodeReady() {
		return
	}
	if ok, err := er.esClient.SetShardAllocation(api.ShardAllocationAll); !ok {
		er.L().Error(err, "Unable to enable shard allocation")
	}
}

func (er *ElasticsearchRequest) tryEnsureNoTransitiveShardAllocations() {
	if !er.AnyNodeReady() {
		return
	}
	if success, err := er.esClient.ClearTransientShardAllocation(); !success {
		er.L().Error(err, "Unable to clear transient shard allocation")
	}
}

func (er *ElasticsearchRequest) updateReplicas() {
	if er.ClusterReady() {
		replicaCount := int32(calculateReplicaCount(er.cluster))
		if err := er.esClient.UpdateReplicaCount(replicaCount); err != nil {
			er.L().Error(err, "Unable to update replica count")
		}
	}
}

func (er *ElasticsearchRequest) updatePrimaryShards() {
	if er.ClusterReady() {
		primaryCount := int32(calculatePrimaryCount(er.cluster))
		if err := er.esClient.UpdateTemplatePrimaryShards(primaryCount); err != nil {
			er.L().Error(err, "Unable to update primary count")
		}
	}
}

func (er *ElasticsearchRequest) isIndexBlocked(index string) bool {
	currSetting, err := er.esClient.GetIndexSettings(index)
	if err != nil {
		er.L().Error(err, "failed to get index settings",
			"index", index)
		return false
	}

	if currSetting != nil {
		if currSetting.Settings.Index != nil {
			if currSetting.Settings.Index.Blocks != nil {
				if currSetting.Settings.Index.Blocks.ReadOnlyAllowDelete != nil {
					if *currSetting.Settings.Index.Blocks.ReadOnlyAllowDelete == "true" {
						return true
					}
				}
			}
		}
	}
	return false
}

func (er *ElasticsearchRequest) unblockIndex(index string) error {
	settings := &estypes.IndexSettings{
		Index: &estypes.IndexingSettings{
			Blocks: &estypes.IndexBlocksSettings{
				ReadOnlyAllowDelete: nil,
			},
		},
	}

	if err := er.esClient.UpdateIndexSettings(index, settings); err != nil {
		return kverrors.Wrap(err, "failed to unblock index",
			"index", index)
	}

	return nil
}
