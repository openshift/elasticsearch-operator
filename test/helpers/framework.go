package helpers

import (
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type E2ETestFramework struct {
	KubeClient *kubernetes.Clientset
	Client     client.Client
}

func NewE2ETestFramework() *E2ETestFramework {
	clientset, config := newKubeClient()
	dynaClient, err := client.New(config, client.Options{})
	if err != nil {
		panic(err.Error())
	}
	return &E2ETestFramework{
		KubeClient: clientset,
		Client:     dynaClient,
	}
}

//newKubeClient returns a client using the KUBECONFIG env var or incluster settings
func newKubeClient() (*kubernetes.Clientset, *rest.Config) {

	var config *rest.Config
	var err error
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return clientset, config
}
