package elasticsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	estypes "github.com/openshift/elasticsearch-operator/internal/types/elasticsearch"
	"github.com/openshift/elasticsearch-operator/internal/utils"
	"k8s.io/apimachinery/pkg/util/sets"
)

func (ec *esClient) CreateIndexTemplate(name string, template *estypes.IndexTemplate) error {

	es := ec.client
	indexBody, err := utils.ToJSON(template)
	body := ioutil.NopCloser(bytes.NewBufferString(indexBody))
	res, err := es.Indices.PutTemplate(name, body, es.Indices.PutTemplate.WithPretty())

	if err != nil {
		return err
	}

	if res.IsError() || res.StatusCode != http.StatusOK {
		resBody, _ := ioutil.ReadAll(res.Body)
		errorMsg := string(resBody)
		return ec.errorCtx().New("Failed to create Index Template",
			"response_error", res.String(),
			"response_status", res.StatusCode,
			"response_body", errorMsg)
	}
	return nil

}

func (ec *esClient) DeleteIndexTemplate(name string) error {
	es := ec.client
	res, err := es.Indices.DeleteTemplate(name, es.Indices.DeleteTemplate.WithPretty())

	if err != nil {
		return err
	}

	if res.IsError() || res.StatusCode != http.StatusOK {
		resBody, _ := ioutil.ReadAll(res.Body)
		errorMsg := string(resBody)
		return ec.errorCtx().New("Failed to Delete Index Template",
			"response_error", res.String(),
			"response_status", res.StatusCode,
			"response_body", errorMsg)
	}
	return nil

}

// ListTemplates returns a list of templates
func (ec *esClient) ListTemplates() (sets.String, error) {
	es := ec.client
	res, err := es.Indices.GetTemplate(es.Indices.GetTemplate.WithPretty())
	response := sets.NewString()
	if err != nil {
		return response, err
	}

	defer res.Body.Close()

	if res.IsError() {
		return response, fmt.Errorf("ERROR: %s: %s", res.Status(), res)
	}
	if res.IsError() || res.StatusCode != http.StatusOK {
		resBody, _ := ioutil.ReadAll(res.Body)
		errorMsg := string(resBody)
		return response, ec.errorCtx().New("Failed to get list of templates",
			"response_error", res.String(),
			"response_status", res.StatusCode,
			"response_body", errorMsg)
	}

	body, _ := ioutil.ReadAll(res.Body)
	m := make(map[string]interface{})
	err = json.Unmarshal(body, &m)
	//log.Error(err, "error")

	if err != nil {
		return response, err
	}
	for name := range m {
		response.Insert(name)
	}
	return response, nil
}

func (ec *esClient) GetIndexTemplates() (map[string]estypes.GetIndexTemplate, error) {
	pattern := "common.*"
	es := ec.client
	res, err := es.Indices.GetTemplate(es.Indices.GetTemplate.WithName(pattern))
	templates := map[string]estypes.GetIndexTemplate{}

	if err != nil {
		return templates, err
	}

	defer res.Body.Close()

	if res.IsError() || res.StatusCode != http.StatusOK {
		resBody, _ := ioutil.ReadAll(res.Body)
		errorMsg := string(resBody)
		return templates, ec.errorCtx().New("Failed to Get Index Template",
			"response_error", res.String(),
			"response_status", res.StatusCode,
			"response_body", errorMsg)
	}

	body, _ := ioutil.ReadAll(res.Body)
	err = json.Unmarshal(body, &templates)

	if err != nil {
		return templates, fmt.Errorf("failed decoding raw response body into `map[string]estypes.GetIndexTemplate` for %s in namespace %s: %v", ec.cluster, ec.namespace, err)
	}

	return templates, nil
}

func (ec *esClient) updateAllIndexTemplateReplicas(replicaCount int32) (bool, error) {
	es := ec.client
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
			acknowledged := false
			if res.IsError() || res.StatusCode != http.StatusOK {
				resBody, _ := ioutil.ReadAll(res.Body)
				errorMsg := string(resBody)
				return false, ec.errorCtx().New("Failed to Update Template Replicas",
					"response_error", res.String(),
					"response_status", res.StatusCode,
					"response_body", errorMsg)
			} else {
				acknowledged = true
			}

			if !acknowledged {
				return false, fmt.Errorf("Unable to update template for cluster: %s  namespace: %s template: %s", ec.cluster, ec.namespace, templateName)
			}
		}
	}

	return true, nil
}

func (ec *esClient) UpdateTemplatePrimaryShards(shardCount int32) error {
	// get the index template and then update the shards and put it

	es := ec.client
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
			jsonStr := string(templateJSON)

			if err != nil {
				return err
			}

			//body := bytes.NewReader(templateJSON)
			body := strings.NewReader(jsonStr)
			res, err := es.Indices.PutTemplate(templateName, body, es.Indices.PutTemplate.WithPretty())

			if err != nil {
				return err
			}
			acknowledged := false
			defer res.Body.Close()

			if res.IsError() || res.StatusCode != http.StatusOK {
				resBody, _ := ioutil.ReadAll(res.Body)
				errorMsg := string(resBody)
				return ec.errorCtx().New("Failed to Update Template Shards",
					"response_error", res.String(),
					"response_status", res.StatusCode,
					"response_body", errorMsg)
			} else {
				acknowledged = true
			}

			if !acknowledged {
				return fmt.Errorf("Failed to Update Template Shards for cluster: %s  namespace: %s template: %s", ec.cluster, ec.namespace, templateName)
			}
		}
	}

	return nil
}
