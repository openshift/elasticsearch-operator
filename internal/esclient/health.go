package elasticsearch

import (
	"io/ioutil"
	"log"

	api "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	"github.com/tidwall/gjson"
)

func (ec *esClient) GetClusterHealth() (api.ClusterHealth, error) {
	clusterHealth := api.ClusterHealth{}

	es := ec.eoclient

	res, err := es.Cluster.Health(es.Cluster.Health.WithPretty())

	if err != nil {
		return clusterHealth, err

	}
	defer res.Body.Close()
	if res.IsError() {
		log.Printf("ERROR: %s: %s", res.Status(), res)
	} else {
		body, _ := ioutil.ReadAll(res.Body)
		jsonStr := string(body)
		log.Printf(jsonStr)
		clusterHealth.Status = gjson.Get(jsonStr, "status").Str
		clusterHealth.NumNodes = int32(gjson.Get(jsonStr, "number_of_nodes").Int())
		clusterHealth.NumDataNodes = int32(gjson.Get(jsonStr, "number_of_data_nodes").Int())
		clusterHealth.ActivePrimaryShards = int32(gjson.Get(jsonStr, "active_primary_shards").Int())
		clusterHealth.ActiveShards = int32(gjson.Get(jsonStr, "active_shards").Int())
		clusterHealth.RelocatingShards = int32(gjson.Get(jsonStr, "relocating_shards").Int())
		clusterHealth.InitializingShards = int32(gjson.Get(jsonStr, "initializing_shards").Int())
		clusterHealth.UnassignedShards = int32(gjson.Get(jsonStr, "unassigned_shards").Int())
		clusterHealth.PendingTasks = int32(gjson.Get(jsonStr, "number_of_pending_tasks").Int())
	}
	return clusterHealth, nil
}

func (ec *esClient) GetClusterHealthStatus() (string, error) {
	es := ec.eoclient

	res, err := es.Cluster.Health(es.Cluster.Health.WithPretty())
	status := ""
	if err != nil {
		return status, err

	}
	defer res.Body.Close()

	if res.IsError() {
		log.Printf("ERROR: %s: %s", res.Status(), res)
	} else {
		body, _ := ioutil.ReadAll(res.Body)
		jsonStr := string(body)
		status = gjson.Get(jsonStr, "status").Str
	}
	return status, err
}

func (ec *esClient) GetClusterNodeCount() (int32, error) {
	es := ec.eoclient

	res, err := es.Cluster.Health(es.Cluster.Health.WithPretty())
	nodeCount := int32(0)
	if err != nil {
		return nodeCount, err

	}
	defer res.Body.Close()

	if res.IsError() {
		log.Printf("ERROR: %s: %s", res.Status(), res)
	} else {
		body, _ := ioutil.ReadAll(res.Body)
		jsonStr := string(body)
		log.Printf(jsonStr)
		nodeCount = int32(gjson.Get(jsonStr, "number_of_nodes").Int())
	}
	return nodeCount, err
}
