package kibana

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	defaultKibanaMemory     = resource.MustParse("736Mi")
	defaultKibanaCPURequest = resource.MustParse("100m")

	defaultKibanaProxyMemory     = resource.MustParse("256Mi")
	defaultKibanaProxyCPURequest = resource.MustParse("100m")
	kibanaDefaultImage           = "quay.io/openshift/origin-logging-kibana6:latest"
	kibanaProxyDefaultImage      = "quay.io/openshift/origin-oauth-proxy:latest"
)
