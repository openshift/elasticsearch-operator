package kibana

import (
	"context"

	kibana "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/pkg/elasticsearch"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type KibanaRequest struct {
	client   client.Client
	cluster  *kibana.Kibana
	esClient elasticsearch.Client
}

// TODO: determine if this is even necessary
func (clusterRequest *KibanaRequest) isManaged() bool {
	return clusterRequest.cluster.Spec.ManagementState == kibana.ManagementStateManaged
}

func (clusterRequest *KibanaRequest) Create(object runtime.Object) error {
	return clusterRequest.client.Create(context.TODO(), object)
}

//Update the runtime Object or return error
func (clusterRequest *KibanaRequest) Update(object runtime.Object) (err error) {
	return clusterRequest.client.Update(context.TODO(), object)
}

//Update the runtime Object status or return error
func (clusterRequest *KibanaRequest) UpdateStatus(object runtime.Object) (err error) {
	return clusterRequest.client.Status().Update(context.TODO(), object)
}

func (clusterRequest *KibanaRequest) Get(objectName string, object runtime.Object) error {
	namespacedName := types.NamespacedName{Name: objectName, Namespace: clusterRequest.cluster.Namespace}
	return clusterRequest.client.Get(context.TODO(), namespacedName, object)
}

func (clusterRequest *KibanaRequest) GetClusterResource(objectName string, object runtime.Object) error {
	namespacedName := types.NamespacedName{Name: objectName}
	err := clusterRequest.client.Get(context.TODO(), namespacedName, object)
	return err
}

func (clusterRequest *KibanaRequest) List(selector map[string]string, object runtime.Object) error {
	listOpts := []client.ListOption{
		client.InNamespace(clusterRequest.cluster.Namespace),
		client.MatchingLabels(selector),
	}

	return clusterRequest.client.List(
		context.TODO(),
		object,
		listOpts...,
	)
}

func (clusterRequest *KibanaRequest) Delete(object runtime.Object) error {
	return clusterRequest.client.Delete(context.TODO(), object)
}
