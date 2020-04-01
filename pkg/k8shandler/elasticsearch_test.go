package k8shandler

import (
	"fmt"
	"testing"

	elasticsearch "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/test/helpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	chatter *helpers.FakeElasticsearchChatter

	esTestRequest = &ElasticsearchRequest{
		client: fake.NewFakeClient(),
		cluster: &elasticsearch.Elasticsearch{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "elasticsearch",
				Namespace: "openshift-logging",
			},
		},
		FnCurlEsService: func(clusterName, namespace string, payload *esCurlStruct, client client.Client) {
			chatter.Requests[payload.URI] = payload.RequestBody
			if val, found := chatter.GetResponse(payload.URI); found {
				payload.Error = val.Error
				payload.StatusCode = val.StatusCode
				payload.ResponseBody = val.BodyAsResponseBody()
			} else {
				payload.Error = fmt.Errorf("No fake response found for uri %q: %v", payload.URI, payload)
			}
		},
	}
)

func TestHeaderGenEmptyToken(t *testing.T) {
	tokenFile := "../../test/files/emptyToken"

	_, ok := readSAToken(tokenFile)

	if ok {
		t.Errorf("Expected to be unable to read file [%s] due to empty file but could", tokenFile)
	}
}

func TestHeaderGenWithToken(t *testing.T) {
	tokenFile := "../../test/files/testToken"

	expected := "test\n"

	actual, ok := readSAToken(tokenFile)

	if !ok {
		t.Errorf("Expected to be able to read file [%s] but couldn't", tokenFile)

	} else {
		if actual != expected {
			t.Errorf("Expected %q but got %q", expected, actual)
		}
	}
}

func TestHeaderGenWithNoToken(t *testing.T) {
	tokenFile := "../../test/files/errorToken"

	_, ok := readSAToken(tokenFile)

	if ok {
		t.Errorf("Expected to be unable to read file [%s]", tokenFile)
	}
}

func TestAliasAllNeededPass(t *testing.T) {
	chatter = helpers.NewFakeElasticsearchChatter(
		map[string]helpers.FakeElasticsearchResponse{
			"project.*,.operations.*/_alias": {
				Error:      nil,
				StatusCode: 200,
				Body: `{
					"project.test": {
						"aliases": {}
					},
					".operations.date": {
						"aliases": {}
					}
				}`,
			},
			"project.test/_alias/app": {
				Error:      nil,
				StatusCode: 200,
				Body: `{
					"acknowledged": true
				}`,
			},
			".operations.date/_alias/infra": {
				Error:      nil,
				StatusCode: 200,
				Body: `{
					"acknowledged": true
				}`,
			},
		},
	)

	successful := esTestRequest.AddAliasForOldIndices()
	if !successful {
		t.Errorf("Expected creation of aliases to succeed")
	}
}

func TestAliasProjectNeededPass(t *testing.T) {

	chatter = helpers.NewFakeElasticsearchChatter(
		map[string]helpers.FakeElasticsearchResponse{
			"project.*,.operations.*/_alias": {
				Error:      nil,
				StatusCode: 200,
				Body: `{
					"project.test": {
						"aliases": {}
					}
				}`,
			},
			"project.test/_alias/app": {
				Error:      nil,
				StatusCode: 200,
				Body: `{
					"acknowledged": true
				}`,
			},
		},
	)

	successful := esTestRequest.AddAliasForOldIndices()
	if !successful {
		t.Errorf("Expected creation of aliases to succeed")
	}
}

func TestAliasOperationsNeededPass(t *testing.T) {

	chatter = helpers.NewFakeElasticsearchChatter(
		map[string]helpers.FakeElasticsearchResponse{
			"project.*,.operations.*/_alias": {
				Error:      nil,
				StatusCode: 200,
				Body: `{
					".operations.date": {
						"aliases": {}
					}
				}`,
			},
			".operations.date/_alias/infra": {
				Error:      nil,
				StatusCode: 200,
				Body: `{
					"acknowledged": true
				}`,
			},
		},
	)

	successful := esTestRequest.AddAliasForOldIndices()
	if !successful {
		t.Errorf("Expected creation of aliases to succeed")
	}
}

