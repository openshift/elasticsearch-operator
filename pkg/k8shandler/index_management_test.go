package k8shandler

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	elasticsearch "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/test/helpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Index Management", func() {
	defer GinkgoRecover()
	var (
		chatter *helpers.FakeElasticsearchChatter
		mapping = elasticsearch.IndexManagementPolicyMappingSpec{
			Name:    "node.infra",
			Aliases: []string{"infra"},
		}
		request = &ElasticsearchRequest{
			client: fake.NewFakeClient(),
			cluster: &elasticsearch.Elasticsearch{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "elasticsearch",
					Namespace: "openshift-logging",
				},
				Spec: elasticsearch.ElasticsearchSpec{
					RedundancyPolicy: elasticsearch.SingleRedundancy,
					Nodes: []elasticsearch.ElasticsearchNode{
						elasticsearch.ElasticsearchNode{Roles: []elasticsearch.ElasticsearchNodeRole{elasticsearch.ElasticsearchRoleData}, NodeCount: 1},
						elasticsearch.ElasticsearchNode{Roles: []elasticsearch.ElasticsearchNodeRole{elasticsearch.ElasticsearchRoleData}, NodeCount: 1},
						elasticsearch.ElasticsearchNode{Roles: []elasticsearch.ElasticsearchNodeRole{elasticsearch.ElasticsearchRoleData}, NodeCount: 1},
					},
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

	Describe("#CreateOrUpdateIndexManagement", func() {

		Context("when IndexManagement is not spec'd", func() {
			It("should process the resource as a noop", func() {
				Expect(request.CreateOrUpdateIndexManagement()).To(BeNil())
			})

		})
	})

	Describe("#cullIndexManagement", func() {
		var (
			mappings []elasticsearch.IndexManagementPolicyMappingSpec
		)
		BeforeEach(func() {
			mappings = []elasticsearch.IndexManagementPolicyMappingSpec{mapping}
			chatter = helpers.NewFakeElasticsearchChatter(
				map[string]helpers.FakeElasticsearchResponse{
					"_template": helpers.FakeElasticsearchResponse{
						nil, 200, `{
							"ocp-gen-my-deleted-one": {},
							"ocp-gen-node.infra": {},
							"user-created": {}
						}`,
					},
					"_template/ocp-gen-my-deleted-one": helpers.FakeElasticsearchResponse{
						nil, 200, `{
							"acknowleged": true
						}`,
					},
				},
			)
		})
		Context("when a template does not have a policy mapping", func() {
			It("should be culled from Elasticsearch", func() {
				request.cullIndexManagement(mappings)
				_, found := chatter.GetRequest("_template/user-created")
				Expect(found).To(BeFalse(), "to not delete a user created template")
				_, found = chatter.GetRequest("_template/ocp-gen-node.infra")
				Expect(found).To(BeFalse(), "to not delete a template that is for a defined mapping")
				_, found = chatter.GetRequest("_template/ocp-gen-my-deleted-one")
				fmt.Printf("requests %v\n", chatter.Requests)
				Expect(found).To(BeTrue(), "_template/ocp-gen-my-deleted-one wasn't called to be deleted")
			})
		})
	})
	Describe("#createOrUpdateIndexTemplate", func() {
		BeforeEach(func() {
			chatter = helpers.NewFakeElasticsearchChatter(
				map[string]helpers.FakeElasticsearchResponse{
					"_template/ocp-gen-node.infra": helpers.FakeElasticsearchResponse{
						nil, 200, `{ "acknowledged": true}`,
					},
				},
			)
		})
		It("should create an elasticsearch index template to support the index", func() {
			Expect(request.createOrUpdateIndexTemplate(mapping)).To(BeNil())
			body, _ := chatter.GetRequest("_template/ocp-gen-node.infra")
			helpers.ExpectJson(body).ToEqual(
				`{
					"aliases": {
						"infra": {},
						"node.infra" : {}
					},
					"settings": {
						"number_of_replicas": 1,
						"number_of_shards": 3
					},
					"template": "node.infra*"
				}`)
		})
	})
	Describe("#initializeIndexIfNeeded", func() {
		Context("when an index matching the pattern for rolling indices does not exist", func() {
			It("should create it", func() {
				chatter = helpers.NewFakeElasticsearchChatter(
					map[string]helpers.FakeElasticsearchResponse{
						"_alias/node.infra-write": helpers.FakeElasticsearchResponse{
							nil, 404, `{ "error": "some error", "status": 404}`,
						},
						"node.infra-000001": helpers.FakeElasticsearchResponse{
							nil, 200, `{ "acknowledged": true}`,
						},
					},
				)
				Expect(request.initializeIndexIfNeeded(mapping)).To(BeNil())
				body, _ := chatter.GetRequest("node.infra-000001")
				helpers.ExpectJson(body).ToEqual(
					`{
						"aliases": {
							"infra": {},
							"node.infra" : {},
							"node.infra-write": {
								"is_write_index": true
							}
						},
						"settings": {
							"number_of_replicas": 1,
							"number_of_shards": 3
						}
					}`)
			})
		})
		Context("when an index matching the pattern for rolling indices exist", func() {
			It("should not try creating it", func() {
				chatter = helpers.NewFakeElasticsearchChatter(
					map[string]helpers.FakeElasticsearchResponse{
						"_alias/node.infra-write": helpers.FakeElasticsearchResponse{
							nil, 200, `{
								"node.infra-000003": {},
								"node.infra-000004": {}
							}`,
						},
						"node.infra-000001": helpers.FakeElasticsearchResponse{
							nil, 400, `{ "error": "exists"}`,
						},
					},
				)
				Expect(request.initializeIndexIfNeeded(mapping)).To(BeNil())
				_, found := chatter.GetRequest("node.infra-000001")
				Expect(found).To(BeFalse(), "to not make a create request")
			})
		})
	})
})
