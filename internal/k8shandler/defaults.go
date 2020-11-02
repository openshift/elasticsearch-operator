package k8shandler

import (
	"fmt"

	"github.com/ViaQ/logerr/kverrors"
	api "github.com/openshift/elasticsearch-operator/apis/logging/v1"
)

const (
	modeUnique    = "unique"
	modeSharedOps = "shared_ops"

	defaultMode = modeSharedOps
	// ES
	defaultESCpuRequest    = "100m"
	defaultESMemoryLimit   = "4Gi"
	defaultESMemoryRequest = "1Gi"
	// ESProxy
	defaultESProxyCPURequest    = "100m"
	defaultESProxyMemoryLimit   = "256Mi"
	defaultESProxyMemoryRequest = "256Mi"

	maxMasterCount       = 3
	maxPrimaryShardCount = 5

	elasticsearchCertsPath  = "/etc/openshift/elasticsearch/secret"
	elasticsearchConfigPath = "/usr/share/java/elasticsearch/config"
	heapDumpLocation        = "/elasticsearch/persistent/heapdump.hprof"

	yellowClusterState = "yellow"
	greenClusterState  = "green"
)

var desiredClusterStates = []string{yellowClusterState, greenClusterState}

func kibanaIndexMode(mode string) (string, error) {
	if mode == "" {
		return defaultMode, nil
	}
	if mode == modeUnique || mode == modeSharedOps {
		return mode, nil
	}
	return "", kverrors.New("invalid kibana index mode provided",
		"mode", mode)
}

func esUnicastHost(clusterName, namespace string) string {
	return fmt.Sprintf("%v-cluster.%v.svc", clusterName, namespace)
}

func calculatePrimaryCount(dpl *api.Elasticsearch) int {
	dataNodeCount := int(getDataCount(dpl))
	if dataNodeCount > maxPrimaryShardCount {
		return maxPrimaryShardCount
	}

	// we can just return this without error checking because we validate
	// we have at least one data node in the cluster
	return dataNodeCount
}

func calculateReplicaCount(dpl *api.Elasticsearch) int {
	dataNodeCount := int(getDataCount(dpl))
	repType := dpl.Spec.RedundancyPolicy
	switch repType {
	case api.FullRedundancy:
		return dataNodeCount - 1
	case api.MultipleRedundancy:
		return (dataNodeCount - 1) / 2
	case api.SingleRedundancy:
		return 1
	case api.ZeroRedundancy:
		return 0
	default:
		if dataNodeCount == 1 {
			return 0
		}
		return 1
	}
}
