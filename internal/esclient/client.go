package elasticsearch

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"path"
	"time"

	"github.com/ViaQ/logerr/kverrors"
	elasticsearch6 "github.com/elastic/go-elasticsearch/v6"
	api "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	estypes "github.com/openshift/elasticsearch-operator/internal/types/elasticsearch"
	"k8s.io/apimachinery/pkg/util/sets"
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
	GetThresholdEnabled() (bool, error)
	GetDiskWatermarks() (interface{}, interface{}, error)
	GetMinMasterNodes() (int32, error)
	SetMinMasterNodes(numberMasters int32) (bool, error)
	DoSynchronizedFlush() (bool, error)

	// Cluster State API
	GetLowestClusterVersion() (string, error)
	IsNodeInCluster(nodeName string) (bool, error)

	// Health API
	GetClusterHealth() (api.ClusterHealth, error)
	GetClusterHealthStatus() (string, error)
	GetClusterNodeCount() (int32, error)

	// Index API
	GetIndex(name string) (*estypes.Index, error)
	CreateIndex(name string, index *estypes.Index) error
	ReIndex(src, dst, script, lang string) error
	GetAllIndices(name string) (estypes.CatIndicesResponses, error)

	// Index Alias API
	ListIndicesForAlias(aliasPattern string) ([]string, error)
	UpdateAlias(actions estypes.AliasActions) error
	AddAliasForOldIndices() bool

	// Index Settings API
	GetIndexSettings(name string) (*estypes.IndexSettings, error)
	UpdateIndexSettings(name string, settings *estypes.IndexSettings) error

	// Nodes API
	GetNodeDiskUsage(nodeName string) (string, float64, error)

	// Replicas
	UpdateReplicaCount(replicaCount int32) error
	GetIndexReplicaCounts() (map[string]interface{}, error)

	// Shards API
	ClearTransientShardAllocation() (bool, error)
	GetShardAllocation() (string, error)
	SetShardAllocation(state api.ShardAllocationState) (bool, error)

	// Index Templates API
	CreateIndexTemplate(name string, template *estypes.IndexTemplate) error
	DeleteIndexTemplate(name string) error
	ListTemplates() (sets.String, error)
	GetIndexTemplates() (map[string]estypes.GetIndexTemplate, error)
	UpdateTemplatePrimaryShards(shardCount int32) error
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

func getRootCA() *x509.CertPool {
	certPool := x509.NewCertPool()

	// load cert into []byte
	f := path.Join("./", "admin-ca")
	caPem, err := ioutil.ReadFile(f)
	if err != nil {
		log.Panicf("Unable to read file to get contents %v", err)
		return nil
	}
	log.Printf("ca pem %v", string(caPem))
	certPool.AppendCertsFromPEM(caPem)

	return certPool
}

func getClientCertificates() []tls.Certificate {
	certificate, err := tls.LoadX509KeyPair(
		path.Join("./", "admin-cert"),
		path.Join("./", "admin-key"),
	)
	if err != nil {
		log.Println("erro load key pairs")
		return []tls.Certificate{}
	}
	return []tls.Certificate{
		certificate,
	}
}

func oauthEsClient(esAddr, token, caPath, certPath, keyPath string) (*elasticsearch6.Client, error) {
	es := &elasticsearch6.Client{}
	httpTranport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			RootCAs:            getRootCA(),
			// Certificates:       getClientCertificates(),
		},
	}

	header := http.Header{}
	header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	cfg := elasticsearch6.Config{
		Header:    header,
		Addresses: []string{esAddr},
		Transport: httpTranport,
	}
	es, err := elasticsearch6.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error creating the client: %v", err)
	}
	return es, nil
}

func mTLSEsClient(esAddr, caPath, certPath, keyPath string) (*elasticsearch6.Client, error) {
	es := &elasticsearch6.Client{}
	httpTranport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			RootCAs:            getRootCA(),
			Certificates:       getClientCertificates(),
		},
	}

	cfg := elasticsearch6.Config{
		Addresses: []string{esAddr},
		Transport: httpTranport,
	}
	es, err := elasticsearch6.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error creating the mtls client: %v", err)
	}
	return es, nil
}

func getESClient(esAddr string) (*elasticsearch6.Client, error) {
	es := &elasticsearch6.Client{}

	if esAddr == "" {
		log.Fatalf("es address is empty")
	}
	log.Printf("es address: %s\n", esAddr)

	// Setup es client
	es, err := elasticsearch6.NewClient(elasticsearch6.Config{
		Addresses: []string{esAddr},
	})
	if err != nil {
		log.Fatalf("Error creating the client: %s\n", err)
	}
	return es, nil
}
