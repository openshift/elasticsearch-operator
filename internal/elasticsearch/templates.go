package elasticsearch

import (
	estypes "github.com/openshift/elasticsearch-operator/internal/types/elasticsearch"
	"k8s.io/apimachinery/pkg/util/sets"
)

func (ec *esClient) CreateIndexTemplate(name string, template *estypes.IndexTemplate) error {

	return nil
}

func (ec *esClient) DeleteIndexTemplate(name string) error {
	return nil
}

// ListTemplates returns a list of templates
func (ec *esClient) ListTemplates() (sets.String, error) {

	response := sets.NewString()

	return response, nil
}

func (ec *esClient) GetIndexTemplates() (map[string]estypes.GetIndexTemplate, error) {

	templates := map[string]estypes.GetIndexTemplate{}

	return templates, nil
}

func (ec *esClient) updateAllIndexTemplateReplicas(replicaCount int32) (bool, error) {

	return true, nil
}

func (ec *esClient) UpdateTemplatePrimaryShards(shardCount int32) error {
	// get the index template and then update the shards and put it

	return nil
}
