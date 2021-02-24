package elasticsearch

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ViaQ/logerr/kverrors"
	estypes "github.com/openshift/elasticsearch-operator/internal/types/elasticsearch"
	"github.com/openshift/elasticsearch-operator/internal/utils"
)

func (ec *esClient) GetIndex(name string) (*estypes.Index, error) {
	index := &estypes.Index{}

	return index, nil
}

func (ec *esClient) GetAllIndices(name string) (estypes.CatIndicesResponses, error) {

	res := estypes.CatIndicesResponses{}

	return res, nil
}

func (ec *esClient) CreateIndex(name string, index *estypes.Index) error {

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

	return nil
}

// ListIndicesForAlias returns a list of indices and the alias for the given pattern (e.g. foo-*, *-write)
func (ec *esClient) ListIndicesForAlias(aliasPattern string) ([]string, error) {

	return []string{}, nil
}

func (ec *esClient) UpdateAlias(actions estypes.AliasActions) error {

	return nil
}

func (ec *esClient) AddAliasForOldIndices() bool {

	return true
}
