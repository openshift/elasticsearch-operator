module github.com/openshift/elasticsearch-operator

go 1.14

// Pinned to kubernetes-1.18.3 and openshift release-4.6
require (
	github.com/ViaQ/logerr v1.0.8
	github.com/coreos/prometheus-operator v0.38.1-0.20200424145508-7e176fda06cc
	github.com/elastic/go-elasticsearch/v6 v6.8.10 // indirect
	github.com/go-logr/logr v0.2.1
	github.com/go-logr/zapr v0.2.0 // indirect
	github.com/google/go-cmp v0.5.0
	github.com/inhies/go-bytesize v0.0.0-20151001220322-5990f52c6ad6
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/openshift/api v0.0.0-20200602204738-768b7001fe69
	github.com/operator-framework/operator-sdk v0.19.4
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/sirupsen/logrus v1.6.0 // indirect
	github.com/tidwall/gjson v1.6.3 // indirect
	go.uber.org/zap v1.16.0 // indirect
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.18.8
	k8s.io/apimachinery v0.18.8
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.6.3
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/client-go => k8s.io/client-go v0.18.8 // Required by prometheus-operator
)
