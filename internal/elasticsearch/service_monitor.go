package elasticsearch

import (
	"context"
	"fmt"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/openshift/elasticsearch-operator/internal/manifests/servicemonitor"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	prometheusCAFile = "service-ca.crt"
)

// CreateOrUpdateServiceMonitors ensures the existence of ServiceMonitors for Elasticsearch cluster
func (er *ElasticsearchRequest) CreateOrUpdateServiceMonitors() error {
	dpl := er.cluster

	serviceMonitorName := fmt.Sprintf("monitor-%s-%s", dpl.Name, "cluster")

	labelsWithDefault := appendDefaultLabel(dpl.Name, dpl.Labels)
	labelsSelector := appendDefaultLabel(dpl.Name, map[string]string{
		"scrape-metrics": "enabled",
	})

	tlsConfig := monitoringv1.TLSConfig{
		SafeTLSConfig: monitoringv1.SafeTLSConfig{
			CA: monitoringv1.SecretOrConfigMap{
				ConfigMap: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: serviceCABundleName(dpl.Name),
					},
					Key: prometheusCAFile,
				},
			},
			// ServerName can be e.g. elasticsearch-metrics.openshift-logging.svc
			ServerName: fmt.Sprintf("%s-%s.%s.svc", dpl.Name, "metrics", dpl.Namespace),
		},
	}

	tokenSecret := corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: serviceMonitorServiceAccountTokenName(dpl.Name),
		},
		Key: "token",
	}

	endpoints := []monitoringv1.Endpoint{
		{
			Port:              dpl.Name,
			Path:              "/metrics",
			Scheme:            "https",
			TLSConfig:         &tlsConfig,
			BearerTokenSecret: tokenSecret,
		},
		{
			Port:              dpl.Name,
			Path:              "/_prometheus/metrics",
			Scheme:            "https",
			TLSConfig:         &tlsConfig,
			BearerTokenSecret: tokenSecret,
		},
	}

	monitor := servicemonitor.New(serviceMonitorName, dpl.Namespace, labelsWithDefault).
		WithJobLabel("monitor-elasticsearch").
		WithSelector(metav1.LabelSelector{
			MatchLabels: labelsSelector,
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
