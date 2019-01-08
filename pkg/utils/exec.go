package utils

import (
	"bytes"
	"fmt"

	// "github.com/operator-framework/operator-sdk/pkg/k8sclient"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

type ExecConfig struct {
	Pod            *v1.Pod
	ContainerName  string
	Command        []string
	KubeConfigPath string
	MasterURL      string
	StdOut         bool
	StdErr         bool
	Tty            bool
}

func PodExec(execConfig *ExecConfig) (*bytes.Buffer, *bytes.Buffer, error) {
	var (
		execOut bytes.Buffer
		execErr bytes.Buffer
	)

	esPod := execConfig.Pod
	if esPod.Status.Phase != v1.PodRunning {
		return nil, nil, fmt.Errorf("elasticsearch pod [%s] found but isn't running", esPod.Name)
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("error when creating rest client: %v", err)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("error when creating rest client: %v", err)
	}
	// client := k8sclient.GetKubeClient()

	execRequest := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(esPod.Name).
		Namespace(esPod.Namespace).
		SubResource("exec")

	execRequest.VersionedParams(&v1.PodExecOptions{
		Container: execConfig.ContainerName,
		Command:   execConfig.Command,
		Stdout:    execConfig.StdOut,
		Stderr:    execConfig.StdErr,
	}, scheme.ParameterCodec)

	restClientConfig, err := clientcmd.BuildConfigFromFlags(execConfig.MasterURL, execConfig.KubeConfigPath)
	if err != nil {
		return nil, nil, fmt.Errorf("error when creating rest client command: %v", err)
	}
	exec, err := remotecommand.NewSPDYExecutor(restClientConfig, "POST", execRequest.URL())
	if err != nil {
		return nil, nil, fmt.Errorf("error when creating remote command executor: %v", err)
	}
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &execOut,
		Stderr: &execErr,
		Tty:    execConfig.Tty,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("remote execution failed: %v", err)
	}

	return &execOut, &execErr, nil
}
