package k8shandler

import (
	"net/http"
	"testing"

	"github.com/openshift/elasticsearch-operator/internal/constants"
	"github.com/openshift/elasticsearch-operator/internal/elasticsearch"
	"github.com/openshift/elasticsearch-operator/test/helpers"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestIndexIsNotBlocked(t *testing.T) {
	var (
		chatter   *helpers.FakeElasticsearchChatter
		client    elasticsearch.Client
		k8sClient = fake.NewFakeClient()
	)

	const (
		esCluster   = "elasticsearch"
		esNamespace = "openshift-logging"
	)

	chatter = helpers.NewFakeElasticsearchChatter(map[string]helpers.FakeElasticsearchResponses{
		".security/_settings": {
			{
				StatusCode: 200,
				Body:       `{".security": {"settings": {}}}`,
			},
		},
	})
	client = helpers.NewFakeElasticsearchClient(esCluster, esNamespace, k8sClient, chatter)

	er := ElasticsearchRequest{
		client:   k8sClient,
		esClient: client,
	}

	if isBlocked := er.isIndexBlocked(constants.SecurityIndex); isBlocked != false {
		t.Errorf("Expected index to be not blocked, but found blocked.")
	}

	// Get Index settings first
	req, found := chatter.GetRequest(".security/_settings")
	if found != true {
		t.Errorf("Expected true but got false.")
	}
	if req.Method != http.MethodGet {
		t.Errorf("Expected: %v, got: %v", http.MethodGet, req.Method)
	}
}

func TestIndexIsBlocked(t *testing.T) {
	var (
		chatter   *helpers.FakeElasticsearchChatter
		client    elasticsearch.Client
		k8sClient = fake.NewFakeClient()
	)

	const (
		esCluster   = "elasticsearch"
		esNamespace = "openshift-logging"
	)

	chatter = helpers.NewFakeElasticsearchChatter(map[string]helpers.FakeElasticsearchResponses{
		".security/_settings": {
			{
				StatusCode: 200,
				Body:       `{".security": {"settings": {"index": {"blocks": {"read_only_allow_delete": "true"}}}}}`,
			},
		},
	})
	client = helpers.NewFakeElasticsearchClient(esCluster, esNamespace, k8sClient, chatter)

	er := ElasticsearchRequest{
		client:   k8sClient,
		esClient: client,
	}

	if isBlocked := er.isIndexBlocked(constants.SecurityIndex); isBlocked != true {
		t.Errorf("Expected index to be blocked, but found unblocked.")
	}

	// Get Index settings first
	req, found := chatter.GetRequest(".security/_settings")
	if found != true {
		t.Errorf("Expected true but got false.")
	}
	if req.Method != http.MethodGet {
		t.Errorf("Expected: %v, got: %v", http.MethodGet, req.Method)
	}
}

func TestCreateSettingToUnblock(t *testing.T) {
	var (
		chatter   *helpers.FakeElasticsearchChatter
		client    elasticsearch.Client
		k8sClient = fake.NewFakeClient()
	)

	const (
		esCluster   = "elasticsearch"
		esNamespace = "openshift-logging"
	)

	chatter = helpers.NewFakeElasticsearchChatter(map[string]helpers.FakeElasticsearchResponses{
		"app-00001/_settings": {
			{
				StatusCode: 200,
				Body:       `{}`,
			},
		},
	})
	client = helpers.NewFakeElasticsearchClient(esCluster, esNamespace, k8sClient, chatter)

	er := ElasticsearchRequest{
		client:   k8sClient,
		esClient: client,
	}

	if err := er.unblockIndex("app-00001"); err != nil {
		t.Errorf("Expected to unblock but got err: %v", err)
	}

	// Update Index setting
	req, found := chatter.GetRequest("app-00001/_settings")
	if found != true {
		t.Errorf("Expected true but got false.")
	}
	if req.Method != http.MethodPut {
		t.Errorf("Expected: %v, got: %v", http.MethodPut, req.Method)
	}
	helpers.ExpectJSON(req.Body).ToEqual(
		`{
				"index": {
					"blocks": {
						"read_only_allow_delete": null
					}
				}
			}`)
}
