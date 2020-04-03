package kibana

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/openshift/elasticsearch-operator/pkg/utils"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"

	route "github.com/openshift/api/route/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//NewRoute stubs an instance of a Route
func NewRoute(routeName, namespace, serviceName, cafilePath string) *route.Route {
	return &route.Route{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Route",
			APIVersion: route.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: namespace,
			Labels: map[string]string{
				"component":     "support",
				"logging-infra": "support",
				"provider":      "openshift",
			},
		},
		Spec: route.RouteSpec{
			To: route.RouteTargetReference{
				Name: serviceName,
				Kind: "Service",
			},
			TLS: &route.TLSConfig{
				Termination:                   route.TLSTerminationReencrypt,
				InsecureEdgeTerminationPolicy: route.InsecureEdgeTerminationPolicyRedirect,
				CACertificate:                 string(utils.GetFileContents(cafilePath)),
				DestinationCACertificate:      string(utils.GetFileContents(cafilePath)),
			},
		},
	}
}

//GetRouteURL retrieves the route URL from a given route and namespace
func (clusterRequest *KibanaRequest) GetRouteURL(routeName string) (string, error) {

	foundRoute := &route.Route{}

	if err := clusterRequest.Get(routeName, foundRoute); err != nil {
		if !errors.IsNotFound(err) {
			logrus.Errorf("Failed to check for kibana object: %v", err)
		}
		return "", err
	}

	return fmt.Sprintf("%s%s", "https://", foundRoute.Spec.Host), nil
}

//RemoveRoute with given name and namespace
func (clusterRequest *KibanaRequest) RemoveRoute(routeName string) error {

	route := NewRoute(
		routeName,
		clusterRequest.cluster.Namespace,
		routeName,
		"",
	)

	err := clusterRequest.Delete(route)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Failure deleting %v route %v", routeName, err)
	}

	return nil
}

func (clusterRequest *KibanaRequest) CreateOrUpdateRoute(newRoute *route.Route) error {

	err := clusterRequest.Create(newRoute)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("Failure creating route for %q: %v", clusterRequest.cluster.Name, err)
		}

		// else -- try to update it if its a valid change (e.g. spec.tls)
		current := &route.Route{}

		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := clusterRequest.Get(newRoute.Name, current); err != nil {
				return fmt.Errorf("Failed to get route: %v", err)
			}

			if !reflect.DeepEqual(current.Spec.TLS, newRoute.Spec.TLS) {
				current.Spec.TLS = newRoute.Spec.TLS
				return clusterRequest.Update(current)
			}

			return nil
		})
	}

	return nil
}

func (clusterRequest *KibanaRequest) createOrUpdateKibanaRoute() error {

	cluster := clusterRequest.cluster

	kibanaRoute := NewRoute(
		"kibana",
		cluster.Namespace,
		"kibana",
		utils.GetWorkingDirFilePath("ca.crt"),
	)

	utils.AddOwnerRefToObject(kibanaRoute, getOwnerRef(cluster))

	if err := clusterRequest.CreateOrUpdateRoute(kibanaRoute); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("Failure creating Kibana route for %q: %v", cluster.Name, err)
		}
	}

	kibanaURL, err := clusterRequest.GetRouteURL("kibana")
	if err != nil {
		return err
	}

	sharedConfig := createSharedConfig(cluster.Namespace, kibanaURL, kibanaURL)
	utils.AddOwnerRefToObject(sharedConfig, getOwnerRef(cluster))

	err = clusterRequest.Create(sharedConfig)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("Failure creating Kibana route shared config: %v", err)
	}

	sharedRole := NewRole(
		"sharing-config-reader",
		cluster.Namespace,
		NewPolicyRules(
			NewPolicyRule(
				[]string{""},
				[]string{"configmaps"},
				[]string{"sharing-config"},
				[]string{"get"},
			),
		),
	)

	utils.AddOwnerRefToObject(sharedRole, getOwnerRef(clusterRequest.cluster))

	err = clusterRequest.Create(sharedRole)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("Failure creating Kibana route shared config role for %q: %v", cluster.Name, err)
	}

	sharedRoleBinding := NewRoleBinding(
		"openshift-logging-sharing-config-reader-binding",
		cluster.Namespace,
		"sharing-config-reader",
		NewSubjects(
			NewSubject(
				"Group",
				"system:authenticated",
			),
		),
	)

	utils.AddOwnerRefToObject(sharedRoleBinding, getOwnerRef(clusterRequest.cluster))

	err = clusterRequest.Create(sharedRoleBinding)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("Failure creating Kibana route shared config role binding for %q: %v", cluster.Name, err)
	}

	return nil
}

func (clusterRequest *KibanaRequest) createOrUpdateKibanaConsoleExternalLogLink() (err error) {
	cluster := clusterRequest.cluster

	kibanaURL, err := clusterRequest.GetRouteURL("kibana")
	if err != nil {
		return err
	}

	consoleExternalLogLink := NewConsoleExternalLogLink(
		"kibana",
		cluster.Namespace,
		"Show in Kibana",
		strings.Join([]string{kibanaURL,
			"/app/kibana#/discover?_g=(time:(from:now-1w,mode:relative,to:now))&_a=(columns:!(kubernetes.container_name,message),query:(query_string:(analyze_wildcard:!t,query:'",
			strings.Join([]string{
				"kubernetes.pod_name:\"${resourceName}\"",
				"kubernetes.namespace_name:\"${resourceNamespace}\"",
				"kubernetes.container_name.raw:\"${containerName}\"",
			}, " AND "),
			"')),sort:!('@timestamp',desc))"},
			""),
	)

	utils.AddOwnerRefToObject(consoleExternalLogLink, getOwnerRef(cluster))

	// In case the object already exists we delete it first
	if err = clusterRequest.RemoveConsoleExternalLogLink("kibana"); err != nil {
		return
	}

	err = clusterRequest.Create(consoleExternalLogLink)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("Failure creating Kibana console external log link for %q: %v", cluster.Name, err)
	}
	return nil
}
