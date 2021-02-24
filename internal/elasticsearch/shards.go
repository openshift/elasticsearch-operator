package elasticsearch

import (
	api "github.com/openshift/elasticsearch-operator/apis/logging/v1"
)

func (ec *esClient) ClearTransientShardAllocation() (bool, error) {
	return true, nil
}

func (ec *esClient) SetShardAllocation(state api.ShardAllocationState) (bool, error) {
	return true, nil
}

func (ec *esClient) GetShardAllocation() (string, error) {

	allocationString := ""

	return allocationString, nil
}
