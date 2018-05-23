package k8shandler

import (
	"fmt"

	v1alpha1 "github.com/t0ffel/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"

	"github.com/sirupsen/logrus"
)

// type ESClusterConfig struct {
// 	Name			string
// 	Masters			[]string
// 	MastersQuorum	int32
// 	DataNodes 		[]string
// 	DataNodeQuorum	int32
// 	ClientNodes		[]string
// }

// Reconcile reconciles the vault cluster's state to the spec specified by vr
// by preparing the TLS secrets, deploying the etcd and vault cluster,
// and finally updating the vault deployment if needed.
func Reconcile(es *v1alpha1.Elasticsearch) (err error) {
	logrus.Info("Started reconciliation")

	// Ensure existence of services
	err = createOrUpdateServices(es)
	if err != nil {
		return fmt.Errorf("Failed to reconcile services for Elasticsearch cluster: %v", err)
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
