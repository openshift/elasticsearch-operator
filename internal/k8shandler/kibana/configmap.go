package kibana

import (
	"reflect"

	"github.com/ViaQ/logerr/kverrors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// NewConfigMap stubs an instance of Configmap
func NewConfigMap(configmapName string, namespace string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmapName,
			Namespace: namespace,
		},
		Data: data,
	}
}

func (clusterRequest *KibanaRequest) CreateOrUpdateConfigMap(configMap *corev1.ConfigMap) error {
	err := clusterRequest.Create(configMap)
	if err == nil {
		return nil
	}

	errCtx := kverrors.NewContext("configmap", configMap.Name,
		"namespace", configMap.Namespace,
		"cluster", configMap.ClusterName)

	if !apierrors.IsAlreadyExists(err) {
		return errCtx.Wrap(err, "failed to create configmap")
	}

	current := &corev1.ConfigMap{}

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err = clusterRequest.Get(configMap.Name, current); err != nil {
			if apierrors.IsNotFound(err) {
				// the object doesn't exist -- it was likely culled
				// recreate it on the next time through if necessary
				return nil
			}
			return errCtx.Wrap(err, "failed to get configmap")

		}

		if reflect.DeepEqual(configMap.Data, current.Data) {
			return nil
		}
		current.Data = configMap.Data

		changed := false
		// if configMap specified labels ensure that current has them...
		if len(configMap.ObjectMeta.Labels) > 0 {
			for key, val := range configMap.ObjectMeta.Labels {
				if currentVal, ok := current.ObjectMeta.Labels[key]; ok {
					if currentVal != val {
						current.ObjectMeta.Labels[key] = val
						changed = true
					}
				} else {
					current.ObjectMeta.Labels[key] = val
					changed = true
				}
			}
		} else {
			return nil
		}
		if !changed {
			// shortcut updating -- we didn't change anything
			return nil
		}

		return clusterRequest.Update(current)
	})
	if retryErr != nil {
		return errCtx.Wrap(err, "failed to create or update configmap")
	}
	return nil
}

// RemoveConfigMap with a given name and namespace
func (clusterRequest *KibanaRequest) RemoveConfigMap(configmapName string) error {
	configMap := NewConfigMap(
		configmapName,
		clusterRequest.cluster.Namespace,
		map[string]string{},
	)

	err := clusterRequest.Delete(configMap)
	if err != nil && !apierrors.IsNotFound(err) {
		return kverrors.Wrap(err, "failure deleting configmap",
			"configmap", configmapName)
	}

	return nil
}
