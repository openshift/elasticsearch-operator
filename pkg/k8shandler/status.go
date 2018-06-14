package k8shandler

import (
	"fmt"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	v1alpha1 "github.com/t0ffel/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	"k8s.io/api/core/v1"
)

// UpdateStatus updates the status of Elasticsearch CRD
func UpdateStatus(dpl *v1alpha1.Elasticsearch) error {
	// TODO: add Elasticsearch cluster health
	// TODO: add Elasticsearch nodes list/roles
	// TODO: add configmap hash
	// TODO: add status of the cluster: i.e. is cluster restart in progress?
	// TODO: add secrets hash

	podList, err := listPods(dpl)
	//podList := podList()
	//labelSelector := labels.SelectorFromSet(LabelsForESCluster(dpl.Name)).String()
	//listOps := &metav1.ListOptions{LabelSelector: labelSelector}
	//err := query.List(dpl.Namespace, podList, query.WithListOptions(listOps))
	if err != nil {
		return fmt.Errorf("failed to list pods: %v", err)
	}
	//podNames := getPodNames(podList.Items)
	dpl.Status.Nodes = []v1alpha1.ElasticsearchNodeStatus{}
	for _, pod := range podList.Items {
		//	logrus.Infof("Examining pod %v", pod)
		updatePodStatus(pod, &dpl.Status)
	}
	err = sdk.Update(dpl)
	if err != nil {
		return fmt.Errorf("failed to update Elasticsearch status: %v", err)
	}

	return nil
}

func updatePodStatus(pod v1.Pod, dpl *v1alpha1.ElasticsearchStatus) error {
	for _, podStatus := range dpl.Nodes {
		if podStatus.PodName == pod.Name {
			podStatus.Status = string(pod.Status.Phase)
			return nil
		}
	}
	nodeStatus := v1alpha1.ElasticsearchNodeStatus{
		PodName: pod.Name,
		Status:  string(pod.Status.Phase),
	}
	dpl.Nodes = append(dpl.Nodes, nodeStatus)
	return nil
}
