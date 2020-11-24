package elasticsearch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"
	estypes "github.com/openshift/elasticsearch-operator/internal/types/elasticsearch"
	"github.com/openshift/elasticsearch-operator/internal/utils"
)

func (ec *esClient) GetIndex(name string) (*estypes.Index, error) {
	es := ec.client
	indexName := []string{name}
	res, err := es.Indices.Get(indexName, es.Indices.Get.WithPretty())
	if err != nil {
		return nil, err
	}
	ec.fnSendEsRequest(ec.cluster, ec.namespace, payload, ec.k8sClient)
	if payload.Error != nil {
		return nil, payload.Error
	}
	if payload.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if payload.StatusCode != http.StatusOK {
		return nil, ec.errorCtx().New("failed to get index",
			"index", name,
			"response_status", payload.StatusCode,
			"response_body", payload.ResponseBody)
	}
	body, _ := ioutil.ReadAll(res.Body)
	jsonStr := string(body)
	result := gjson.Get(jsonStr, name).Raw
	index := &estypes.Index{}
	err = json.Unmarshal([]byte(result), index)

	index := &estypes.Index{}
	err := json.Unmarshal([]byte(payload.RawResponseBody), index)
	if err != nil {
		return nil, kverrors.Wrap(err, "failed decoding raw response body into `estypes.Index`",
			"index", name)
	}
	index.Name = name
	return index, nil
}

func (ec *esClient) GetAllIndices(name string) (estypes.CatIndicesResponses, error) {
	es := ec.client
	res, err := es.Cat.Indices(es.Cat.Indices.WithIndex(name), es.Cat.Indices.WithFormat("json"), es.Cat.Indices.WithPretty())

	if err != nil {
		return nil, err
	}
	ec.fnSendEsRequest(ec.cluster, ec.namespace, payload, ec.k8sClient)
	if payload.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if payload.Error != nil {
		return nil, payload.Error
	}
	if payload.StatusCode != http.StatusOK {
		return nil, ec.errorCtx().New("failed to get index",
			"index", name,
			"response_status", payload.StatusCode,
			"response_body", payload.ResponseBody)
	}

	res := estypes.CatIndicesResponses{}
	raw := payload.ResponseBody["results"].(string)
	err := json.Unmarshal([]byte(raw), &res)
	if err != nil {
		return nil, kverrors.Wrap(err, "failed to parse _cat/indices response body",
			"index", name)
	}
	return res, nil
}

func (ec *esClient) CreateIndex(name string, index *estypes.Index) error {
	es := ec.client
	indexBody, err := utils.ToJSON(index)
	body := ioutil.NopCloser(bytes.NewBufferString(indexBody))
	res, err := es.Indices.Create(name, es.Indices.Create.WithBody(body))

	if err != nil {
		return err
	}
	payload := &EsRequest{
		Method:      http.MethodPut,
		URI:         name,
		RequestBody: body,
	}
	ec.fnSendEsRequest(ec.cluster, ec.namespace, payload, ec.k8sClient)
	if payload.Error != nil {
		return payload.Error
	}
	if payload.StatusCode != 200 && payload.StatusCode != 201 {
		return ec.errorCtx().New("failed to create index",
			"index", index.Name,
			"response_status", payload.StatusCode,
			"response_body", payload.ResponseBody)
	}
	return nil
}

func (ec *esClient) GetIndexSettings(name string) (*estypes.IndexSettings, error) {
	payload := &EsRequest{
		Method: http.MethodGet,
		URI:    fmt.Sprintf("%s/_settings", name),
	}
	ec.fnSendEsRequest(ec.cluster, ec.namespace, payload, ec.k8sClient)
	if payload.Error != nil {
		return nil, payload.Error
	}
	if payload.StatusCode != http.StatusOK {
		return nil, ec.errorCtx().New("failed to get index settings",
			"index", name,
			"response_status", payload.StatusCode,
			"response_body", payload.ResponseBody)
	}

	settings := &estypes.IndexSettings{}
	err := json.Unmarshal([]byte(payload.RawResponseBody), settings)
	if err != nil {
		return nil, kverrors.Wrap(err, "failed to decode response body",
			"destination_type", "estypes.IndexSettings",
			"index", name)
	}
	return settings, nil
}

func (ec *esClient) UpdateIndexSettings(name string, settings *estypes.IndexSettings) error {
	body, err := utils.ToJSON(settings)
	if err != nil {
		return err
	}
	payload := &EsRequest{
		Method:      http.MethodPut,
		URI:         fmt.Sprintf("%s/_settings", name),
		RequestBody: body,
	}
	ec.fnSendEsRequest(ec.cluster, ec.namespace, payload, ec.k8sClient)
	if payload.Error != nil {
		return payload.Error
	}
	if payload.StatusCode != http.StatusOK && payload.StatusCode != http.StatusCreated {
		return ec.errorCtx().New("failed to update index settings",
			"index", name,
			"response_status", payload.StatusCode,
			"response_body", payload.ResponseBody)
	}
	return nil
}

