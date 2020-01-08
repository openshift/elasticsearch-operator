package kibana_handler

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"

	configv1 "github.com/openshift/api/config/v1"
	kibana "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/pkg/constants"
	"github.com/openshift/elasticsearch-operator/pkg/utils"
	"github.com/sirupsen/logrus"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	annotationOauthSecretUpdatedAt = "logging.openshift.io/oauthSecretUpdatedAt"
	kibanaServiceAccountName       = "kibana"
	kibanaOAuthRedirectReference   = "{\"kind\":\"OAuthRedirectReference\",\"apiVersion\":\"v1\",\"reference\":{\"kind\":\"Route\",\"name\":\"kibana\"}}"
)

var (
	kibanaServiceAccountAnnotations = map[string]string{
		"serviceaccounts.openshift.io/oauth-redirectreference.first": kibanaOAuthRedirectReference,
	}
)

func ReconcileKibana(requestCluster *kibana.Kibana, requestClient client.Client) error {
	clusterKibanaRequest := ClusterKibanaRequest{
		client:  requestClient,
		cluster: requestCluster,
	}

	proxyNamespacedName := types.NamespacedName{Name: constants.ProxyName}
	proxyConfig := &configv1.Proxy{}
	if err := requestClient.Get(context.TODO(), proxyNamespacedName, proxyConfig); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("Encountered unexpected error getting %v. Error: %s", proxyNamespacedName,
				err.Error())
		}
	}

	if clusterKibanaRequest.cluster == nil {
		return nil
	}

	if err := clusterKibanaRequest.CreateOrUpdateServiceAccount(kibanaServiceAccountName, &kibanaServiceAccountAnnotations); err != nil {
		return err
	}

	if err := clusterKibanaRequest.RemoveClusterRoleBinding("kibana-proxy-oauth-delegator"); err != nil {
		return err
	}

	if err := clusterKibanaRequest.createOrUpdateKibanaService(); err != nil {
		return err
	}

	if err := clusterKibanaRequest.createOrUpdateKibanaRoute(); err != nil {
		return err
	}

	//Kibana secrets are handled by cluster-logging-operator
	//if err := clusterKibanaRequest.createOrUpdateKibanaSecret(); err != nil {
	//	return err
	//}

	if err := clusterKibanaRequest.createOrUpdateKibanaConsoleExternalLogLink(); err != nil {
		return err
	}

	if err := clusterKibanaRequest.RemoveOAuthClient("kibana-proxy"); err != nil {
		return err
	}

	//proxyConfig := &configv1.Proxy{}
	if err := clusterKibanaRequest.createOrUpdateKibanaDeployment(proxyConfig); err != nil {
		return err
	}

	kibanaStatus, err := clusterKibanaRequest.getKibanaStatus()
	cluster := clusterKibanaRequest.cluster

	if err != nil {
		return fmt.Errorf("Failed to get Kibana status for %q: %v", cluster.Name, err)
	}

	printUpdateMessage := true
	retryErr := retry.RetryOnConflict(retry.DefaultRetry,
		func() error {
			if !compareKibanaStatus(kibanaStatus,
				cluster.Status) {
				if printUpdateMessage {
					logrus.Infof("Updating status of Kibana")
					printUpdateMessage = false
				}
				cluster.Status = kibanaStatus
				return clusterKibanaRequest.UpdateStatus(cluster)
			}
			return nil
		})
	if retryErr != nil {
		return fmt.Errorf("Failed to update Kibana status for %q: %v", cluster.Name, retryErr)
	}

	return nil
}

func compareKibanaStatus(lhs, rhs []kibana.KibanaStatus) bool {
	// there should only ever be a single kibana status object
	if len(lhs) != len(rhs) {
		return false
	}

	if len(lhs) > 0 {
		for index, _ := range lhs {
			if lhs[index].Deployment != rhs[index].Deployment {
				return false
			}

			if lhs[index].Replicas != rhs[index].Replicas {
				return false
			}

			if len(lhs[index].ReplicaSets) != len(rhs[index].ReplicaSets) {
				return false
			}

			if len(lhs[index].ReplicaSets) > 0 {
				if !reflect.DeepEqual(lhs[index].ReplicaSets, rhs[index].ReplicaSets) {
					return false
				}
			}

			if len(lhs[index].Pods) != len(rhs[index].Pods) {
				return false
			}

			if len(lhs[index].Pods) > 0 {
				if !reflect.DeepEqual(lhs[index].Pods, rhs[index].Pods) {
					return false
				}
			}

			if len(lhs[index].Conditions) != len(rhs[index].Conditions) {
				return false
			}

			if len(lhs[index].Conditions) > 0 {
				if !reflect.DeepEqual(lhs[index].Conditions, rhs[index].Conditions) {
					return false
				}
			}
		}
	}

	return true
}

