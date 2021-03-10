package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	CertRestart      string = "cert_restart"
	RollingRestart   string = "rolling_restart"
	ScheduledRestart string = "scheduled_restart"
	ManagedState     string = "managed"
	UnmanagedState   string = "unmanaged"
)

var (
	restartCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "eo_elasticsearch_cr_restart_total",
			Help: "Total number of times the nodes restarted due to Cert Restart or Rolling Restart or Scheduled Restart.",
		}, []string{"reason"})

	esClusterManagementState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "eo_elasticsearch_cr_cluster_management_state",
			Help: "Number of Elasticsearch cluster that are in Managed state or Unmanaged state.",
		}, []string{"state"})

	metricList = []prometheus.Collector{
		restartCounter,
		esClusterManagementState,
	}
)

// This function registers the custom metrics to the kubernetes controller-runtime default metrics.
func RegisterCustomMetrics() {
	for _, metric := range metricList {
		metrics.Registry.MustRegister(metric)
	}
}

// Increment the metric value by "1" when the node restarts due to cert.
func IncrementRestartCounterCert() {
	restartCounter.With(prometheus.Labels{
		"reason": CertRestart,
	}).Inc()
}

// Increment the metric value by "1" when the node restarts due to rolling.
func IncrementRestartCounterRolling() {
	restartCounter.With(prometheus.Labels{
		"reason": RollingRestart,
	}).Inc()
}

// Increment the metric value by "1" when the node is scheduled for cert restart or rolling restart.
func IncrementRestartCounterScheduled() {
	restartCounter.With(prometheus.Labels{
		"reason": ScheduledRestart,
	}).Inc()
}

// Sets the metric value to "n" when the ES Cluster Management State is Managed.
func SetEsClusterManagementStateManaged() {
	esClusterManagementState.With(prometheus.Labels{
		"state": ManagedState,
	}).Set(1)

	esClusterManagementState.With(prometheus.Labels{
		"state": UnmanagedState,
	}).Set(0)
}

// Sets the metric value to "n" when the ES Cluster Management State is Unmanaged.
func SetEsClusterManagementStateUnmanaged() {
	esClusterManagementState.With(prometheus.Labels{
		"state": ManagedState,
	}).Set(0)

	esClusterManagementState.With(prometheus.Labels{
		"state": UnmanagedState,
	}).Set(1)
}
