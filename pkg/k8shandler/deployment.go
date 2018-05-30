package k8shandler

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/operator-framework/operator-sdk/pkg/sdk/action"
	"github.com/operator-framework/operator-sdk/pkg/sdk/query"
	v1alpha1 "github.com/t0ffel/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	apps "k8s.io/api/apps/v1beta2"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type deploymentNode struct {
	resource apps.Deployment
}

func (node *deploymentNode) isNodeConfigured(dpl *v1alpha1.Elasticsearch) bool {
	label := node.resource.ObjectMeta.Labels["es-node-role"]
	for _, cmpNode := range dpl.Spec.Nodes {
		if cmpNode.NodeRole == label {
			return true
		}
	}
	return false
}

func (node *deploymentNode) getResource() runtime.Object {
	return &node.resource
}

func (node *deploymentNode) isDifferent(cfg *elasticsearchNode) (bool, error) {
	// Check replicas number
	actualReplicas := *node.resource.Spec.Replicas
	if cfg.getReplicas() != actualReplicas {
		logrus.Infof("Different number of replicas detected, updating deployment %v", cfg.DeployName)
		return true, nil
	}

	// Check image of Elasticsearch container
	for _, container := range node.resource.Spec.Template.Spec.Containers {
		if container.Name == "elasticsearch" {
			if container.Image != cfg.ESNodeSpec.Config.Image {
				logrus.Infof("Container image '%v' is different that desired, updating..", container.Image)
				return true, nil
			}
		}
	}

	// Check if the Variables are the desired ones

	return false, nil
}

func (node *deploymentNode) query() error {
	err := query.Get(&node.resource)
	return err
}

// constructNodeDeployment creates the deployment for the node
func (node *deploymentNode) constructNodeResource(cfg *elasticsearchNode, owner metav1.OwnerReference) (runtime.Object, error) {

	// Check if deployment exists

	// FIXME: remove hardcode

	affinity := cfg.getAffinity()
	replicas := cfg.getReplicas()

	deployment := node.resource
	//deployment(cfg.DeployName, node.resource.ObjectMeta.Namespace)
	deployment.ObjectMeta.Labels = cfg.getLabels()
	deployment.Spec = apps.DeploymentSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: cfg.getLabels(),
		},
		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: cfg.getLabels(),
			},
			Spec: v1.PodSpec{
				Affinity: &affinity,
				Containers: []v1.Container{
					cfg.getESContainer(),
				},
				Volumes: cfg.getVolumes(),
				// ImagePullSecrets: TemplateImagePullSecrets(imagePullSecrets),
			},
		},
	}

	// if storageClass != "default" {
	// 	deployment.Spec.VolumeClaimTemplates[0].Annotations = map[string]string{
	// 		"volume.beta.kubernetes.io/storage-class": storageClass,
	// 	}
	// }
	// sset, _ := json.Marshal(deployment)
	// s := string(sset[:])

	// logrus.Infof(s)
	addOwnerRefToObject(&deployment, owner)

	return &deployment, nil
}

func (node *deploymentNode) delete() error {
	err := action.Delete(&node.resource)
	if err != nil {
		return fmt.Errorf("Unable to delete Deployment %v: ", err)
	}
	return nil
}
