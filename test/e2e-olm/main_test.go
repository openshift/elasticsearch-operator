package e2e

import (
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	TestOperatorNamespaceEnv = "TEST_OPERATOR_NAMESPACE"
)

var (
	operatorNamespace string
	k8sClient         client.Client
	k8sConfig         *rest.Config
	projectRootDir    string
)
