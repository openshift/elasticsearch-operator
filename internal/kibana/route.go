package kibana

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/ViaQ/logerr/v2/kverrors"
	"github.com/openshift/elasticsearch-operator/internal/manifests/console"
	"github.com/openshift/elasticsearch-operator/internal/manifests/route"
	"github.com/openshift/elasticsearch-operator/internal/utils"

	routev1 "github.com/openshift/api/route/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const KibanaConsoleLinkName = "kibana-public-url"

const icon = "data:image/svg+xml;base64,PHN2ZyBpZD0iYWZiNDE1NDktYzU3MC00OWI3LTg1Y2QtNjU3NjAwZWRmMmUxIiBkYXRhLW5hbWU9IkxheWVyIDEiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIgdmlld0JveD0iMCAwIDcyMS4xNSA3MjEuMTUiPgogIDxkZWZzPgogICAgPHN0eWxlPgogICAgICAuYTQ0OGZkZWEtNGE0Yy00Njc4LTk3NmEtYzM3ODUzMDhhZTA2IHsKICAgICAgICBmaWxsOiAjZGIzOTI3OwogICAgICB9CgogICAgICAuZTEzMzA4YjgtNzQ4NS00Y2IwLTk3NjUtOGE1N2I5M2Y5MWE2IHsKICAgICAgICBmaWxsOiAjY2IzNzI4OwogICAgICB9CgogICAgICAuZTc3Mjg2ZjEtMjJkYS00NGQxLThlZmItMWQxNGIwY2NhZTYyIHsKICAgICAgICBmaWxsOiAjZmZmOwogICAgICB9CgogICAgICAuYTA0MjBjYWMtZWJlNi00YzE4LWI5ODEtYWJiYTBiYTliMzY1IHsKICAgICAgICBmaWxsOiAjZTVlNWU0OwogICAgICB9CiAgICA8L3N0eWxlPgogIDwvZGVmcz4KICA8Y2lyY2xlIGNsYXNzPSJhNDQ4ZmRlYS00YTRjLTQ2NzgtOTc2YS1jMzc4NTMwOGFlMDYiIGN4PSIzNjAuNTgiIGN5PSIzNjAuNTgiIHI9IjM1OC4yOCIvPgogIDxwYXRoIGNsYXNzPSJlMTMzMDhiOC03NDg1LTRjYjAtOTc2NS04YTU3YjkzZjkxYTYiIGQ9Ik02MTMuNTQsMTA3LjMsMTA2Ljg4LDYxNGMxNDAsMTM4LjUxLDM2NS44MiwxMzguMDYsNTA1LjI2LTEuMzlTNzUyLDI0Ny4zMyw2MTMuNTQsMTA3LjNaIi8+CiAgPGc+CiAgICA8Y2lyY2xlIGNsYXNzPSJlNzcyODZmMS0yMmRhLTQ0ZDEtOGVmYi0xZDE0YjBjY2FlNjIiIGN4PSIyMzQuNyIgY3k9IjM1Ny4zIiByPSI0Ny43MiIvPgogICAgPGNpcmNsZSBjbGFzcz0iZTc3Mjg2ZjEtMjJkYS00NGQxLThlZmItMWQxNGIwY2NhZTYyIiBjeD0iMjM0LjciIGN5PSIxODIuOTQiIHI9IjQ3LjcyIi8+CiAgICA8Y2lyY2xlIGNsYXNzPSJlNzcyODZmMS0yMmRhLTQ0ZDEtOGVmYi0xZDE0YjBjY2FlNjIiIGN4PSIyMzQuNyIgY3k9IjUzOC4yMSIgcj0iNDcuNzIiLz4KICA8L2c+CiAgPHBvbHlnb24gY2xhc3M9ImU3NzI4NmYxLTIyZGEtNDRkMS04ZWZiLTFkMTRiMGNjYWU2MiIgcG9pbnRzPSI0MzUuMTkgMzQ3LjMgMzkwLjU0IDM0Ny4zIDM5MC41NCAxNzIuOTQgMzE2LjE2IDE3Mi45NCAzMTYuMTYgMTkyLjk0IDM3MC41NCAxOTIuOTQgMzcwLjU0IDM0Ny4zIDMxNi4xNiAzNDcuMyAzMTYuMTYgMzY3LjMgMzcwLjU0IDM2Ny4zIDM3MC41NCA1MjEuNjcgMzE2LjE2IDUyMS42NyAzMTYuMTYgNTQxLjY3IDM5MC41NCA1NDEuNjcgMzkwLjU0IDM2Ny4zIDQzNS4xOSAzNjcuMyA0MzUuMTkgMzQ3LjMiLz4KICA8cG9seWdvbiBjbGFzcz0iZTc3Mjg2ZjEtMjJkYS00NGQxLThlZmItMWQxNGIwY2NhZTYyIiBwb2ludHM9IjU5OS43NCAzMTcuMDMgNTU3Ljk3IDMxNy4wMyA1NTAuOTcgMzE3LjAzIDU1MC45NyAzMTAuMDMgNTUwLjk3IDI2OC4yNiA1NTAuOTcgMjY4LjI2IDQ2NC4zNiAyNjguMjYgNDY0LjM2IDQ0Ni4zNCA1OTkuNzQgNDQ2LjM0IDU5OS43NCAzMTcuMDMgNTk5Ljc0IDMxNy4wMyIvPgogIDxwb2x5Z29uIGNsYXNzPSJhMDQyMGNhYy1lYmU2LTRjMTgtYjk4MS1hYmJhMGJhOWIzNjUiIHBvaW50cz0iNTk5Ljc0IDMxMC4wMyA1NTcuOTcgMjY4LjI2IDU1Ny45NyAzMTAuMDMgNTk5Ljc0IDMxMC4wMyIvPgo8L3N2Zz4K"

// GetRouteURL retrieves the route URL from a given route and namespace
func (clusterRequest *KibanaRequest) GetRouteURL(routeName string) (string, error) {
	key := client.ObjectKey{Name: routeName, Namespace: clusterRequest.cluster.Namespace}
	r, err := route.Get(context.TODO(), clusterRequest.client, key)
	if err != nil {
		if !apierrors.IsNotFound(kverrors.Root(err)) {
			clusterRequest.log.Error(err, "Failed to check for kibana object")
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
		clusterRequest.log.Info("could not read CA certificate for kibana route",
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
			DestinationCACertificate:      string(caCert),
		}).
		//WithCA(caCert).
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

	cl := console.NewConsoleLink(KibanaConsoleLinkName, kibanaURL, "Logging", icon, "Observability")

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
