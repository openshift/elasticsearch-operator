package elasticsearch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/openshift/elasticsearch-operator/pkg/log"
)

// This will idempotently update the index templates and update indices' replica count
func (ec *esClient) UpdateReplicaCount(replicaCount int32) error {
	if ok, _ := ec.updateAllIndexTemplateReplicas(replicaCount); ok {
		if _, err := ec.updateAllIndexReplicas(replicaCount); err != nil {
			return err
		}
	}
	return nil
}

func (ec *esClient) updateAllIndexReplicas(replicaCount int32) (bool, error) {

	indexHealth, _ := ec.GetIndexReplicaCounts()

	// get list of indices and call updateIndexReplicas for each one
	for index, health := range indexHealth {
		if healthMap, ok := health.(map[string]interface{}); ok {
			// only update replicas for indices that don't have same replica count
			if numberOfReplicas := parseString("settings.index.number_of_replicas", healthMap); numberOfReplicas != "" {
				currentReplicas, err := strconv.ParseInt(numberOfReplicas, 10, 32)
				if err != nil {
					return false, err
				}

				if int32(currentReplicas) != replicaCount {
					// best effort initially?
					if ack, err := ec.updateIndexReplicas(index, replicaCount); err != nil {
						return ack, err
					}
				}
			}
		} else {
			log.Error(nil, "unable to evaluate the number of replicas for index",
				"index", index,
				"health", health,
				"cluster", ec.cluster,
				"namespace", ec.namespace)
			return false, fmt.Errorf("unable to evaluate number of replicas for index")
		}
	}

	return true, nil
}

func (ec *esClient) GetIndexReplicaCounts() (map[string]interface{}, error) {
	payload := &EsRequest{
		Method: http.MethodGet,
		URI:    "*/_settings/index.number_of_replicas",
	}

	ec.fnSendEsRequest(ec.cluster, ec.namespace, payload, ec.k8sClient)

	return payload.ResponseBody, payload.Error
}

func (ec *esClient) updateAllIndexTemplateReplicas(replicaCount int32) (bool, error) {

	// get the index template and then update the replica and put it
	indexTemplates, _ := ec.GetIndexTemplates()

	for templateName := range indexTemplates {

		if template, ok := indexTemplates[templateName].(map[string]interface{}); ok {
			if settings, ok := template["settings"].(map[string]interface{}); ok {
				if index, ok := settings["index"].(map[string]interface{}); ok {
					currentReplicas, ok := index["number_of_replicas"].(string)

					if ok && currentReplicas != fmt.Sprintf("%d", replicaCount) {
						template["settings"].(map[string]interface{})["index"].(map[string]interface{})["number_of_replicas"] = fmt.Sprintf("%d", replicaCount)

						templateJson, _ := json.Marshal(template)

						payload := &EsRequest{
							Method:      http.MethodPut,
							URI:         fmt.Sprintf("_template/%s", templateName),
							RequestBody: string(templateJson),
						}

						ec.fnSendEsRequest(ec.cluster, ec.namespace, payload, ec.k8sClient)

						acknowledged := false
						if acknowledgedBool, ok := payload.ResponseBody["acknowledged"].(bool); ok {
							acknowledged = acknowledgedBool
						}

						if !(payload.StatusCode == 200 && acknowledged) {
							log.Error(payload.Error, "unable to update tmeplate", "template", templateName)
						}
					}
				}
			}
		}

	}

	return true, nil
}

func (ec *esClient) updateIndexReplicas(index string, replicaCount int32) (bool, error) {
	payload := &EsRequest{
		Method:      http.MethodPut,
		URI:         fmt.Sprintf("%s/_settings", index),
		RequestBody: fmt.Sprintf("{%q:\"%d\"}}", "index.number_of_replicas", replicaCount),
	}

	ec.fnSendEsRequest(ec.cluster, ec.namespace, payload, ec.k8sClient)

	acknowledged := false
	if acknowledgedBool, ok := payload.ResponseBody["acknowledged"].(bool); ok {
		acknowledged = acknowledgedBool
	}
	return payload.StatusCode == 200 && acknowledged, payload.Error
}
