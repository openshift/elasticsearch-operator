package k8shandler

import (
	"context"
	"fmt"

	"github.com/openshift/elasticsearch-operator/pkg/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

const (
	namespaceNameEnvVar  = "POD_NAMESPACE"
	defaultNamespaceName = "openshift-operators-redhat"
	namespaceLabelKey    = "openshift.io/elasticsearch-operator"
	namespaceLabelValue  = "true"
)

func getOperatorNamespace() string {

	return utils.LookupEnvWithDefault(namespaceNameEnvVar, defaultNamespaceName)
}

func (elasticsearchRequest *ElasticsearchRequest) EnsureNamespaceLabel() error {

	namespaceName := getOperatorNamespace()
	namespace, err := elasticsearchRequest.getNamespace(namespaceName)
	if err != nil {
		return fmt.Errorf("Unable to get namespace while ensuring labels: %v", err)
	}

	if namespace.ObjectMeta.Labels == nil {
		namespace.ObjectMeta.Labels = make(map[string]string)
	}

	value, ok := namespace.ObjectMeta.Labels[namespaceLabelKey]
	if !ok || value != namespaceLabelValue {
		namespace.ObjectMeta.Labels[namespaceLabelKey] = namespaceLabelValue
		return elasticsearchRequest.updateNamespace(namespace)
	}

	return nil
}

func (elasticsearchRequest *ElasticsearchRequest) updateNamespace(desired *v1.Namespace) error {

	client := elasticsearchRequest.client

	current := &v1.Namespace{}
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := client.Get(context.TODO(), types.NamespacedName{Name: desired.Name}, current); err != nil {
			if errors.IsNotFound(err) {
				// the object doesn't exist -- it was likely culled
				// recreate it on the next time through if necessary
				return nil
			}
			return fmt.Errorf("Failed to get namespace %q: %v", desired.Name, err)
		}

		current.Labels = desired.Labels
		if err := client.Update(context.TODO(), current); err != nil {
			return err
		}
		return nil
	})
	if retryErr != nil {
		return retryErr
	}

	return nil
}

func (elasticsearchRequest *ElasticsearchRequest) getNamespace(namespaceName string) (*v1.Namespace, error) {

	client := elasticsearchRequest.client

	namespace := &v1.Namespace{}

	if err := client.Get(context.TODO(), types.NamespacedName{Name: namespaceName}, namespace); err != nil {
		if !errors.IsNotFound(err) {
			return namespace, fmt.Errorf("Failed to get namespace %q: %v", namespaceName, err)
		}
	}

	return namespace, nil
}
