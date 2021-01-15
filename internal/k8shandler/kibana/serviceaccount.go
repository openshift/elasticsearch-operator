package kibana

import (
	"github.com/ViaQ/logerr/kverrors"
	"github.com/openshift/elasticsearch-operator/internal/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewServiceAccount stubs a new instance of ServiceAccount
func NewServiceAccount(accountName string, namespace string) *core.ServiceAccount {
	return &core.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: core.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      accountName,
			Namespace: namespace,
		},
	}
}

// CreateOrUpdateServiceAccount creates or updates a ServiceAccount for logging with the given name
func (clusterRequest *KibanaRequest) CreateOrUpdateServiceAccount(name string, annotations *map[string]string) error {
	serviceAccount := NewServiceAccount(name, clusterRequest.cluster.Namespace)
	if annotations != nil {
		if serviceAccount.GetObjectMeta().GetAnnotations() == nil {
			serviceAccount.GetObjectMeta().SetAnnotations(make(map[string]string))
		}
		for key, value := range *annotations {
			serviceAccount.GetObjectMeta().GetAnnotations()[key] = value
		}
	}

	utils.AddOwnerRefToObject(serviceAccount, getOwnerRef(clusterRequest.cluster))

	err := clusterRequest.Create(serviceAccount)
	if err == nil {
		return nil
	}

	errCtx := kverrors.NewContext("service_account", serviceAccount.Name,
		"namespace", clusterRequest.cluster.Namespace,
		"cluster", clusterRequest.cluster.Name)

	if !apierrors.IsAlreadyExists(err) {
		return errCtx.Wrap(err, "failed to create service account")
	}

	current := &core.ServiceAccount{}
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err = clusterRequest.Get(serviceAccount.Name, current); err != nil {
			if apierrors.IsNotFound(err) {
				// the object doesn't exist -- it was likely culled
				// recreate it on the next time through if necessary
				return nil
			}
			return kverrors.Wrap(err, "failed to get service account")
		}
		if annotations != nil && serviceAccount.GetObjectMeta().GetAnnotations() != nil {
			if current.GetObjectMeta().GetAnnotations() == nil {
				current.GetObjectMeta().SetAnnotations(make(map[string]string))
			}
			for key, value := range serviceAccount.GetObjectMeta().GetAnnotations() {
				current.GetObjectMeta().GetAnnotations()[key] = value
			}
		}
		if err = clusterRequest.Update(current); err != nil {
			return err
		}
		return nil
	})
	if retryErr != nil {
		return errCtx.Wrap(retryErr, "failed to update service account")
	}
	return nil
}

// RemoveServiceAccount of given name and namespace
func (clusterRequest *KibanaRequest) RemoveServiceAccount(serviceAccountName string) error {
	serviceAccount := NewServiceAccount(serviceAccountName, clusterRequest.cluster.Namespace)

	if serviceAccountName == "logcollector" {
		// remove our finalizer from the list and update it.
		serviceAccount.ObjectMeta.Finalizers = utils.RemoveString(serviceAccount.ObjectMeta.Finalizers, metav1.FinalizerDeleteDependents)
	}

	err := clusterRequest.Delete(serviceAccount)
	if err != nil && !apierrors.IsNotFound(err) {
		return kverrors.Wrap(err, "failure deleting service account",
			"service_account", serviceAccountName)
	}

	return nil
}
