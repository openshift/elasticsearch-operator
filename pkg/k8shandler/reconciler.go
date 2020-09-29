package k8shandler

import (
	"fmt"

	"github.com/go-logr/logr"
	elasticsearchv1 "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/pkg/elasticsearch"
	"github.com/openshift/elasticsearch-operator/pkg/log"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

type ElasticsearchRequest struct {
	client   client.Client
	cluster  *elasticsearchv1.Elasticsearch
	esClient elasticsearch.Client
	ll       logr.Logger
}

// L is the logger used for this request.
// TODO This needs to be removed in favor of using context.Context() with values.
func (er *ElasticsearchRequest) L() logr.Logger {
	if er.ll == nil {
		er.ll = log.WithValues("cluster", er.cluster.Name, "namespace", er.cluster.Namespace)
	}
	return er.ll
}

func Reconcile(requestCluster *elasticsearchv1.Elasticsearch, requestClient client.Client) error {

	esClient := elasticsearch.NewClient(requestCluster.Name, requestCluster.Namespace, requestClient)

	elasticsearchRequest := ElasticsearchRequest{
		client:   requestClient,
		cluster:  requestCluster,
		esClient: esClient,
		ll:       log.WithValues("cluster", requestCluster.Name, "namespace", requestCluster.Namespace),
	}

	// Ensure existence of servicesaccount
	if err := elasticsearchRequest.CreateOrUpdateServiceAccount(); err != nil {
		return fmt.Errorf("Failed to reconcile ServiceAccount for Elasticsearch cluster: %v", err)
	}

	// Ensure existence of clusterroles and clusterrolebindings
	if err := elasticsearchRequest.CreateOrUpdateRBAC(); err != nil {
		return fmt.Errorf("Failed to reconcile Roles and RoleBindings for Elasticsearch cluster: %v", err)
	}

	// Ensure existence of config maps
	if err := elasticsearchRequest.CreateOrUpdateConfigMaps(); err != nil {
		return fmt.Errorf("Failed to reconcile ConfigMaps for Elasticsearch cluster: %v", err)
	}

	if err := elasticsearchRequest.CreateOrUpdateServices(); err != nil {
		return fmt.Errorf("Failed to reconcile Services for Elasticsearch cluster: %v", err)
	}

	if err := elasticsearchRequest.CreateOrUpdateDashboards(); err != nil {
		return fmt.Errorf("Failed to reconcile Dashboards for Elasticsearch cluster: %v", err)
	}

	// Ensure Elasticsearch cluster itself is up to spec
	//if err = elasticsearch.CreateOrUpdateElasticsearchCluster(cluster, "elasticsearch", "elasticsearch"); err != nil {
	if err := elasticsearchRequest.CreateOrUpdateElasticsearchCluster(); err != nil {
		return fmt.Errorf("Failed to reconcile Elasticsearch deployment spec: %v", err)
	}

	// Ensure existence of service monitors
	if err := elasticsearchRequest.CreateOrUpdateServiceMonitors(); err != nil {
		return fmt.Errorf("Failed to reconcile Service Monitors for Elasticsearch cluster: %v", err)
	}

	// Ensure existence of prometheus rules
	if err := elasticsearchRequest.CreateOrUpdatePrometheusRules(); err != nil {
		return fmt.Errorf("Failed to reconcile PrometheusRules for Elasticsearch cluster: %v", err)
	}

	// Ensure index management is in place
	if err := elasticsearchRequest.CreateOrUpdateIndexManagement(); err != nil {
		return fmt.Errorf("Failed to reconcile IndexMangement for Elasticsearch cluster: %v", err)
	}

	return nil
}
