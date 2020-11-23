package e2e

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	consolev1 "github.com/openshift/api/console/v1"
	routev1 "github.com/openshift/api/route/v1"
	apps "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	loggingv1 "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	// controllers "github.com/openshift/elasticsearch-operator/controllers/logging"
	// +kubebuilder:scaffold:imports
)

const (
	TestOperatorNamespaceEnv = "TEST_OPERATOR_NAMESPACE"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg               *rest.Config
	k8sClient         client.Client
	testEnv           *envtest.Environment
	operatorNamespace string
	projectRootDir    string
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	projectRootDir = getProjectRootPath("elasticsearch-operator")

	junitReporter := reporters.NewJUnitReporter(filepath.Join(projectRootDir, "junit.xml"))
	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}, junitReporter})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("bootstrapping test environment")
	ok := false
	operatorNamespace, ok = os.LookupEnv(TestOperatorNamespaceEnv)
	Expect(ok).Should(BeTrue(), "TEST_OPERATOR_NAMESPACE is unset")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")},
	}

	var err error
	cfg, err = config.GetConfig()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = loggingv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = consolev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = routev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	// wait for elasticsearch-operator to be ready
	esOperatorLookupKey := types.NamespacedName{Name: "elasticsearch-operator", Namespace: operatorNamespace}
	esOperatorDeploy := &apps.Deployment{}
	Eventually(func() bool {
		err := k8sClient.Get(context.Background(), esOperatorLookupKey, esOperatorDeploy)
		if err != nil {
			return false
		}
		if int(esOperatorDeploy.Status.AvailableReplicas) != 1 {
			return false
		}
		return true
	}, timeout, interval).Should(BeTrue())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
