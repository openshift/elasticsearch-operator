package k8shandler

import (
	"context"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"
	"github.com/go-logr/logr"
	elasticsearchv1 "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/internal/elasticsearch"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func SecretReconcile(requestCluster *elasticsearchv1.Elasticsearch, requestClient client.Client) error {
	var secretChanged bool

	newSecretHash := getSecretDataHash(requestCluster.Name, requestCluster.Namespace, requestClient)

	nretries := -1
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		nretries++

		cluster := &elasticsearchv1.Elasticsearch{}
		if err := requestClient.Get(context.TODO(), types.NamespacedName{Name: requestCluster.Name, Namespace: requestCluster.Namespace}, cluster); err != nil {
			return err
		}

		// compare the new secret with current one in the nodes
		for _, node := range nodes[nodeMapKey(requestCluster.Name, requestCluster.Namespace)] {
			if node.getSecretHash() != "" && newSecretHash != node.getSecretHash() {

				// Cluster's secret has been updated, update the cluster status to be redeployed
				_, nodeStatus := getNodeStatus(node.name(), &cluster.Status)
				if nodeStatus.UpgradeStatus.ScheduledForCertRedeploy != corev1.ConditionTrue {
					secretChanged = true
					nodeStatus.UpgradeStatus.ScheduledForCertRedeploy = corev1.ConditionTrue
				}
			}
		}

		if secretChanged {
			if err := requestClient.Status().Update(context.TODO(), cluster); err != nil {
				return err
			}
		}
		return nil
	})

	if retryErr != nil {
		return kverrors.Wrap(retryErr, "failed to update status for cert redeploys",
			"cluster", requestCluster.Name,
			"retries", nretries)
	}

	return nil
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
		return kverrors.Wrap(err, "Failed to reconcile ServiceAccount for Elasticsearch cluster")
	}

	// Ensure existence of clusterroles and clusterrolebindings
	if err := elasticsearchRequest.CreateOrUpdateRBAC(); err != nil {
		return kverrors.Wrap(err, "Failed to reconcile Roles and RoleBindings for Elasticsearch cluster")
	}

	// Ensure existence of config maps
	if err := elasticsearchRequest.CreateOrUpdateConfigMaps(); err != nil {
		return kverrors.Wrap(err, "Failed to reconcile ConfigMaps for Elasticsearch cluster")
	}

	if err := elasticsearchRequest.CreateOrUpdateServices(); err != nil {
		return kverrors.Wrap(err, "Failed to reconcile Services for Elasticsearch cluster")
	}

	if err := elasticsearchRequest.CreateOrUpdateDashboards(); err != nil {
		return kverrors.Wrap(err, "Failed to reconcile Dashboards for Elasticsearch cluster")
	}

	// Ensure Elasticsearch cluster itself is up to spec
	if err := elasticsearchRequest.CreateOrUpdateElasticsearchCluster(); err != nil {
		return kverrors.Wrap(err, "Failed to reconcile Elasticsearch deployment spec")
	}

	// Ensure existence of service monitors
	if err := elasticsearchRequest.CreateOrUpdateServiceMonitors(); err != nil {
		return kverrors.Wrap(err, "Failed to reconcile Service Monitors for Elasticsearch cluster")
	}

	// Ensure existence of prometheus rules
	if err := elasticsearchRequest.CreateOrUpdatePrometheusRules(); err != nil {
		// no need to error out here, we can just mark ourselves as degraded and report why
		elasticsearchRequest.UpdateDegradedCondition(true, "Missing Prometheus Rules", err.Error())
	} else {
		// TODO: when we eventually add additional cases to be degraded we will need to adjust this
		// so we don't inadvertently remove other degraded status messages
		elasticsearchRequest.UpdateDegradedCondition(false, "", "")
	}

	// Ensure index management is in place
	if err := elasticsearchRequest.CreateOrUpdateIndexManagement(); err != nil {
		return kverrors.Wrap(err, "Failed to reconcile IndexMangement for Elasticsearch cluster")
	}

	return nil
}
