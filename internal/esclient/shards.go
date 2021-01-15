package elasticsearch

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/ViaQ/logerr/log"
	api "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/tidwall/gjson"
)

func (ec *esClient) ClearTransientShardAllocation() (bool, error) {

	es := ec.eoclient
	settings := fmt.Sprintf("{%q:{%q:null}}", "transient", "cluster.routing.allocation.enable")
	body := ioutil.NopCloser(bytes.NewBufferString(settings))
	res, err := es.Cluster.PutSettings(body, es.Cluster.PutSettings.WithPretty())

	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	if res.IsError() || res.StatusCode != http.StatusOK {
		return false, ec.errorCtx().New("failed to set shard allocation",
			"response_error", res.String,
			"response_status", res.StatusCode,
			"response_body", res.Body)
	}
	acknowledged := false

	resBody, _ := ioutil.ReadAll(res.Body)
	jsonStr := string(resBody)
	acknowledged = gjson.Get(jsonStr, "acknowledged").Bool()

	if !acknowledged {
		log.Error(nil, "failed to clear shard allocation", "cluster", ec.cluster, "namespace", ec.namespace)
	}
	return true, nil
}

func (ec *esClient) SetShardAllocation(state api.ShardAllocationState) (bool, error) {
	es := ec.eoclient
	settings := fmt.Sprintf("{%q:{%q:%q}}", "persistent", "cluster.routing.allocation.enable", state)
	body := ioutil.NopCloser(bytes.NewBufferString(settings))
	res, err := es.Cluster.PutSettings(body, es.Cluster.PutSettings.WithPretty())

	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	if res.IsError() || res.StatusCode != http.StatusOK {
		return false, ec.errorCtx().New("failed to set shard allocation",
			"response_error", res.String,
			"response_status", res.StatusCode,
			"response_body", res.Body)
	}
	acknowledged := false

	resBody, _ := ioutil.ReadAll(res.Body)
	jsonStr := string(resBody)
	acknowledged = gjson.Get(jsonStr, "acknowledged").Bool()

	if !acknowledged {
		log.Error(nil, "failed to set shard allocation", "cluster", ec.cluster, "namespace", ec.namespace)
	}
	return true, nil
}

func (ec *esClient) GetShardAllocation() (string, error) {

	es := ec.eoclient
	res, err := es.Cluster.GetSettings(es.Cluster.GetSettings.WithIncludeDefaults(true), es.Cluster.GetSettings.WithPretty())
	allocationString := ""

	if err != nil {
		return allocationString, err
	}
	defer res.Body.Close()

	if res.IsError() || res.StatusCode != http.StatusOK {
		return allocationString, ec.errorCtx().New("failed to get shard allocation",
			"response_error", res.String,
			"response_status", res.StatusCode,
			"response_body", res.Body)
	}

	body, _ := ioutil.ReadAll(res.Body)
	jsonStr := string(body)

	var allocation interface{}
	if value := gjson.Get(jsonStr, "defaults.cluster.routing.allocation.enable"); value.Type != gjson.Null {
		allocation = value
	}

	if value := gjson.Get(jsonStr, "persistent.cluster.routing.allocation.enable"); value.Type != gjson.Null {
		allocation = value
	}

	if value := gjson.Get(jsonStr, "transient.cluster.routing.allocation.enable"); value.Type != gjson.Null {
		allocation = value
	}

	allocationString, ok := allocation.(string)
	if !ok {
		allocationString = ""
	}

	return allocationString, nil
}
