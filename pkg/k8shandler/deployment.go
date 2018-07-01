package k8shandler

import (
	"fmt"

	v1alpha1 "github.com/ViaQ/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	"github.com/sirupsen/logrus"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	apps "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type deploymentNode struct {
	resource apps.Deployment
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
			if container.Image != cfg.ESNodeSpec.Spec.Image {
				logrus.Infof("Container image '%v' is different that desired, updating..", container.Image)
				return true, nil
			}
		}
	}

	// TODO: Check if the Variables are the desired ones

	// Check that storage configuration is the same
	// Maybe this needs to be split into a separate method since this
	// may indicate that we need a new cluster spin up, not rolling restart
	for _, volume := range node.resource.Spec.Template.Spec.Volumes {
		if volume.Name == "elasticsearch-storage" {
			switch {
			case volume.PersistentVolumeClaim != nil && cfg.ESNodeSpec.Storage.PersistentVolumeClaim != nil:
				if volume.PersistentVolumeClaim.ClaimName == cfg.ESNodeSpec.Storage.PersistentVolumeClaim.ClaimName {
					return false, nil
				}
			case volume.PersistentVolumeClaim != nil && cfg.ESNodeSpec.Storage.VolumeClaimTemplate != nil:
				// FIXME: don't forget to fix this
				desiredClaimName := fmt.Sprintf("%s-%s", cfg.ESNodeSpec.Storage.VolumeClaimTemplate.Name, node.resource.Name)
				if volume.PersistentVolumeClaim.ClaimName == desiredClaimName {
					return false, nil
				}
			case volume.HostPath != nil && cfg.ESNodeSpec.Storage.HostPath != nil:
				return false, nil
			case volume.EmptyDir != nil && (cfg.ESNodeSpec.Storage.EmptyDir != nil || cfg.ESNodeSpec.Storage == v1alpha1.ElasticsearchNodeStorageSource{}):
				return false, nil
			default:
				logrus.Infof("Detected change in storage")
				return true, nil
			}
		}
	}
	return false, nil
}

func (node *deploymentNode) query() error {
	err := sdk.Get(&node.resource)
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
				ServiceAccountName: cfg.ServiceAccountName,
			},
		},
	}

	nodeSelector, ok := cfg.getSelector()
	if ok {
		deployment.Spec.Template.Spec.NodeSelector = nodeSelector
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
	err := sdk.Delete(&node.resource)
	if err != nil {
		return fmt.Errorf("Unable to delete Deployment %v: ", err)
	}
	return nil
}
