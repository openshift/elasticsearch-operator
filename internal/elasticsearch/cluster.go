package elasticsearch

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/openshift/elasticsearch-operator/internal/utils/comparators"
	"github.com/tidwall/gjson"
)

func (ec *esClient) GetClusterNodeVersions() ([]string, error) {
	es := ec.client

	res, err := es.Cluster.Stats(es.Cluster.Stats.WithPretty())

	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var nodeVersions []string

	if res.IsError() || res.StatusCode != http.StatusOK {
		resBody, _ := ioutil.ReadAll(res.Body)
		errorMsg := string(resBody)
		return nil, ec.errorCtx().New("Failed to Get Cluster Node Version",
			"response_error", res.String(),
			"response_status", res.StatusCode,
			"response_body", errorMsg)

	} else {
		body, _ := ioutil.ReadAll(res.Body)
		jsonStr := string(body)
		log.Printf(jsonStr)
		results := gjson.Get(jsonStr, "nodes.versions")
		for _, name := range results.Array() {
			nodeVersions = append(nodeVersions, name.String())
		}

		if err != nil {
			return nil, fmt.Errorf("ERROR: %s", err)
		}
	}
	return nodeVersions, nil
}

func (ec *esClient) GetThresholdEnabled() (bool, error) {

	es := ec.client
	res, err := es.Cluster.GetSettings(es.Cluster.GetSettings.WithPretty())

	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	if res.IsError() || res.StatusCode != http.StatusOK {
		resBody, _ := ioutil.ReadAll(res.Body)
		errorMsg := string(resBody)
		return false, ec.errorCtx().New("Failed to Get Threshold Enabled",
			"response_error", res.String(),
			"response_status", res.StatusCode,
			"response_body", errorMsg)

	}

	var enabled gjson.Result
	body, _ := ioutil.ReadAll(res.Body)
	jsonStr := string(body)

	if value := gjson.Get(jsonStr, "defaults.cluster.routing.allocation.disk.threshold_enabled"); value.Type != gjson.Null {
		enabled = value
	}

	if value := gjson.Get(jsonStr, "persistent.cluster.routing.allocation.disk.threshold_enabled"); value.Type != gjson.Null {
		enabled = value
	}

	if value := gjson.Get(jsonStr, "transient.cluster.routing.allocation.disk.threshold_enabled"); value.Type != gjson.Null {
		enabled = value
	}
	if enabled.Type == gjson.Null || enabled.Type == gjson.False {
		return false, nil
	}

	return enabled.Bool(), nil
}

func (ec *esClient) GetDiskWatermarks() (interface{}, interface{}, error) {

	var low interface{}
	var high interface{}
	es := ec.client
	res, err := es.Cluster.GetSettings(es.Cluster.GetSettings.WithPretty())

	if err != nil {
		return low, high, err
	}
	defer res.Body.Close()

	if res.IsError() || res.StatusCode != http.StatusOK {
		resBody, _ := ioutil.ReadAll(res.Body)
		errorMsg := string(resBody)
		return low, high, ec.errorCtx().New("Failed to Get Disk Water Marks",
			"response_error", res.String(),
			"response_status", res.StatusCode,
			"response_body", errorMsg)

	} else {
		body, _ := ioutil.ReadAll(res.Body)
		jsonStr := string(body)

		if value := gjson.Get(jsonStr,
			"defaults.cluster.routing.allocation.disk.watermark.low"); value.Type != gjson.Null {
			low = value.Str
		}

		if value := gjson.Get(jsonStr,
			"defaults.cluster.routing.allocation.disk.watermark.high"); value.Type != gjson.Null {
			high = value.Str
		}

		if value := gjson.Get(jsonStr,
			"persistent.cluster.routing.allocation.disk.watermark.low"); value.Type != gjson.Null {
			low = value.Str
		}

		if value := gjson.Get(jsonStr,
			"persistent.cluster.routing.allocation.disk.watermark.high"); value.Type != gjson.Null {
			high = value.Str
		}

		if value := gjson.Get(jsonStr,
			"transient.cluster.routing.allocation.disk.watermark.low"); value.Type != gjson.Null {
			low = value.Str
		}

		if value := gjson.Get(jsonStr,
			"transient.cluster.routing.allocation.disk.watermark.high"); value.Type != gjson.Null {
			high = value.Str
		}

	}

	if lowString, ok := low.(string); ok {
		if strings.HasSuffix(lowString, "%") {
			low, _ = strconv.ParseFloat(strings.TrimSuffix(lowString, "%"), 64)
		} else {
			if strings.HasSuffix(lowString, "b") {
				low = strings.TrimSuffix(lowString, "b")
			}
		}
	}

	if highString, ok := high.(string); ok {
		if strings.HasSuffix(highString, "%") {
			high, _ = strconv.ParseFloat(strings.TrimSuffix(highString, "%"), 64)
		} else {
			if strings.HasSuffix(highString, "b") {
				high = strings.TrimSuffix(highString, "b")
			}
		}
	}

	return low, high, err
}

