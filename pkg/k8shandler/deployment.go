package k8shandler

import (
	"context"
	"fmt"

	"github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1alpha1"
	"github.com/sirupsen/logrus"

	apps "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type deploymentNode struct {
	resource apps.Deployment
}

func (node *deploymentNode) getResource() runtime.Object {
	return &node.resource
}

func (node *deploymentNode) isDifferent(cfg *desiredNodeState) (bool, error) {

	if node.resource.Spec.Paused == false {
		logrus.Debugf("Deployment %v is not currently paused.", node.resource.Name)
		return true, nil
	}

	// Check replicas number
	actualReplicas := *node.resource.Spec.Replicas
	if cfg.getReplicas() != actualReplicas {
		logrus.Debugf("Different number of replicas detected, updating deployment %v", cfg.DeployName)
		return true, nil
	}

	// Check image of Elasticsearch container
	for _, container := range node.resource.Spec.Template.Spec.Containers {
		if container.Name == "elasticsearch" {
			if container.Image != cfg.ESNodeSpec.Spec.Image {
				logrus.Debugf("Resource '%s' has different container image than desired", node.resource.Name)
				return true, nil
			}
		}
	}

	// Check if labels are correct
	for label, value := range cfg.Labels {
		val, ok := node.resource.Labels[label]
		if !ok || val != value {
			logrus.Debugf("Labels on deployment '%v' need update..", node.resource.GetName())
			return true, nil
		}
	}

	// Check if the Variables are the desired ones
	envVars := node.resource.Spec.Template.Spec.Containers[0].Env
	desiredVars := cfg.EnvVars
	for index, value := range envVars {
		if value.ValueFrom == nil {
			if desiredVars[index] != value {
				logrus.Debugf("Env vars are different for %q", node.resource.GetName())
				return true, nil
			}
		} else {
			if desiredVars[index].ValueFrom.FieldRef.FieldPath != value.ValueFrom.FieldRef.FieldPath {
				logrus.Debugf("Env vars are different for %q", node.resource.GetName())
				return true, nil
			}
		}
	}

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
				logrus.Warn("Detected change in storage")
				return true, nil
			}
		}
	}
	return false, nil
}

func (node *deploymentNode) query(client client.Client) error {
	err := client.Get(context.TODO(), types.NamespacedName{Name: node.resource.Name, Namespace: node.resource.Namespace}, &apps.Deployment{})
	return err
}

// constructNodeDeployment creates the deployment for the node
func (node *deploymentNode) constructNodeResource(client client.Client, cfg *desiredNodeState, owner metav1.OwnerReference) (runtime.Object, error) {

	// Check if deployment exists

	// FIXME: remove hardcode

	replicas := cfg.getReplicas()

	// deployment := node.resource
	deployment := apps.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.DeployName,
			Namespace: cfg.Namespace,
		},
	}

	progressDeadlineSeconds := int32(1800)
	deployment.ObjectMeta.Labels = cfg.getLabels()
	deployment.Spec = apps.DeploymentSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: cfg.getLabelSelector(),
		},
		Strategy: apps.DeploymentStrategy{
			Type: "Recreate",
		},
		ProgressDeadlineSeconds: &progressDeadlineSeconds,
		Template:                cfg.constructPodTemplateSpec(client),
		Paused:                  cfg.Paused,
	}

	// if storageClass != "default" {
	// 	deployment.Spec.VolumeClaimTemplates[0].Annotations = map[string]string{
	// 		"volume.beta.kubernetes.io/storage-class": storageClass,
	// 	}
	// }
	// sset, _ := json.Marshal(deployment)
	// s := string(sset[:])

	addOwnerRefToObject(&deployment, owner)

	return &deployment, nil
}

func (node *deploymentNode) delete(client client.Client) error {
	err := client.Delete(context.TODO(), &node.resource)
	if err != nil {
		return fmt.Errorf("Unable to delete Deployment %v: ", err)
	}
	return nil
}
