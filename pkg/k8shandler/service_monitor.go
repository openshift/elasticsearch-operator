package k8shandler

import (
	"fmt"
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	prometheusCAFile = "/etc/prometheus/configmaps/serving-certs-ca-bundle/service-ca.crt"
)

// CreateOrUpdateServiceMonitors ensures the existence of ServiceMonitors for Elasticsearch cluster
func CreateOrUpdateServiceMonitors(dpl *v1.Elasticsearch, client client.Client) error {
	serviceMonitorName := fmt.Sprintf("monitor-%s-%s", dpl.Name, "cluster")
	owner := getOwnerRef(dpl)

	labelsWithDefault := appendDefaultLabel(dpl.Name, dpl.Labels)

	elasticsearchScMonitor := createServiceMonitor(serviceMonitorName, dpl.Name, dpl.Namespace, labelsWithDefault)
	addOwnerRefToObject(elasticsearchScMonitor, owner)
	err := client.Create(context.TODO(), elasticsearchScMonitor)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("Failure constructing Elasticsearch ServiceMonitor: %v", err)
	}

	// TODO: handle update - use retry.RetryOnConflict

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
	endpoint := monitoringv1.Endpoint{
		Port:            fmt.Sprintf("%s-%s", clusterName, "metrics"),
		Path:            "/_prometheus/metrics",
		Scheme:          "https",
		BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
		TLSConfig:       &tlsConfig,
	}
	svcMonitor.Spec = monitoringv1.ServiceMonitorSpec{
		JobLabel:  "monitor-elasticsearch",
		Endpoints: []monitoringv1.Endpoint{endpoint},
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
			Kind:       monitoringv1.DefaultCrdKinds.ServiceMonitor.Kind,
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceMonitorName,
			Namespace: namespace,
			Labels:    labels,
		},
	}
}
