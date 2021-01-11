package k8shandler

import (
	"context"
	"fmt"

	"github.com/ViaQ/logerr/kverrors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateOrUpdateServices ensures the existence of the services for Elasticsearch cluster
func (er *ElasticsearchRequest) CreateOrUpdateServices() error {
	dpl := er.cluster

	annotations := make(map[string]string)
	serviceName := fmt.Sprintf("%s-%s", dpl.Name, "cluster")

	errCtx := kverrors.NewContext("service_name", serviceName,
		"cluster", er.cluster.Name,
		"namespace", er.cluster.Namespace,
	)

	err := er.createOrUpdateService(
		serviceName,
		dpl.Namespace,
		dpl.Name,
		"cluster",
		9300,
		selectorForES("es-node-master", dpl.Name),
		annotations,
		true,
		map[string]string{},
	)
	if err != nil {
		return errCtx.Wrap(err, "failed to create service")
	}

	err = er.createOrUpdateService(
		dpl.Name,
		dpl.Namespace,
		dpl.Name,
		"restapi",
		9200,
		selectorForES("es-node-client", dpl.Name),
		annotations,
		false,
		map[string]string{},
	)
	if err != nil {
		return errCtx.Wrap(err, "failed to create service")
	}

	// legacy metrics service that likely can be rolled into the single service that goes through the proxy
	annotations["service.beta.openshift.io/serving-cert-secret-name"] = fmt.Sprintf("%s-%s", dpl.Name, "metrics")
	err = er.createOrUpdateService(
		fmt.Sprintf("%s-%s", dpl.Name, "metrics"),
		dpl.Namespace,
		dpl.Name,
		"metrics",
		60001,
		selectorForES("es-node-client", dpl.Name),
		annotations,
		false,
		map[string]string{
			"scrape-metrics": "enabled",
		},
	)
	if err != nil {
		return errCtx.Wrap(err, "failed to create service")
	}
	return nil
}

func (er *ElasticsearchRequest) createOrUpdateService(serviceName, namespace, clusterName, targetPortName string, port int32, selector, annotations map[string]string, publishNotReady bool, labels map[string]string) error {
	client := er.client
	cluster := er.cluster

	labels = appendDefaultLabel(clusterName, labels)

	service := newService(
		serviceName,
		namespace,
		clusterName,
		targetPortName,
		port,
		selector,
		annotations,
		labels,
		publishNotReady,
	)

	cluster.AddOwnerRefTo(service)

	err := client.Create(context.TODO(), service)
	if err == nil {
		return nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return kverrors.Wrap(err, "failed to construct service",
			"service_name", service.Name)
	}

	current := new(v1.Service)
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err = client.Get(context.TODO(), types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, current); err != nil {
			if apierrors.IsNotFound(err) {
				// the object doesn't exist -- it was likely culled
				// recreate it on the next time through if necessary
				return nil
			}
			return kverrors.Wrap(err, "failed to get service",
				"service_name", service.Name)
		}

		current.Spec.Ports = service.Spec.Ports
		current.Spec.Selector = service.Spec.Selector
		current.Spec.PublishNotReadyAddresses = service.Spec.PublishNotReadyAddresses
		current.Labels = service.Labels
		current.Annotations = service.Annotations
		if err = client.Update(context.TODO(), current); err != nil {
			return err
		}
		return nil
	})
	if retryErr != nil {
		return kverrors.Wrap(retryErr, "failed to update service",
			"service_name", current.Name)
	}

	return nil
}

func newService(serviceName, namespace, clusterName, targetPortName string, port int32, selector, annotations, labels map[string]string, publishNotReady bool) *v1.Service {
	return &v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: v1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        serviceName,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: v1.ServiceSpec{
			Selector: selector,
			Ports: []v1.ServicePort{
				{
					Port:       port,
					Protocol:   "TCP",
					TargetPort: intstr.FromString(targetPortName),
					Name:       clusterName,
				},
			},
			PublishNotReadyAddresses: publishNotReady,
		},
	}
}
