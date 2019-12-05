package constants

const (
	SingletonName              = "instance"
	OpenshiftNS                = "openshift-logging"
	ProxyName                  = "cluster"
	TrustedCABundleKey         = "ca-bundle.crt"
	TrustedCABundleMountDir    = "/etc/pki/ca-trust/extracted/pem/"
	TrustedCABundleMountFile   = "tls-ca-bundle.pem"
	InjectTrustedCABundleLabel = "config.openshift.io/inject-trusted-cabundle"
	TrustedCABundleHashName    = "logging.openshift.io/hash"
	FluentdTrustedCAName       = "fluentd-trusted-ca-bundle"
	KibanaTrustedCAName        = "kibana-trusted-ca-bundle"
	ElasticsearchFQDN          = "elasticsearch.openshift-logging.svc.cluster.local"
	ElasticsearchPort          = "9200"
)
