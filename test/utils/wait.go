package utils

import (
	"context"
	"reflect"
	"strconv"
	"testing"
	"time"

	loggingv1 "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/pkg/elasticsearch"

	"github.com/operator-framework/operator-sdk/pkg/test"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func WaitForPods(t *testing.T, f *test.Framework, namespace string, labels map[string]string, retryInterval, timeout time.Duration) (*corev1.PodList, error) {
	pods := &corev1.PodList{}

	err := wait.Poll(retryInterval, timeout, func() (bool, error) {
		opts := []client.ListOption{
			client.InNamespace(namespace),
			client.MatchingLabels(labels),
		}
		err := f.Client.Client.List(context.TODO(), pods, opts...)
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

func WaitForNodeStatusCondition(t *testing.T, f *test.Framework, namespace, name string, condition loggingv1.ElasticsearchNodeUpgradeStatus, retryInterval, timeout time.Duration) error {
	elasticsearchCR := &loggingv1.Elasticsearch{}
	elasticsearchName := types.NamespacedName{Name: name, Namespace: namespace}

	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		err = f.Client.Get(context.TODO(), elasticsearchName, elasticsearchCR)
		if err != nil {
			if errors.IsNotFound(err) {
				t.Logf("Waiting for availability of %s elasticsearch\n", name)
				return false, nil
			}
			return false, err
		}

		allMatch := true
		for _, node := range elasticsearchCR.Status.Nodes {
			t.Log("\tActual  status", node.UpgradeStatus)
			t.Log("\tDesired status", condition)
			if !reflect.DeepEqual(node.UpgradeStatus, condition) {
				t.Log("\t\tDid not match")
				allMatch = false
			} else {
				t.Log("\t\tMatch!")
				break
			}
		}

		if allMatch {
			return true, nil
		}
		t.Logf("Waiting for full condition match of %s elasticsearch\n", name)
		return false, nil
	})
	if err != nil {
		return err
	}
	t.Logf("Full condition matches\n")
	return nil
}

func WaitForClusterStatusCondition(t *testing.T, f *test.Framework, namespace, name string, condition loggingv1.ClusterCondition, retryInterval, timeout time.Duration) error {
	elasticsearchCR := &loggingv1.Elasticsearch{}
	elasticsearchName := types.NamespacedName{Name: name, Namespace: namespace}

	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		err = f.Client.Get(context.TODO(), elasticsearchName, elasticsearchCR)
		if err != nil {
			if errors.IsNotFound(err) {
				t.Logf("Waiting for availability of %s elasticsearch\n", name)
				return false, nil
			}
			return false, err
		}

		contained := false
		for _, clusterCondition := range elasticsearchCR.Status.Conditions {
			t.Log("\tExpected condition", condition)
			t.Log("\tReal     condition", clusterCondition)
			if reflect.DeepEqual(clusterCondition, condition) {
				contained = true
			}
		}

		if contained {
			return true, nil
		}
		t.Logf("Waiting for full condition match of %s elasticsearch\n", name)
		return false, nil
	})
	if err != nil {
		return err
	}
	t.Logf("Full condition matches\n")
	return nil
}

func WaitForReadyDeployment(t *testing.T, kubeclient kubernetes.Interface, namespace, name string, replicas int,
	retryInterval, timeout time.Duration) error {
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		deployment, err := kubeclient.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				t.Logf("Waiting for availability of Deployment: %s in Namespace: %s \n", name, namespace)
				return false, nil
			}
			return false, err
		}

		if int(deployment.Status.ReadyReplicas) >= replicas {
			return true, nil
		}
		t.Logf("Waiting for full readiness of %s deployment (%d/%d)\n", name,
			deployment.Status.ReadyReplicas, replicas)
		return false, nil
	})
	if err != nil {
		return err
	}
	t.Logf("Deployment ready (%d/%d)\n", replicas, replicas)
	return nil
}

