package utils

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/ViaQ/logerr/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/elasticsearch-operator/internal/utils"
)

func GetFileContents(filePath string) []byte {
	contents, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Error(err, "Unable to read file to get contents")
		return nil
	}

	return contents
}

func ConfigMap(name, namespace string, labels, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: data,
	}
}

func Secret(secretName string, namespace string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: "Opaque",
		Data: data,
	}
}

func WaitForPods(ctx context.Context, k8sClient client.Client, namespace string, labels map[string]string, retryInterval, timeout time.Duration) (*corev1.PodList, error) {
	pods := &corev1.PodList{}

	err := wait.Poll(retryInterval, timeout, func() (bool, error) {
		opts := []client.ListOption{
			client.InNamespace(namespace),
			client.MatchingLabels(labels),
		}
		err := k8sClient.List(ctx, pods, opts...)
		if err != nil {
			if errors.IsNotFound(err) {
				return false, err
			}
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}

	return pods, nil
}

func WaitForRolloutComplete(ctx context.Context, k8sClient client.Client, namespace string, labels map[string]string, excludePods []string, expectedPodCount int, retryInterval, timeout time.Duration) (*corev1.PodList, error) {
	pods := &corev1.PodList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(labels),
	}

	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		err = k8sClient.List(ctx, pods, opts...)
		if err != nil {
			if errors.IsNotFound(err) {
				fmt.Printf("Waiting for availability of pods with labels: %v in Namespace: %s \n", labels, namespace)
				return false, nil
			}
			return false, err
		}

		readyPods := 0
		for _, pod := range pods.Items {
			for _, excluded := range excludePods {
				if pod.GetName() == excluded {
					// Retry we matched at least one excluded pod
					return false, nil
				}
			}

			for _, cond := range pod.Status.Conditions {
				if cond.Type == corev1.PodReady {
					readyPods = readyPods + 1
				}
			}
		}

		if readyPods == expectedPodCount && len(pods.Items) == readyPods {
			return true, nil
		}

		fmt.Printf("Waiting for availability of pods with labels: %v in Namespace: %s (%d/%d)\n",
			labels, namespace, readyPods, len(pods.Items),
		)

		return false, nil
	})
	if err != nil {
		return nil, err
	}
	fmt.Println("Pods ready")
	return pods, nil
}

func GenerateUUID() string {
	uuid, err := utils.RandStringBytes(8)
	if err != nil {
		return ""
	}

	return uuid
}
