package k8shandler

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	elasticsearch "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	estypes "github.com/openshift/elasticsearch-operator/pkg/types/elasticsearch"
)

var (
	request       *ElasticsearchRequest
	indexTemplate *estypes.IndexTemplate = estypes.NewIndexTemplate("abc-**", []string{"foo"}, 1, 0)
)

func reconcilerSetUp(responseCode int, err error) {
	request = &ElasticsearchRequest{
		client: nil,
		cluster: &elasticsearch.Elasticsearch{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "elasticsearch",
				Namespace: "openshift-logging",
			},
		},
		FnCurlEsService: func(clusterName, namespace string, payload *esCurlStruct, client client.Client) {
			payload.Error = err
			payload.StatusCode = responseCode
		},
	}

}

func TestCreateIndexTemplateWhenError(t *testing.T) {

	reconcilerSetUp(500, fmt.Errorf("test error %s", t.Name()))

	if request.CreateIndexTemplate("foo", indexTemplate) == nil {
		t.Error("Exp. to return an error but did not")
	}
}
func TestCreateIndexTemplateWhenResponseNot200(t *testing.T) {

	reconcilerSetUp(500, nil)

	if request.CreateIndexTemplate("foo", indexTemplate) == nil {
		t.Error("Exp. to return an error but did not")
	}
}
func TestCreateIndexTemplateWhenResponse200(t *testing.T) {

	reconcilerSetUp(200, nil)

	if err := request.CreateIndexTemplate("foo", indexTemplate); err != nil {
		t.Errorf("Exp. to not return an error %v", err)
	}
}
func TestCreateIndexTemplateWhenResponse201(t *testing.T) {

	reconcilerSetUp(201, nil)

	if err := request.CreateIndexTemplate("foo", indexTemplate); err != nil {
		t.Errorf("Exp. to not return an error %v", err)
	}
}
