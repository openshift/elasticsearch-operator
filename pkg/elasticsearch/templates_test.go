package elasticsearch_test

import (
	"fmt"
	"net/http"
	"testing"

	estypes "github.com/openshift/elasticsearch-operator/pkg/types/elasticsearch"
	testhelpers "github.com/openshift/elasticsearch-operator/test/helpers"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	cluster                              = "elasticsearch"
	namespace                            = "openshift-logging"
	k8sClient                            = fake.NewFakeClient()
	indexTemplate *estypes.IndexTemplate = estypes.NewIndexTemplate("abc-**", []string{"foo"}, 1, 0)
)

func TestCreateIndexTemplateWhenError(t *testing.T) {
	chatter := testhelpers.NewFakeElasticsearchChatter(
		map[string]testhelpers.FakeElasticsearchResponses{
			"_template/foo": {
				{
					Error:      fmt.Errorf("test error %s", t.Name()),
					StatusCode: http.StatusInternalServerError,
					Body:       "{}",
				},
			},
		})
	esClient := testhelpers.NewFakeElasticsearchClient(cluster, namespace, k8sClient, chatter)

	if esClient.CreateIndexTemplate("foo", indexTemplate) == nil {
		t.Error("Exp. to return an error but did not")
	}
}

func TestCreateIndexTemplateWhenResponseNot200(t *testing.T) {
	chatter := testhelpers.NewFakeElasticsearchChatter(
		map[string]testhelpers.FakeElasticsearchResponses{
			"_template/foo": {
				{
					Error:      nil,
					StatusCode: 500,
					Body:       "{}",
				},
			},
		})
	esClient := testhelpers.NewFakeElasticsearchClient(cluster, namespace, k8sClient, chatter)

	if esClient.CreateIndexTemplate("foo", indexTemplate) == nil {
		t.Error("Exp. to return an error but did not")
	}
}

func TestCreateIndexTemplateWhenResponse200(t *testing.T) {
	chatter := testhelpers.NewFakeElasticsearchChatter(
		map[string]testhelpers.FakeElasticsearchResponses{
			"_template/foo": {
				{
					Error:      nil,
					StatusCode: 200,
					Body:       "{}",
				},
			},
		})
	esClient := testhelpers.NewFakeElasticsearchClient(cluster, namespace, k8sClient, chatter)

	if err := esClient.CreateIndexTemplate("foo", indexTemplate); err != nil {
		t.Errorf("Exp. to not return an error %v", err)
	}
}

func TestCreateIndexTemplateWhenResponse201(t *testing.T) {
	chatter := testhelpers.NewFakeElasticsearchChatter(
		map[string]testhelpers.FakeElasticsearchResponses{
			"_template/foo": {
				{
					Error:      nil,
					StatusCode: 201,
					Body:       "{}",
				},
			},
		})
	esClient := testhelpers.NewFakeElasticsearchClient(cluster, namespace, k8sClient, chatter)

	if err := esClient.CreateIndexTemplate("foo", indexTemplate); err != nil {
		t.Errorf("Exp. to not return an error %v", err)
	}
}
