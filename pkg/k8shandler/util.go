package k8shandler

import (
	"fmt"

	api "github.com/ViaQ/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/sdk"

	apps "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	defaultMasterCPULimit   = "100m"
	defaultMasterCPURequest = "100m"
	defaultCPULimit         = "4000m"
	defaultCPURequest       = "100m"
	defaultMemoryLimit      = "4Gi"
	defaultMemoryRequest    = "1Gi"
)

// addOwnerRefToObject appends the desired OwnerReference to the object
func addOwnerRefToObject(o metav1.Object, r metav1.OwnerReference) {
	if (metav1.OwnerReference{}) != r {
		o.SetOwnerReferences(append(o.GetOwnerReferences(), r))
	}
}

func selectorForES(nodeRole string, clusterName string) map[string]string {

	return map[string]string{
		nodeRole:  "true",
		"cluster": clusterName,
	}
}

func labelsForESCluster(clusterName string) map[string]string {

	return map[string]string{
		"cluster": clusterName,
	}
}

// asOwner returns an owner reference set as the vault cluster CR
func asOwner(v *api.Elasticsearch) metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: api.SchemeGroupVersion.String(),
		Kind:       v.Kind,
		Name:       v.Name,
		UID:        v.UID,
		Controller: &trueVar,
	}
}

func getReadinessProbe() v1.Probe {
	return v1.Probe{
		TimeoutSeconds:      30,
		InitialDelaySeconds: 10,
		FailureThreshold:    15,
		Handler: v1.Handler{
			TCPSocket: &v1.TCPSocketAction{
				Port: intstr.FromInt(9300),
			},
		},
	}
}

func getResourceRequirements(commonResRequirements, nodeResRequirements v1.ResourceRequirements) v1.ResourceRequirements {
	limitCPU := nodeResRequirements.Limits.Cpu()
	if limitCPU.IsZero() {
		if commonResRequirements.Limits.Cpu().IsZero() {
			CPU, _ := resource.ParseQuantity(defaultCPULimit)
			limitCPU = &CPU
		} else {
			limitCPU = commonResRequirements.Limits.Cpu()
		}
	}
	limitMem := nodeResRequirements.Limits.Memory()
	if limitMem.IsZero() {
		if commonResRequirements.Limits.Memory().IsZero() {
			Mem, _ := resource.ParseQuantity(defaultMemoryLimit)
			limitMem = &Mem
		} else {
			limitMem = commonResRequirements.Limits.Memory()
		}

	}
	requestCPU := nodeResRequirements.Requests.Cpu()
	if requestCPU.IsZero() {
		if commonResRequirements.Requests.Cpu().IsZero() {
			CPU, _ := resource.ParseQuantity(defaultCPURequest)
			requestCPU = &CPU
		} else {
			requestCPU = commonResRequirements.Requests.Cpu()
		}
	}
	requestMem := nodeResRequirements.Requests.Memory()
	if requestMem.IsZero() {
		if commonResRequirements.Requests.Memory().IsZero() {
			Mem, _ := resource.ParseQuantity(defaultMemoryRequest)
			requestMem = &Mem
		} else {
			requestMem = commonResRequirements.Requests.Memory()
		}
	}

	return v1.ResourceRequirements{
		Limits: v1.ResourceList{
			"cpu":    *limitCPU,
			"memory": *limitMem,
		},
		Requests: v1.ResourceList{
			"cpu":    *requestCPU,
			"memory": *requestMem,
		},
	}

}

func listDeployments(clusterName, namespace string) (*apps.DeploymentList, error) {
	list := deploymentList()
	labelSelector := labels.SelectorFromSet(labelsForESCluster(clusterName)).String()
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
	labelSelector := labels.SelectorFromSet(labelsForESCluster(clusterName)).String()
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
