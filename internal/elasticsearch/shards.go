package elasticsearch

import (
	"fmt"
	"net/http"

	api "github.com/openshift/elasticsearch-operator/apis/logging/v1"
)

func (ec *esClient) ClearTransientShardAllocation() (bool, error) {

	es := ec.client
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

	ec.fnSendEsRequest(ec.cluster, ec.namespace, payload, ec.k8sClient)

	acknowledged := false
	if acknowledgedBool, ok := payload.ResponseBody["acknowledged"].(bool); ok {
		acknowledged = acknowledgedBool
	}
	return payload.StatusCode == 200 && acknowledged, ec.errorCtx().Wrap(payload.Error, "failed to clear shard allocation",
		"response", payload.RawResponseBody)
}

func (ec *esClient) SetShardAllocation(state api.ShardAllocationState) (bool, error) {
	es := ec.client
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

	ec.fnSendEsRequest(ec.cluster, ec.namespace, payload, ec.k8sClient)

	acknowledged := false
	if acknowledgedBool, ok := payload.ResponseBody["acknowledged"].(bool); ok {
		acknowledged = acknowledgedBool
	}
	return payload.StatusCode == 200 && acknowledged, ec.errorCtx().Wrap(payload.Error, "failed to set shard allocation")
}

func (ec *esClient) GetShardAllocation() (string, error) {

	es := ec.client
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

	ec.fnSendEsRequest(ec.cluster, ec.namespace, payload, ec.k8sClient)

	var allocation interface{}

	if value := walkInterfaceMap(
		"defaults.cluster.routing.allocation.enable",
		payload.ResponseBody); value != nil {
		allocation = value
	}

	if value := walkInterfaceMap(
		"persistent.cluster.routing.allocation.enable",
		payload.ResponseBody); value != nil {
		allocation = value
	}

	if value := walkInterfaceMap(
		"transient.cluster.routing.allocation.enable",
		payload.ResponseBody); value != nil {
		allocation = value
	}

	allocationString, ok := allocation.(string)
	if !ok {
		allocationString = ""
	}

	return allocationString, payload.Error
}
