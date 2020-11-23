package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	consolev1 "github.com/openshift/api/console/v1"
	routev1 "github.com/openshift/api/route/v1"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	loggingv1 "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/internal/k8shandler/kibana"
	"github.com/openshift/elasticsearch-operator/test/utils"
)

var _ = Describe("Kibana controller", func() {
	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		KibanaName      = "test-kibana"
		KibanaNamespace = "default"

		timeout  = time.Second * 600
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	var (
		esUUID   string
		dataUUID string
	)

	BeforeEach(func() {
		By("By creating an elasticsearch secret")
		fmt.Printf("Test test... in namespace %s\n", operatorNamespace)
		ctx := context.Background()

		// wait for elasticsearch-operator to be ready
		esOperatorLookupKey := types.NamespacedName{Name: "elasticsearch-operator", Namespace: operatorNamespace}
		esOperatorDeploy := &apps.Deployment{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, esOperatorLookupKey, esOperatorDeploy)
			if err != nil {
				return false
			}
			if int(esOperatorDeploy.Status.AvailableReplicas) != 1 {
				return false
			}
			return true
		}, timeout, interval).Should(BeTrue())
		fmt.Println("Setup complete")
	})

	AfterEach(func() {
		Eventually(func() error {
			f := &corev1.Secret{}
			esSecretLookupKey := types.NamespacedName{Name: elasticsearchNameFor(esUUID), Namespace: operatorNamespace}
			k8sClient.Get(context.Background(), esSecretLookupKey, f)
			return k8sClient.Delete(context.Background(), f)
		}, timeout, interval).Should(Succeed())

		Eventually(func() error {
			es := &loggingv1.Elasticsearch{}
			esLookupKey := types.NamespacedName{Name: elasticsearchNameFor(esUUID), Namespace: operatorNamespace}
			k8sClient.Get(context.Background(), esLookupKey, es)
			return k8sClient.Delete(context.Background(), es)
		}, timeout, interval).Should(Succeed())

		Eventually(func() error {
			f := &corev1.Secret{}
			kSecretLookupKey := types.NamespacedName{Name: kibanaCRName, Namespace: operatorNamespace}
			k8sClient.Get(context.Background(), kSecretLookupKey, f)
			return k8sClient.Delete(context.Background(), f)
		}, timeout, interval).Should(Succeed())

		Eventually(func() error {
			f := &corev1.Secret{}
			kpSecretLookupKey := types.NamespacedName{Name: fmt.Sprintf("%s-proxy", kibanaCRName), Namespace: operatorNamespace}
			k8sClient.Get(context.Background(), kpSecretLookupKey, f)
			return k8sClient.Delete(context.Background(), f)
		}, timeout, interval).Should(Succeed())

		Eventually(func() error {
			kibana := &loggingv1.Kibana{}
			kibanaLookupKey := types.NamespacedName{Name: "kibana", Namespace: operatorNamespace}
			k8sClient.Get(context.Background(), kibanaLookupKey, kibana)
			return k8sClient.Delete(context.Background(), kibana)
		}, timeout, interval).Should(Succeed())
	})

	Context("Kibana deployment for elasticsearch", func() {
		It("Should create successfully", func() {
			By("By creating an Kibana CR")

			ctx := context.Background()
			replicas := 1

			esUUID = utils.GenerateUUID()
			fmt.Printf("Using UUID for elasticsearch CR: %v\n", esUUID)

			dataUUID = utils.GenerateUUID()
			fmt.Printf("Using GenUUID for data nodes: %v\n", dataUUID)

			err := createElasticsearchSecret(ctx, esUUID)
			Expect(err).ToNot(HaveOccurred())

			esDeploymentName := fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
			es, err := createElasticsearchCR(ctx, esUUID, dataUUID, replicas)
			Expect(err).ToNot(HaveOccurred())
			Expect(es).ToNot(BeNil())

			// wait for ES deployment
			esDeploymentLookupKey := types.NamespacedName{Name: esDeploymentName, Namespace: operatorNamespace}
			createdEsDeploy := &apps.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, esDeploymentLookupKey, createdEsDeploy)
				if err != nil {
					return false
				}
				if int(createdEsDeploy.Status.AvailableReplicas) != replicas {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			err = createKibanaSecret(ctx, esUUID)
			Expect(err).ToNot(HaveOccurred())

			err = createKibanaProxySecret(ctx, esUUID)
			Expect(err).ToNot(HaveOccurred())

			// Create kibana CR
			kibanaCR := createKibanaCR(operatorNamespace)
			Expect(k8sClient.Create(ctx, kibanaCR)).Should(Succeed())

			// wait for Kibana deployment
			kibanaDeploymentLookupKey := types.NamespacedName{Name: "kibana", Namespace: operatorNamespace}
			createdKibanaDeploy := &apps.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, kibanaDeploymentLookupKey, createdKibanaDeploy)
				if err != nil {
					return false
				}
				if int(createdKibanaDeploy.Status.AvailableReplicas) != replicas {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			consoleLink := &consolev1.ConsoleLink{}
			key := types.NamespacedName{Name: kibana.KibanaConsoleLinkName}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, key, consoleLink)
				if err != nil {
					fmt.Printf("not found: %v", err)
					return false
				}
				return true
			}, 5*time.Second, interval).Should(BeTrue())

			// Test recovering route after deletion
			name := "kibana"
			routeInst := kibana.NewRoute(name, operatorNamespace, name)
			err = k8sClient.Delete(ctx, routeInst)
			Expect(err).ToNot(HaveOccurred())

			route := &routev1.Route{}
			key = types.NamespacedName{Name: name, Namespace: operatorNamespace}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, key, route)
				if err != nil {
					fmt.Printf("not found route: %v", err)
					return false
				}
				return true
			}, 5*time.Second, interval).Should(BeTrue())
		})
	})
})