func (ec *esClient) GetMinMasterNodes() (int32, error) {
	es := ec.client
	res, err := es.Cluster.GetSettings(es.Cluster.GetSettings.WithPretty())
	masterCount := int32(0)

	if err != nil {
		return masterCount, err
	}
	defer res.Body.Close()

	if res.IsError() || res.StatusCode != http.StatusOK {
		resBody, _ := ioutil.ReadAll(res.Body)
		errorMsg := string(resBody)
		return masterCount, ec.errorCtx().New("Failed to Get Min Master Nodes",
			"response_error", res.String(),
			"response_status", res.StatusCode,
			"response_body", errorMsg)

	}

	body, _ := ioutil.ReadAll(res.Body)
	jsonStr := string(body)

	if value := gjson.Get(jsonStr,
		"persistent.discovery.zen.minimum_master_nodes"); value.Type != gjson.Null {
		masterCount = int32(value.Int())

	}

	return masterCount, nil
}

func (ec *esClient) SetMinMasterNodes(numberMasters int32) (bool, error) {

	es := ec.client
	requestBody := fmt.Sprintf("{%q:{%q:%d}}", "persistent", "discovery.zen.minimum_master_nodes", numberMasters)

	body := ioutil.NopCloser(bytes.NewReader([]byte(requestBody)))
	res, err := es.Cluster.PutSettings(body, es.Cluster.PutSettings.WithPretty())

	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	if res.IsError() || res.StatusCode != http.StatusOK {
		resBody, _ := ioutil.ReadAll(res.Body)
		errorMsg := string(resBody)
		return false, ec.errorCtx().New("Failed to Set Min Master Nodes",
			"response_error", res.String(),
			"response_status", res.StatusCode,
			"response_body", errorMsg)

	}

	acknowledged := false
	resBody, _ := ioutil.ReadAll(res.Body)
	jsonStr := string(resBody)

	if value := gjson.Get(jsonStr,
		"acknowledged"); value.Type != gjson.Null {
		acknowledged, _ = strconv.ParseBool(value.Raw)

	}
	return acknowledged, err

}

func (ec *esClient) GetLowestClusterVersion() (string, error) {

	es := ec.client
	res, err := es.Cluster.Stats(es.Cluster.Stats.WithPretty())

	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.IsError() || res.StatusCode != http.StatusOK {
		resBody, _ := ioutil.ReadAll(res.Body)
		errorMsg := string(resBody)
		return "", ec.errorCtx().New("Failed to Get Threshold Enabled",
			"response_error", res.String(),
			"response_status", res.StatusCode,
			"response_body", errorMsg)

	}
	body, _ := ioutil.ReadAll(res.Body)
	jsonStr := string(body)
	results := gjson.Get(jsonStr, "nodes.versions")

	if len(results.Array()) == 1 {
		return results.Array()[0].String(), nil
	}

	lowestVersion := results.Array()[0].String()
	for _, version := range results.Array() {
		comparison := comparators.CompareVersions(lowestVersion, version.String())

		if comparison < 0 {
			lowestVersion = version.String()
		}
	}

	return lowestVersion, nil
}

func (ec *esClient) IsNodeInCluster(nodeName string) (bool, error) {
	es := ec.client
	res, err := es.Nodes.Info(es.Nodes.Info.WithPretty())

	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	if res.IsError() || res.StatusCode != http.StatusOK {
		resBody, _ := ioutil.ReadAll(res.Body)
		errorMsg := string(resBody)
		return false, ec.errorCtx().New("Failed to Get Threshold Enabled",
			"response_error", res.String(),
			"response_status", res.StatusCode,
			"response_body", errorMsg)

	}
	body, _ := ioutil.ReadAll(res.Body)
	jsonStr := string(body)

	results := gjson.Get(jsonStr, "nodes.*.name*")

	for _, name := range results.Array() {
		if name.String() == nodeName {
			return true, nil
		}
	}

	return false, nil
}

func (ec *esClient) DoSynchronizedFlush() (bool, error) {

	es := ec.client
	res, err := es.Indices.FlushSynced(es.Indices.FlushSynced.WithPretty())

	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	if res.IsError() || res.StatusCode != http.StatusOK {
		resBody, _ := ioutil.ReadAll(res.Body)
		errorMsg := string(resBody)
		return false, ec.errorCtx().New("Failed to Get Threshold Enabled",
			"response_error", res.String(),
			"response_status", res.StatusCode,
			"response_body", errorMsg)

	}

	body, _ := ioutil.ReadAll(res.Body)
	jsonStr := string(body)

	if value := gjson.Get(jsonStr,
		"_shards.failed"); value.Type != gjson.Null {
		failed := int32(value.Int())

		if failed != 0 {
			return false, nil
		}
	}

	return true, nil
}
