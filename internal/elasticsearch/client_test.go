package elasticsearch

import (
	"net/http"

	elasticsearch6 "github.com/elastic/go-elasticsearch/v6"
	"github.com/elastic/go-elasticsearch/v6/esapi"
)

type mockTransp struct {
	response *http.Response
	err      error
}

func (t *mockTransp) Perform(req *http.Request) (*http.Response, error) {
	return t.response, t.err
}

func getFakeESClient(clusterName, namespace string, res *http.Response, err error) Client {

	mocktransport := &mockTransp{res, err}
	elasticsearchClient := elasticsearch6.Client{Transport: mocktransport, API: esapi.New(mocktransport)}

	esClient := NewClient(clusterName, namespace, nil)
	esClient.SetESClient(elasticsearchClient)

	return esClient
}

/*
func TestGetIndexTemplates_actual(t *testing.T) {
	esAddr := "http://localhost:9200"

	elasticsearchClient, err := getESClient(esAddr)

	esClient := NewClient("default", "default", nil)
	esClient.SetESClient(*elasticsearchClient)

	err = esClient.UpdateTemplatePrimaryShards(5)
	if err != nil {
		t.Errorf("got err: %s", err)
	}
}*/
