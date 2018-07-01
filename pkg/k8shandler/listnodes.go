package k8shandler

import (
	"fmt"
	//"github.com/sirupsen/logrus"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	apps "k8s.io/api/apps/v1"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func listDeployments(clusterName, namespace string) (*apps.DeploymentList, error) {
	list := deploymentList()
	labelSelector := labels.SelectorFromSet(LabelsForESCluster(clusterName)).String()
	listOps := &metav1.ListOptions{LabelSelector: labelSelector}
	err := sdk.List(namespace, list, sdk.WithListOptions(listOps))
	if err != nil {
		return list, fmt.Errorf("Unable to list deployments: %v", err)
	}

	return list, nil
}

func deploymentList() *apps.DeploymentList {
	return &apps.DeploymentList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
	}
}

func popDeployment(deployments *apps.DeploymentList, cfg desiredNodeState) (*apps.DeploymentList, apps.Deployment, bool) {
	var deployment apps.Deployment
	var index = -1
	for i, dpl := range deployments.Items {
		if dpl.Name == cfg.DeployName {
			deployment = dpl
			index = i
			break
		}
	}
	if index == -1 {
		return deployments, deployment, false
	}
	dpls := deploymentList()
	deployments.Items[index] = deployments.Items[len(deployments.Items)-1]
	dpls.Items = deployments.Items[:len(deployments.Items)-1]
	return dpls, deployment, true
}

// podList returns a v1.PodList object
func podList() *v1.PodList {
	return &v1.PodList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
	}
}

func listPods(clusterName, namespace string) (*v1.PodList, error) {
	podList := podList()
	labelSelector := labels.SelectorFromSet(LabelsForESCluster(clusterName)).String()
	listOps := &metav1.ListOptions{LabelSelector: labelSelector}
	err := sdk.List(namespace, podList, sdk.WithListOptions(listOps))
	if err != nil {
		return podList, fmt.Errorf("failed to list pods: %v", err)
	}
	return podList, nil
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []v1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}
