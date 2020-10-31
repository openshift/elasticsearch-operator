package k8shandler

import (
	"context"
	"fmt"

	"github.com/ViaQ/logerr/kverrors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

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

	elasticsearchScMonitor := createServiceMonitor(serviceMonitorName, dpl.Name, dpl.Namespace, labelsWithDefault)
	dpl.AddOwnerRefTo(elasticsearchScMonitor)

	err := er.client.Create(context.TODO(), elasticsearchScMonitor)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return kverrors.Wrap(err, "failed to construct Elasticsearch ServiceMonitor")
	}

	return nil
}

func createServiceMonitor(serviceMonitorName, clusterName, namespace string, labels map[string]string) *monitoringv1.ServiceMonitor {
	svcMonitor := serviceMonitor(serviceMonitorName, namespace, labels)
	labelSelector := metav1.LabelSelector{
		MatchLabels: labels,
	}
	tlsConfig := monitoringv1.TLSConfig{
		CAFile:     prometheusCAFile,
		ServerName: fmt.Sprintf("%s-%s.%s.svc", clusterName, "metrics", namespace),
		// ServerName can be e.g. elasticsearch-metrics.openshift-logging.svc
	}
	proxy := monitoringv1.Endpoint{
		Port:            clusterName,
		Path:            "/metrics",
		Scheme:          "https",
		BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
		TLSConfig:       &tlsConfig,
	}
	elasticsearch := monitoringv1.Endpoint{
		Port:            clusterName,
		Path:            "/_prometheus/metrics",
		Scheme:          "https",
		BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
		TLSConfig:       &tlsConfig,
	}
	svcMonitor.Spec = monitoringv1.ServiceMonitorSpec{
		JobLabel:  "monitor-elasticsearch",
		Endpoints: []monitoringv1.Endpoint{proxy, elasticsearch},
		Selector:  labelSelector,
		NamespaceSelector: monitoringv1.NamespaceSelector{
			MatchNames: []string{namespace},
		},
	}
	return svcMonitor
}

func serviceMonitor(serviceMonitorName string, namespace string, labels map[string]string) *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       monitoringv1.ServiceMonitorsKind,
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceMonitorName,
			Namespace: namespace,
			Labels:    labels,
		},
	}
}
