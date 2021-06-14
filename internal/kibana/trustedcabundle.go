package kibana

import (
	"context"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/openshift/elasticsearch-operator/internal/constants"
	"github.com/openshift/elasticsearch-operator/internal/manifests/configmap"
	"github.com/openshift/elasticsearch-operator/internal/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

/*
 * Create or Get Trusted CA Bundle ConfigMap.
 * By setting label "config.openshift.io/inject-trusted-cabundle: true", the cert is automatically filled/updated.
 * Thus, we need the get the contents once again.
 */
func (clusterRequest *KibanaRequest) createOrGetTrustedCABundleConfigMap(name string) (*corev1.ConfigMap, error) {
	configMap := configmap.New(
		name,
		clusterRequest.cluster.Namespace,
		map[string]string{
			constants.InjectTrustedCABundleLabel: "true",
		},
		map[string]string{
			constants.TrustedCABundleKey: "",
		},
	)

	utils.AddOwnerRefToObject(configMap, getOwnerRef(clusterRequest.cluster))

	err := configmap.Create(context.TODO(), clusterRequest.client, configMap)
	if err != nil && !apierrors.IsAlreadyExists(kverrors.Root(err)) {
		return nil, kverrors.Wrap(err, "failed to create trusted CA bundle config map",
			"cluster", clusterRequest.cluster.Name,
		)
	}

	// Get the existing config map which may include an injected CA bundle
	key := client.ObjectKey{Name: name, Namespace: clusterRequest.cluster.Namespace}
	configMap, err = configmap.Get(context.TODO(), clusterRequest.client, key)
	if err != nil {
		return nil, kverrors.Wrap(err, "failed to get trusted CA bundle config map",
			"cluster", clusterRequest.cluster.Name,
		)
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
