package k8shandler

import (
	"context"
	"io/ioutil"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"
	"github.com/openshift/elasticsearch-operator/internal/utils"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultElasticDashboardFile = "/etc/elasticsearch-operator/files/dashboards/logging-dashboard-elasticsearch.json"
	grafanaCMName               = "grafana-dashboard-elasticsearch"
	grafanaCMNameSpace          = "openshift-config-managed"
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
			Name:      grafanaCMName,
			Namespace: grafanaCMNameSpace,
			Labels: map[string]string{
				"console.openshift.io/dashboard": "true",
			},
		},
		Data: map[string]string{
			"openshift-elasticsearch.json": string(b),
		},
	}

	return er.CreateOrUpdateConfigMap(cm)
}

// RemoveDashboardConfigMap removes the config map in the grafana dashboard
func RemoveDashboardConfigMap(client client.Client) {
	cm := getConfigmap(grafanaCMName, grafanaCMNameSpace, client)
	if cm == nil {
		return
	}
	err := client.Delete(context.TODO(), cm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return
		}
		log.Error(err, "error deleting grafana configmap")
	}
}
