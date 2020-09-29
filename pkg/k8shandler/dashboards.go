package k8shandler

import (
	"github.com/openshift/elasticsearch-operator/pkg/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultElasticDashboardFile = "/etc/elasticsearch-operator/files/dashboards/logging-dashboard-elasticsearch.json"
)

// CreateOrUpdateDashboards creates/updates the cluster logging dashboard ConfigMap
func (er *ElasticsearchRequest) CreateOrUpdateDashboards() error {
	fp := utils.LookupEnvWithDefault("ES_DASHBOARD_FILE", defaultElasticDashboardFile)
	cm := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: v1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "grafana-dashboard-elasticsearch",
			Namespace: "openshift-config-managed",
			Labels: map[string]string{
				"console.openshift.io/dashboard": "true",
			},
		},
		Data: map[string]string{
			"openshift-elasticsearch.json": string(utils.GetFileContents(fp)),
		},
	}
	return er.CreateOrUpdateConfigMap(cm)
}
