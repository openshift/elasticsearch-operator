package k8shandler

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	api "github.com/openshift/elasticsearch-operator/apis/logging/v1"
)

var (
	dpl           *api.Elasticsearch
	dataNodeCount int
	dataNode      api.ElasticsearchNode
)

var _ = Describe("defaults", func() {
	defer GinkgoRecover()

	BeforeEach(func() {
		dataNode = api.ElasticsearchNode{
			Roles: []api.ElasticsearchNodeRole{
				api.ElasticsearchRoleClient,
				api.ElasticsearchRoleData,
			},
		}
	})

	Describe("#getPrimaryShardCount with excess data nodes", func() {
		JustBeforeEach(func() {
			dataNodeCount = 20
			dataNode.NodeCount = int32(dataNodeCount)

			dpl = &api.Elasticsearch{
				Spec: api.ElasticsearchSpec{
					Nodes: []api.ElasticsearchNode{
						dataNode,
					},
				},
			}
		})
		It("should return maxPrimaryShardCount", func() {
			Expect(calculatePrimaryCount(dpl)).To(Equal(maxPrimaryShardCount))
		})
	})

	Describe("#getPrimaryShardCount with 3 data nodes", func() {
		JustBeforeEach(func() {
			dataNodeCount = 3
			dataNode.NodeCount = int32(dataNodeCount)

			dpl = &api.Elasticsearch{
				Spec: api.ElasticsearchSpec{
					Nodes: []api.ElasticsearchNode{
						dataNode,
					},
				},
			}
		})
		It("should return data node count", func() {
			Expect(calculatePrimaryCount(dpl)).To(Equal(dataNodeCount))
		})
	})
})
