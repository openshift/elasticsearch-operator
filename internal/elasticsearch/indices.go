package elasticsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"
	estypes "github.com/openshift/elasticsearch-operator/internal/types/elasticsearch"
	"github.com/openshift/elasticsearch-operator/internal/utils"
	"github.com/tidwall/gjson"
)

func (ec *esClient) GetIndex(name string) (*estypes.Index, error) {
	es := ec.client
	indexName := []string{name}
	res, err := es.Indices.Get(indexName, es.Indices.Get.WithPretty())
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("ERROR: %s: %s", res.Status(), res)
	}
	body, _ := ioutil.ReadAll(res.Body)
	jsonStr := string(body)
	result := gjson.Get(jsonStr, name).Raw
	index := &estypes.Index{}
	err = json.Unmarshal([]byte(result), index)

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
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("ERROR: %s: %s", res.Status(), res)
	}
	body, _ := ioutil.ReadAll(res.Body)
	jsonStr := string(body)
	resIndices := estypes.CatIndicesResponses{}
	err = json.Unmarshal([]byte(jsonStr), &resIndices)

	if err != nil {
		return nil, kverrors.Wrap(err, "failed to parse _cat/indices response body",
			"index", name)
	}

	return resIndices, nil
}

func (ec *esClient) CreateIndex(name string, index *estypes.Index) error {
	es := ec.client
	indexBody, err := utils.ToJSON(index)
	body := ioutil.NopCloser(bytes.NewBufferString(indexBody))
	res, err := es.Indices.Create(name, es.Indices.Create.WithBody(body))

	if err != nil {
		return err
	}

	if res.IsError() || res.StatusCode != http.StatusOK {
		return ec.errorCtx().New("failed to create Index",
			"response_error", res.String,
			"response_status", res.StatusCode,
			"response_body", res.Body)
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

	if res.IsError() || res.StatusCode != http.StatusOK {
		return ec.errorCtx().New("failed to reindex",
			"from", src,
			"to", dst,
			"response_error", res.Status,
			"response_status", res.StatusCode,
			"response_body", res.Body)
	}

	return nil
}

// ListIndicesForAlias returns a list of indices and the alias for the given pattern (e.g. foo-*, *-write)
func (ec *esClient) ListIndicesForAlias(aliasPattern string) ([]string, error) {
	es := ec.client
	res, err := es.Indices.GetAlias(es.Indices.GetAlias.WithIndex(aliasPattern), es.Indices.GetAlias.WithPretty())
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() || res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ERROR: %s: %s", res.Status(), res)
	}

	body, _ := ioutil.ReadAll(res.Body)
	jsonStr := string(body)

	m, _ := gjson.Parse(jsonStr).Value().(map[string]interface{})
	var response []string
	for _, element := range m {
		data, _ := json.Marshal(element)
		response = append(response, string(data))
	}
	return response, nil
}

func (ec *esClient) UpdateAlias(actions estypes.AliasActions) error {

	es := ec.client
	actionBody, err := utils.ToJSON(actions)
	body := ioutil.NopCloser(bytes.NewBufferString(actionBody))
	res, err := es.Indices.UpdateAliases(body, es.Indices.UpdateAliases.WithPretty())

	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() || res.StatusCode != http.StatusOK {
		return fmt.Errorf("ERROR: %s: %s", res.Status(), res)
	}
	return nil
}

//TODO Implement
func (ec *esClient) AddAliasForOldIndices() bool {
	successful := true

	aliasPattern := "project.*,.operations.*/_alias"
	es := ec.client
	res, err := es.Indices.GetAlias(es.Indices.GetAlias.WithIndex(aliasPattern), es.Indices.GetAlias.WithPretty())
	if err != nil {
		return false
	}
	defer res.Body.Close()

	if res.IsError() || res.StatusCode != http.StatusOK {
		return false
	}

	body, _ := ioutil.ReadAll(res.Body)
	response := make(map[string]interface{})
	err = json.Unmarshal(body, &response)

	if err != nil {
		return false
	}

	// alias name choice based on https://github.com/openshift/enhancements/blob/master/enhancements/cluster-logging/cluster-logging-es-rollover-data-design.md#data-model
	for index := range response {
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

		if response[index] != nil {
			indexBody, ok := response[index].(map[string]interface{})
			if !ok {
				log.Error(nil, "unable to unmarshal index",
					"index", index,
					"cluster", ec.cluster,
					"type", fmt.Sprintf("%T", response[index]),
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
					indexName := []string{index}
					res, err := es.Indices.PutAlias(indexName, indexAlias, es.Indices.PutAlias.WithPretty())
					if err != nil {
						return false
					}
					defer res.Body.Close()

					if res.IsError() || res.StatusCode != http.StatusOK {
						return false
					}
					acknowledged := false

					resBody, _ := ioutil.ReadAll(res.Body)
					jsonStr := string(resBody)
					acknowledged = gjson.Get(jsonStr, "acknowledged").Bool()
					// check the response here -- if any failed then we want to return "false"
					// but want to continue trying to process as many as we can now.
					successful = acknowledged

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
