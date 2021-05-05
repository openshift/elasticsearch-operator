package k8shandler

import (
	"testing"

	elasticsearchv1 "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/internal/elasticsearch"
	"github.com/openshift/elasticsearch-operator/test/helpers"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDiskUtilizationBelowFloodWatermark(t *testing.T) {
	nodes = map[string][]NodeTypeInterface{}
	var (
		chatter   *helpers.FakeElasticsearchChatter
		client    elasticsearch.Client
		k8sClient = fake.NewFakeClient()
		cluster   = &elasticsearchv1.Elasticsearch{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "elasticsearch",
				Namespace: "openshift-logging",
			},
		}
	)

	const (
		esCluster   = "elasticsearch"
		esNamespace = "openshift-logging"
	)

	chatter = helpers.NewFakeElasticsearchChatter(map[string]helpers.FakeElasticsearchResponses{
		"_nodes/stats/fs": {
			{
				StatusCode: 200,
				Body:       `{"nodes": {"7EN-Wa_EQC6LoANvWcoyHQ": {"name": "elasticsearch-cdm-1-deadbeef", "fs": {"total": {"total_in_bytes": 32737570816, "free_in_bytes": 16315211776, "available_in_bytes": 16315211776}}}}}`,
			},
		},
	})
	client = helpers.NewFakeElasticsearchClient(esCluster, esNamespace, k8sClient, chatter)

	er := ElasticsearchRequest{
		cluster:  cluster,
		client:   k8sClient,
		esClient: client,
	}

	// Populate nodes in operator memory
	key := nodeMapKey(esCluster, esNamespace)
	nodes[key] = populateSingleNode(esCluster)

	if isDiskUtilizationBelow := er.isDiskUtilizationBelowFloodWatermark(); isDiskUtilizationBelow != true {
		t.Errorf("Expected threshold value to be below 95 percent but got more.")
	}
}

func populateSingleNode(clusterName string) []NodeTypeInterface {
	nodes := []NodeTypeInterface{}
	deployments := []runtime.Object{
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "elasticsearch-cdm-1-deadbeef",
				Namespace: "openshift-logging",
			},
		},
	}
	for _, dpl := range deployments {
		dpl := dpl.(*appsv1.Deployment)
		node := &deploymentNode{
			clusterName: clusterName,
			self:        *dpl,
		}
		nodes = append(nodes, node)
	}
	return nodes
}
