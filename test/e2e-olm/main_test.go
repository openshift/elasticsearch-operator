package e2e

import (
	// "testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	TestOperatorNamespaceEnv = "TEST_OPERATOR_NAMESPACE"
)

var (
	operatorNamespace string
	k8sClient         client.Client
	projectRootDir    string
)
