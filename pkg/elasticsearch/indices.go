package elasticsearch

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/openshift/elasticsearch-operator/pkg/logger"
	estypes "github.com/openshift/elasticsearch-operator/pkg/types/elasticsearch"
	"github.com/openshift/elasticsearch-operator/pkg/utils"
)

func (ec *esClient) CreateIndex(name string, index *estypes.Index) error {
	body, err := utils.ToJson(index)
	if err != nil {
		return err
	}
	payload := &EsRequest{
		Method:      http.MethodPut,
		URI:         name,
		RequestBody: body,
	}
	logger.DebugObject("CreateIndex with payload: %s", index)
	ec.fnSendEsRequest(ec.cluster, ec.namespace, payload, ec.k8sClient)
	if payload.Error != nil {
		return payload.Error
	}
	if payload.StatusCode != 200 && payload.StatusCode != 201 {
		return fmt.Errorf("There was an error creating index %s. Error code: %v, %v", index.Name, payload.StatusCode != 200, payload.ResponseBody)
	}
	return nil
}

//ListIndicesForAlias returns a list of indices and the alias for the given pattern (e.g. foo-*, *-write)
func (ec *esClient) ListIndicesForAlias(aliasPattern string) ([]string, error) {
	payload := &EsRequest{
		Method: http.MethodGet,
		URI:    fmt.Sprintf("_alias/%s", aliasPattern),
	}

	ec.fnSendEsRequest(ec.cluster, ec.namespace, payload, ec.k8sClient)
	if payload.Error != nil {
		return nil, payload.Error
	}
	if payload.StatusCode == 404 {
		return []string{}, nil
	}
	if payload.StatusCode != 200 {
		return nil, fmt.Errorf("There was an error retrieving list of indices aliased to %s. Error code: %v, %v", aliasPattern, payload.StatusCode != 200, payload.ResponseBody)
	}
	response := []string{}
	for index := range payload.ResponseBody {
		response = append(response, index)
	}
	return response, nil
}

func (ec *esClient) AddAliasForOldIndices() bool {
	// get .operations.*/_alias
	// get project.*/_alias
	/*
	   {
	       "project.test.107d38b1-413b-11ea-a2cd-0a3ee645943a.2020.01.27" : {
	           "aliases" : {
	               "test" : { }
	           }
	       },
	       "project.test2.8fe8b95e-4147-11ea-91e1-062a8c33f2ae.2020.01.27" : {
	           "aliases" : { }
	       }
	   }
	*/

	successful := true

	payload := &EsRequest{
		Method: http.MethodGet,
		URI:    "project.*,.operations.*/_alias",
	}

	ec.fnSendEsRequest(ec.cluster, ec.namespace, payload, ec.k8sClient)

	// alias name choice based on https://github.com/openshift/enhancements/blob/master/enhancements/cluster-logging/cluster-logging-es-rollover-data-design.md#data-model
	for index := range payload.ResponseBody {
		// iterate over each index, if they have no aliases that match the new format
		// then PUT the alias

		indexAlias := ""
		if strings.HasPrefix(index, "project.") {
			// it is a container log index
			indexAlias = "app"
		} else {
			// it is an operations index
			indexAlias = "infra"
		}

		if payload.ResponseBody[index] != nil {
			indexBody, ok := payload.ResponseBody[index].(map[string]interface{})
			if !ok {
				logger.Warnf("unable to unmarshal index '%s' response body for cluster '%s'. Type: %s",
					index,
					ec.cluster,
					reflect.TypeOf(payload.ResponseBody[index]).String())
				continue
			}
			if indexBody["aliases"] != nil {
				aliasBody, ok := indexBody["aliases"].(map[string]interface{})
				if !ok {
					logger.Warnf("unable to unmarshal alias index '%s' body for cluster '%s'. Type: %s",
						index,
						ec.cluster,
						reflect.TypeOf(indexBody["aliases"]).String())
					continue
				}

				found := false
				for alias := range aliasBody {
					if alias == indexAlias {
						found = true
						break
					}
				}

				if !found {
					// put <index>/_alias/<alias>
					putPayload := &EsRequest{
						Method: http.MethodPut,
						URI:    fmt.Sprintf("%s/_alias/%s", index, indexAlias),
					}
					ec.fnSendEsRequest(ec.cluster, ec.namespace, putPayload, ec.k8sClient)

					// check the response here -- if any failed then we want to return "false"
					// but want to continue trying to process as many as we can now.
					if putPayload.Error != nil || !parseBool("acknowledged", putPayload.ResponseBody) {
						successful = false
					}
				}
			} else {
				// if for some reason we received a response without an "aliases" field
				// we want to retry -- es may not be in a good state?
				successful = false
			}
		} else {
			// if for some reason we received a response without an index field
			// we want to retry -- es may not be in a good state?
			successful = false
		}
	}

	return successful
}
