package metrics

import (
	"reflect"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	apis "github.com/openshift/elasticsearch-operator/apis/logging/v1"
)

const (
	labelCertRestart            string = "cert_restart"
	labelRollingRestart         string = "rolling_restart"
	labelScheduledRestart       string = "scheduled_restart"
	labelManagedState           string = "managed"
	labelUnmanagedState         string = "unmanaged"
	labelPersistantStorage      string = "persistent"
	labelEphemeralStorage       string = "ephemeral"
	labelFullRedundancy         string = "full"
	labelMultipleRedundancy     string = "multiple"
	labelSingleRedundancy       string = "single"
	labelZeroRedundancy         string = "zero"
	labelRolloverIndexOperation string = "rollover"
	labelDeleteIndexOperation   string = "delete"
)

var (
	restartMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "eo_es_restart_total",
			Help: "Number of times a node has restarted",
		}, []string{"reason"},
	)

	managementStateMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "eo_es_cluster_management_state_info",
			Help: "Management state used by the cluster",
		}, []string{"state"},
	)

	storageTypeMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "eo_es_storage_info",
			Help: "Number of nodes using emphimeral or persistent storage",
		}, []string{"type"},
	)

	redundancyPolicyTypeMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "eo_es_redundancy_policy_info",
			Help: "Redundancy policy used by the cluster",
		}, []string{"policy"},
	)

	documentAgeMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "eo_es_index_retention_seconds",
			Help: "Number of seconds that documents are retained per policy operation",
		}, []string{"policy", "op"},
	)

	deleteNamespaceMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "eo_es_defined_delete_namespaces_total",
			Help: "Number of defined namespaces deleted per index policy",
		}, []string{"policy"},
	)

	memoryConfigurationMetric = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "eo_es_misconfigured_memory_resources_info",
			Help: "Number of nodes with misconfigured memory resources",
		},
	)
)

// This function registers the custom metrics to the kubernetes controller-runtime default metrics.
func RegisterCustomMetrics() {
	metricCollectors := []prometheus.Collector{
		restartMetric,
		managementStateMetric,
		storageTypeMetric,
		redundancyPolicyTypeMetric,
		documentAgeMetric,
		deleteNamespaceMetric,
		memoryConfigurationMetric,
	}

	for _, metric := range metricCollectors {
		metrics.Registry.MustRegister(metric)
	}
}

func CollectNodeMetrics(spec *apis.ElasticsearchSpec) {
	var (
		nodesUsingEphemeral = ephemeralStorageNodeCount(spec.Nodes)
		misconfiguredNodes  = misconfiguredMemoryNodeCount(spec.Nodes, spec.Spec)
	)

	setStorageMetric(true, nodesUsingEphemeral)
	setStorageMetric(false, len(spec.Nodes)-nodesUsingEphemeral)
	setResourceMisconfigurationMetric(misconfiguredNodes)
}

// Increment the metric value by "1" when the node restarts due to cert.
func IncrementRestartCounterCert() {
	restartMetric.With(prometheus.Labels{
		"reason": labelCertRestart,
	}).Inc()
}

// Increment the metric value by "1" when the node restarts due to rolling.
func IncrementRestartCounterRolling() {
	restartMetric.With(prometheus.Labels{
		"reason": labelRollingRestart,
	}).Inc()
}

// Increment the metric value by "1" when the node is scheduled for cert restart or rolling restart.
func IncrementRestartCounterScheduled() {
	restartMetric.With(prometheus.Labels{
		"reason": labelScheduledRestart,
	}).Inc()
}

// Sets the metric value with the number of seconds that a document
// is retained for in a given index for a rollover or delete operation.
func SetIndexRetentionDocumentAge(isDeleteOp bool, mapping string, seconds uint64) {
	label := labelRolloverIndexOperation
	if isDeleteOp {
		label = labelDeleteIndexOperation
	}
	documentAgeMetric.With(prometheus.Labels{
		"policy": mapping,
		"op":     label,
	}).Set(float64(seconds))
}

