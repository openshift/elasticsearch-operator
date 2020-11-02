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

		validatePoliciesForSpec = func(policySpecs ...esapi.IndexManagementPolicySpec) {
			cluster.Spec.IndexManagement.Policies = append(cluster.Spec.IndexManagement.Policies, policySpecs...)
			validatePolicies(cluster, result)
		}
	)

	BeforeEach(func() {
		result = &esapi.IndexManagementSpec{}
		cluster = &esapi.Elasticsearch{
			Spec: esapi.ElasticsearchSpec{
				IndexManagement: &esapi.IndexManagementSpec{},
			},
			Status: esapi.ElasticsearchStatus{
				IndexManagementStatus: &esapi.IndexManagementStatus{},
			},
		}
	})

	Context("validatePolicies", func() {
		Context("with no indexmanagement spec's", func() {
			It("should not error", func() {
				cluster.Spec.IndexManagement = nil
				validatePolicies(cluster, result)
			})
		})

		Context("Name", func() {
			It("should spec a name", func() {
				validatePoliciesForSpec(esapi.IndexManagementPolicySpec{})
				expectStatus(cluster).hasPolicy("policy[0]").
					withPolicyState(esapi.IndexManagementPolicyStateDropped).
					withPolicyCondition(esapi.IndexManagementPolicyConditionTypeName, esapi.IndexManagementPolicyReasonMissing)
			})
			It("should spec a unique name", func() {
				validatePoliciesForSpec(esapi.IndexManagementPolicySpec{
					Name: "foo",
				}, esapi.IndexManagementPolicySpec{
					Name: "foo",
				})
				expectStatus(cluster).hasPolicy("policy[1]").
					withPolicyState(esapi.IndexManagementPolicyStateDropped).
					withPolicyCondition(esapi.IndexManagementPolicyConditionTypeName, esapi.IndexManagementPolicyReasonNonUnique)
			})
		})
		Context("PollInterval", func() {
			It("should spec a value", func() {
				validatePoliciesForSpec(esapi.IndexManagementPolicySpec{
					Name: "foo",
				})
				expectStatus(cluster).hasPolicy("foo").
					withPolicyState(esapi.IndexManagementPolicyStateDropped).
					withPolicyCondition(esapi.IndexManagementPolicyConditionTypePollInterval, esapi.IndexManagementPolicyReasonMalformed).
					withPolicyConditionMessage("The pollInterval is missing or requires a valid time unit (e.g. 3d)")
			})
			It("should spec an acceptible time unit", func() {
				validatePoliciesForSpec(esapi.IndexManagementPolicySpec{
					Name:         "foo",
					PollInterval: "100ds",
				})
				expectStatus(cluster).hasPolicy("foo").
					withPolicyState(esapi.IndexManagementPolicyStateDropped).
					withPolicyCondition(esapi.IndexManagementPolicyConditionTypePollInterval, esapi.IndexManagementPolicyReasonMalformed).
					withPolicyConditionMessage("The pollInterval is missing or requires a valid time unit (e.g. 3d)")
			})
		})
		Context("Phase time unit", func() {
			Context("hot phase", func() {
				It("should spec a value", func() {
					validatePoliciesForSpec(esapi.IndexManagementPolicySpec{
						Name:         "foo",
						PollInterval: "10s",
						Phases: esapi.IndexManagementPhasesSpec{
							Hot: &esapi.IndexManagementHotPhaseSpec{},
						},
					})
					expectStatus(cluster).hasPolicy("foo").
						withPolicyState(esapi.IndexManagementPolicyStateDropped).
						withPolicyCondition(esapi.IndexManagementPolicyConditionTypeTimeUnit, esapi.IndexManagementPolicyReasonMalformed).
						withPolicyConditionMessage("The hot phase 'maxAge' is missing or requires a valid time unit (e.g. 3d)")
				})
			})
			Context("delete phase", func() {
				It("should spec a value", func() {
					validatePoliciesForSpec(esapi.IndexManagementPolicySpec{
						Name:         "delete",
						PollInterval: "15m",
						Phases: esapi.IndexManagementPhasesSpec{
							Delete: &esapi.IndexManagementDeletePhaseSpec{},
						},
					})
					expectStatus(cluster).hasPolicy("delete").
						withPolicyState(esapi.IndexManagementPolicyStateDropped).
						withPolicyCondition(esapi.IndexManagementPolicyConditionTypeTimeUnit, esapi.IndexManagementPolicyReasonMalformed).
						withPolicyConditionMessage("The delete phase 'minAge' is missing or requires a valid time unit (e.g. 3d)")
				})
			})
			It("should spec an acceptible time unit", func() {
				validatePoliciesForSpec(esapi.IndexManagementPolicySpec{
					Name:         "foo",
					PollInterval: "100ds",
					Phases: esapi.IndexManagementPhasesSpec{
						Hot: &esapi.IndexManagementHotPhaseSpec{},
					},
				})
				expectStatus(cluster).hasPolicy("foo").
					withPolicyState(esapi.IndexManagementPolicyStateDropped).
					withPolicyCondition(esapi.IndexManagementPolicyConditionTypeTimeUnit, esapi.IndexManagementPolicyReasonMalformed).
					withPolicyConditionMessage("The hot phase 'maxAge' is missing or requires a valid time unit (e.g. 3d)")
			})
		})
		It("should accept a valid policy", func() {
			validatePoliciesForSpec(esapi.IndexManagementPolicySpec{
				Name:         "foo",
				PollInterval: "10s",
				Phases: esapi.IndexManagementPhasesSpec{
					Hot: &esapi.IndexManagementHotPhaseSpec{
						Actions: esapi.IndexManagementActionsSpec{
							Rollover: &esapi.IndexManagementActionSpec{
								MaxAge: "3d",
							},
						},
					},
					Delete: &esapi.IndexManagementDeletePhaseSpec{
						MinAge: "7d",
					},
				},
			})
			expectStatus(cluster).hasPolicy("foo").
				withPolicyState(esapi.IndexManagementPolicyStateAccepted).
				withPolicyStatusReason(esapi.IndexManagementPolicyReasonConditionsMet)
		})
	})
})
