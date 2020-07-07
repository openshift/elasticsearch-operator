package utils

import (
	"context"
	"fmt"

	loggingv1 "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"

	"github.com/operator-framework/operator-sdk/pkg/test"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	KibanaCRName = "kibana"
)

func CreateKibanaCR(namespace string) *loggingv1.Kibana {
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

func CreateKibanaSecret(f *test.Framework, ctx *test.Context, esUUID string) error {
	namespace, err := ctx.GetWatchNamespace()
	if err != nil {
		return fmt.Errorf("Could not get namespace: %v", err)
	}

	kibanaSecret := NewSecret(
		KibanaCRName,
		namespace,
		map[string][]byte{
			"key":  getCertificateContents("system.logging.kibana.key", esUUID),
			"cert": getCertificateContents("system.logging.kibana.crt", esUUID),
			"ca":   getCertificateContents("ca.crt", esUUID),
		},
	)

	cleanupOpts := &test.CleanupOptions{
		TestContext:   ctx,
		Timeout:       DefaultCleanupTimeout,
		RetryInterval: DefaultCleanupRetryInterval,
	}

	err = f.Client.Create(context.TODO(), kibanaSecret, cleanupOpts)
	if err != nil {
		return err
	}

	return nil
}

func CreateKibanaProxySecret(f *test.Framework, ctx *test.Context, esUUID string) error {
	namespace, err := ctx.GetWatchNamespace()
	if err != nil {
		return fmt.Errorf("Could not get namespace: %v", err)

	}

	kibanaProxySecret := NewSecret(
		fmt.Sprintf("%s-proxy", KibanaCRName),
		namespace,
		map[string][]byte{
			"server-key":     getCertificateContents("kibana-internal.key", esUUID),
			"server-cert":    getCertificateContents("kibana-internal.crt", esUUID),
			"session-secret": []byte("TG85VUMyUHBqbWJ1eXo1R1FBOGZtYTNLTmZFWDBmbkY="),
		},
	)

	cleanupOpts := &test.CleanupOptions{
		TestContext:   ctx,
		Timeout:       DefaultCleanupTimeout,
		RetryInterval: DefaultCleanupRetryInterval,
	}

	err = f.Client.Create(context.TODO(), kibanaProxySecret, cleanupOpts)
	if err != nil {
		return err
	}

	return nil
}
