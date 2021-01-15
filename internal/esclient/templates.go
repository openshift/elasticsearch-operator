package elasticsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/ViaQ/logerr/log"
	estypes "github.com/openshift/elasticsearch-operator/internal/types/elasticsearch"
	"github.com/openshift/elasticsearch-operator/internal/utils"
	"github.com/tidwall/gjson"
	"k8s.io/apimachinery/pkg/util/sets"
)

func (ec *esClient) CreateIndexTemplate(name string, template *estypes.IndexTemplate) error {

	es := ec.eoclient
	indexBody, err := utils.ToJSON(template)
	body := ioutil.NopCloser(bytes.NewBufferString(indexBody))
	res, err := es.Indices.PutTemplate(name, body, es.Indices.PutTemplate.WithPretty())

	if err != nil {
		return err
	}

	if res.IsError() || res.StatusCode != http.StatusOK {
		return ec.errorCtx().New("failed to create Index Template",
			"response_error", res.String,
			"response_status", res.StatusCode,
			"response_body", res.Body)
	}
	return nil

}

func (ec *esClient) DeleteIndexTemplate(name string) error {
	es := ec.eoclient
	res, err := es.Indices.DeleteTemplate(name, es.Indices.DeleteTemplate.WithPretty())

	if err != nil {
		return err
	}

	if res.IsError() || res.StatusCode != http.StatusOK {
		return ec.errorCtx().New("failed to delete Index Template",
			"response_error", res.String,
			"response_status", res.StatusCode,
			"response_body", res.Body)
	}
	return nil

}

// ListTemplates returns a list of templates
func (ec *esClient) ListTemplates() (sets.String, error) {
	es := ec.eoclient
	res, err := es.Indices.GetTemplate(es.Indices.GetTemplate.WithPretty())
	response := sets.NewString()
	if err != nil {
		return response, err
	}

	defer res.Body.Close()

	if res.IsError() {
		return response, fmt.Errorf("ERROR: %s: %s", res.Status(), res)
	}

	body, _ := ioutil.ReadAll(res.Body)
	jsonStr := string(body)
	results := gjson.Get(jsonStr, "name")
	for _, name := range results.Array() {
		response.Insert(name.String())
	}
	return response, nil
}

func (ec *esClient) GetIndexTemplates() (map[string]estypes.GetIndexTemplate, error) {
	pattern := "common.*"
	es := ec.eoclient
	res, err := es.Indices.GetTemplate(es.Indices.GetTemplate.WithName(pattern))
	templates := map[string]estypes.GetIndexTemplate{}
	if err != nil {
		return templates, err
	}

	defer res.Body.Close()

	if res.IsError() {
		return templates, fmt.Errorf("ERROR: %s: %s", res.Status(), res)
	}

	body, _ := ioutil.ReadAll(res.Body)
	err = json.Unmarshal(body, &templates)

	if err != nil {
		return templates, fmt.Errorf("failed decoding raw response body into `map[string]estypes.GetIndexTemplate` for %s in namespace %s: %v", ec.cluster, ec.namespace, err)
	}

	return templates, nil
}

func (ec *esClient) updateAllIndexTemplateReplicas(replicaCount int32) (bool, error) {
	es := ec.eoclient
	indexTemplates, err := ec.GetIndexTemplates()
	if err != nil {
		return false, err
	}

	replicaString := fmt.Sprintf("%d", replicaCount)

	for templateName, template := range indexTemplates {

		currentReplicas := template.Settings.Index.NumberOfReplicas
		if currentReplicas != replicaString {
			template.Settings.Index.NumberOfReplicas = replicaString

			templateJSON, err := json.Marshal(template)
			if err != nil {
				return false, err
			}
			body := bytes.NewReader(templateJSON)
			res, err := es.Indices.PutTemplate(templateName, body, es.Indices.PutTemplate.WithPretty())

			if err != nil {
				return false, err
			}
			defer res.Body.Close()

			if res.IsError() || res.StatusCode != http.StatusOK {
				return false, ec.errorCtx().New("failed to update Template Replicas",
					"response_error", res.String,
					"response_status", res.StatusCode,
					"response_body", res.Body)
			}

			acknowledged := false

			resBody, _ := ioutil.ReadAll(res.Body)
			jsonStr := string(resBody)
			acknowledged = gjson.Get(jsonStr, "acknowledged").Bool()

			if !acknowledged {
				log.Error(nil, "unable to update template", "cluster", ec.cluster, "namespace", ec.namespace, "template", templateName)
			}
		}
	}

	return true, nil
}

func (ec *esClient) UpdateTemplatePrimaryShards(shardCount int32) error {
	// get the index template and then update the shards and put it

	es := ec.eoclient
	indexTemplates, err := ec.GetIndexTemplates()
	if err != nil {
		return err
	}

	shardString := fmt.Sprintf("%d", shardCount)

	for templateName, template := range indexTemplates {

		currentShards := template.Settings.Index.NumberOfShards
		if currentShards != shardString {
			template.Settings.Index.NumberOfShards = shardString

			templateJSON, err := json.Marshal(template)
			if err != nil {
				return err
			}

			body := bytes.NewReader(templateJSON)
			res, err := es.Indices.PutTemplate(templateName, body, es.Indices.PutTemplate.WithPretty())

			if err != nil {
				return err
			}
			defer res.Body.Close()

			if res.IsError() || res.StatusCode != http.StatusOK {
				return ec.errorCtx().New("failed to update Template Replicas",
					"response_error", res.String,
					"response_status", res.StatusCode,
					"response_body", res.Body)
			}

			acknowledged := false

			resBody, _ := ioutil.ReadAll(res.Body)
			jsonStr := string(resBody)
			acknowledged = gjson.Get(jsonStr, "acknowledged").Bool()

			if !acknowledged {
				log.Error(nil, "unable to update template", "cluster", ec.cluster, "namespace", ec.namespace, "template", templateName)
			}
		}
	}

	return nil
}
