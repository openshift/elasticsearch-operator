package k8shandler

import (
	apps "k8s.io/api/apps/v1beta2"
)

func (cfg *ESClusterNodeConfig) isDifferent(sset *apps.StatefulSet) (bool, error) {
	// Check replicas number
	if cfg.getReplicas() != *sset.Spec.Replicas {
		return true, nil
	}

	// Check if the Variables are the desired ones

	return false, nil
}