func WaitForRolloutComplete(t *testing.T, f *test.Framework, namespace string, labels map[string]string, excludePods []string, retryInterval, timeout time.Duration) (*corev1.PodList, error) {
	pods := &corev1.PodList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(labels),
	}

	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		err = f.Client.Client.List(context.TODO(), pods, opts...)
		if err != nil {
			if errors.IsNotFound(err) {
				t.Logf("Waiting for availability of pods with labels: %v in Namespace: %s \n", labels, namespace)
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

		if len(pods.Items) == readyPods {
			return true, nil
		}

		t.Logf("Waiting for availability of pods with labels: %v in Namespace: %s (%d/%d)\n",
			labels, namespace, readyPods, len(pods.Items),
		)

		return false, nil
	})
	if err != nil {
		return nil, err
	}
	t.Logf("Pods ready")
	return pods, nil
}

func WaitForStatefulset(t *testing.T, kubeclient kubernetes.Interface, namespace, name string, replicas int, retryInterval, timeout time.Duration) error {
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		statefulset, err := kubeclient.AppsV1().StatefulSets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				t.Logf("Waiting for availability of %s statefulset\n", name)
				return false, nil
			}
			return false, err
		}

		if int(statefulset.Status.ReadyReplicas) == replicas {
			return true, nil
		}
		t.Logf("Waiting for full availability of %s statefulset (%d/%d)\n", name, statefulset.Status.ReadyReplicas, replicas)
		return false, nil
	})
	if err != nil {
		return err
	}
	t.Logf("Statefulset available (%d/%d)\n", replicas, replicas)
	return nil
}

func WaitForObject(t *testing.T, client test.FrameworkClient, key types.NamespacedName, obj runtime.Object, retryInterval, timeout time.Duration) error {
	return wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		err = client.Get(context.TODO(), key, obj)
		if err != nil {
			if errors.IsNotFound(err) {
				return true, err
			}
			return false, nil
		}
		t.Logf("Found object %s", key.String())
		return true, nil
	})
}

func WaitForIndexTemplateReplicas(t *testing.T, kubeclient kubernetes.Interface, namespace, clusterName string, replicas int32, retryInterval, timeout time.Duration) error {
	// mock out Secret response from client
	mockClient := fake.NewFakeClient(getMockedSecret(clusterName, namespace))
	esClient := elasticsearch.NewClient(clusterName, namespace, mockClient)

	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		// get all index replica count
		indexTemplates, err := esClient.GetIndexTemplates()
		if err != nil {
			t.Logf("Received error: %v", err)
			return false, nil
		}

		// for each index -- check replica count
		for templateName, template := range indexTemplates {
			if numberOfReplicas := parseString("settings.index.number_of_replicas", template.(map[string]interface{})); numberOfReplicas != "" {
				currentReplicas, err := strconv.ParseInt(numberOfReplicas, 10, 32)
				if err != nil {
					return false, err
				}

				if int32(currentReplicas) == replicas {
					continue
				}

				t.Logf("Index template %s did not have correct replica count (%d/%d)", templateName, currentReplicas, replicas)
				return false, nil
			} else {
				return false, nil
			}
		}

		return true, nil
	})
	if err != nil {
		return err
	}
	t.Logf("All index templates have correct replica count of %d\n", replicas)
	return nil
}

func WaitForIndexReplicas(t *testing.T, kubeclient kubernetes.Interface, namespace, clusterName string, replicas int32, retryInterval, timeout time.Duration) error {
	// mock out Secret response from client
	mockClient := fake.NewFakeClient(getMockedSecret(clusterName, namespace))
	esClient := elasticsearch.NewClient(clusterName, namespace, mockClient)

	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {

		// get all index replica count
		indexHealth, err := esClient.GetIndexReplicaCounts()
		if err != nil {
			return false, nil
		}

		// for each index -- check replica count
		for index, health := range indexHealth {
			if numberOfReplicas := parseString("settings.index.number_of_replicas", health.(map[string]interface{})); numberOfReplicas != "" {
				currentReplicas, err := strconv.ParseInt(numberOfReplicas, 10, 32)
				if err != nil {
					return false, err
				}

				if int32(currentReplicas) == replicas {
					continue
				}

				t.Logf("Index %s did not have correct replica count (%d/%d)", index, currentReplicas, replicas)
				return false, nil
			} else {
				return false, nil
			}
		}

		return true, nil
	})
	if err != nil {
		return err
	}
	t.Logf("All indices have correct replica count of %d\n", replicas)
	return nil
}
