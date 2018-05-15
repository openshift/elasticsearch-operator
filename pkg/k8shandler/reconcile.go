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
	if err != nil  {
		return fmt.Errorf("Failed to reconcile Elasticsearch deployment spec: %v", err)
	}

	// Update Status section with pod names
	err = updateStatus(es)
	if err != nil {
		return fmt.Errorf("Failed to update Elasticsearch status: %v", err)
	}
	return nil

	// Simulate initializer.
	// changed := vr.SetDefaults()
	// if changed {
	// 	return action.Update(vr)
	// }
	// // After first time reconcile, phase will switch to "Running".
	// if vr.Status.Phase == api.ClusterPhaseInitial {
	// 	err = prepareEtcdTLSSecrets(vr)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	// etcd cluster should only be created in first time reconcile.
	// 	ec, err := deployEtcdCluster(vr)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	// Check if etcd cluster is up and running.
	// 	// If not, we need to wait until etcd cluster is up before proceeding to the next state;
	// 	// Hence, we return from here and let the Watch triggers the handler again.
	// 	ready, err := isEtcdClusterReady(ec)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to check if etcd cluster is ready: %v", err)
	// 	}
	// 	if !ready {
	// 		logrus.Infof("Waiting for EtcdCluster (%v) to become ready", ec.Name)
	// 		return nil
	// 	}
	// }

	// err = prepareDefaultVaultTLSSecrets(vr)
	// if err != nil {
	// 	return err
	// }

	// err = prepareVaultConfig(vr)
	// if err != nil {
	// 	return err
	// }

	// err = deployVault(vr)
	// if err != nil {
	// 	return err
	// }

	// err = syncVaultClusterSize(vr)
	// if err != nil {
	// 	return err
	// }

	// vcs, err := getVaultStatus(vr)
	// if err != nil {
	// 	return err
	// }

	// err = syncUpgrade(vr, vcs)
	// if err != nil {
	// 	return err
	// }

	// return updateVaultStatus(vr, vcs)
}

