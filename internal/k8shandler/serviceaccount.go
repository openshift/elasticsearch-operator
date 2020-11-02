package k8shandler

import (
	"context"

	"github.com/ViaQ/logerr/kverrors"
	loggingv1 "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateOrUpdateServiceAccount ensures the existence of the serviceaccount for Elasticsearch cluster
func (er *ElasticsearchRequest) CreateOrUpdateServiceAccount() (err error) {
	dpl := er.cluster

	err = createOrUpdateServiceAccount(dpl.Name, dpl.Namespace, er.cluster, er.client)
	if err != nil {
		return kverrors.Wrap(err, "Failure creating ServiceAccount")
	}

	return nil
}

func createOrUpdateServiceAccount(serviceAccountName, namespace string, cluster *loggingv1.Elasticsearch, client client.Client) error {
	serviceAccount := newServiceAccount(serviceAccountName, namespace)
	cluster.AddOwnerRefTo(serviceAccount)

	err := client.Create(context.TODO(), serviceAccount)
	if err != nil {
		if !apierrors.IsAlreadyExists(kverrors.Root(err)) {
			return kverrors.Wrap(err, "failed to construct ServiceAccount for the Elasticsearch cluster")
		}
	}

	return nil
}

// serviceAccount returns a v1.ServiceAccount object
func newServiceAccount(serviceAccountName string, namespace string) *v1.ServiceAccount {
	return &v1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: v1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}
}