// Sets the metric value with the number of namespaces that are affected
// by the delete by query operation per index retention policy.
func SetIndexRetentionDeleteNamespaceMetrics(mapping string, namespaces int) {
	deleteNamespaceMetric.With(prometheus.Labels{
		"policy": mapping,
	}).Set(float64(namespaces))
}

// Sets the metric value of the active management state to 1 and the rest to 0.
func SetManagementStateMetric(isManaged bool) {
	managementStateMetric.With(prometheus.Labels{
		"state": labelManagedState,
	}).Set(boolValue(isManaged))

	managementStateMetric.With(prometheus.Labels{
		"state": labelUnmanagedState,
	}).Set(boolValue(!isManaged))
}

// Sets the metric value of the active redudancy policy to 1 and the rest to 0.
func SetRedundancyMetric(policy apis.RedundancyPolicyType) {
	redundancyPolicyTypeMetric.With(prometheus.Labels{
		"policy": labelFullRedundancy,
	}).Set(boolValue(policy == apis.FullRedundancy))

	redundancyPolicyTypeMetric.With(prometheus.Labels{
		"policy": labelMultipleRedundancy,
	}).Set(boolValue(policy == apis.MultipleRedundancy))

	redundancyPolicyTypeMetric.With(prometheus.Labels{
		"policy": labelSingleRedundancy,
	}).Set(boolValue(policy == apis.SingleRedundancy))

	redundancyPolicyTypeMetric.With(prometheus.Labels{
		"policy": labelZeroRedundancy,
	}).Set(boolValue(policy == apis.ZeroRedundancy))
}

func setStorageMetric(isEphemeral bool, nodesUsing int) {
	label := labelPersistantStorage
	if isEphemeral {
		label = labelEphemeralStorage
	}

	storageTypeMetric.With(prometheus.Labels{
		"type": label,
	}).Set(float64(nodesUsing))
}

func setResourceMisconfigurationMetric(nodesMisconfigured int) {
	memoryConfigurationMetric.Set(float64(nodesMisconfigured))
}

func ephemeralStorageNodeCount(nodes []apis.ElasticsearchNode) int {
	count := 0
	emptySpecVol := apis.ElasticsearchStorageSpec{}

	for _, node := range nodes {
		if reflect.DeepEqual(node.Storage, emptySpecVol) || node.Storage.Size == nil {
			count++
		}
	}

	return count
}

func misconfiguredMemoryNodeCount(nodes []apis.ElasticsearchNode, commonSpec apis.ElasticsearchNodeSpec) int {
	var (
		count = 0

		commonLimit   = commonSpec.Resources.Limits.Memory()
		commonRequest = commonSpec.Resources.Requests.Memory()
	)

	// Refer to `newResourceRequirements` in internal/elasticsearch/common.go for
	// better explanation as to when the commonSpec and default values are used.
	for _, node := range nodes {
		var (
			limitMemory   = node.Resources.Limits.Memory()
			requestMemory = node.Resources.Requests.Memory()
		)

		if commonRequest.IsZero() && commonLimit.IsZero() {
			if requestMemory.IsZero() && !limitMemory.IsZero() {
				requestMemory = limitMemory
			} else if !requestMemory.IsZero() && limitMemory.IsZero() {
				limitMemory = requestMemory
			}
		} else {
			if requestMemory.IsZero() {
				requestMemory = commonRequest
				if commonRequest.IsZero() {
					requestMemory = commonLimit
				}
			}

			if limitMemory.IsZero() {
				limitMemory = commonLimit
				if commonLimit.IsZero() {
					limitMemory = commonRequest
				}
			}
		}

		if !reflect.DeepEqual(requestMemory.Value(), limitMemory.Value()) {
			count++
		}
	}

	return count
}

func boolValue(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
