package elasticsearchop

import (
	"encoding/json"

	"github.com/ViaQ/logerr/kverrors"
	elasticsearch6 "github.com/elastic/go-elasticsearch/v6"
)

const (
	certLocalPath = "/tmp/"
	k8sTokenFile  = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

// Client interface
type Client interface {
	ClusterName() string

	// Cluster Settings API
	GetClusterNodeVersions() ([]string, error)
}

type esClient struct {
	cluster   string
	namespace string
	eoclient  elasticsearch6.Client
}

// NewClient Getting new client
func NewClient(cluster, namespace string, client elasticsearch6.Client) Client {
	return &esClient{
		cluster:   cluster,
		namespace: namespace,
		eoclient:  client,
	}
}

func (ec *esClient) ClusterName() string {
	return ec.cluster
}

func (ec *esClient) errorCtx() kverrors.Context {
	return kverrors.NewContext(
		"namespace", ec.namespace,
		"cluster", ec.ClusterName(),
	)
}

func getMapFromBody(rawBody string) (map[string]interface{}, error) {
	if rawBody == "" {
		return make(map[string]interface{}), nil
	}
	var results map[string]interface{}
	err := json.Unmarshal([]byte(rawBody), &results)
	if err != nil {
		results = make(map[string]interface{})
		results["results"] = rawBody
	}

	return results, nil
}
