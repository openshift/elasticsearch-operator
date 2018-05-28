package k8shandler

import (
	"fmt"

	v1alpha1 "github.com/t0ffel/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"

	"github.com/sirupsen/logrus"
)

// Reconcile reconciles the cluster's state to the spec specified
func Reconcile(es *v1alpha1.Elasticsearch) (err error) {
	logrus.Info("Started reconciliation")

	// Ensure existence of services
	err = createOrUpdateServices(es)
	if err != nil {
		return fmt.Errorf("Failed to reconcile Services for Elasticsearch cluster: %v", err)
	}

	// Ensure existence of services
	err = createOrUpdateConfigMaps(es)
	if err != nil {
		return fmt.Errorf("Failed to reconcile ConfigMaps for Elasticsearch cluster: %v", err)
	}

	// TODO: Ensure existence of storage?

	// Ensure Elasticsearch cluster itself is up to spec
	err = createOrUpdateElasticsearchCluster(es)
	if err != nil {
		return fmt.Errorf("Failed to reconcile Elasticsearch deployment spec: %v", err)
	}

	// Update Status section with pod names
	err = updateStatus(es)
	if err != nil {
		return fmt.Errorf("Failed to update Elasticsearch status: %v", err)
	}
	return nil

}
