package k8shandler

import (
	"fmt"
	//"github.com/sirupsen/logrus"

	"github.com/operator-framework/operator-sdk/pkg/sdk/query"
	apps "k8s.io/api/apps/v1beta2"

	v1alpha1 "github.com/t0ffel/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func listStatefulSets(dpl *v1alpha1.Elasticsearch) ([]NodeTypeInterface, error) {
	list := ssList()
	labelSelector := labels.SelectorFromSet(LabelsForESCluster(dpl.Name)).String()
	listOps := &metav1.ListOptions{LabelSelector: labelSelector}
	err := query.List(dpl.Namespace, list, query.WithListOptions(listOps))
	if err != nil {
		return []NodeTypeInterface{}, fmt.Errorf("Unable to list StatefulSets: %v", err)
	}
	nodeList := []NodeTypeInterface{}
	for _, res := range list.Items {
		nodeList = append(nodeList, &statefulSetNode{resource: res})
	}

	return nodeList, nil
}

func ssList() *apps.StatefulSetList {
	return &apps.StatefulSetList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
	}
}

func listDeployments(dpl *v1alpha1.Elasticsearch) (*apps.DeploymentList, error) {
	list := deploymentList()
	labelSelector := labels.SelectorFromSet(LabelsForESCluster(dpl.Name)).String()
	listOps := &metav1.ListOptions{LabelSelector: labelSelector}
	err := query.List(dpl.Namespace, list, query.WithListOptions(listOps))
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

func listNodes(dpl *v1alpha1.Elasticsearch) ([]NodeTypeInterface, error) {
	statefulSets, err := listStatefulSets(dpl)
	if err != nil {
		return []NodeTypeInterface{}, fmt.Errorf("Unable to list Elasticsearch's StatefulSets: %v", err)
	}
	return statefulSets, nil
}

func (cState *clusterState) amendDeployments(dpl *v1alpha1.Elasticsearch) error {
	deployments, err := listDeployments(dpl)
	var element apps.Deployment
	var ok bool
	if err != nil {
		return fmt.Errorf("Unable to list Elasticsearch's Deployments: %v", err)
	}
	for _, node := range cState.Nodes {
		deployments, element, ok = popDeployment(deployments, node.Config)
		if ok {
			node.setDeployment(element)
		}
	}
	if len(deployments.Items) != 0 {
		cState.DanglingDeployments = deployments
	}
	return nil
}

func popDeployment(deployments *apps.DeploymentList, cfg elasticsearchNode) (*apps.DeploymentList, apps.Deployment, bool) {
	var deployment apps.Deployment
	var index int = -1
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

func listPods(dpl *v1alpha1.Elasticsearch) (*v1.PodList, error) {
	podList := podList()
	labelSelector := labels.SelectorFromSet(LabelsForESCluster(dpl.Name)).String()
	listOps := &metav1.ListOptions{LabelSelector: labelSelector}
	err := query.List(dpl.Namespace, podList, query.WithListOptions(listOps))
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
