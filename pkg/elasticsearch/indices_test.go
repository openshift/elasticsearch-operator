package elasticsearch_test

import (
	"testing"

	testhelpers "github.com/openshift/elasticsearch-operator/test/helpers"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var fakeClient = fake.NewFakeClient()

func TestAliasAllNeededPass(t *testing.T) {
	chatter := testhelpers.NewFakeElasticsearchChatter(
		map[string]testhelpers.FakeElasticsearchResponses{
			"project.*,.operations.*/_alias": {
				{
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
			},
			"project.test/_alias/app": {
				{
					Error:      nil,
					StatusCode: 200,
					Body: `{
                      "acknowledged": true
                    }`,
				},
			},
			".operations.date/_alias/infra": {
				{
					Error:      nil,
					StatusCode: 200,
					Body: `{
                      "acknowledged": true
                    }`,
				},
			},
		},
	)
	esClient := testhelpers.NewFakeElasticsearchClient("elasticsearch", "openshift-logging", fakeClient, chatter)

	successful := esClient.AddAliasForOldIndices()
	if !successful {
		t.Errorf("Expected creation of aliases to succeed")
	}
}

func TestAliasProjectNeededPass(t *testing.T) {
	chatter := testhelpers.NewFakeElasticsearchChatter(
		map[string]testhelpers.FakeElasticsearchResponses{
			"project.*,.operations.*/_alias": {
				{
					Error:      nil,
					StatusCode: 200,
					Body: `{
                        "project.test": {
                            "aliases": {}
                        }
                    }`,
				},
			},
			"project.test/_alias/app": {
				{
					Error:      nil,
					StatusCode: 200,
					Body: `{
                        "acknowledged": true
                    }`,
				},
			},
		},
	)
	esClient := testhelpers.NewFakeElasticsearchClient("elasticsearch", "openshift-logging", fakeClient, chatter)

	successful := esClient.AddAliasForOldIndices()
	if !successful {
		t.Errorf("Expected creation of aliases to succeed")
	}
}

func TestAliasOperationsNeededPass(t *testing.T) {
	chatter := testhelpers.NewFakeElasticsearchChatter(
		map[string]testhelpers.FakeElasticsearchResponses{
			"project.*,.operations.*/_alias": {
				{
					Error:      nil,
					StatusCode: 200,
					Body: `{
                        ".operations.date": {
                            "aliases": {}
                        }
                    }`,
				},
			},
			".operations.date/_alias/infra": {
				{
					Error:      nil,
					StatusCode: 200,
					Body: `{
                        "acknowledged": true
                    }`,
				},
			},
		},
	)
	esClient := testhelpers.NewFakeElasticsearchClient("elasticsearch", "openshift-logging", fakeClient, chatter)

	successful := esClient.AddAliasForOldIndices()
	if !successful {
		t.Errorf("Expected creation of aliases to succeed")
	}
}

func TestAliasAllNeededFail(t *testing.T) {
	chatter := testhelpers.NewFakeElasticsearchChatter(
		map[string]testhelpers.FakeElasticsearchResponses{
			"project.*,.operations.*/_alias": {
				{
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
			},
			"project.test/_alias/app": {
				{
					Error:      nil,
					StatusCode: 500,
					Body: `{
                        "acknowledged": false
                    }`,
				},
			},
			".operations.date/_alias/infra": {
				{
					Error:      nil,
					StatusCode: 500,
					Body: `{
                        "acknowledged": false
                    }`,
				},
			},
		},
	)
	esClient := testhelpers.NewFakeElasticsearchClient("elasticsearch", "openshift-logging", fakeClient, chatter)

	successful := esClient.AddAliasForOldIndices()
	if successful {
		t.Errorf("Expected creation of aliases to fail")
	}
}

func TestAliasProjectNeededFail(t *testing.T) {
	chatter := testhelpers.NewFakeElasticsearchChatter(
		map[string]testhelpers.FakeElasticsearchResponses{
			"project.*,.operations.*/_alias": {
				{
					Error:      nil,
					StatusCode: 200,
					Body: `{
                        "project.test": {
                            "aliases": {}
                        }
                    }`,
				},
			},
			"project.test/_alias/app": {
				{
					Error:      nil,
					StatusCode: 500,
					Body: `{
                        "acknowledged": false
                    }`,
				},
			},
		},
	)
	esClient := testhelpers.NewFakeElasticsearchClient("elasticsearch", "openshift-logging", fakeClient, chatter)

	successful := esClient.AddAliasForOldIndices()
	if successful {
		t.Errorf("Expected creation of aliases to fail")
	}
}

func TestAliasOperationsNeededFail(t *testing.T) {
	chatter := testhelpers.NewFakeElasticsearchChatter(
		map[string]testhelpers.FakeElasticsearchResponses{
			"project.*,.operations.*/_alias": {
				{
					Error:      nil,
					StatusCode: 200,
					Body: `{
                        ".operations.date": {
                            "aliases": {}
                        }
                    }`,
				},
			},
			".operations.date/_alias/infra": {
				{
					Error:      nil,
					StatusCode: 500,
					Body: `{
                        "acknowledged": false
                    }`,
				},
			},
		},
	)
	esClient := testhelpers.NewFakeElasticsearchClient("elasticsearch", "openshift-logging", fakeClient, chatter)

	successful := esClient.AddAliasForOldIndices()
	if successful {
		t.Errorf("Expected creation of aliases to fail")
	}
}

func TestAliasAllNotNeeded(t *testing.T) {
	chatter := testhelpers.NewFakeElasticsearchChatter(
		map[string]testhelpers.FakeElasticsearchResponses{
			"project.*,.operations.*/_alias": {
				{
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
			},
			"project.test/_alias/app": {
				{
					Error:      nil,
					StatusCode: 500,
					Body: `{
                        "acknowledged": false
                    }`,
				},
			},
			".operations.date/_alias/infra": {
				{
					Error:      nil,
					StatusCode: 500,
					Body: `{
                        "acknowledged": false
                    }`,
				},
			},
		},
	)
	esClient := testhelpers.NewFakeElasticsearchClient("elasticsearch", "openshift-logging", fakeClient, chatter)

	successful := esClient.AddAliasForOldIndices()
	if !successful {
		t.Errorf("Expected creation of aliases to succeed")
	}
}

func TestAliasProjectNotNeeded(t *testing.T) {
	chatter := testhelpers.NewFakeElasticsearchChatter(
		map[string]testhelpers.FakeElasticsearchResponses{
			"project.*,.operations.*/_alias": {
				{
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
			},
			"project.test/_alias/app": {
				{
					Error:      nil,
					StatusCode: 500,
					Body: `{
                        "acknowledged": false
                    }`,
				},
			},
		},
	)
	esClient := testhelpers.NewFakeElasticsearchClient("elasticsearch", "openshift-logging", fakeClient, chatter)

	successful := esClient.AddAliasForOldIndices()
	if !successful {
		t.Errorf("Expected creation of aliases to succeed")
	}
}

func TestAliasOperationsNotNeeded(t *testing.T) {
	chatter := testhelpers.NewFakeElasticsearchChatter(
		map[string]testhelpers.FakeElasticsearchResponses{
			"project.*,.operations.*/_alias": {
				{
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
			},
			".operations.date/_alias/infra": {
				{
					Error:      nil,
					StatusCode: 500,
					Body: `{
                        "acknowledged": false
                    }`,
				},
			},
		},
	)
	esClient := testhelpers.NewFakeElasticsearchClient("elasticsearch", "openshift-logging", fakeClient, chatter)

	successful := esClient.AddAliasForOldIndices()
	if !successful {
		t.Errorf("Expected creation of aliases to succeed")
	}
}
