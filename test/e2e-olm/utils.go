package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/ViaQ/logerr/kverrors"
	consolev1 "github.com/openshift/api/console/v1"
	routev1 "github.com/openshift/api/route/v1"
	loggingv1 "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/test/utils"

	"github.com/operator-framework/operator-sdk/pkg/test"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	retryInterval        = time.Second * 3
	timeout              = time.Second * 300
	cleanupRetryInterval = time.Second * 3
	cleanupTimeout       = time.Second * 30
	elasticsearchCRName  = "elasticsearch"
	kibanaCRName         = "kibana"
	exampleSecretsPath   = "/tmp/example-secrets"
)

func registerSchemes(t *testing.T) {
	elasticsearchList := &loggingv1.ElasticsearchList{}
	err := test.AddToFrameworkScheme(loggingv1.SchemeBuilder.AddToScheme, elasticsearchList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	kibanaList := &loggingv1.KibanaList{}
	err = test.AddToFrameworkScheme(loggingv1.SchemeBuilder.AddToScheme, kibanaList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	consoleLinkList := &consolev1.ConsoleLinkList{}
	err = test.AddToFrameworkScheme(consolev1.Install, consoleLinkList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	routeList := &routev1.RouteList{}
	err = test.AddToFrameworkScheme(routev1.Install, routeList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}
}

func elasticsearchNameFor(uuid string) string {
	return fmt.Sprintf("%s-%s", elasticsearchCRName, uuid)
}

func createElasticsearchCR(t *testing.T, f *test.Framework, ctx *test.Context, esUUID, dataUUID string, replicas int) (*loggingv1.Elasticsearch, error) {
	namespace, err := ctx.GetWatchNamespace()
	if err != nil {
		return nil, kverrors.Wrap(err, "failed to get namespace")
	}

	cpuValue := resource.MustParse("256m")
	memValue := resource.MustParse("1Gi")

	storageClassSize := resource.MustParse("2Gi")

	esDataNode := loggingv1.ElasticsearchNode{
		Roles: []loggingv1.ElasticsearchNodeRole{
			loggingv1.ElasticsearchRoleClient,
			loggingv1.ElasticsearchRoleData,
			loggingv1.ElasticsearchRoleMaster,
		},
		Storage: loggingv1.ElasticsearchStorageSpec{
			Size: &storageClassSize,
		},
		NodeCount: int32(replicas),
		GenUUID:   &dataUUID,
	}

	// create elasticsearch custom resource
	cr := &loggingv1.Elasticsearch{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Elasticsearch",
			APIVersion: loggingv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      elasticsearchNameFor(esUUID),
			Namespace: namespace,
			Annotations: map[string]string{
				"loggingv1.openshift.io/develLogAppender": "console",
				"loggingv1.openshift.io/loglevel":         "trace",
			},
		},
		Spec: loggingv1.ElasticsearchSpec{
			Spec: loggingv1.ElasticsearchNodeSpec{
				Image: "",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("5Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    cpuValue,
						corev1.ResourceMemory: memValue,
					},
				},
			},
			Nodes: []loggingv1.ElasticsearchNode{
				esDataNode,
			},
			ManagementState:  loggingv1.ManagementStateManaged,
			RedundancyPolicy: loggingv1.ZeroRedundancy,
		},
	}

	t.Logf("Creating Elasticsearch CR: %v", cr)

	cleanupOpts := &test.CleanupOptions{
		TestContext:   ctx,
		Timeout:       cleanupTimeout,
		RetryInterval: cleanupRetryInterval,
	}

	err = f.Client.Create(context.TODO(), cr, cleanupOpts)
	if err != nil {
		return nil, kverrors.Wrap(err, "failed to create elasticsearch CR",
			"name", cr.Name,
			"namespace", cr.Namespace)
	}

	return cr, nil
}

func updateElasticsearchSpec(t *testing.T, f *test.Framework, desired *loggingv1.Elasticsearch) error {
	return wait.Poll(retryInterval, timeout, func() (bool, error) {
		current := &loggingv1.Elasticsearch{}
		key := client.ObjectKey{Name: desired.GetName(), Namespace: desired.GetNamespace()}

		if err := f.Client.Get(context.TODO(), key, current); err != nil {
			if apierrors.IsNotFound(kverrors.Root(err)) {
				// Stop retry because CR not found
				return false, err
			}

			// Transient error retry
			return false, nil
		}

		current.Spec = desired.Spec

		t.Logf("Update Spec: %#v", current.Spec)

		if err := f.Client.Update(context.TODO(), current); err != nil {
			if apierrors.IsConflict(kverrors.Root(err)) {
				// Retry update because resource needs to get updated
				return false, nil
			}

			// Stop retry not recoverable error
			return false, err
		}

		return true, nil
	})
}

// Create the secret that would be generated by CLO normally
func createElasticsearchSecret(t *testing.T, f *test.Framework, ctx *test.Context, uuid string) error {
	t.Log("Creating required secret")
	namespace, err := ctx.GetWatchNamespace()
	if err != nil {
		return kverrors.Wrap(err, "failed to get watch namespace")
	}

	if err := generateCertificates(t, namespace, uuid); err != nil {
		return err
	}

	elasticsearchSecret := utils.Secret(
		elasticsearchNameFor(uuid),
		namespace,
		map[string][]byte{
			"elasticsearch.key": getCertificateContents("elasticsearch.key", uuid),
			"elasticsearch.crt": getCertificateContents("elasticsearch.crt", uuid),
			"logging-es.key":    getCertificateContents("logging-es.key", uuid),
			"logging-es.crt":    getCertificateContents("logging-es.crt", uuid),
			"admin-key":         getCertificateContents("system.admin.key", uuid),
			"admin-cert":        getCertificateContents("system.admin.crt", uuid),
			"admin-ca":          getCertificateContents("ca.crt", uuid),
		},
	)

	t.Logf("Creating secret %s/%s", elasticsearchSecret.Namespace, elasticsearchSecret.Name)

	cleanupOpts := &test.CleanupOptions{
		TestContext:   ctx,
		Timeout:       cleanupTimeout,
		RetryInterval: cleanupRetryInterval,
	}

	err = f.Client.Create(context.TODO(), elasticsearchSecret, cleanupOpts)
	if err != nil {
		return kverrors.Wrap(err, "failed to create elasticsearch secret",
			"name", elasticsearchSecret.Name,
			"namespace", elasticsearchSecret.Namespace)
	}

	return nil
}

func updateElasticsearchSecret(t *testing.T, f *test.Framework, ctx *test.Context, uuid string) error {
	namespace, err := ctx.GetWatchNamespace()
	if err != nil {
		return kverrors.Wrap(err, "failed to get watch namespace")
	}

	elasticsearchSecret := &corev1.Secret{}

	name := elasticsearchNameFor(uuid)
	secretName := types.NamespacedName{Name: name, Namespace: namespace}

	if err = f.Client.Get(context.TODO(), secretName, elasticsearchSecret); err != nil {
		return kverrors.Wrap(err, "failed to get secret",
			"name", secretName,
			"namespace", namespace)
	}

	elasticsearchSecret.Data = map[string][]byte{
		"elasticsearch.key": getCertificateContents("elasticsearch.key", uuid),
		"elasticsearch.crt": getCertificateContents("elasticsearch.crt", uuid),
		"logging-es.key":    getCertificateContents("logging-es.key", uuid),
		"logging-es.crt":    getCertificateContents("logging-es.crt", uuid),
		"admin-key":         getCertificateContents("system.admin.key", uuid),
		"admin-cert":        getCertificateContents("system.admin.crt", uuid),
		"admin-ca":          getCertificateContents("ca.crt", uuid),
		"dummy":             []byte("blah"),
	}

	t.Log("Updating required secret...")
	err = f.Client.Update(context.TODO(), elasticsearchSecret)
	if err != nil {
		return err
	}

	return nil
}

func createKibanaCR(namespace string) *loggingv1.Kibana {
	cpuValue, _ := resource.ParseQuantity("100m")
	memValue, _ := resource.ParseQuantity("256Mi")

	return &loggingv1.Kibana{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kibana",
			Namespace: namespace,
		},
		Spec: loggingv1.KibanaSpec{
			ManagementState: loggingv1.ManagementStateManaged,
			Replicas:        1,
			Resources: &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: memValue,
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    cpuValue,
					corev1.ResourceMemory: memValue,
				},
			},
		},
	}
}

