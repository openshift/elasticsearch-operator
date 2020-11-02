package k8shandler

import (
	"github.com/ViaQ/logerr/kverrors"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	api "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	v1 "k8s.io/api/core/v1"
)

var (
	r             ClusterRestart
	restarter     Restarter
	testNodes     []NodeTypeInterface
	updateStatus  func()
	nodeStatus    *api.ElasticsearchNodeStatus
	clusterStatus *api.ElasticsearchStatus
)

var _ = Describe("clusterrestart", func() {
	defer GinkgoRecover()

	BeforeEach(func() {
		testNodes = []NodeTypeInterface{}

		r = ClusterRestart{
			scheduledNodes: testNodes,
		}

		nodeStatus = &api.ElasticsearchNodeStatus{
			UpgradeStatus: api.ElasticsearchNodeUpgradeStatus{
				ScheduledForUpgrade: v1.ConditionTrue,
			},
		}
		clusterStatus = &api.ElasticsearchStatus{
			Conditions: api.ClusterConditions{},
		}

		updateStatus = func() {}
	})

	// ---------------------
	// node restart tests
	// ---------------------

	// test to ensure the node restart gating progresses all the way through
	Context("node restarts", func() {
		JustBeforeEach(func() {
			testNode := &deploymentNode{}
			testNodes = []NodeTypeInterface{
				testNode,
			}

			restarter = Restarter{
				scheduledNodes:   testNodes,
				clusterName:      "test-cluster",
				clusterNamespace: "test-namespace",
				precheck:         r.restartNoop,
				prep:             r.restartNoop,
				main:             r.restartNoop,
				post:             r.restartNoop,
				recovery:         r.restartNoop,
			}

			restarter.nodeStatus = nodeStatus
			restarter.setNodeConditions(updateStatus)
		})

		It("should be able to complete without error", func() {
			expectedStatus := nodeStatus
			expectedStatus.UpgradeStatus.UpgradePhase = api.ControllerUpdated
			expectedStatus.UpgradeStatus.UnderUpgrade = ""

			Expect(restarter.restartCluster()).To(BeNil())
			Expect(restarter.nodeStatus).To(BeEquivalentTo(expectedStatus))
		})
	})

	Context("node fails precheck", func() {
		JustBeforeEach(func() {
			testNode := &deploymentNode{}
			testNodes = []NodeTypeInterface{
				testNode,
			}

			restarter = Restarter{
				scheduledNodes:   testNodes,
				clusterName:      "test-cluster",
				clusterNamespace: "test-namespace",
				precheck:         r.restartFail,
			}

			restarter.nodeStatus = nodeStatus
			restarter.setNodeConditions(updateStatus)
		})

		It("should fail to begin restarting", func() {
			expectedStatus := nodeStatus

			Expect(restarter.restartCluster()).NotTo(BeNil())
			Expect(restarter.nodeStatus).To(BeEquivalentTo(expectedStatus))
		})
	})

	Context("node fails prep", func() {
		JustBeforeEach(func() {
			testNode := &deploymentNode{}
			testNodes = []NodeTypeInterface{
				testNode,
			}

			restarter = Restarter{
				scheduledNodes:   testNodes,
				clusterName:      "test-cluster",
				clusterNamespace: "test-namespace",
				precheck:         r.restartNoop,
				prep:             r.restartFail,
			}

			restarter.nodeStatus = nodeStatus
			restarter.setNodeConditions(updateStatus)
		})

		It("should fail to complete restarting", func() {
			expectedStatus := nodeStatus
			expectedStatus.UpgradeStatus.UnderUpgrade = v1.ConditionTrue
			expectedStatus.UpgradeStatus.ScheduledForUpgrade = ""

			Expect(restarter.restartCluster()).NotTo(BeNil())
			Expect(restarter.nodeStatus).To(BeEquivalentTo(expectedStatus))
		})
	})

	Context("node fails main", func() {
		JustBeforeEach(func() {
			testNode := &deploymentNode{}
			testNodes = []NodeTypeInterface{
				testNode,
			}

			restarter = Restarter{
				scheduledNodes:   testNodes,
				clusterName:      "test-cluster",
				clusterNamespace: "test-namespace",
				precheck:         r.restartNoop,
				prep:             r.restartNoop,
				main:             r.restartFail,
			}

			restarter.nodeStatus = nodeStatus
			restarter.setNodeConditions(updateStatus)
		})

		It("should fail to complete restarting", func() {
			expectedStatus := nodeStatus
			expectedStatus.UpgradeStatus.UnderUpgrade = v1.ConditionTrue
			expectedStatus.UpgradeStatus.ScheduledForUpgrade = ""
			expectedStatus.UpgradeStatus.UpgradePhase = api.PreparationComplete

			Expect(restarter.restartCluster()).NotTo(BeNil())
			Expect(restarter.nodeStatus).To(BeEquivalentTo(expectedStatus))
		})
	})

	Context("node fails post", func() {
		JustBeforeEach(func() {
			testNode := &deploymentNode{}
			testNodes = []NodeTypeInterface{
				testNode,
			}

			restarter = Restarter{
				scheduledNodes:   testNodes,
				clusterName:      "test-cluster",
				clusterNamespace: "test-namespace",
				precheck:         r.restartNoop,
				prep:             r.restartNoop,
				main:             r.restartNoop,
				post:             r.restartFail,
			}

			restarter.nodeStatus = nodeStatus
			restarter.setNodeConditions(updateStatus)
		})

		It("should fail to complete restarting", func() {
			expectedStatus := nodeStatus
			expectedStatus.UpgradeStatus.UnderUpgrade = v1.ConditionTrue
			expectedStatus.UpgradeStatus.ScheduledForUpgrade = ""
			expectedStatus.UpgradeStatus.UpgradePhase = api.NodeRestarting

			Expect(restarter.restartCluster()).NotTo(BeNil())
			Expect(restarter.nodeStatus).To(BeEquivalentTo(expectedStatus))
		})
	})

	Context("node fails recovery", func() {
		JustBeforeEach(func() {
			testNode := &deploymentNode{}
			testNodes = []NodeTypeInterface{
				testNode,
			}

			restarter = Restarter{
				scheduledNodes:   testNodes,
				clusterName:      "test-cluster",
				clusterNamespace: "test-namespace",
				precheck:         r.restartNoop,
				prep:             r.restartNoop,
				main:             r.restartNoop,
				post:             r.restartNoop,
				recovery:         r.restartFail,
			}

			restarter.nodeStatus = nodeStatus
			restarter.setNodeConditions(updateStatus)
		})

		It("should fail to complete restarting", func() {
			expectedStatus := nodeStatus
			expectedStatus.UpgradeStatus.UnderUpgrade = v1.ConditionTrue
			expectedStatus.UpgradeStatus.ScheduledForUpgrade = ""
			expectedStatus.UpgradeStatus.UpgradePhase = api.RecoveringData

			Expect(restarter.restartCluster()).NotTo(BeNil())
			Expect(restarter.nodeStatus).To(BeEquivalentTo(expectedStatus))
		})
	})

	// ---------------------
	// cluster restart tests
	// ---------------------

	// test to ensure the cluster restart gating progresses all the way through
	Context("cluster restarts", func() {
		JustBeforeEach(func() {
			restarter = Restarter{
				scheduledNodes:   testNodes,
				clusterName:      "test-cluster",
				clusterNamespace: "test-namespace",
				precheck:         r.restartNoop,
				prep:             r.restartNoop,
				main:             r.restartNoop,
				post:             r.restartNoop,
				recovery:         r.restartNoop,
			}

			restarter.clusterStatus = clusterStatus
			restarter.setClusterConditions(updateStatus)
		})

		It("should be able to complete without error", func() {
			expectedStatus := clusterStatus

			Expect(restarter.restartCluster()).To(BeNil())
			Expect(restarter.clusterStatus).To(BeEquivalentTo(expectedStatus))
		})
	})

	Context("cluster fails precheck", func() {
		JustBeforeEach(func() {
			restarter = Restarter{
				scheduledNodes:   testNodes,
				clusterName:      "test-cluster",
				clusterNamespace: "test-namespace",
				precheck:         r.restartFail,
			}

			restarter.clusterStatus = clusterStatus
			restarter.setClusterConditions(updateStatus)
		})

		It("should fail to begin restarting", func() {
			expectedStatus := clusterStatus

			Expect(restarter.restartCluster()).NotTo(BeNil())
			Expect(restarter.clusterStatus).To(BeEquivalentTo(expectedStatus))
		})
	})

	Context("cluster fails prep", func() {
		JustBeforeEach(func() {
			testNode := &deploymentNode{}
			testNodes = []NodeTypeInterface{
				testNode,
			}

			restarter = Restarter{
				scheduledNodes:   testNodes,
				clusterName:      "test-cluster",
				clusterNamespace: "test-namespace",
				precheck:         r.restartNoop,
				prep:             r.restartFail,
			}

			restarter.clusterStatus = clusterStatus
			restarter.setClusterConditions(updateStatus)
		})

		It("should fail to complete restarting", func() {
			expectedStatus := clusterStatus
			expectedStatus.Conditions = api.ClusterConditions{
				{
					Type:   api.UpdatingESSettings,
					Status: v1.ConditionTrue,
				},
			}

			Expect(restarter.restartCluster()).NotTo(BeNil())
			Expect(restarter.clusterStatus).To(BeEquivalentTo(expectedStatus))
		})
	})

	Context("cluster fails main", func() {
		JustBeforeEach(func() {
			testNode := &deploymentNode{}
			testNodes = []NodeTypeInterface{
				testNode,
			}

			restarter = Restarter{
				scheduledNodes:   testNodes,
				clusterName:      "test-cluster",
				clusterNamespace: "test-namespace",
				precheck:         r.restartNoop,
				prep:             r.restartNoop,
				main:             r.restartFail,
			}

			restarter.clusterStatus = clusterStatus
			restarter.setClusterConditions(updateStatus)
		})

		It("should fail to complete restarting", func() {
			expectedStatus := clusterStatus
			expectedStatus.Conditions = api.ClusterConditions{
				{
					Type:   api.UpdatingESSettings,
					Status: v1.ConditionFalse,
				},
				{
					Type:   api.Restarting,
					Status: v1.ConditionTrue,
				},
			}

			Expect(restarter.restartCluster()).NotTo(BeNil())
			Expect(restarter.clusterStatus).To(BeEquivalentTo(expectedStatus))
		})
	})

	Context("cluster fails post", func() {
		JustBeforeEach(func() {
			testNode := &deploymentNode{}
			testNodes = []NodeTypeInterface{
				testNode,
			}

			restarter = Restarter{
				scheduledNodes:   testNodes,
				clusterName:      "test-cluster",
				clusterNamespace: "test-namespace",
				precheck:         r.restartNoop,
				prep:             r.restartNoop,
				main:             r.restartNoop,
				post:             r.restartFail,
			}

			restarter.clusterStatus = clusterStatus
			restarter.setClusterConditions(updateStatus)
		})

		It("should fail to complete restarting", func() {
			expectedStatus := clusterStatus
			expectedStatus.Conditions = api.ClusterConditions{
				{
					Type:   api.UpdatingESSettings,
					Status: v1.ConditionTrue,
				},
				{
					Type:   api.Restarting,
					Status: v1.ConditionTrue,
				},
			}

			Expect(restarter.restartCluster()).NotTo(BeNil())
			Expect(restarter.clusterStatus).To(BeEquivalentTo(expectedStatus))
		})
	})

	Context("cluster fails recovery", func() {
		JustBeforeEach(func() {
			testNode := &deploymentNode{}
			testNodes = []NodeTypeInterface{
				testNode,
			}

			restarter = Restarter{
				scheduledNodes:   testNodes,
				clusterName:      "test-cluster",
				clusterNamespace: "test-namespace",
				precheck:         r.restartNoop,
				prep:             r.restartNoop,
				main:             r.restartNoop,
				post:             r.restartNoop,
				recovery:         r.restartFail,
			}

			restarter.clusterStatus = clusterStatus
			restarter.setClusterConditions(updateStatus)
		})

		It("should fail to complete restarting", func() {
			expectedStatus := clusterStatus
			expectedStatus.Conditions = api.ClusterConditions{
				{
					Type:   api.Recovering,
					Status: v1.ConditionTrue,
				},
			}

			Expect(restarter.restartCluster()).NotTo(BeNil())
			Expect(restarter.clusterStatus).To(BeEquivalentTo(expectedStatus))
		})
	})
})

func (cr ClusterRestart) restartFail() error {
	return kverrors.New("we apologise for the fault in this function. Those responsible have been sacked.")
}
