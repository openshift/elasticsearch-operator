package elasticsearch

import (
	"context"
	"fmt"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/openshift/elasticsearch-operator/internal/manifests/service"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

	svc := service.New(serviceName, namespace, labels).
		WithAnnotations(annotations).
		WithSelector(selector).
		WithServicePorts(v1.ServicePort{
			Port:       port,
			Protocol:   v1.ProtocolTCP,
			TargetPort: intstr.FromString(targetPortName),
			Name:       clusterName,
		}).
		WithPublishNotReady(publishNotReady).
		Build()

	cluster.AddOwnerRefTo(svc)

	err := service.CreateOrUpdate(context.TODO(), client, svc, service.Equal, service.Mutate)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update elasticsearch service",
			"cluster", cluster.Name,
			"namespace", cluster.Namespace,
		)
	}

	return nil
}