func TestAliasAllNeededFail(t *testing.T) {

	chatter = helpers.NewFakeElasticsearchChatter(
		map[string]helpers.FakeElasticsearchResponse{
			"project.*,.operations.*/_alias": {
				Error:      nil,
				StatusCode: 200,
				Body: `{
					"project.test": {
						"aliases": {}
					},
					".operations.date": {
						"aliases": {}
					}
				}`,
			},
			"project.test/_alias/app": {
				Error:      nil,
				StatusCode: 500,
				Body: `{
					"acknowledged": false
				}`,
			},
			".operations.date/_alias/infra": {
				Error:      nil,
				StatusCode: 500,
				Body: `{
					"acknowledged": false
				}`,
			},
		},
	)

	successful := esTestRequest.AddAliasForOldIndices()
	if successful {
		t.Errorf("Expected creation of aliases to fail")
	}
}

func TestAliasProjectNeededFail(t *testing.T) {

	chatter = helpers.NewFakeElasticsearchChatter(
		map[string]helpers.FakeElasticsearchResponse{
			"project.*,.operations.*/_alias": {
				Error:      nil,
				StatusCode: 200,
				Body: `{
					"project.test": {
						"aliases": {}
					}
				}`,
			},
			"project.test/_alias/app": {
				Error:      nil,
				StatusCode: 500,
				Body: `{
					"acknowledged": false
				}`,
			},
		},
	)

	successful := esTestRequest.AddAliasForOldIndices()
	if successful {
		t.Errorf("Expected creation of aliases to fail")
	}
}

func TestAliasOperationsNeededFail(t *testing.T) {

	chatter = helpers.NewFakeElasticsearchChatter(
		map[string]helpers.FakeElasticsearchResponse{
			"project.*,.operations.*/_alias": {
				Error:      nil,
				StatusCode: 200,
				Body: `{
					".operations.date": {
						"aliases": {}
					}
				}`,
			},
			".operations.date/_alias/infra": {
				Error:      nil,
				StatusCode: 500,
				Body: `{
					"acknowledged": false
				}`,
			},
		},
	)

	successful := esTestRequest.AddAliasForOldIndices()
	if successful {
		t.Errorf("Expected creation of aliases to fail")
	}
}

func TestAliasAllNotNeeded(t *testing.T) {

	chatter = helpers.NewFakeElasticsearchChatter(
		map[string]helpers.FakeElasticsearchResponse{
			"project.*,.operations.*/_alias": {
				Error:      nil,
				StatusCode: 200,
				Body: `{
					"project.test": {
						"aliases": {
							"app": {}
						}
					},
					".operations.date": {
						"aliases": {
							"infra": {}
						}
					}
				}`,
			},
			"project.test/_alias/app": {
				Error:      nil,
				StatusCode: 500,
				Body: `{
					"acknowledged": false
				}`,
			},
			".operations.date/_alias/infra": {
				Error:      nil,
				StatusCode: 500,
				Body: `{
					"acknowledged": false
				}`,
			},
		},
	)

	successful := esTestRequest.AddAliasForOldIndices()
	if !successful {
		t.Errorf("Expected creation of aliases to succeed")
	}
}

func TestAliasProjectNotNeeded(t *testing.T) {

	chatter = helpers.NewFakeElasticsearchChatter(
		map[string]helpers.FakeElasticsearchResponse{
			"project.*,.operations.*/_alias": {
				Error:      nil,
				StatusCode: 200,
				Body: `{
					"project.test": {
						"aliases": {
							"app": {}
						}
					}
				}`,
			},
			"project.test/_alias/app": {
				Error:      nil,
				StatusCode: 500,
				Body: `{
					"acknowledged": false
				}`,
			},
		},
	)

	successful := esTestRequest.AddAliasForOldIndices()
	if !successful {
		t.Errorf("Expected creation of aliases to succeed")
	}
}

func TestAliasOperationsNotNeeded(t *testing.T) {

	chatter = helpers.NewFakeElasticsearchChatter(
		map[string]helpers.FakeElasticsearchResponse{
			"project.*,.operations.*/_alias": {
				Error:      nil,
				StatusCode: 200,
				Body: `{
					".operations.date": {
						"aliases": {
							"infra": {}
						}
					}
				}`,
			},
			".operations.date/_alias/infra": {
				Error:      nil,
				StatusCode: 500,
				Body: `{
					"acknowledged": false
				}`,
			},
		},
	)

	successful := esTestRequest.AddAliasForOldIndices()
	if !successful {
		t.Errorf("Expected creation of aliases to succeed")
	}
}