func (clusterRequest *ClusterKibanaRequest) removeKibana() (err error) {
	if clusterRequest.isManaged() {
		name := "kibana"
		proxyName := "kibana-proxy"
		if err = clusterRequest.RemoveDeployment(name); err != nil {
			return
		}

		if err = clusterRequest.RemoveOAuthClient(proxyName); err != nil {
			return
		}

		if err = clusterRequest.RemoveSecret(name); err != nil {
			return
		}

		if err = clusterRequest.RemoveSecret(proxyName); err != nil {
			return
		}

		if err = clusterRequest.RemoveRoute(name); err != nil {
			return
		}

		if err = clusterRequest.RemoveConfigMap(name); err != nil {
			return
		}

		if err = clusterRequest.RemoveConfigMap("sharing-config"); err != nil {
			return
		}

		if err = clusterRequest.RemoveConfigMap(constants.KibanaTrustedCAName); err != nil {
			return
		}

		if err = clusterRequest.RemoveService(name); err != nil {
			return
		}

		if err = clusterRequest.RemoveServiceAccount(name); err != nil {
			return
		}

		if err = clusterRequest.RemoveConsoleExternalLogLink(name); err != nil {
			return
		}

	}

	return nil
}

func (clusterRequest *ClusterKibanaRequest) createOrUpdateKibanaDeployment(proxyConfig *configv1.Proxy) (err error) {
	kibanaTrustBundle := &v1.ConfigMap{}

	// Create cluster proxy trusted CA bundle.
	if proxyConfig != nil {
		err = clusterRequest.createOrUpdateTrustedCABundleConfigMap(constants.KibanaTrustedCAName)
		if err != nil {
			return
		}

		// kibana-trusted-ca-bundle
		kibanaTrustBundleName := types.NamespacedName{
			Name:      constants.KibanaTrustedCAName,
			Namespace: clusterRequest.cluster.Namespace,
		}
		if err := clusterRequest.client.Get(context.TODO(), kibanaTrustBundleName, kibanaTrustBundle); err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		}
	}

	kibanaPodSpec := newKibanaPodSpec(clusterRequest, "elasticsearch.openshift-logging.svc.cluster.local", proxyConfig, kibanaTrustBundle)
	kibanaDeployment := NewDeployment(
		"kibana",
		clusterRequest.cluster.Namespace,
		"kibana",
		"kibana",
		kibanaPodSpec,
	)
	kibanaDeployment.Spec.Replicas = &clusterRequest.cluster.Spec.Replicas

	utils.AddOwnerRefToObject(kibanaDeployment, utils.AsOwner(clusterRequest.cluster))

	err = clusterRequest.Create(kibanaDeployment)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("Failure creating Kibana deployment for %q: %v", clusterRequest.cluster.Name, err)
	}

	if clusterRequest.isManaged() {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			current := &apps.Deployment{}

			if err := clusterRequest.Get(kibanaDeployment.Name, current); err != nil {
				if errors.IsNotFound(err) {
					// the object doesn't exist -- it was likely culled
					// recreate it on the next time through if necessary
					logrus.Debugf("Returning nil. The deployment %q was not found even though create previously failed.  Was it culled?", kibanaDeployment.Name)
					return nil
				}
				return fmt.Errorf("Failed to get Kibana deployment: %v", err)
			}

			current, different := isDeploymentDifferent(current, kibanaDeployment)

			// Check trustedCA certs have been updated or not by comparing the hash values in annotation.
			newTrustedCAHashedValue := calcTrustedCAHashValue(kibanaTrustBundle)
			trustedCAHashedValue, _ := current.Spec.Template.ObjectMeta.Annotations[constants.TrustedCABundleHashName]
			if trustedCAHashedValue != newTrustedCAHashedValue {
				different = true
				if kibanaDeployment.Spec.Template.ObjectMeta.Annotations == nil {
					kibanaDeployment.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
				}
				kibanaDeployment.Spec.Template.ObjectMeta.Annotations[constants.TrustedCABundleHashName] = newTrustedCAHashedValue
			}

			if different {
				current.Spec = kibanaDeployment.Spec
				return clusterRequest.Update(current)
			}
			return nil
		})
	}

	return nil
}

