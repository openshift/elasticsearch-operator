package kibana

import (
	"context"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/openshift/elasticsearch-operator/internal/manifests/serviceaccount"
	"github.com/openshift/elasticsearch-operator/internal/utils"
)

// CreateOrUpdateServiceAccount creates or updates a ServiceAccount for logging with the given name
func (clusterRequest *KibanaRequest) CreateOrUpdateServiceAccount(name string, annotations map[string]string) error {
	sa := serviceaccount.New(name, clusterRequest.cluster.Namespace, annotations)

	utils.AddOwnerRefToObject(sa, getOwnerRef(clusterRequest.cluster))

	err := serviceaccount.CreateOrUpdate(context.TODO(), clusterRequest.client, sa)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update kibana serviceaccount",
			"cluster", clusterRequest.cluster.Name,
			"namespace", clusterRequest.cluster.Namespace,
		)
	}

	return nil
}
