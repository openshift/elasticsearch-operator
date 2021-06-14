package elasticsearch

import (
	"context"
	"fmt"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/openshift/elasticsearch-operator/internal/manifests/servicemonitor"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	prometheusCAFile = "/etc/prometheus/configmaps/serving-certs-ca-bundle/service-ca.crt"
)

// CreateOrUpdateServiceMonitors ensures the existence of ServiceMonitors for Elasticsearch cluster
func (er *ElasticsearchRequest) CreateOrUpdateServiceMonitors() error {
	dpl := er.cluster
	serviceMonitorName := fmt.Sprintf("monitor-%s-%s", dpl.Name, "cluster")

	labelsWithDefault := appendDefaultLabel(dpl.Name, dpl.Labels)
	labelsWithDefault["scrape-metrics"] = "enabled"

	tlsConfig := monitoringv1.TLSConfig{
		CAFile:     prometheusCAFile,
		ServerName: fmt.Sprintf("%s-%s.%s.svc", dpl.Name, "metrics", dpl.Namespace),
		// ServerName can be e.g. elasticsearch-metrics.openshift-logging.svc
	}
	endpoints := []monitoringv1.Endpoint{
		{
			Port:            dpl.Name,
			Path:            "/metrics",
			Scheme:          "https",
			BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
			TLSConfig:       &tlsConfig,
		},
		{
			Port:            dpl.Name,
			Path:            "/_prometheus/metrics",
			Scheme:          "https",
			BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
			TLSConfig:       &tlsConfig,
		},
	}

	monitor := servicemonitor.New(serviceMonitorName, dpl.Namespace, labelsWithDefault).
		WithJobLabel("monitor-elasticsearch").
		WithSelector(metav1.LabelSelector{
			MatchLabels: labelsWithDefault,
		}).
		WithNamespaceSelector(monitoringv1.NamespaceSelector{
			MatchNames: []string{dpl.Namespace},
		}).
		WithEndpoints(endpoints...).
		Build()

	dpl.AddOwnerRefTo(monitor)

	err := servicemonitor.CreateOrUpdate(context.TODO(), er.client, monitor, servicemonitor.Equal, servicemonitor.Mutate)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update elasticsearch servicemonitor",
			"cluster", er.cluster.Name,
			"namespace", er.cluster.Namespace,
		)
	}

	return nil
}