func isDeploymentDifferent(current *apps.Deployment, desired *apps.Deployment) (*apps.Deployment, bool) {

	different := false

	// is this needed?
	if !utils.AreMapsSame(current.Spec.Template.Spec.NodeSelector, desired.Spec.Template.Spec.NodeSelector) {
		logrus.Debugf("Visualization nodeSelector change found, updating '%s'", current.Name)
		current.Spec.Template.Spec.NodeSelector = desired.Spec.Template.Spec.NodeSelector
		different = true
	}

	// is this needed?
	if !utils.AreTolerationsSame(current.Spec.Template.Spec.Tolerations, desired.Spec.Template.Spec.Tolerations) {
		logrus.Debugf("Visualization tolerations change found, updating '%s'", current.Name)
		current.Spec.Template.Spec.Tolerations = desired.Spec.Template.Spec.Tolerations
		different = true
	}

	if isDeploymentImageDifference(current, desired) {
		logrus.Debugf("Visualization image change found, updating %q", current.Name)
		current = updateCurrentDeploymentImages(current, desired)
		different = true
	}

	if utils.AreResourcesDifferent(current, desired) {
		logrus.Debugf("Visualization resource(s) change found, updating %q", current.Name)
		different = true
	}

	if !utils.EnvValueEqual(current.Spec.Template.Spec.Containers[0].Env, desired.Spec.Template.Spec.Containers[0].Env) {
		current.Spec.Template.Spec.Containers[0].Env = desired.Spec.Template.Spec.Containers[0].Env
		different = true
	}

	return current, different
}

func isDeploymentImageDifference(current *apps.Deployment, desired *apps.Deployment) bool {

	for _, curr := range current.Spec.Template.Spec.Containers {
		for _, des := range desired.Spec.Template.Spec.Containers {
			// Only compare the images of containers with the same name
			if curr.Name == des.Name {
				if curr.Image != des.Image {
					return true
				}
			}
		}
	}

	return false
}

func updateCurrentDeploymentImages(current *apps.Deployment, desired *apps.Deployment) *apps.Deployment {

	containers := current.Spec.Template.Spec.Containers

	for index, curr := range current.Spec.Template.Spec.Containers {
		for _, des := range desired.Spec.Template.Spec.Containers {
			// Only compare the images of containers with the same name
			if curr.Name == des.Name {
				if curr.Image != des.Image {
					containers[index].Image = des.Image
				}
			}
		}
	}

	return current
}

func (clusterRequest *ClusterKibanaRequest) createOrUpdateKibanaRoute() error {

	cluster := clusterRequest.cluster

	kibanaRoute := NewRoute(
		"kibana",
		cluster.Namespace,
		"Kibana",
		utils.GetWorkingDirFilePath("ca.crt"),
	)

	utils.AddOwnerRefToObject(kibanaRoute, utils.AsOwner(cluster))

	err := clusterRequest.Create(kibanaRoute)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("Failure creating Kibana route for %q: %v", cluster.Name, err)
	}

	kibanaURL, err := clusterRequest.GetRouteURL("kibana")
	if err != nil {
		return err
	}

	sharedConfig := createSharedConfig(cluster.Namespace, kibanaURL, kibanaURL)
	utils.AddOwnerRefToObject(sharedConfig, utils.AsOwner(cluster))

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

	utils.AddOwnerRefToObject(sharedRole, utils.AsOwner(clusterRequest.cluster))

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

	utils.AddOwnerRefToObject(sharedRoleBinding, utils.AsOwner(clusterRequest.cluster))

	err = clusterRequest.Create(sharedRoleBinding)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("Failure creating Kibana route shared config role binding for %q: %v", cluster.Name, err)
	}

	return nil
}