func (ec *esClient) ReIndex(src, dst, script, lang string) error {
	reIndex := estypes.ReIndex{
		Source: estypes.IndexRef{Index: src},
		Dest:   estypes.IndexRef{Index: dst},
		Script: estypes.ReIndexScript{
			Inline: script,
			Lang:   lang,
		},
	}

	indexBody, err := utils.ToJSON(reIndex)
	body := ioutil.NopCloser(bytes.NewBufferString(indexBody))

	es := ec.client
	res, err := es.Reindex(body, es.Reindex.WithPretty())

	if err != nil {
		return err
	}
	payload := &EsRequest{
		Method:      http.MethodPost,
		URI:         "_reindex",
		RequestBody: body,
	}
	ec.fnSendEsRequest(ec.cluster, ec.namespace, payload, ec.k8sClient)
	if payload.Error != nil || payload.StatusCode != http.StatusOK {
		return ec.errorCtx().New("failed to reindex",
			"from", src,
			"to", dst,
			"response_error", payload.Error,
			"response_status", payload.StatusCode,
			"response_body", payload.ResponseBody)
	}

	return nil
}

// ListIndicesForAlias returns a list of indices and the alias for the given pattern (e.g. foo-*, *-write)
func (ec *esClient) ListIndicesForAlias(aliasPattern string) ([]string, error) {
	es := ec.client
	res, err := es.Indices.GetAlias(es.Indices.GetAlias.WithIndex(aliasPattern), es.Indices.GetAlias.WithPretty())
	if err != nil {
		return err
	}
	payload := &EsRequest{
		Method:      http.MethodPost,
		URI:         "_aliases",
		RequestBody: body,
	}
	log.Info("Updating aliases", "payload", actions)
	ec.fnSendEsRequest(ec.cluster, ec.namespace, payload, ec.k8sClient)
	if payload.Error != nil {
		return payload.Error
	}
	if payload.StatusCode != http.StatusOK && payload.StatusCode != http.StatusCreated {
		return ec.errorCtx().New("failed to update aliases",
			"response_error", payload.Error,
			"response_status", payload.StatusCode,
			"response_body", payload.ResponseBody)
	}
	return nil
}

func (ec *esClient) UpdateAlias(actions estypes.AliasActions) error {

	es := ec.client
	actionBody, err := utils.ToJSON(actions)
	body := ioutil.NopCloser(bytes.NewBufferString(actionBody))
	res, err := es.Indices.UpdateAliases(body, es.Indices.UpdateAliases.WithPretty())

	if err != nil {
		return err
	}

	ec.fnSendEsRequest(ec.cluster, ec.namespace, payload, ec.k8sClient)
	if payload.StatusCode == 404 {
		return []string{}, nil
	}
	if payload.Error != nil || payload.StatusCode != 200 {
		return nil, ec.errorCtx().New("failed to get list of indices from alias",
			"alias", aliasPattern,
			"response_error", payload.Error,
			"response_status", payload.StatusCode,
			"response_body", payload.ResponseBody)
	}
	var response []string
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

	aliasPattern := "project.*,.operations.*/_alias"
	es := ec.client
	res, err := es.Indices.GetAlias(es.Indices.GetAlias.WithIndex(aliasPattern), es.Indices.GetAlias.WithPretty())
	if err != nil {
		return false
	}
	defer res.Body.Close()

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
				log.Error(nil, "unable to unmarshal index",
					"index", index,
					"cluster", ec.cluster,
					"type", fmt.Sprintf("%T", payload.ResponseBody[index]),
				)
				continue
			}
			if indexBody["aliases"] != nil {
				aliasBody, ok := indexBody["aliases"].(map[string]interface{})
				if !ok {
					log.Error(nil, "unable to unmarshal alias index",
						"index", index,
						"cluster", ec.cluster,
						"type", fmt.Sprintf("%T", indexBody["aliases"]),
					)
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
<<<<<<< HEAD
=======

func (ec *esClient) GetIndexSettings(name string) (*estypes.IndexSettings, error) {

	es := ec.client
	res, err := es.Indices.GetSettings(es.Indices.GetSettings.WithName(name), es.Indices.GetSettings.WithPretty())
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() || res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ERROR: %s: %s", res.Status(), res)
	}
	body, _ := ioutil.ReadAll(res.Body)
	jsonStr := string(body)
	log.Info(jsonStr)

	settings := &estypes.IndexSettings{}
	err = json.Unmarshal(body, settings)
	if err != nil {
		return nil, kverrors.Wrap(err, "failed to decode response body",
			"destination_type", "estypes.IndexSettings",
			"index", name)
	}
	return settings, nil
}

func (ec *esClient) UpdateIndexSettings(name string, settings *estypes.IndexSettings) error {

	settingsStr, err := utils.ToJSON(settings)
	body := ioutil.NopCloser(bytes.NewBufferString(settingsStr))

	if err != nil {
		return err
	}

	es := ec.client
	res, err := es.Indices.PutSettings(body, es.Indices.PutSettings.WithPretty())

	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() || res.StatusCode != http.StatusOK {
		return ec.errorCtx().New("failed to update index settings",
			"index", name,
			"response_status", res.StatusCode,
			"response_body", res)

	}

	return nil
}
>>>>>>> 5cef9ca2 (updating test)
