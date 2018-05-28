package k8shandler

import (
	"fmt"

	"github.com/operator-framework/operator-sdk/pkg/sdk/action"
	"github.com/operator-framework/operator-sdk/pkg/sdk/query"
	v1alpha1 "github.com/t0ffel/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	apps "k8s.io/api/apps/v1beta2"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type statefulSetNode struct {
	resource apps.StatefulSet
}

func (node *statefulSetNode) isNodeConfigured(dpl *v1alpha1.Elasticsearch) bool {
	label := node.resource.ObjectMeta.Labels["es-node-role"]
	for _, cmpNode := range dpl.Spec.Nodes {
		if cmpNode.NodeRole == label {
			return true
		}
	}
	return false
}

func (node *statefulSetNode) getResource() runtime.Object {
	return &node.resource
}

func (node *statefulSetNode) isDifferent(cfg *elasticsearchNode) (bool, error) {
	// Check replicas number
	if cfg.getReplicas() != *node.resource.Spec.Replicas {
		return true, nil
	}

	// Check if the Variables are the desired ones

	return false, nil
}

func (node *statefulSetNode) query() error {
	err := query.Get(&node.resource)
	return err
}

// constructNodeStatefulSet creates the StatefulSet for the node
func (node *statefulSetNode) constructNodeResource(cfg *elasticsearchNode, owner metav1.OwnerReference) (runtime.Object, error) {

	secretName := fmt.Sprintf("%s-certs", cfg.ClusterName)

	// Check if StatefulSet exists

	// FIXME: remove hardcode
	volumeSize, _ := resource.ParseQuantity("1Gi")
	storageClass := "default"

	affinity := cfg.getAffinity()
	replicas := cfg.getReplicas()

	statefulSet := node.resource
	//statefulSet(cfg.DeployName, node.resource.ObjectMeta.Namespace)
	statefulSet.ObjectMeta.Labels = cfg.getLabels()
	statefulSet.Spec = apps.StatefulSetSpec{
		Replicas:    &replicas,
		ServiceName: cfg.DeployName,
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
				Volumes: []v1.Volume{
					v1.Volume{
						Name: "certificates",
						VolumeSource: v1.VolumeSource{
							Secret: &v1.SecretVolumeSource{
								SecretName: secretName,
							},
						},
					},
				},
				// ImagePullSecrets: TemplateImagePullSecrets(imagePullSecrets),
			},
		},
		VolumeClaimTemplates: []v1.PersistentVolumeClaim{
			v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "es-data",
					Labels: cfg.getLabels(),
				},
				Spec: v1.PersistentVolumeClaimSpec{
					AccessModes: []v1.PersistentVolumeAccessMode{
						v1.ReadWriteOnce,
					},
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceStorage: volumeSize,
						},
					},
				},
			},
		},
	}

	if storageClass != "default" {
		statefulSet.Spec.VolumeClaimTemplates[0].Annotations = map[string]string{
			"volume.beta.kubernetes.io/storage-class": storageClass,
		}
	}
	// sset, _ := json.Marshal(statefulSet)
	// s := string(sset[:])

	// logrus.Infof(s)
	addOwnerRefToObject(&statefulSet, owner)

	return &statefulSet, nil
}

func (node *statefulSetNode) delete() error {
	err := action.Delete(&node.resource)
	if err != nil {
		return fmt.Errorf("Unable to delete StatefulSet %v: ", err)
	}
	return nil
}
