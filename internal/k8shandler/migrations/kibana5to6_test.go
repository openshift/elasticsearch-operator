package migrations

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/openshift/elasticsearch-operator/internal/elasticsearch"
	"github.com/openshift/elasticsearch-operator/test/helpers"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Migrating", func() {
	defer GinkgoRecover()

	var (
		chatter   *helpers.FakeElasticsearchChatter
		client    elasticsearch.Client
		k8sClient = fake.NewFakeClient()
	)

	const (
		esCluster   = "elasticsearch"
		esNamespace = "openshift-logging"
	)

	Describe("index `.kibana` into `.kibana-6`", func() {
		Context("updating the settings of the old `.kibana` index", func() {
			It("should sets it index to read only", func() {
				chatter = helpers.NewFakeElasticsearchChatter(map[string]helpers.FakeElasticsearchResponses{
					".kibana/_settings": {
						{
							StatusCode: 200,
							Body:       `{"index": {"blocks": {"write": false}}}`,
						},
						{
							StatusCode: 200,
							Body:       "{}",
						},
					},
				})
				client = helpers.NewFakeElasticsearchClient(esCluster, esNamespace, k8sClient, chatter)

				kr := migrationRequest{
					client:   k8sClient,
					esClient: client,
				}

				Expect(kr.setKibanaIndexReadOnly()).Should(Succeed())

				// Get Index settings first
				req, found := chatter.GetRequest(".kibana/_settings")
				Expect(found).To(BeTrue())
				Expect(req.Method).To(Equal(http.MethodGet))

				// Update Index settings
				req, found = chatter.GetRequest(".kibana/_settings")
				Expect(found).To(BeTrue())
				Expect(req.Method).To(Equal(http.MethodPut))
				Expect(req.Body).Should(MatchJSON(expectedKibanaROPayload))
			})

			It("should skip updating the settings if already read-only", func() {
				chatter = helpers.NewFakeElasticsearchChatter(map[string]helpers.FakeElasticsearchResponses{
					".kibana/_settings": {
						{
							StatusCode: 200,
							Body:       `{".kibana": {"settings": {"index": {"blocks": {"write": true}}}}}`,
						},
					},
				})
				client = helpers.NewFakeElasticsearchClient(esCluster, esNamespace, k8sClient, chatter)

				kr := migrationRequest{
					client:   k8sClient,
					esClient: client,
				}

				Expect(kr.setKibanaIndexReadOnly()).Should(Succeed())

				// Get Index settings first
				req, found := chatter.GetRequest(".kibana/_settings")
				Expect(found).To(BeTrue())
				Expect(req.Method).To(Equal(http.MethodGet))
				Expect(req.Body).To(BeEmpty())
			})
		})

		Context("creating the new index `.kibana-6`", func() {
			It("should succeed if not existing", func() {
				chatter = helpers.NewFakeElasticsearchChatter(map[string]helpers.FakeElasticsearchResponses{
					".kibana-6": {
						{
							StatusCode: 404,
							Body:       `{}`,
						},
						{
							StatusCode: 200,
							Body:       "{}",
						},
					},
				})
				client = helpers.NewFakeElasticsearchClient(esCluster, esNamespace, k8sClient, chatter)

				kr := migrationRequest{
					client:   k8sClient,
					esClient: client,
				}

				Expect(kr.createNewKibana6Index()).Should(Succeed())

				// Check if index exists
				req, found := chatter.GetRequest(".kibana-6")
				Expect(found).To(BeTrue())
				Expect(req.Method).To(Equal(http.MethodGet))

				req, found = chatter.GetRequest(".kibana-6")
				Expect(found).To(BeTrue())
				Expect(req.Method).To(Equal(http.MethodPut))
				Expect(req.Body).Should(MatchJSON(expectedKibana6Payload))
			})

			It("should skip if index existing", func() {
				chatter = helpers.NewFakeElasticsearchChatter(map[string]helpers.FakeElasticsearchResponses{
					".kibana-6": {
						{
							StatusCode: 200,
							Body:       `{"Settings": {}}`,
						},
					},
				})
				client = helpers.NewFakeElasticsearchClient(esCluster, esNamespace, k8sClient, chatter)

				kr := migrationRequest{
					client:   k8sClient,
					esClient: client,
				}

				Expect(kr.createNewKibana6Index()).Should(Succeed())

				// Check if index exists
				req, found := chatter.GetRequest(".kibana-6")
				Expect(found).To(BeTrue())
				Expect(req.Method).To(Equal(http.MethodGet))
			})
		})

		Context("re-indexing `.kibana` to `.kibana-6`", func() {
			It("should succeed if migration not completed", func() {
				chatter = helpers.NewFakeElasticsearchChatter(map[string]helpers.FakeElasticsearchResponses{
					"_cat/indices/.kibana-6?format=json": {
						{
							StatusCode: 200,
							Body:       `[{"health":"green","status":"open","index":".kibana-6","uuid":"KNegGDiRSs6dxWzdxWqkaQ","pri":"1","rep":"1","docs.count":"0","docs.deleted":"0","store.size":"6.4kb","pri.store.size":"3.2kb"}]`,
						},
					},
					"_reindex": {
						{
							StatusCode: 200,
							Body:       "{}",
						},
					},
				})
				client = helpers.NewFakeElasticsearchClient(esCluster, esNamespace, k8sClient, chatter)

				kr := migrationRequest{
					client:   k8sClient,
					esClient: client,
				}

				Expect(kr.reIndexIntoKibana6()).Should(Succeed())

				req, found := chatter.GetRequest("_cat/indices/.kibana-6?format=json")
				Expect(found).To(BeTrue())
				Expect(req.Method).To(Equal(http.MethodGet))

				req, found = chatter.GetRequest("_reindex")
				Expect(found).To(BeTrue())
				Expect(req.Method).To(Equal(http.MethodPost))
			})

			It("should skip if migration completed", func() {
				chatter = helpers.NewFakeElasticsearchChatter(map[string]helpers.FakeElasticsearchResponses{
					"_cat/indices/.kibana-6?format=json": {
						{
							StatusCode: 200,
							Body:       `[{"health":"green","status":"open","index":".kibana-6","uuid":"KNegGDiRSs6dxWzdxWqkaQ","pri":"1","rep":"1","docs.count":"1","docs.deleted":"0","store.size":"6.4kb","pri.store.size":"3.2kb"}]`,
						},
					},
					"_reindex": {
						{
							StatusCode: 200,
							Body:       "{}",
						},
					},
				})
				client = helpers.NewFakeElasticsearchClient(esCluster, esNamespace, k8sClient, chatter)

				kr := migrationRequest{
					client:   k8sClient,
					esClient: client,
				}

				Expect(kr.reIndexIntoKibana6()).Should(Succeed())

				req, found := chatter.GetRequest("_cat/indices/.kibana-6?format=json")
				Expect(found).To(BeTrue())
				Expect(req.Method).To(Equal(http.MethodGet))

				_, found = chatter.GetRequest("_reindex")
				Expect(found).To(BeFalse())
			})
		})

		Context("aliasing `.kibana` to `.kibana-6`", func() {
			It("should succeed if not set", func() {
				chatter = helpers.NewFakeElasticsearchChatter(map[string]helpers.FakeElasticsearchResponses{
					"_alias/.kibana": {
						{
							StatusCode: 404,
							Body:       "{}",
						},
					},
					"_aliases": {
						{
							StatusCode: 200,
							Body:       "{}",
						},
					},
				})
				client = helpers.NewFakeElasticsearchClient(esCluster, esNamespace, k8sClient, chatter)

				kr := migrationRequest{
					client:   k8sClient,
					esClient: client,
				}

				Expect(kr.aliasKibana()).Should(Succeed())

				req, found := chatter.GetRequest("_alias/.kibana")
				Expect(found).To(BeTrue())
				Expect(req.Method).To(Equal(http.MethodGet))

				req, found = chatter.GetRequest("_aliases")
				Expect(found).To(BeTrue())
				Expect(req.Method).To(Equal(http.MethodPost))
				Expect(req.Body).Should(MatchJSON(expectedAliasesPayload))
			})
		})

		It("should migrate the `.kibana` index to `.kibana-6`", func() {
			chatter = helpers.NewFakeElasticsearchChatter(map[string]helpers.FakeElasticsearchResponses{
				"_cluster/stats": {
					{
						StatusCode: 200,
						Body:       `{"nodes": {"versions": ["6.8.1"]}}`,
					},
				},
				"_cat/indices/.kibana-6?format=json": {
					{
						StatusCode: 200,
						Body:       `[{"health":"green","status":"open","index":".kibana-6","uuid":"KNegGDiRSs6dxWzdxWqkaQ","pri":"1","rep":"1","docs.count":"0","docs.deleted":"0","store.size":"6.4kb","pri.store.size":"3.2kb"}]`,
					},
				},
				"_alias/.kibana": {
					// Initial check if migration complete
					{
						StatusCode: 404,
						Body:       "{}",
					},
					// Check migration complete before switching alias
					{
						StatusCode: 404,
						Body:       "{}",
					},
				},
				".kibana/_settings": {
					{
						StatusCode: 200,
						Body:       `{"index": {"blocks": {"write": false}}}`,
					},
					{
						StatusCode: 200,
						Body:       "{}",
					},
				},
				".kibana-6": {
					{
						StatusCode: 404,
						Body:       `{}`,
					},
					{
						StatusCode: 200,
						Body:       "{}",
					},
				},
				"_reindex": {
					{
						StatusCode: 200,
						Body:       "{}",
					},
				},
				"_aliases": {
					{
						StatusCode: 200,
						Body:       "{}",
					},
				},
			})
			client = helpers.NewFakeElasticsearchClient(esCluster, esNamespace, k8sClient, chatter)

			kr := migrationRequest{
				client:   k8sClient,
				esClient: client,
			}

			Expect(kr.reIndexKibana5to6()).Should(Succeed())

			expectedReq := map[string][]int{
				"_cluster/stats":                     {1},
				"_alias/.kibana":                     {2, 9}, // Check migration completed
				"_cat/indices/.kibana-6?format=json": {7},    // Check index docs count before re-index
				".kibana/_settings":                  {3, 4}, // Check & update read-only
				".kibana-6":                          {5, 6}, // Check & create new index
				"_reindex":                           {8},    // ReIndex
				"_aliases":                           {10},   // Set alias
			}

			for key, seqNos := range expectedReq {
				seqNo := seqNos[0]
				expectedReq[key] = seqNos[1:]

				req, found := chatter.GetRequest(key)
				Expect(found).To(BeTrue())
				Expect(req.SeqNo).To(Equal(seqNo), fmt.Sprintf("URI: %s", req.URI))
			}
		})
	})
})

