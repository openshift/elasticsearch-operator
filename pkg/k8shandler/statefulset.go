package k8shandler

import (
	"context"
	"fmt"

	apps "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type statefulSetNode struct {
	resource apps.StatefulSet
}

func (node *statefulSetNode) getResource() runtime.Object {
	return &node.resource
}

func (node *statefulSetNode) isDifferent(cfg *desiredNodeState) (bool, error) {
	// Check replicas number
	if cfg.getReplicas() != *node.resource.Spec.Replicas {
		return true, nil
	}

	// Check if the Variables are the desired ones

	return false, nil
}

func (node *statefulSetNode) query(client client.Client) error {
	err := client.Get(context.TODO(), types.NamespacedName{Name: node.resource.Name, Namespace: node.resource.Namespace}, &apps.StatefulSet{})
	return err
}

// constructNodeStatefulSet creates the StatefulSet for the node
func (node *statefulSetNode) constructNodeResource(client client.Client, cfg *desiredNodeState, owner metav1.OwnerReference) (runtime.Object, error) {

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
		Template: cfg.constructPodTemplateSpec(client),
	}

	pvc, ok, err := cfg.generateMasterPVC()
	if err != nil {
		return &statefulSet, err
	}
	if ok {
		statefulSet.Spec.VolumeClaimTemplates = []v1.PersistentVolumeClaim{
			pvc,
		}
	}

	addOwnerRefToObject(&statefulSet, owner)

	return &statefulSet, nil
}

func (node *statefulSetNode) delete(client client.Client) error {
	err := client.Delete(context.TODO(), &node.resource)
	if err != nil {
		return fmt.Errorf("Unable to delete StatefulSet %v: ", err)
	}
	return nil
}
