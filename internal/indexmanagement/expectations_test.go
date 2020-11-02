package indexmanagement

import (
	"fmt"

	. "github.com/onsi/gomega"

	esapi "github.com/openshift/elasticsearch-operator/apis/logging/v1"
)

type statusTestContext struct {
	cluster          *esapi.Elasticsearch
	policyStatus     *esapi.IndexManagementPolicyStatus
	policyCondition  *esapi.IndexManagementPolicyCondition
	mappingStatus    *esapi.IndexManagementMappingStatus
	mappingCondition *esapi.IndexManagementMappingCondition
}

func expectStatus(testCluster *esapi.Elasticsearch) *statusTestContext {
	return &statusTestContext{
		cluster: testCluster,
	}
}

func (cxt *statusTestContext) hasState(state esapi.IndexManagementState) *statusTestContext {
	Expect(cxt.cluster).ToNot(BeNil(), "Cluster is nil")
	Expect(cxt.cluster.Status).ToNot(BeNil(), "cluster.Status is nil")
	Expect(cxt.cluster.Status.IndexManagementStatus).ToNot(BeNil(), "cluster.Status.IndexManagementStatus is nil")
	Expect(cxt.cluster.Status.IndexManagementStatus.State).To(Equal(state), fmt.Sprintf("status: %v:", cxt.cluster.Status.IndexManagementStatus))
	return cxt
}

func (cxt *statusTestContext) withMessage(message string) *statusTestContext {
	Expect(cxt.cluster).ToNot(BeNil(), "Cluster is nil")
	Expect(cxt.cluster.Status).ToNot(BeNil(), "cluster.Status is nil")
	Expect(cxt.cluster.Status.IndexManagementStatus).ToNot(BeNil(), "cluster.Status.IndexManagementStatus is nil")
	Expect(cxt.cluster.Status.IndexManagementStatus.Message).To(Equal(message))
	return cxt
}

func (cxt *statusTestContext) withReason(reason esapi.IndexManagementStatusReason) *statusTestContext {
	Expect(cxt.cluster).ToNot(BeNil(), "Cluster is nil")
	Expect(cxt.cluster.Status).ToNot(BeNil(), "cluster.Status is nil")
	Expect(cxt.cluster.Status.IndexManagementStatus).ToNot(BeNil(), "cluster.Status.IndexManagementStatus is nil")
	Expect(cxt.cluster.Status.IndexManagementStatus.Reason).To(Equal(reason))
	return cxt
}

func (cxt *statusTestContext) hasPolicy(name string) *statusTestContext {
	Expect(cxt.cluster).ToNot(BeNil(), "Cluster is nil")
	Expect(cxt.cluster.Status).ToNot(BeNil(), "cluster.Status is nil")
	Expect(cxt.cluster.Spec.IndexManagement.Policies).ToNot(BeNil(), "cluster.Status.IndexManagementStatus.Policies is nil")
	for i, policy := range cxt.cluster.Status.IndexManagementStatus.Policies {
		if policy.Name == name {
			cxt.policyStatus = &cxt.cluster.Status.IndexManagementStatus.Policies[i]
			break
		}
	}
	// if we get here then the validation failed
	Expect(cxt.policyStatus).ToNot(BeNil(), fmt.Sprintf("PolicyStatus not found for %s: %v", name, cxt.cluster.Status.IndexManagementStatus.Policies))
	return cxt
}

func (cxt *statusTestContext) hasMapping(name string) *statusTestContext {
	Expect(cxt.cluster).ToNot(BeNil(), "Cluster is nil")
	Expect(cxt.cluster.Status).ToNot(BeNil(), "cluster.Status is nil")
	Expect(cxt.cluster.Spec.IndexManagement.Mappings).ToNot(BeNil(), "cluster.Status.IndexManagementStatus.Mappings is nil")
	for i, mapping := range cxt.cluster.Status.IndexManagementStatus.Mappings {
		if mapping.Name == name {
			cxt.mappingStatus = &cxt.cluster.Status.IndexManagementStatus.Mappings[i]
			break
		}
	}
	// if we get here then the validation failed
	Expect(cxt.mappingStatus).ToNot(BeNil(), fmt.Sprintf("MappingStatus not found for %s: %v", name, cxt.cluster.Status.IndexManagementStatus.Mappings))
	return cxt
}

func (cxt *statusTestContext) withPolicyState(state esapi.IndexManagementPolicyState) *statusTestContext {
	Expect(cxt.policyStatus.State).To(Equal(state))
	return cxt
}

func (cxt *statusTestContext) withMappingState(state esapi.IndexManagementMappingState) *statusTestContext {
	Expect(cxt.mappingStatus.State).To(Equal(state), fmt.Sprintf("status: %v", cxt.mappingStatus))
	return cxt
}

func (cxt *statusTestContext) withPolicyStatusReason(reason esapi.IndexManagementPolicyReason) *statusTestContext {
	Expect(cxt.policyStatus.Reason).To(Equal(reason))
	return cxt
}

func (cxt *statusTestContext) withMappingStatusReason(reason esapi.IndexManagementMappingReason) *statusTestContext {
	Expect(cxt.mappingStatus.Reason).To(Equal(reason))
	return cxt
}

func (cxt *statusTestContext) withPolicyCondition(conditionType esapi.IndexManagementPolicyConditionType, reason esapi.IndexManagementPolicyConditionReason) *statusTestContext {
	for i, condition := range cxt.policyStatus.Conditions {
		if condition.Type == conditionType {
			cxt.policyCondition = &cxt.policyStatus.Conditions[i]
			break
		}
	}
	Expect(cxt.policyCondition).ToNot(BeNil(), "The condition type %q wasn't found during validation", conditionType)
	Expect(cxt.policyCondition.Reason).To(Equal(reason))
	return cxt
}

func (cxt *statusTestContext) withMappingCondition(conditionType esapi.IndexManagementMappingConditionType, reason esapi.IndexManagementMappingConditionReason) *statusTestContext {
	for i, condition := range cxt.mappingStatus.Conditions {
		if condition.Type == conditionType {
			cxt.mappingCondition = &cxt.mappingStatus.Conditions[i]
			break
		}
	}
	Expect(cxt.mappingCondition).ToNot(BeNil(), "The condition type %q wasn't found during validation", conditionType)
	Expect(cxt.mappingCondition.Reason).To(Equal(reason))
	return cxt
}

func (cxt *statusTestContext) withPolicyConditionMessage(message string) *statusTestContext {
	Expect(cxt.policyCondition.Message).To(Equal(message))
	return cxt
}

func (cxt *statusTestContext) withMappingConditionMessage(message string) *statusTestContext {
	Expect(cxt.mappingCondition.Message).To(Equal(message))
	return cxt
}
