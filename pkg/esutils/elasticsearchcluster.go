package esutils

import (
	"github.com/t0ffel/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	"github.com/t0ffel/elasticsearch-operator/pkg/k8shandler"
)

type ElasticsearchCluster struct {
	Name      string
	Namespace string
	Secure    bool
	Selector  map[string]string
	URL       string
}

// New creates new ElasticsearchCluster
func New(es *v1alpha1.Elasticsearch) ElasticsearchCluster {
	cluster := ElasticsearchCluster{
		Name:      es.ObjectMeta.Name,
		Namespace: es.ObjectMeta.Namespace,
		Secure:    es.Spec.Secure.Enabled,
	}

	cluster.Selector = k8shandler.LabelsForESCluster(es.ObjectMeta.Name)
	return cluster
}
