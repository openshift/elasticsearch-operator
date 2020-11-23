package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	loggingv1 "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/test/utils"
)

var _ = Describe("Elasticsearch Controller", func() {
	var (
		esUUID   string
		dataUUID string
	)

	BeforeEach(func() {
		fmt.Println("Before each context")
		ctx := context.Background()

		esUUID = utils.GenerateUUID()
		fmt.Printf("Using UUID for elasticsearch CR: %v\n", esUUID)

		dataUUID = utils.GenerateUUID()
		fmt.Printf("Using GenUUID for data nodes: %v\n", dataUUID)

		err := createElasticsearchSecret(ctx, esUUID)
		Expect(err).ToNot(HaveOccurred())

		fmt.Println("Setup complete")
	})

	AfterEach(func() {
		Eventually(func() error {
			f := &corev1.Secret{}
			esSecretLookupKey := types.NamespacedName{Name: elasticsearchNameFor(esUUID), Namespace: operatorNamespace}
			k8sClient.Get(context.Background(), esSecretLookupKey, f)
			return k8sClient.Delete(context.Background(), f)
		}, timeout, interval).Should(Succeed())

		es := &loggingv1.Elasticsearch{}
		esLookupKey := types.NamespacedName{Name: elasticsearchNameFor(esUUID), Namespace: operatorNamespace}
		Eventually(func() error {
			k8sClient.Get(context.Background(), esLookupKey, es)
			return k8sClient.Delete(context.Background(), es)
		}, timeout, interval).Should(Succeed())

		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), esLookupKey, es)
			if err != nil && errors.IsNotFound(err) {
				return true
			}
			fmt.Printf("Wait for cleaning up CR deployment %s\n", es.Name)
			return false
		}, timeout, retryInterval).Should(BeTrue())
	})

	Context("Single node", func() {
		var (
			ctx context.Context
			es  *loggingv1.Elasticsearch
			err error
		)
		BeforeEach(func() {
			ctx = context.Background()

			// Create CR with a single node with client, data and master roles
			es, err = createElasticsearchCR(ctx, esUUID, dataUUID, 1)
			Expect(err).ToNot(HaveOccurred())
			Expect(es).ToNot(BeNil())
		})

		It("Should create successfully", func() {
			fmt.Println("Single node test")
			// wait for ES deployment
			esDeploymentName := fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
			esDeploymentLookupKey := types.NamespacedName{Name: esDeploymentName, Namespace: operatorNamespace}
			createdEsDeploy := &apps.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, esDeploymentLookupKey, createdEsDeploy)
				if err != nil {
					return false
				}
				if int(createdEsDeploy.Status.AvailableReplicas) != 1 {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})

		It("Should scaleUpNodes successfully", func() {
			fmt.Println("Scaleup node test")
			// wait for ES deployment
			esDeploymentName := fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
			esDeploymentLookupKey := types.NamespacedName{Name: esDeploymentName, Namespace: operatorNamespace}
			createdEsDeploy := &apps.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, esDeploymentLookupKey, createdEsDeploy)
				if err != nil {
					return false
				}
				if int(createdEsDeploy.Status.AvailableReplicas) != 1 {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			fmt.Println("Adding a new data node")
			es.Spec.Nodes[0].NodeCount = int32(2)
			err = updateElasticsearchSpec(ctx, es)
			Expect(err).ToNot(HaveOccurred())

			// wait for ES deployment
			for i := 0; i < 2; i++ {
				esDeploymentName := fmt.Sprintf("elasticsearch-%v-cdm-%v-%d", esUUID, dataUUID, i+1)
				esDeploymentLookupKey := types.NamespacedName{Name: esDeploymentName, Namespace: operatorNamespace}
				createdEsDeploy := &apps.Deployment{}

				fmt.Printf("Waiting for deployment %s\n", esDeploymentName)
				Eventually(func() bool {
					err := k8sClient.Get(ctx, esDeploymentLookupKey, createdEsDeploy)
					if err != nil {
						return false
					}
					if int(createdEsDeploy.Status.AvailableReplicas) != 1 {
						return false
					}
					return true
				}, timeout, interval).Should(BeTrue())
			}
		})
	})

	Context("Multiple node", func() {
		var (
			ctx      context.Context
			es       *loggingv1.Elasticsearch
			err      error
			replicas int
		)

		BeforeEach(func() {
			ctx = context.Background()
			replicas = 2

			// Create CR with a single node with client, data and master roles
			es, err = createElasticsearchCR(ctx, esUUID, dataUUID, replicas)
			Expect(err).ToNot(HaveOccurred())
			Expect(es).ToNot(BeNil())
		})

		waitForEsDeployment := func() {
			// wait for ES deployment
			for i := 0; i < replicas; i++ {
				esDeploymentName := fmt.Sprintf("elasticsearch-%v-cdm-%v-%d", esUUID, dataUUID, i+1)
				esDeploymentLookupKey := types.NamespacedName{Name: esDeploymentName, Namespace: operatorNamespace}
				createdEsDeploy := &apps.Deployment{}

				fmt.Printf("Waiting for deployment %s\n", esDeploymentName)
				Eventually(func() bool {
					err := k8sClient.Get(ctx, esDeploymentLookupKey, createdEsDeploy)
					if err != nil {
						return false
					}
					if int(createdEsDeploy.Status.AvailableReplicas) != 1 {
						return false
					}
					return true
				}, timeout, retryInterval).Should(BeTrue())
			}
		}

		WaitForReadyDeployment := func() {
			// wait for ready deployment
			for i := 0; i < replicas; i++ {
				esDeploymentName := fmt.Sprintf("elasticsearch-%v-cdm-%v-%d", esUUID, dataUUID, i+1)
				esDeploymentLookupKey := types.NamespacedName{Name: esDeploymentName, Namespace: operatorNamespace}
				createdEsDeploy := &apps.Deployment{}

				fmt.Printf("Waiting for deployment %s\n", esDeploymentName)
				Eventually(func() bool {
					err := k8sClient.Get(ctx, esDeploymentLookupKey, createdEsDeploy)
					if err != nil {
						return false
					}
					if int(createdEsDeploy.Status.ReadyReplicas) >= 1 {
						return true
					}
					fmt.Printf("Waiting for full readiness of %s deployment (%d/%d)\n", esDeploymentName,
						createdEsDeploy.Status.ReadyReplicas, 1)
					return false
				}, timeout*2, retryInterval).Should(BeTrue(), "timed out waiting for %s deployment %v: %v", esDeploymentName, err)
			}
		}

		It("Should create successfully", func() {
			fmt.Println("Multiple node test")
			waitForEsDeployment()
		})

		It("Should add a single non-data node successfully", func() {
			waitForEsDeployment()

			nonDataUUID := utils.GenerateUUID()
			fmt.Printf("Using GenUUID for non data nodes: %v\n", nonDataUUID)

			storageClassSize := resource.MustParse("2G")
			esNonDataNode := loggingv1.ElasticsearchNode{
				Roles: []loggingv1.ElasticsearchNodeRole{
					loggingv1.ElasticsearchRoleClient,
					loggingv1.ElasticsearchRoleMaster,
				},
				NodeCount: int32(1),
				Storage: loggingv1.ElasticsearchStorageSpec{
					Size: &storageClassSize,
				},
				GenUUID: &nonDataUUID,
			}

			fmt.Println("Adding non-data node")
			es.Spec.Nodes = append(es.Spec.Nodes, esNonDataNode)

			err = updateElasticsearchSpec(ctx, es)
			Expect(err).ToNot(HaveOccurred(), "could not update elasticsearch CR with an additional non-data node: %v", err)

			statefulSetName := fmt.Sprintf("elasticsearch-%v-cm-%v", esUUID, nonDataUUID)
			lookupKey := types.NamespacedName{Name: statefulSetName, Namespace: operatorNamespace}
			createdEsStatefulSet := &apps.StatefulSet{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookupKey, createdEsStatefulSet)
				if err != nil {
					return false
				}
				if int(createdEsStatefulSet.Status.ReadyReplicas) != 1 {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			fmt.Println("Created non-data statefulset")
		})

		It("Should full cluster redeploy successfully", func() {
			fmt.Println("Full cluster redeploy")
			waitForEsDeployment()

			matchingLabels := map[string]string{
				"cluster-name": es.GetName(),
				"component":    "elasticsearch",
			}

			initialPods, err := utils.WaitForPods(ctx, k8sClient, operatorNamespace, matchingLabels, retryInterval, timeout)
			Expect(err).ToNot(HaveOccurred(), "failed to wait for pods: %v", err)
			var initPodNames []string
			for _, pod := range initialPods.Items {
				initPodNames = append(initPodNames, pod.GetName())
			}
			fmt.Printf("Cluster pods before rolling restart: %v\n", initPodNames)

			// Scale up to SingleRedundancy
			es.Spec.RedundancyPolicy = loggingv1.SingleRedundancy

			fmt.Printf("Updating redundancy policy to %v\n", es.Spec.RedundancyPolicy)
			err = updateElasticsearchSpec(ctx, es)
			Expect(err).ToNot(HaveOccurred(),
				"could not update elasticsearch CR to be SingleRedundancy: %v", err)

			// Update the secret to force a full cluster redeploy
			err = updateElasticsearchSecret(ctx, esUUID)
			Expect(err).ToNot(HaveOccurred(),
				"Unable to update secret")

			fmt.Println("Waiting for redeployment after secret update")
			WaitForReadyDeployment()

			pods, err := utils.WaitForRolloutComplete(ctx, k8sClient, operatorNamespace, matchingLabels, initPodNames, 2, retryInterval, timeout)
			Expect(err).ToNot(HaveOccurred(), err)
			var podNames []string
			for _, pod := range pods.Items {
				podNames = append(podNames, pod.GetName())
			}
			fmt.Printf("Cluster pods after full cluster redeploy: %v\n", podNames)
			Expect(len(pods.Items)).To(Equal(2), "No matching pods found for labels: %#v", matchingLabels)
		})

		It("Should Rolling restart successfully", func() {
			fmt.Println("Rolling restart")
			waitForEsDeployment()

			matchingLabels := map[string]string{
				"cluster-name": es.GetName(),
				"component":    "elasticsearch",
			}

			initialPods, err := utils.WaitForPods(ctx, k8sClient, operatorNamespace, matchingLabels, retryInterval, timeout)
			Expect(err).ToNot(HaveOccurred(), "failed to wait for pods: %v", err)

			var initPodNames []string
			for _, pod := range initialPods.Items {
				initPodNames = append(initPodNames, pod.GetName())
			}
			fmt.Printf("Cluster pods before rolling restart: %v\n", initPodNames)

			// Update the resource spec for the cluster
			oldMemValue := es.Spec.Spec.Resources.Limits.Memory()
			memValue := es.Spec.Spec.Resources.Requests.Memory().DeepCopy()
			memValue.Add(resource.MustParse("1Mi"))
			cpuValue := es.Spec.Spec.Resources.Requests.Cpu().DeepCopy()
			cpuValue.Add(resource.MustParse("1m"))
			desiredResources := corev1.ResourceRequirements{
				Limits: es.Spec.Spec.Resources.Limits,
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    cpuValue,
					corev1.ResourceMemory: memValue,
				},
			}
			fmt.Printf("Updating Limits.Memory and Requests.Memory to trigger a rolling restart\n")
			fmt.Printf("Updating from %s to %s\n", oldMemValue.String(), memValue.String())

			es.Spec.Spec.Resources = desiredResources
			err = updateElasticsearchSpec(ctx, es)
			Expect(err).ToNot(HaveOccurred(),
				"could not update elasticsearch CR to desired resources: %v", err)

			fmt.Println("Waiting for restart after resource requests/limits update")

			WaitForReadyDeployment()
			pods, err := utils.WaitForRolloutComplete(ctx, k8sClient, operatorNamespace, matchingLabels, initPodNames, 2, retryInterval, timeout)
			Expect(err).ToNot(HaveOccurred(), err)
			var podNames []string
			for _, pod := range pods.Items {
				podNames = append(podNames, pod.GetName())
			}
			fmt.Printf("Cluster pods after rolling restart: %v\n", podNames)
			Expect(len(pods.Items)).To(Equal(2), "No matching pods found for labels: %#v", matchingLabels)

			for _, pod := range pods.Items {
				diff := cmp.Diff(pod.Spec.Containers[0].Resources, desiredResources)
				Expect(diff).To(Equal(""), "failed to match pods with resources:\n%s", diff)
			}
		})
	})
	Context("Invalid cases", func() {
		It("Invalid master count should fail", func() {
			ctx := context.Background()

			// Create CR with invalid case: four nodes all sharing client, data and master roles
			es, err := createElasticsearchCR(ctx, esUUID, dataUUID, 4)
			Expect(err).ToNot(HaveOccurred())
			Expect(es).ToNot(BeNil())

			key := client.ObjectKey{Name: es.GetName(), Namespace: es.GetNamespace()}
			err = k8sClient.Get(ctx, key, es)
			Expect(err).ToNot(HaveOccurred(), "failed to get updated CR: %s", key)

			time.Sleep(10 * time.Second)

			for _, condition := range es.Status.Conditions {
				if condition.Type == loggingv1.InvalidMasters {
					Expect(condition.Status == corev1.ConditionFalse || condition.Status == "").To(BeFalse())
				}
			}
		})
	})
})
