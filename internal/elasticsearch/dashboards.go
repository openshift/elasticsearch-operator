package elasticsearch

import (
	"context"
	"io/ioutil"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"
	"github.com/openshift/elasticsearch-operator/internal/manifests/configmap"
	"github.com/openshift/elasticsearch-operator/internal/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	cm := configmap.New(
		grafanaCMName,
		grafanaCMNameSpace,
		map[string]string{
			"console.openshift.io/dashboard": "true",
		},
		map[string]string{
			"openshift-elasticsearch.json": string(b),
		},
	)

	key := client.ObjectKey{Name: grafanaCMName, Namespace: grafanaCMNameSpace}
	err = configmap.Delete(context.TODO(), er.client, key)
	if err != nil && !apierrors.IsNotFound(kverrors.Root(err)) {
		return kverrors.Wrap(err, "failed to delete elasticsearch dashboard config map",
			"cluster", er.cluster.Name,
			"namespace", er.cluster.Namespace,
		)
	}

	err = configmap.Create(context.TODO(), er.client, cm)
	if err != nil && !apierrors.IsAlreadyExists(kverrors.Root(err)) {
		return kverrors.Wrap(err, "failed to create elasticsearch dashboard config map",
			"cluster", er.cluster.Name,
			"namespace", er.cluster.Namespace,
		)
	}

	return nil
}

// RemoveDashboardConfigMap removes the config map in the grafana dashboard
func RemoveDashboardConfigMap(c client.Client) {
	key := client.ObjectKey{Name: grafanaCMName, Namespace: grafanaCMNameSpace}

	err := configmap.Delete(context.TODO(), c, key)
	if err != nil {
		if apierrors.IsNotFound(kverrors.Root(err)) {
			return
		}
		log.Error(err, "error deleting grafana configmap")
	}
}
