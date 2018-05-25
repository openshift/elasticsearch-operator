package k8shandler

import (
	"fmt"

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
		return true, nil
	}

	// Check if the Variables are the desired ones

	return false, nil
}

func (node *deploymentNode) query() error {
	err := query.Get(&node.resource)
	return err
}

// addOwnerRefToObject appends the desired OwnerReference to the object
func (node *deploymentNode) addOwnerRefToObject(r metav1.OwnerReference) {
	addOwnerRefToObject(&node.resource, r)
}

// constructNodeDeployment creates the deployment for the node
func (node *deploymentNode) constructNodeResource(cfg *elasticsearchNode) (runtime.Object, error) {

	secretName := fmt.Sprintf("%s-certs", cfg.ClusterName)

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
					cfg.getContainer(),
				},
				Volumes: []v1.Volume{
					v1.Volume{
						Name: "certificates",
						VolumeSource: v1.VolumeSource{
							Secret: &v1.SecretVolumeSource{
								SecretName: secretName,
							},
						},
					},
					v1.Volume{
						Name: "es-data",
						VolumeSource: v1.VolumeSource{
							PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
								ClaimName: "es-data-elastic1-clientdatamaster-0",
								ReadOnly:  false,
							},
						},
					},
				},
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

	return &deployment, nil
}

func (node *deploymentNode) delete() error {
	err := action.Delete(&node.resource)
	if err != nil {
		return fmt.Errorf("Unable to delete Deployment %v: ", err)
	}
	return nil
}
