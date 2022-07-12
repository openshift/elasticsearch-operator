module github.com/openshift/elasticsearch-operator

go 1.16

// Pinned to kubernetes-1.21.3 and openshift release-4.8
require (
	github.com/ViaQ/logerr v1.0.10
	github.com/coreos/prometheus-operator v0.38.1-0.20200424145508-7e176fda06cc
	github.com/go-logr/logr v0.4.0
	github.com/google/go-cmp v0.5.5
	github.com/inhies/go-bytesize v0.0.0-20151001220322-5990f52c6ad6
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/openshift/api v0.0.0-20210713130143-be21c6cb1bea
	github.com/prometheus/client_golang v1.12.1
	golang.org/x/text v0.3.7 // indirect
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/utils v0.0.0-20211116205334-6203023598ed
	sigs.k8s.io/controller-runtime v0.9.2
)

replace k8s.io/client-go => k8s.io/client-go v0.21.2 // Required by prometheus-operator
