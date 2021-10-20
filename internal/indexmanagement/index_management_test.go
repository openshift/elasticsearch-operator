package indexmanagement

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	elasticsearch "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/internal/constants"
	"github.com/openshift/elasticsearch-operator/test/helpers"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
		request = &IndexManagementRequest{
			client: fake.NewFakeClient(),
			cluster: &elasticsearch.Elasticsearch{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "elasticsearch",
					Namespace: "openshift-logging",
				},
				Spec: elasticsearch.ElasticsearchSpec{
					RedundancyPolicy: elasticsearch.SingleRedundancy,
					Nodes: []elasticsearch.ElasticsearchNode{
						{Roles: []elasticsearch.ElasticsearchNodeRole{elasticsearch.ElasticsearchRoleData}, NodeCount: 1},
						{Roles: []elasticsearch.ElasticsearchNodeRole{elasticsearch.ElasticsearchRoleData}, NodeCount: 1},
						{Roles: []elasticsearch.ElasticsearchNodeRole{elasticsearch.ElasticsearchRoleData}, NodeCount: 1},
					},
				},
			},
		}
	)

	Describe("#CreateOrUpdateIndexManagement", func() {
		Context("when IndexManagement is not spec'd", func() {
			It("should process the resource as a noop", func() {
				Expect(request.createOrUpdateIndexManagement()).To(BeNil())
			})
		})

		Context("when elasticsearch pods", func() {
			var (
				req    *IndexManagementRequest
				esPods []runtime.Object
			)

			BeforeEach(func() {
				esPods = []runtime.Object{
					&corev1.Pod{

						ObjectMeta: metav1.ObjectMeta{
							Name:      "elasticsearch-deadbeef-cdm-acabacab-1",
							Namespace: "openshift-logging",
							Labels: map[string]string{
								"cluster-name": "elasticsearch",
								"component":    "elasticsearch",
							},
						},
					},
				}

				req = &IndexManagementRequest{
					client: fake.NewFakeClient(),
					cluster: &elasticsearch.Elasticsearch{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "elasticsearch",
							Namespace: "openshift-logging",
						},
						Spec: elasticsearch.ElasticsearchSpec{
							IndexManagement: &elasticsearch.IndexManagementSpec{
								Policies: []elasticsearch.IndexManagementPolicySpec{
									{
										Name:         "infra-policy",
										PollInterval: elasticsearch.TimeUnit("1m"),
										Phases: elasticsearch.IndexManagementPhasesSpec{
											Hot: &elasticsearch.IndexManagementHotPhaseSpec{
												Actions: elasticsearch.IndexManagementActionsSpec{
													Rollover: &elasticsearch.IndexManagementActionSpec{
														MaxAge: elasticsearch.TimeUnit("2m"),
													},
												},
											},
											Delete: &elasticsearch.IndexManagementDeletePhaseSpec{
												MinAge: elasticsearch.TimeUnit("5m"),
											},
										},
									},
								},
								Mappings: []elasticsearch.IndexManagementPolicyMappingSpec{
									{
										Name:      "infra",
										PolicyRef: "infra-policy",
										Aliases:   []string{"infra", "logs.infra"},
									},
								},
							},
						},
					},
				}
			})

			It("should suspend all cronjobs when non available", func() {
				Expect(req.createOrUpdateIndexManagement()).To(BeNil())

				cj := &batchv1beta1.CronJob{}
				key := client.ObjectKey{Name: "elasticsearch-im-infra", Namespace: "openshift-logging"}
				Expect(req.client.Get(context.TODO(), key, cj)).To(BeNil())
				Expect(*cj.Spec.Suspend).To(BeTrue())
			})

			It("should unsuspend all cronjobs when at least on elasticsearch pod running", func() {
				req.client = fake.NewFakeClient(esPods...)
				Expect(req.createOrUpdateIndexManagement()).To(BeNil())

				cj := &batchv1beta1.CronJob{}
				key := client.ObjectKey{Name: "elasticsearch-im-infra", Namespace: "openshift-logging"}
				Expect(req.client.Get(context.TODO(), key, cj)).To(BeNil())
				Expect(*cj.Spec.Suspend).To(BeFalse())
			})
		})
	})

	Describe("#cullIndexManagement", func() {
		var (
			mappings  []elasticsearch.IndexManagementPolicyMappingSpec
			policyMap = elasticsearch.PolicyMap{}
		)
		BeforeEach(func() {
			mappings = []elasticsearch.IndexManagementPolicyMappingSpec{mapping}
			chatter = helpers.NewFakeElasticsearchChatter(
				map[string]helpers.FakeElasticsearchResponses{
					"_template": {
						{
							Error:      nil,
							StatusCode: 200,
							Body: `{
                                "ocp-gen-my-deleted-one": {},
                                "ocp-gen-node.infra": {},
                                "user-created": {}
                            }`,
						},
					},
					"_template/ocp-gen-my-deleted-one": {
						{
							Error:      nil,
							StatusCode: 200,
							Body: `{
                                "acknowleged": true
                            }`,
						},
					},
				},
			)
			request.esClient = helpers.NewFakeElasticsearchClient("elastichsearch", "openshift-logging", request.client, chatter)
		})
		Context("when an Elasticsearch template does not have an associated policy mapping", func() {
			It("should be culled from Elasticsearch", func() {
				request.cullIndexManagement(mappings, policyMap)
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
			templateURI := fmt.Sprintf("_template/common.*,%s-*", constants.OcpTemplatePrefix)

			chatter = helpers.NewFakeElasticsearchChatter(
				map[string]helpers.FakeElasticsearchResponses{
					templateURI: {
						{
							Error:      nil,
							StatusCode: 200,
							Body:       `{}`,
						},
					},
					"_template/ocp-gen-node.infra": {
						{
							Error:      nil,
							StatusCode: 200,
							Body:       `{ "acknowledged": true}`,
						},
					},
				},
			)
			request.esClient = helpers.NewFakeElasticsearchClient("elasticsearch", "openshift-logging", request.client, chatter)
		})
		It("should create an elasticsearch index template to support the index", func() {
			Expect(request.createOrUpdateIndexTemplate(mapping)).To(BeNil())
			req, _ := chatter.GetRequest("_template/ocp-gen-node.infra")
			helpers.ExpectJSON(req.Body).ToEqual(
				`{
					"aliases": {
						"infra": {},
						"node.infra" : {}
					},
					"settings": {
						"index": {
							"number_of_replicas": "1",
							"number_of_shards": "3"
						}
					},
					"index_patterns": ["node.infra*"]
				}`)
		})
	})
	Describe("#initializeIndexIfNeeded", func() {
		Context("when an index matching the pattern for rolling indices does not exist", func() {
			It("should create it", func() {
				chatter = helpers.NewFakeElasticsearchChatter(
					map[string]helpers.FakeElasticsearchResponses{
						"_alias/node.infra-write": {
							{
								Error:      nil,
								StatusCode: 404,
								Body:       `{ "error": "some error", "status": 404}`,
							},
						},
						"node.infra-000001": {
							{
								Error:      nil,
								StatusCode: 200,
								Body:       `{ "acknowledged": true}`,
							},
						},
					},
				)
				request.esClient = helpers.NewFakeElasticsearchClient("elastichsearch", "openshift-logging", request.client, chatter)
				Expect(request.initializeIndexIfNeeded(mapping)).To(BeNil())
				req, _ := chatter.GetRequest("node.infra-000001")
				helpers.ExpectJSON(req.Body).ToEqual(
					`{
						"aliases": {
							"infra": {},
							"node.infra" : {},
							"node.infra-write": {
								"is_write_index": true
							}
						},
						"settings": {
							"index": {
								"number_of_replicas": "1",
								"number_of_shards": "3"
							}
						}
					}`)
			})
		})
		Context("when an index matching the pattern for rolling indices exist", func() {
			It("should not try creating it", func() {
				chatter = helpers.NewFakeElasticsearchChatter(
					map[string]helpers.FakeElasticsearchResponses{
						"_alias/node.infra-write": {
							{
								Error:      nil,
								StatusCode: 200,
								Body: `{
                                    "node.infra-000003": {},
                                    "node.infra-000004": {}
                                }`,
							},
						},
						"node.infra-000001": {
							{
								Error:      nil,
								StatusCode: 400,
								Body:       `{ "error": "exists"}`,
							},
						},
					},
				)
				request.esClient = helpers.NewFakeElasticsearchClient("elastichsearch", "openshift-logging", request.client, chatter)
				Expect(request.initializeIndexIfNeeded(mapping)).To(BeNil())
				_, found := chatter.GetRequest("node.infra-000001")
				Expect(found).To(BeFalse(), "to not make a create request")
			})
		})
	})
})