func (clusterRequest *ClusterKibanaRequest) createOrUpdateKibanaConsoleExternalLogLink() (err error) {
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

	utils.AddOwnerRefToObject(consoleExternalLogLink, utils.AsOwner(cluster))

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

func (clusterRequest *ClusterKibanaRequest) createOrUpdateKibanaService() error {

	kibanaService := NewService(
		"kibana",
		clusterRequest.cluster.Namespace,
		"kibana",
		[]v1.ServicePort{
			{Port: 443, TargetPort: intstr.IntOrString{
				Type:   intstr.String,
				StrVal: "oaproxy",
			}},
		})

	utils.AddOwnerRefToObject(kibanaService, utils.AsOwner(clusterRequest.cluster))

	err := clusterRequest.Create(kibanaService)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("Failure constructing Kibana service for %q: %v", clusterRequest.cluster.Name, err)
	}

	return nil
}

func (clusterRequest *ClusterKibanaRequest) createOrUpdateKibanaSecret() error {

	kibanaSecret := NewSecret(
		"kibana",
		clusterRequest.cluster.Namespace,
		map[string][]byte{
			"ca":   utils.GetWorkingDirFileContents("ca.crt"),
			"key":  utils.GetWorkingDirFileContents("system.logging.kibana.key"),
			"cert": utils.GetWorkingDirFileContents("system.logging.kibana.crt"),
		})

	utils.AddOwnerRefToObject(kibanaSecret, utils.AsOwner(clusterRequest.cluster))

	err := clusterRequest.CreateOrUpdateSecret(kibanaSecret)
	if err != nil {
		return err
	}

	proxySecret := NewSecret(
		"kibana-proxy",
		clusterRequest.cluster.Namespace,
		map[string][]byte{
			"session-secret": utils.GetRandomWord(32),
			"server-key":     utils.GetWorkingDirFileContents("kibana-internal.key"),
			"server-cert":    utils.GetWorkingDirFileContents("kibana-internal.crt"),
		})

	utils.AddOwnerRefToObject(proxySecret, utils.AsOwner(clusterRequest.cluster))

	err = clusterRequest.CreateOrUpdateSecret(proxySecret)
	if err != nil {
		return err
	}

	return nil
}

