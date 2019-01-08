package k8shandler

import (
	"context"
	"fmt"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1alpha1"
)

// CreateOrUpdateServiceAccount ensures the existence of the serviceaccount for Elasticsearch cluster
func CreateOrUpdateServiceAccount(client client.Client, dpl *v1alpha1.Elasticsearch) (string, error) {
	serviceAccountName := v1alpha1.ServiceAccountName

	owner := asOwner(dpl)

	err := createOrUpdateServiceAccount(client, serviceAccountName, dpl.Namespace, owner)
	if err != nil {
		return serviceAccountName, fmt.Errorf("Failure creating ServiceAccount %v", err)
	}

	return serviceAccountName, nil
}

func createOrUpdateServiceAccount(client client.Client, serviceAccountName, namespace string, owner metav1.OwnerReference) error {
	err := client.Get(context.TODO(), types.NamespacedName{Name: serviceAccountName, Namespace: namespace}, &v1.ServiceAccount{})
	if err != nil {
		elasticsearchSA := serviceAccount(serviceAccountName, namespace)
		addOwnerRefToObject(elasticsearchSA, owner)
		err = client.Create(context.TODO(), elasticsearchSA)
		if err != nil {
			return fmt.Errorf("Failure constructing serviceaccount for the Elasticsearch cluster: %v", err)
		}
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
