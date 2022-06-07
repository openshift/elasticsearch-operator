package elasticsearch

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	loggingv1 "github.com/openshift/elasticsearch-operator/apis/logging/v1"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestCreateOrUpdateServiceMonitor(t *testing.T) {
	scheme := apiruntime.NewScheme()
	utilruntime.Must(monitoringv1.AddToScheme(scheme))

	tests := []struct {
		desc    string
		cluster *loggingv1.Elasticsearch
		want    *monitoringv1.ServiceMonitor
	}{
		{
			desc: "default labels",
			cluster: &loggingv1.Elasticsearch{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "elasticsearch",
					Namespace: "openshift-logging",
				},
			},
			want: &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "monitor-elasticsearch-cluster",
					Namespace: "openshift-logging",
					Labels:    map[string]string{"cluster-name": "elasticsearch"},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "logging.openshift.io/v1",
							Kind:       "Elasticsearch",
							Name:       "elasticsearch",
							Controller: pointer.Bool(true),
						},
					},
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					JobLabel: "monitor-elasticsearch",
					Endpoints: []monitoringv1.Endpoint{
						{
							Port:            "elasticsearch",
							Path:            "/metrics",
							Scheme:          "https",
							BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
							TLSConfig: &monitoringv1.TLSConfig{
								ServerName: "elasticsearch-metrics.openshift-logging.svc",
								CAFile:     prometheusCAFile,
							},
						},
						{
							Port:            "elasticsearch",
							Path:            "/_prometheus/metrics",
							Scheme:          "https",
							BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
							TLSConfig: &monitoringv1.TLSConfig{
								ServerName: "elasticsearch-metrics.openshift-logging.svc",
								CAFile:     prometheusCAFile,
							},
						},
					},
					NamespaceSelector: monitoringv1.NamespaceSelector{
						MatchNames: []string{"openshift-logging"},
					},
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"cluster-name":   "elasticsearch",
							"scrape-metrics": "enabled",
						},
					},
				},
			},
		},
		{
			desc: "default labels with cr labels",
			cluster: &loggingv1.Elasticsearch{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "elasticsearch",
					Namespace: "openshift-logging",
					Labels: map[string]string{
						"app":                         "jaeger",
						"app.kubernetes.io/component": "elasticsearch",
						"app.kubernetes.io/part-of":   "jaeger",
					},
				},
			},
			want: &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "monitor-elasticsearch-cluster",
					Namespace: "openshift-logging",
					Labels: map[string]string{
						"cluster-name":                "elasticsearch",
						"app":                         "jaeger",
						"app.kubernetes.io/component": "elasticsearch",
						"app.kubernetes.io/part-of":   "jaeger",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "logging.openshift.io/v1",
							Kind:       "Elasticsearch",
							Name:       "elasticsearch",
							Controller: pointer.Bool(true),
						},
					},
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					JobLabel: "monitor-elasticsearch",
					Endpoints: []monitoringv1.Endpoint{
						{
							Port:            "elasticsearch",
							Path:            "/metrics",
							Scheme:          "https",
							BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
							TLSConfig: &monitoringv1.TLSConfig{
								ServerName: "elasticsearch-metrics.openshift-logging.svc",
								CAFile:     prometheusCAFile,
							},
						},
						{
							Port:            "elasticsearch",
							Path:            "/_prometheus/metrics",
							Scheme:          "https",
							BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
							TLSConfig: &monitoringv1.TLSConfig{
								ServerName: "elasticsearch-metrics.openshift-logging.svc",
								CAFile:     prometheusCAFile,
							},
						},
					},
					NamespaceSelector: monitoringv1.NamespaceSelector{
						MatchNames: []string{"openshift-logging"},
					},
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"cluster-name":   "elasticsearch",
							"scrape-metrics": "enabled",
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.desc, func(t *testing.T) {
			client := fake.NewFakeClientWithScheme(scheme)

			req := &ElasticsearchRequest{
				client:  client,
				cluster: test.cluster,
				ll:      log.Log.WithValues("cluster", "test-elasticsearch", "namespace", "test"),
			}

			err := req.CreateOrUpdateServiceMonitors()
			if err != nil {
				t.Errorf("failed with error: %s", err)
			}

			key := types.NamespacedName{
				Name:      "monitor-elasticsearch-cluster",
				Namespace: test.cluster.GetNamespace(),
			}
			got := &monitoringv1.ServiceMonitor{}

			err = client.Get(context.TODO(), key, got)
			if err != nil {
				t.Errorf("failed with error: %s", err)
			}

			if diff := cmp.Diff(got.OwnerReferences, test.want.OwnerReferences); diff != "" {
				t.Errorf("got diff: %s", diff)
			}

			if diff := cmp.Diff(got.Labels, test.want.Labels); diff != "" {
				t.Errorf("got diff: %s", diff)
			}

			if diff := cmp.Diff(got.Spec, test.want.Spec); diff != "" {
				t.Errorf("got diff: %s", diff)
			}
		})
	}
}
