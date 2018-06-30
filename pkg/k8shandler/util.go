package k8shandler

import (
	api "github.com/ViaQ/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// addOwnerRefToObject appends the desired OwnerReference to the object
func addOwnerRefToObject(o metav1.Object, r metav1.OwnerReference) {
	if (metav1.OwnerReference{}) != r {
		o.SetOwnerReferences(append(o.GetOwnerReferences(), r))
	}
}

// labelsForES returns the labels for selecting the resources
// belonging to the given vault name.
func labelsForES(component string, role string, clusterName string) map[string]string {
	var isNodeClient, isNodeMaster, isNodeData string

	if role == "clientdata" {
		isNodeClient = "true"
		isNodeData = "true"
		isNodeMaster = "false"
	} else if role == "clientdatamaster" {
		isNodeClient = "true"
		isNodeData = "true"
		isNodeMaster = "true"
	} else if role == "master" {
		isNodeClient = "false"
		isNodeData = "false"
		isNodeMaster = "true"
	} else if role == "client" {
		isNodeClient = "true"
		isNodeData = "false"
		isNodeMaster = "false"
	}
	return map[string]string{
		"component":      component,
		"es-node-role":   role,
		"es-node-client": isNodeClient,
		"es-node-data":   isNodeData,
		"es-node-master": isNodeMaster,
		"cluster":        clusterName,
	}
}

func selectorForES(nodeRole string, clusterName string) map[string]string {

	return map[string]string{
		nodeRole:  "true",
		"cluster": clusterName,
	}
}

func LabelsForESCluster(clusterName string) map[string]string {

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
