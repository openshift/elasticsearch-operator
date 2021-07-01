package kibana

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"
	"github.com/openshift/elasticsearch-operator/internal/manifests/console"
	"github.com/openshift/elasticsearch-operator/internal/manifests/route"
	"github.com/openshift/elasticsearch-operator/internal/utils"

	routev1 "github.com/openshift/api/route/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const KibanaConsoleLinkName = "kibana-public-url"

// GetRouteURL retrieves the route URL from a given route and namespace
func (clusterRequest *KibanaRequest) GetRouteURL(routeName string) (string, error) {
	key := client.ObjectKey{Name: routeName, Namespace: clusterRequest.cluster.Namespace}
	r, err := route.Get(context.TODO(), clusterRequest.client, key)
	if err != nil {
		if !apierrors.IsNotFound(kverrors.Root(err)) {
			log.Error(err, "Failed to check for kibana object")
		}
		return "", err
	}

	return fmt.Sprintf("%s%s", "https://", r.Spec.Host), nil
}

func (clusterRequest *KibanaRequest) createOrUpdateKibanaRoute() error {
	cluster := clusterRequest.cluster

	fp := utils.GetWorkingDirFilePath("ca.crt")
	caCert, err := ioutil.ReadFile(fp)
	if err != nil {
		log.Info("could not read CA certificate for kibana route",
			"filePath", fp,
			"cause", err)
	}

	labels := map[string]string{
		"component":     "support",
		"logging-infra": "support",
		"provider":      "openshift",
	}

	rt := route.New("kibana", cluster.Namespace, "kibana", labels).
		WithTLSConfig(&routev1.TLSConfig{
			Termination:                   routev1.TLSTerminationReencrypt,
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
		}).
		WithCA(caCert).
		Build()

	utils.AddOwnerRefToObject(rt, getOwnerRef(cluster))

	err = route.CreateOrUpdate(context.TODO(), clusterRequest.client, rt, route.RouteTLSConfigEqual, route.MutateTLSConfigOnly)
	if err != nil {
		return kverrors.Wrap(err, "failed to update Kibana route for cluster",
			"cluster", cluster.Name,
			"namespace", cluster.Namespace,
		)
	}

	return nil
}

func (clusterRequest *KibanaRequest) createOrUpdateKibanaConsoleLink() error {
	cluster := clusterRequest.cluster

	kibanaURL, err := clusterRequest.GetRouteURL("kibana")
	if err != nil {
		return kverrors.Wrap(err, "failed to get route URL for kibana")
	}

	cl := console.NewConsoleLink(KibanaConsoleLinkName, kibanaURL, "Logging", "Observability")

	err = console.CreateOrUpdateConsoleLink(context.TODO(), clusterRequest.client, cl, console.ConsoleLinksEqual, console.MutateConsoleLinkSpecOnly)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update kibana console link CR for cluster",
			"cluster", cluster.Name,
		)
	}

	return nil
}

func (clusterRequest *KibanaRequest) createOrUpdateKibanaConsoleExternalLogLink() (err error) {
	cluster := clusterRequest.cluster

	kibanaURL, err := clusterRequest.GetRouteURL("kibana")
	if err != nil {
		return kverrors.Wrap(err, "failed to get route URL", "cluster", clusterRequest.cluster.Name)
	}

	labels := map[string]string{
		"component":     "support",
		"logging-infra": "support",
		"provider":      "openshift",
	}

	consoleExternalLogLink := console.NewConsoleExternalLogLink(
		"kibana",
		"Show in Kibana",
		strings.Join([]string{
			kibanaURL,
			"/app/kibana#/discover?_g=(time:(from:now-1w,mode:relative,to:now))&_a=(columns:!(kubernetes.container_name,message),query:(query_string:(analyze_wildcard:!t,query:'",
			strings.Join([]string{
				"kubernetes.pod_name:\"${resourceName}\"",
				"kubernetes.namespace_name:\"${resourceNamespace}\"",
				"kubernetes.container_name.raw:\"${containerName}\"",
			}, " AND "),
			"')),sort:!('@timestamp',desc))",
		},
			""),
		labels,
	)

	err = console.CreateOrUpdateConsoleExternalLogLink(
		context.TODO(),
		clusterRequest.client,
		consoleExternalLogLink,
		console.ConsoleExternalLogLinkEqual,
		console.MutateConsoleExternalLogLink,
	)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update kibana console external log link CR for cluster",
			"cluster", cluster.Name,
			"kibana_url", kibanaURL,
		)
	}

	return nil
}