func createKibanaSecret(f *test.Framework, ctx *test.Context, esUUID string) error {
	namespace, err := ctx.GetWatchNamespace()
	if err != nil {
		return kverrors.Wrap(err, "failed to get namespace")
	}

	kibanaSecret := utils.Secret(
		kibanaCRName,
		namespace,
		map[string][]byte{
			"key":  getCertificateContents("system.logging.kibana.key", esUUID),
			"cert": getCertificateContents("system.logging.kibana.crt", esUUID),
			"ca":   getCertificateContents("ca.crt", esUUID),
		},
	)

	cleanupOpts := &test.CleanupOptions{
		TestContext:   ctx,
		Timeout:       cleanupTimeout,
		RetryInterval: cleanupRetryInterval,
	}

	err = f.Client.Create(context.TODO(), kibanaSecret, cleanupOpts)
	if err != nil {
		return err
	}

	return nil
}

func createKibanaProxySecret(f *test.Framework, ctx *test.Context, esUUID string) error {
	namespace, err := ctx.GetWatchNamespace()
	if err != nil {
		return kverrors.Wrap(err, "failed to get namespace")
	}

	kibanaProxySecret := utils.Secret(
		fmt.Sprintf("%s-proxy", kibanaCRName),
		namespace,
		map[string][]byte{
			"server-key":     getCertificateContents("kibana-internal.key", esUUID),
			"server-cert":    getCertificateContents("kibana-internal.crt", esUUID),
			"session-secret": []byte("TG85VUMyUHBqbWJ1eXo1R1FBOGZtYTNLTmZFWDBmbkY="),
		},
	)

	cleanupOpts := &test.CleanupOptions{
		TestContext:   ctx,
		Timeout:       cleanupTimeout,
		RetryInterval: cleanupRetryInterval,
	}

	err = f.Client.Create(context.TODO(), kibanaProxySecret, cleanupOpts)
	if err != nil {
		return err
	}

	return nil
}

func generateCertificates(t *testing.T, namespace, uuid string) error {
	workDir := fmt.Sprintf("%s/%s", exampleSecretsPath, uuid)
	storeName := elasticsearchNameFor(uuid)

	err := os.MkdirAll(workDir, os.ModePerm)
	if err != nil {
		return kverrors.Wrap(err, "failed to create certificate tmp dir",
			"dir", workDir)
	}

	cmd := exec.Command("./hack/cert_generation.sh", workDir, namespace, storeName)
	out, err := cmd.Output()
	if err != nil {
		return kverrors.Wrap(err, "failed to generate certificate",
			"store_name", storeName,
			"output", string(out))
	}

	return nil
}

func getCertificateContents(name, uuid string) []byte {
	filename := fmt.Sprintf("%s/%s/%s", exampleSecretsPath, uuid, name)
	return utils.GetFileContents(filename)
}