func newKibanaPodSpec(cluster *ClusterKibanaRequest, elasticsearchName string, proxyConfig *configv1.Proxy,
	trustedCABundleCM *v1.ConfigMap) v1.PodSpec {
	visSpec := kibana.KibanaSpec{}
	if cluster.cluster != nil {
		visSpec = cluster.cluster.Spec
	}
	var kibanaResources = visSpec.Resources
	if kibanaResources == nil {
		kibanaResources = &v1.ResourceRequirements{
			Limits: v1.ResourceList{v1.ResourceMemory: defaultKibanaMemory},
			Requests: v1.ResourceList{
				v1.ResourceMemory: defaultKibanaMemory,
				v1.ResourceCPU:    defaultKibanaCpuRequest,
			},
		}
	}
	kibanaContainer := NewContainer(
		"kibana",
		"kibana",
		v1.PullIfNotPresent,
		*kibanaResources,
	)

	var endpoint bytes.Buffer

	endpoint.WriteString("https://")
	endpoint.WriteString(elasticsearchName)
	endpoint.WriteString(":9200")

	kibanaContainer.Env = []v1.EnvVar{
		{Name: "ELASTICSEARCH_URL", Value: endpoint.String()},
		{Name: "KIBANA_MEMORY_LIMIT",
			ValueFrom: &v1.EnvVarSource{
				ResourceFieldRef: &v1.ResourceFieldSelector{
					ContainerName: "kibana",
					Resource:      "limits.memory",
				},
			},
		},
	}

	kibanaContainer.VolumeMounts = []v1.VolumeMount{
		{Name: "kibana", ReadOnly: true, MountPath: "/etc/kibana/keys"},
	}

	kibanaContainer.ReadinessProbe = &v1.Probe{
		Handler: v1.Handler{
			Exec: &v1.ExecAction{
				Command: []string{
					"/usr/share/kibana/probe/readiness.sh",
				},
			},
		},
		InitialDelaySeconds: 5, TimeoutSeconds: 4, PeriodSeconds: 5,
	}

	var kibanaProxyResources = visSpec.ProxySpec.Resources
	if kibanaProxyResources == nil {
		kibanaProxyResources = &v1.ResourceRequirements{
			Limits: v1.ResourceList{v1.ResourceMemory: defaultKibanaProxyMemory},
			Requests: v1.ResourceList{
				v1.ResourceMemory: defaultKibanaProxyMemory,
				v1.ResourceCPU:    defaultKibanaProxyCpuRequest,
			},
		}
	}
	kibanaProxyContainer := NewContainer(
		"kibana-proxy",
		"kibana-proxy",
		v1.PullIfNotPresent,
		*kibanaProxyResources,
	)

	kibanaProxyContainer.Args = []string{
		"--upstream-ca=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
		"--https-address=:3000",
		"-provider=openshift",
		"-client-id=system:serviceaccount:openshift-logging:kibana",
		"-client-secret-file=/var/run/secrets/kubernetes.io/serviceaccount/token",
		"-cookie-secret-file=/secret/session-secret",
		"-upstream=http://localhost:5601",
		"-scope=user:info user:check-access user:list-projects",
		"--tls-cert=/secret/server-cert",
		"-tls-key=/secret/server-key",
		"-pass-access-token",
	}

	kibanaProxyContainer.Env = []v1.EnvVar{
		{Name: "OAP_DEBUG", Value: "false"},
		{Name: "OCP_AUTH_PROXY_MEMORY_LIMIT",
			ValueFrom: &v1.EnvVarSource{
				ResourceFieldRef: &v1.ResourceFieldSelector{
					ContainerName: "kibana-proxy",
					Resource:      "limits.memory",
				},
			},
		},
	}

	proxyEnv := utils.SetProxyEnvVars(proxyConfig)
	kibanaProxyContainer.Env = append(kibanaProxyContainer.Env, proxyEnv...)

	kibanaProxyContainer.Ports = []v1.ContainerPort{
		{Name: "oaproxy", ContainerPort: 3000},
	}

	kibanaProxyContainer.VolumeMounts = []v1.VolumeMount{
		{Name: "kibana-proxy", ReadOnly: true, MountPath: "/secret"},
	}

	addTrustedCAVolume := false
	// If trusted CA bundle ConfigMap exists and its hash value is non-zero, mount the bundle.
	if trustedCABundleCM != nil && hasTrustedCABundle(trustedCABundleCM) {
		addTrustedCAVolume = true
		kibanaProxyContainer.VolumeMounts = append(kibanaProxyContainer.VolumeMounts,
			v1.VolumeMount{
				Name:      constants.KibanaTrustedCAName,
				ReadOnly:  true,
				MountPath: constants.TrustedCABundleMountDir,
			})
	}

	kibanaPodSpec := NewPodSpec(
		"kibana",
		[]v1.Container{kibanaContainer, kibanaProxyContainer},
		[]v1.Volume{
			{Name: "kibana", VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: "kibana",
				},
			},
			},
			{Name: "kibana-proxy", VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: "kibana-proxy",
				},
			},
			},
		},
		visSpec.NodeSelector,
		visSpec.Tolerations,
	)

	if addTrustedCAVolume {
		optional := true
		kibanaPodSpec.Volumes = append(kibanaPodSpec.Volumes,
			v1.Volume{
				Name: constants.KibanaTrustedCAName,
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: constants.KibanaTrustedCAName,
						},
						Optional: &optional,
						Items: []v1.KeyToPath{
							{
								Key:  constants.TrustedCABundleKey,
								Path: constants.TrustedCABundleMountFile,
							},
						},
					},
				},
			})
	}

	kibanaPodSpec.Affinity = &v1.Affinity{
		PodAntiAffinity: &v1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []v1.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: v1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{Key: "logging-infra",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"kibana"},
								},
							},
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			},
		},
	}

	return kibanaPodSpec
}

func updateCurrentImages(current *apps.Deployment, desired *apps.Deployment) *apps.Deployment {

	containers := current.Spec.Template.Spec.Containers

	for index, curr := range current.Spec.Template.Spec.Containers {
		for _, des := range desired.Spec.Template.Spec.Containers {
			// Only compare the images of containers with the same name
			if curr.Name == des.Name {
				if curr.Image != des.Image {
					containers[index].Image = des.Image
				}
			}
		}
	}

	return current
}

func createSharedConfig(namespace, kibanaAppURL, kibanaInfraURL string) *v1.ConfigMap {
	return NewConfigMap(
		"sharing-config",
		namespace,
		map[string]string{
			"kibanaAppURL":   kibanaAppURL,
			"kibanaInfraURL": kibanaInfraURL,
		},
	)
}
