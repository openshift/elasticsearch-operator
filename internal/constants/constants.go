package constants

import "github.com/openshift/elasticsearch-operator/internal/utils"

const (
	ProxyName                   = "cluster"
	TrustedCABundleKey          = "ca-bundle.crt"
	TrustedCABundleMountDir     = "/etc/pki/ca-trust/extracted/pem/"
	TrustedCABundleMountFile    = "tls-ca-bundle.pem"
	InjectTrustedCABundleLabel  = "config.openshift.io/inject-trusted-cabundle"
	TrustedCABundleHashName     = "logging.openshift.io/hash"
	KibanaTrustedCAName         = "kibana-trusted-ca-bundle"
	SecretHashPrefix            = "logging.openshift.io/"
	ElasticsearchDefaultImage   = "quay.io/openshift-logging/elasticsearch6:6.8.1"
	ProxyDefaultImage           = "quay.io/openshift-logging/elasticsearch-proxy:1.0"
	TheoreticalShardMaxSizeInMB = 40960

	// OcpTemplatePrefix is the prefix all operator generated templates
	OcpTemplatePrefix = "ocp-gen"
)

var (
	ReconcileForGlobalProxyList = []string{KibanaTrustedCAName}
	packagedElasticsearchImage  = utils.LookupEnvWithDefault("ELASTICSEARCH_IMAGE", ElasticsearchDefaultImage)
	ExpectedSecretKeys          = []string{
		"admin-ca",
		"admin-cert",
		"admin-key",
		"elasticsearch.crt",
		"elasticsearch.key",
		"logging-es.crt",
		"logging-es.key",
	}
)

func PackagedElasticsearchImage() string {
	return packagedElasticsearchImage
}
