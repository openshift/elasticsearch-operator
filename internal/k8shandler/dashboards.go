package k8shandler

import (
	"io/ioutil"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/openshift/elasticsearch-operator/internal/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultElasticDashboardFile = "/etc/elasticsearch-operator/files/dashboards/logging-dashboard-elasticsearch.json"
)

// CreateOrUpdateDashboards creates/updates the cluster logging dashboard ConfigMap
func (er *ElasticsearchRequest) CreateOrUpdateDashboards() error {
	fp := utils.LookupEnvWithDefault("ES_DASHBOARD_FILE", defaultElasticDashboardFile)
	b, err := ioutil.ReadFile(fp)
	if err != nil {
		return kverrors.Wrap(err, "failed to read dashboard file", "filePath", defaultElasticDashboardFile)
	}
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
			"openshift-elasticsearch.json": string(b),
		},
	}

	er.cluster.AddOwnerRefTo(cm)

	return er.CreateOrUpdateConfigMap(cm)
}
