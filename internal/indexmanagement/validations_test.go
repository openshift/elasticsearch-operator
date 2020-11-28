package indexmanagement

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	esapi "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/internal/utils"

	"github.com/openshift/elasticsearch-operator/test/helpers"
)

var _ = Describe("Index Management", func() {
	defer GinkgoRecover()

	var (
		cluster                           *esapi.Elasticsearch
		result                            *esapi.IndexManagementSpec
		VerifyAndNormalizeIndexManagement = func() {
			result = VerifyAndNormalize(cluster)
		}
	)

	BeforeEach(func() {
		result = &esapi.IndexManagementSpec{}
		cluster = &esapi.Elasticsearch{
			Spec: esapi.ElasticsearchSpec{
				IndexManagement: &esapi.IndexManagementSpec{
					Policies: []esapi.IndexManagementPolicySpec{
						{
							Name:         "my-policy",
							PollInterval: "10s",
							Phases: esapi.IndexManagementPhasesSpec{
								Hot: &esapi.IndexManagementHotPhaseSpec{
									Actions: esapi.IndexManagementActionsSpec{
										Rollover: &esapi.IndexManagementActionSpec{
											MaxAge: "1d",
										},
									},
								},
								Delete: &esapi.IndexManagementDeletePhaseSpec{
									MinAge: "7d",
								},
							},
						},
					},
					Mappings: []esapi.IndexManagementPolicyMappingSpec{
						{
							Name:      "foo",
							PolicyRef: "my-policy",
							Aliases:   []string{"somevalue"},
						},
					},
				},
			},
		}
	})

	Describe("#VerifyAndNormalizeIndexManagement", func() {
		Context("when IndexManagment is not spec'd", func() {
			It("should report a Dropped state", func() {
				cluster.Spec.IndexManagement = nil
				VerifyAndNormalizeIndexManagement()
				expectStatus(cluster).
					hasState(esapi.IndexManagementStateDropped).
					withMessage("IndexManagement was not defined")
				Expect(result).To(BeNil(), "Exp to result in nothing to manage")
			})
		})
		Context("when IndexManagment is spec'd with no polices or mappings", func() {
			It("should report a Dropped state", func() {
				cluster.Spec.IndexManagement.Policies = []esapi.IndexManagementPolicySpec{}
				cluster.Spec.IndexManagement.Mappings = []esapi.IndexManagementPolicyMappingSpec{}
				VerifyAndNormalizeIndexManagement()
				expectStatus(cluster).
					hasState(esapi.IndexManagementStateDropped).
					withMessage("IndexManagement was not defined")
				Expect(result).To(BeNil(), "Exp to result in nothing to manage")
			})
		})

		Context("when there are no validation failures", func() {
			It("should result in an Accepted state", func() {
				VerifyAndNormalizeIndexManagement()
				expectStatus(cluster).
					hasState(esapi.IndexManagementStateAccepted).
					withReason(esapi.IndexManagementStatusReasonPassed)
				Expect(result).ToNot(BeNil(), "Expected normalized IndexManagement")
				jsonResult, err := utils.ToJSON(result)
				Expect(err).To(BeNil())
				helpers.ExpectJSON(jsonResult).ToEqual(
					`{
						"mappings": [
							{
								"aliases": [
									"somevalue"
								],
								"name": "foo",
								"policyRef": "my-policy"
							}
						],
						"policies": [
							{
								"name": "my-policy",
								"phases": {
									"delete": {
										"minAge": "7d"
									},
									"hot": {
										"actions": {
											"rollover": {
												"maxAge": "1d"
											}
										}
									}
								},
								"pollInterval": "10s"
							}
						]
					}`)
			})
		})

		Context("when there are some validation failures", func() {
			Context("for mappings", func() {
				It("should result in an Degraded state", func() {
					cluster.Spec.IndexManagement.Mappings = append(cluster.Spec.IndexManagement.Mappings,
						esapi.IndexManagementPolicyMappingSpec{},
					)
					VerifyAndNormalizeIndexManagement()
					expectStatus(cluster).
						hasState(esapi.IndexManagementStateDegraded).
						withReason(esapi.IndexManagementStatusReasonValidationFailed)
					Expect(result).ToNot(BeNil(), "Expected normalized IndexManagement")
					jsonResult, err := utils.ToJSON(result)
					Expect(err).To(BeNil())
					helpers.ExpectJSON(jsonResult).ToEqual(
						`{
							"mappings": [
								{
									"aliases": [
										"somevalue"
									],
									"name": "foo",
									"policyRef": "my-policy"
								}
							],
							"policies": [
								{
									"name": "my-policy",
									"phases": {
										"delete": {
											"minAge": "7d"
										},
										"hot": {
											"actions": {
												"rollover": {
													"maxAge": "1d"
												}
											}
										}
									},
									"pollInterval": "10s"
								}
							]
						}`)
				})
			})
			Context("for polices", func() {
				It("should result in an Degraded state", func() {
					cluster.Spec.IndexManagement.Policies = append(cluster.Spec.IndexManagement.Policies,
						esapi.IndexManagementPolicySpec{},
					)
					VerifyAndNormalizeIndexManagement()
					expectStatus(cluster).
						hasState(esapi.IndexManagementStateDegraded).
						withReason(esapi.IndexManagementStatusReasonValidationFailed)
					Expect(result).ToNot(BeNil(), "Expected normalized IndexManagement")
					jsonResult, err := utils.ToJSON(result)
					Expect(err).To(BeNil())
					helpers.ExpectJSON(jsonResult).ToEqual(
						`{
							"mappings": [
								{
									"aliases": [
										"somevalue"
									],
									"name": "foo",
									"policyRef": "my-policy"
								}
							],
							"policies": [
								{
									"name": "my-policy",
									"phases": {
										"delete": {
											"minAge": "7d"
										},
										"hot": {
											"actions": {
												"rollover": {
													"maxAge": "1d"
												}
											}
										}
									},
									"pollInterval": "10s"
								}
							]
						}`)
				})
			})
		})
	})
})
