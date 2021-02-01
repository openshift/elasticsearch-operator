package elasticsearch

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"
	elasticsearch6 "github.com/elastic/go-elasticsearch/v6"
	api "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	estypes "github.com/openshift/elasticsearch-operator/internal/types/elasticsearch"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
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

	// set esclient for used only for testing
	setESClient(elasticsearch6.Client)
}

type esClient struct {
	cluster   string
	namespace string
	client    elasticsearch6.Client
	k8sclient k8sclient.Client
}

// NewClient Getting new client
func NewClient(cluster, namespace string, client k8sclient.Client) Client {

	transport := getESTransport(cluster, namespace)
	esclient6, err := getESClient(cluster, namespace, transport)

	if err != nil {
		return nil
	}

	return &esClient{
		cluster:   cluster,
		namespace: namespace,
		client:    *esclient6,
		k8sclient: client,
	}
}

func (ec *esClient) setESClient(client elasticsearch6.Client) {
	ec.client = client
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

func getRootCA(clusterName, namespace string) *x509.CertPool {
	certPool := x509.NewCertPool()

	// load cert into []byte
	f := path.Join(certLocalPath, clusterName, "admin-ca")
	caPem, err := ioutil.ReadFile(f)
	if err != nil {
		log.Error(err, "Unable to read file to get contents", "file", f)
		return nil
	}

	certPool.AppendCertsFromPEM(caPem)

	return certPool
}

func getClientCertificates(clusterName, namespace string) []tls.Certificate {
	certificate, err := tls.LoadX509KeyPair(
		path.Join(certLocalPath, clusterName, "admin-cert"),
		path.Join(certLocalPath, clusterName, "admin-key"),
	)
	if err != nil {
		return []tls.Certificate{}
	}

	return []tls.Certificate{
		certificate,
	}
}

func extractSecret(secretName, namespace string, client client.Client) {
	secret := &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: v1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
	}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, secret); err != nil {
		log.Error(err, "Error reading secret", "secret", secretName)
	}

	// make sure that the dir === secretName exists
	secretDir := path.Join(certLocalPath, secretName)
	if _, err := os.Stat(secretDir); os.IsNotExist(err) {
		if err = os.MkdirAll(secretDir, 0o755); err != nil {
			log.Error(err, "Error creating dir", "dir", secretDir)
		}
	}

	for _, key := range []string{"admin-ca", "admin-cert", "admin-key"} {

		value, ok := secret.Data[key]

		// check to see if the map value exists
		if !ok {
			log.Error(nil, "secret key not found", "key", key)
		}

		secretFile := path.Join(certLocalPath, secretName, key)
		if err := ioutil.WriteFile(secretFile, value, 0o644); err != nil {
			log.Error(err, "failed to write value to file", "value", value, "file", secretFile)
		}
	}
}

func getESClient(cluster, namespace string, httpTransport *http.Transport) (*elasticsearch6.Client, error) {
	url := fmt.Sprintf("https://%s.%s.svc:9200", cluster, namespace)

	header := http.Header{}
	header = ensureTokenHeader(header)

	cfg := elasticsearch6.Config{
		Header:    header,
		Addresses: []string{url},
		Transport: httpTransport,
	}
	es, err := elasticsearch6.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error creating the client: %v", err)
	}
	return es, nil
}

func ensureTokenHeader(header http.Header) http.Header {
	if header == nil {
		header = map[string][]string{}
	}

	if saToken, ok := readSAToken(k8sTokenFile); ok {
		header.Set("Authorization", fmt.Sprintf("Bearer %s", saToken))
	}

	return header
}

// we want to read each time so that we can be sure to have the most up to date
// token in the case where our perms change and a new token is mounted
func readSAToken(tokenFile string) (string, bool) {
	// read from /var/run/secrets/kubernetes.io/serviceaccount/token
	token, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		log.Error(err, "Unable to read auth token from file", "file", tokenFile)
		return "", false
	}

	if len(token) == 0 {
		log.Error(nil, "Unable to read auth token from file", "file", tokenFile)
		return "", false
	}

	return string(token), true
}

func getESTransport(clusterName, namespace string) *http.Transport {
	httpTransport := &http.Transport{
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
			RootCAs:            getRootCA(clusterName, namespace),
		},
	}

	return httpTransport
}
