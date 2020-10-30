package indexmanagement

import (
	. "github.com/onsi/ginkgo"

	esapi "github.com/openshift/elasticsearch-operator/apis/logging/v1"
)

var _ = Describe("Index Management", func() {
	defer GinkgoRecover()

	var (
		cluster *esapi.Elasticsearch
		result  *esapi.IndexManagementSpec

		validateMappingsForSpec = func(mappingSpecs ...esapi.IndexManagementPolicyMappingSpec) {
			cluster.Spec.IndexManagement.Mappings = append(cluster.Spec.IndexManagement.Mappings, mappingSpecs...)
			validateMappings(cluster, result)
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
				},
			},
			Status: esapi.ElasticsearchStatus{
				IndexManagementStatus: &esapi.IndexManagementStatus{},
			},
		}
	})

	Context("validateMappings", func() {
		Context("with no indexmanagement spec's", func() {
			It("should not error", func() {
				cluster.Spec.IndexManagement = nil
				validateMappings(cluster, result)
			})
		})
		Context("Name", func() {
			It("should spec a name", func() {
				validateMappingsForSpec(esapi.IndexManagementPolicyMappingSpec{})
				expectStatus(cluster).hasMapping("mapping[0]").
					withMappingState(esapi.IndexManagementMappingStateDropped).
					withMappingCondition(esapi.IndexManagementMappingConditionTypeName, esapi.IndexManagementMappingReasonMissing)
			})
			It("should spec a unique name", func() {
				validateMappingsForSpec(esapi.IndexManagementPolicyMappingSpec{
					Name: "foo",
				}, esapi.IndexManagementPolicyMappingSpec{
					Name: "foo",
				})
				expectStatus(cluster).hasMapping("mapping[1]").
					withMappingState(esapi.IndexManagementMappingStateDropped).
					withMappingCondition(esapi.IndexManagementMappingConditionTypeName, esapi.IndexManagementMappingReasonNonUnique)
			})
		})
		Context("PolicyRef", func() {
			It("should spec a value", func() {
				validateMappingsForSpec(esapi.IndexManagementPolicyMappingSpec{
					Name: "foo",
				})
				expectStatus(cluster).hasMapping("foo").
					withMappingState(esapi.IndexManagementMappingStateDropped).
					withMappingCondition(esapi.IndexManagementMappingConditionTypePolicyRef, esapi.IndexManagementMappingReasonMissing).
					withMappingConditionMessage("A policy mapping must reference a defined IndexManagement policy")
			})
			It("should spec a value that reference a defined policy", func() {
				validateMappingsForSpec(esapi.IndexManagementPolicyMappingSpec{
					Name:      "foo",
					PolicyRef: "foo-bar",
				})
				expectStatus(cluster).hasMapping("foo").
					withMappingState(esapi.IndexManagementMappingStateDropped).
					withMappingCondition(esapi.IndexManagementMappingConditionTypePolicyRef, esapi.IndexManagementMappingReasonMissing).
					withMappingConditionMessage("A policy mapping must reference a defined IndexManagement policy")
			})
		})
		It("should accept a valid policy mapping", func() {
			validateMappingsForSpec(esapi.IndexManagementPolicyMappingSpec{
				Name:      "foo",
				PolicyRef: "my-policy",
				Aliases:   []string{"somevalue"},
			})
			expectStatus(cluster).hasMapping("foo").
				withMappingState(esapi.IndexManagementMappingStateAccepted).
				withMappingStatusReason(esapi.IndexManagementMappingReasonConditionsMet)
		})
	})
})