const (
	expectedKibanaROPayload = `
{
  "index": {
    "blocks": {
      "write": true,
      "read_only_allow_delete": null
    }
  }
}
`
	expectedKibana6Payload = `
{
  "settings" : {
    "index": {
	  "number_of_shards" : "1",
      "format": 6,
      "mapper": {
        "dynamic": false
      }
    }
  },
  "mappings" : {
    "doc": {
      "properties": {
        "type": {
          "type": "keyword"
        },
        "updated_at": {
          "type": "date"
        },
        "config": {
          "properties": {
            "buildNum": {
              "type": "keyword"
            }
          }
        },
        "index-pattern": {
          "properties": {
            "fieldFormatMap": {
              "type": "text"
            },
            "fields": {
              "type": "text"
            },
            "intervalName": {
              "type": "keyword"
            },
            "notExpandable": {
              "type": "boolean"
            },
            "sourceFilters": {
              "type": "text"
            },
            "timeFieldName": {
              "type": "keyword"
            },
            "title": {
              "type": "text"
            }
          }
        },
        "visualization": {
          "properties": {
            "description": {
              "type": "text"
            },
            "kibanaSavedObjectMeta": {
              "properties": {
                "searchSourceJSON": {
                  "type": "text"
                }
              }
            },
            "savedSearchId": {
              "type": "keyword"
            },
            "title": {
              "type": "text"
            },
            "uiStateJSON": {
              "type": "text"
            },
            "version": {
              "type": "integer"
            },
            "visState": {
              "type": "text"
            }
          }
        },
        "search": {
          "properties": {
            "columns": {
              "type": "keyword"
            },
            "description": {
              "type": "text"
            },
            "hits": {
              "type": "integer"
            },
            "kibanaSavedObjectMeta": {
              "properties": {
                "searchSourceJSON": {
                  "type": "text"
                }
              }
            },
            "sort": {
              "type": "keyword"
            },
            "title": {
              "type": "text"
            },
            "version": {
              "type": "integer"
            }
          }
        },
        "dashboard": {
          "properties": {
            "description": {
              "type": "text"
            },
            "hits": {
              "type": "integer"
            },
            "kibanaSavedObjectMeta": {
              "properties": {
                "searchSourceJSON": {
                  "type": "text"
                }
              }
            },
            "optionsJSON": {
              "type": "text"
            },
            "panelsJSON": {
              "type": "text"
            },
            "refreshInterval": {
              "properties": {
                "display": {
                  "type": "keyword"
                },
                "pause": {
                  "type": "boolean"
                },
                "section": {
                  "type": "integer"
                },
                "value": {
                  "type": "integer"
                }
              }
            },
            "timeFrom": {
              "type": "keyword"
            },
            "timeRestore": {
              "type": "boolean"
            },
            "timeTo": {
              "type": "keyword"
            },
            "title": {
              "type": "text"
            },
            "uiStateJSON": {
              "type": "text"
            },
            "version": {
              "type": "integer"
            }
          }
        },
        "url": {
          "properties": {
            "accessCount": {
              "type": "long"
            },
            "accessDate": {
              "type": "date"
            },
            "createDate": {
              "type": "date"
            },
            "url": {
              "type": "text",
              "fields": {
                "keyword": {
                  "type": "keyword",
                  "ignore_above": 2048
                }
              }
            }
          }
        },
        "server": {
          "properties": {
            "uuid": {
              "type": "keyword"
            }
          }
        },
        "timelion-sheet": {
          "properties": {
            "description": {
              "type": "text"
            },
            "hits": {
              "type": "integer"
            },
            "kibanaSavedObjectMeta": {
              "properties": {
                "searchSourceJSON": {
                  "type": "text"
                }
              }
            },
            "timelion_chart_height": {
              "type": "integer"
            },
            "timelion_columns": {
              "type": "integer"
            },
            "timelion_interval": {
              "type": "keyword"
            },
            "timelion_other_interval": {
              "type": "keyword"
            },
            "timelion_rows": {
              "type": "integer"
            },
            "timelion_sheet": {
              "type": "text"
            },
            "title": {
              "type": "text"
            },
            "version": {
              "type": "integer"
            }
          }
        },
        "graph-workspace": {
          "properties": {
            "description": {
              "type": "text"
            },
            "kibanaSavedObjectMeta": {
              "properties": {
                "searchSourceJSON": {
                  "type": "text"
                }
              }
            },
            "numLinks": {
              "type": "integer"
            },
            "numVertices": {
              "type": "integer"
            },
            "title": {
              "type": "text"
            },
            "version": {
              "type": "integer"
            },
            "wsState": {
              "type": "text"
            }
          }
        }
      }
    }
  }
}
`

	expectedAliasesPayload = `
{
  "actions" : [
    { "add":  { "index": ".kibana-6", "alias": ".kibana" } },
    { "remove_index": { "index": ".kibana" } }
  ]
}
`
)
