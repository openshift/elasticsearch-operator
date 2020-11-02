package kibana

import (
	"github.com/ViaQ/logerr/kverrors"
	"github.com/openshift/elasticsearch-operator/internal/constants"
	"github.com/openshift/elasticsearch-operator/internal/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

/*
 * Create or Get Trusted CA Bundle ConfigMap.
 * By setting label "config.openshift.io/inject-trusted-cabundle: true", the cert is automatically filled/updated.
 * Thus, we need the get the contents once again.
 */
func (clusterRequest *KibanaRequest) createOrGetTrustedCABundleConfigMap(name string) (*corev1.ConfigMap, error) {
	configMap := NewConfigMap(
		name,
		clusterRequest.cluster.Namespace,
		map[string]string{
			constants.TrustedCABundleKey: "",
		},
	)
	configMap.ObjectMeta.Labels = make(map[string]string)
	configMap.ObjectMeta.Labels[constants.InjectTrustedCABundleLabel] = "true"

	utils.AddOwnerRefToObject(configMap, getOwnerRef(clusterRequest.cluster))

	err := clusterRequest.Create(configMap)
	if err == nil {
		return configMap, nil
	}

	if !apierrors.IsAlreadyExists(err) {
		return nil, kverrors.Wrap(err, "failed to create trusted CA bundle config map",
			"configmap", name,
			"cluster", clusterRequest.cluster.Name)
	}

	// Get the existing config map which may include an injected CA bundle
	if err = clusterRequest.Get(configMap.Name, configMap); err != nil {
		return nil, kverrors.Wrap(err, "failed to get trusted CA bundle config map",
			"configmap", name,
			"cluster", clusterRequest.cluster.Name)
	}
	return configMap, nil
}

func hasTrustedCABundle(configMap *corev1.ConfigMap) bool {
	if configMap == nil {
		return false
	}
	caBundle, ok := configMap.Data[constants.TrustedCABundleKey]
	return ok && caBundle != ""
}

func calcTrustedCAHashValue(configMap *corev1.ConfigMap) (string, error) {
	hashValue := ""
	var err error

	if configMap == nil {
		return hashValue, nil
	}
	caBundle, ok := configMap.Data[constants.TrustedCABundleKey]
	if ok && caBundle != "" {
		hashValue, err = utils.CalculateMD5Hash(caBundle)
		if err != nil {
			return "", kverrors.Wrap(err, "failed to calculate hash")
		}
	}

	if !ok {
		return "", kverrors.New("expected key does not exist in configmap",
			"key", constants.TrustedCABundleKey,
			"cluster", configMap.ClusterName,
			"configmap", configMap.Name)
	}

	return hashValue, nil
}
