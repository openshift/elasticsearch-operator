package k8shandler

import (
	"fmt"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	v1alpha1 "github.com/t0ffel/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
)

const (
	defaultServiceAccountName = "aggregated-logging-elasticsearch"
)

// CreateOrUpdateServiceAccount ensures the existence of the serviceaccount for Elasticsearch cluster
func CreateOrUpdateServiceAccount(dpl *v1alpha1.Elasticsearch) (string, error) {
	// In case no serviceaccount is specified in the spec, we'll use the default name for service account
	var serviceAccountName string
	if dpl.Spec.ServiceAccountName == "" {
		serviceAccountName = defaultServiceAccountName
	} else {
		serviceAccountName = dpl.Spec.ServiceAccountName
	}

	owner := asOwner(dpl)

	err := createOrUpdateServiceAccount(serviceAccountName, dpl.Namespace, owner)
	if err != nil {
		return serviceAccountName, fmt.Errorf("Failure creating ServiceAccount %v", err)
	}

	return serviceAccountName, nil
}

func createOrUpdateServiceAccount(serviceAccountName, namespace string, owner metav1.OwnerReference) error {
	elasticsearchSA := serviceAccount(serviceAccountName, namespace)
	addOwnerRefToObject(elasticsearchSA, owner)
	err := sdk.Create(elasticsearchSA)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("Failure constructing serviceaccount for the Elasticsearch cluster: %v", err)
	}
	return nil
}

// serviceAccount returns a v1.ServiceAccount object
func serviceAccount(serviceAccountName string, namespace string) *v1.ServiceAccount {
	return &v1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}
}
