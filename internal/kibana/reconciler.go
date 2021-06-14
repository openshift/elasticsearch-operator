package kibana

import (
	"context"
	"fmt"
	"reflect"

	"github.com/ViaQ/logerr/kverrors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	configv1 "github.com/openshift/api/config/v1"
	kibana "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/internal/constants"
	"github.com/openshift/elasticsearch-operator/internal/elasticsearch"
	"github.com/openshift/elasticsearch-operator/internal/elasticsearch/esclient"
	"github.com/openshift/elasticsearch-operator/internal/manifests/deployment"
	"github.com/openshift/elasticsearch-operator/internal/manifests/pod"
	"github.com/openshift/elasticsearch-operator/internal/manifests/secret"
	"github.com/openshift/elasticsearch-operator/internal/manifests/service"
	"github.com/openshift/elasticsearch-operator/internal/utils"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kibanaServiceAccountName     = "kibana"
	kibanaOAuthRedirectReference = "{\"kind\":\"OAuthRedirectReference\",\"apiVersion\":\"v1\",\"reference\":{\"kind\":\"Route\",\"name\":\"kibana\"}}"
	expectedCLOKind              = "ClusterLogging"
	expectedCLOName              = "instance"
	expectedCLOKibana            = "kibana"
	expectedCLONamespace         = "openshift-logging"
)

var kibanaServiceAccountAnnotations = map[string]string{
	"serviceaccounts.openshift.io/oauth-redirectreference.first": kibanaOAuthRedirectReference,
}

func Reconcile(requestCluster *kibana.Kibana, requestClient client.Client, esClient esclient.Client, proxyConfig *configv1.Proxy, eoManagedCerts bool, ownerRef metav1.OwnerReference) error {
	clusterKibanaRequest := KibanaRequest{
		client:   requestClient,
		cluster:  requestCluster,
		esClient: esClient,
	}

	if clusterKibanaRequest.cluster == nil {
		return nil
	}

	if eoManagedCerts {
		cr := elasticsearch.NewCertificateRequest(ownerRef.Name, requestCluster.Namespace, ownerRef, requestClient)
		cr.GenerateKibanaCerts(requestCluster.Name)
	}

	// ensure that we have the certs pulled in from the secret first... required for route generation
	if err := clusterKibanaRequest.readSecrets(); err != nil {
		return err
	}

	if err := clusterKibanaRequest.CreateOrUpdateServiceAccount(kibanaServiceAccountName, kibanaServiceAccountAnnotations); err != nil {
		return err
	}

	if err := clusterKibanaRequest.createOrUpdateKibanaService(); err != nil {
		return err
	}

	if err := clusterKibanaRequest.createOrUpdateKibanaRoute(); err != nil {
		return err
	}

	// we only want to create these if the use case is the CLO one
	// make sure our namespace is "openshift-logging" and our cr name is "kibana"
	// or do we just check that our owner ref is from a cluster logging object?
	if clusterKibanaRequest.isCLOUseCase() {
		if err := clusterKibanaRequest.createOrUpdateKibanaConsoleExternalLogLink(); err != nil {
			return err
		}

		if err := clusterKibanaRequest.createOrUpdateKibanaConsoleLink(); err != nil {
			return err
		}
	}

	if err := clusterKibanaRequest.deleteKibana5Deployment(); err != nil {
		return err
	}

	clusterName := esClient.ClusterName()
	if err := clusterKibanaRequest.createOrUpdateKibanaDeployment(proxyConfig, clusterName); err != nil {
		return err
	}

	return clusterKibanaRequest.UpdateStatus()
}

func GetProxyConfig(r client.Client) (*configv1.Proxy, error) {
	proxyNamespacedName := types.NamespacedName{Name: constants.ProxyName}
	proxyConfig := &configv1.Proxy{}
	if err := r.Get(context.TODO(), proxyNamespacedName, proxyConfig); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, kverrors.Wrap(err, "encountered unexpected error getting proxy",
				"proxy", proxyNamespacedName,
			)
		}
	}
	return proxyConfig, nil
}

func compareKibanaStatus(lhs, rhs []kibana.KibanaStatus) bool {
	// there should only ever be a single kibana status object
	if len(lhs) != len(rhs) {
		return false
	}

	if len(lhs) > 0 {
		for index := range lhs {
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

func (clusterRequest *KibanaRequest) deleteKibana5Deployment() error {
	kibana5 := &apps.Deployment{}
	if err := clusterRequest.Get(clusterRequest.cluster.Name, kibana5); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return kverrors.Wrap(err, "failed to get kibana 5 deployment")
	}

	containers := kibana5.Spec.Template.Spec.Containers
	for _, c := range containers {
		if c.Image == getImage() {
			return nil
		}
	}

	if err := clusterRequest.Delete(kibana5); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return kverrors.Wrap(err, "failed to delete kibana 5 deployment")
	}
	return nil
}

func (clusterRequest *KibanaRequest) isCLOUseCase() bool {
	kibanaCR := clusterRequest.cluster

	if kibanaCR.OwnerReferences != nil {
		// also check for the owner ref being clusterlogging/instance
		for _, ref := range kibanaCR.OwnerReferences {
			if ref.Kind == expectedCLOKind && ref.Name == expectedCLOName {
				return true
			}
		}
	}

	// this is a secondary check to allow for stand-alone EO testing to work
	if kibanaCR.Name == expectedCLOKibana && kibanaCR.Namespace == expectedCLONamespace {
		return true
	}

	return false
}

func (clusterRequest *KibanaRequest) createOrUpdateKibanaDeployment(proxyConfig *configv1.Proxy, clusterName string) (err error) {
	kibanaTrustBundle := &v1.ConfigMap{}

	// Create cluster proxy trusted CA bundle.
	if proxyConfig != nil {
		kibanaTrustBundle, err = clusterRequest.createOrGetTrustedCABundleConfigMap(constants.KibanaTrustedCAName)
		if err != nil {
			return
		}
	}

	kibanaPodSpec := newKibanaPodSpec(
		clusterRequest,
		fmt.Sprintf("%s.%s.svc", clusterName, clusterRequest.cluster.Namespace),
		proxyConfig,
		kibanaTrustBundle,
	)

	kibanaDeployment := NewDeployment(
		"kibana",
		clusterRequest.cluster.Namespace,
		"kibana",
		"kibana",
		clusterRequest.cluster.Spec.Replicas,
		kibanaPodSpec,
	)

	// if we don't have the hash values we shouldn't start/create
	annotations, err := clusterRequest.getKibanaAnnotations(kibanaDeployment)
	if err != nil {
		return err
	}

	kibanaDeployment.Spec.Template.ObjectMeta.Annotations = annotations

	utils.AddOwnerRefToObject(kibanaDeployment, getOwnerRef(clusterRequest.cluster))

	err = deployment.CreateOrUpdate(context.TODO(), clusterRequest.client, kibanaDeployment, compareDeployments, mutateDeployment)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update kibana deployment",
			"cluster", clusterRequest.cluster.Name,
			"namespace", clusterRequest.cluster.Namespace,
		)
	}

	return nil
}

func (clusterRequest *KibanaRequest) getKibanaAnnotations(deployment *apps.Deployment) (map[string]string, error) {
	if deployment.Spec.Template.ObjectMeta.Annotations == nil {
		deployment.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}

	annotations := deployment.Spec.Template.ObjectMeta.Annotations

	kibanaTrustBundle := &v1.ConfigMap{}
	kibanaTrustBundleName := types.NamespacedName{Name: constants.KibanaTrustedCAName, Namespace: clusterRequest.cluster.Namespace}
	if err := clusterRequest.client.Get(context.TODO(), kibanaTrustBundleName, kibanaTrustBundle); err != nil {
		if !apierrors.IsNotFound(err) {
			return annotations, err
		}
	}

	if _, ok := kibanaTrustBundle.Data[constants.TrustedCABundleKey]; !ok {
		return annotations, kverrors.New("trust bundle does not yet contain expected key",
			"bundle", kibanaTrustBundle.Name,
			"key", constants.TrustedCABundleKey,
		)
	}

	trustedCAHashValue, err := calcTrustedCAHashValue(kibanaTrustBundle)
	if err != nil {
		return annotations, kverrors.Wrap(err, "unable to calculate trusted CA value")
	}

	if trustedCAHashValue == "" {
		return annotations, kverrors.New("did not receive hashvalue for trusted CA value")
	}

	annotations[constants.TrustedCABundleHashName] = trustedCAHashValue

	// generate secret hash
	for _, secretName := range []string{"kibana", "kibana-proxy"} {

		hashKey := fmt.Sprintf("%s%s", constants.SecretHashPrefix, secretName)

		key := client.ObjectKey{Name: secretName, Namespace: clusterRequest.cluster.Namespace}
		sec, err := secret.Get(context.TODO(), clusterRequest.client, key)
		if err != nil {
			return annotations, err
		}
		secretHashValue, err := calcSecretHashValue(sec)
		if err != nil {
			return annotations, err
		}

		annotations[hashKey] = secretHashValue
	}

	return annotations, nil
}

func compareDeployments(current, desired *apps.Deployment) bool {
	if !pod.ArePodTemplateSpecEqual(current.Spec.Template, desired.Spec.Template) {
		return false
	}
	if *current.Spec.Replicas != *desired.Spec.Replicas {
		return false
	}

	currentTrustedCAHash := current.Spec.Template.ObjectMeta.Annotations[constants.TrustedCABundleHashName]
	desiredTrustedCAHash := desired.Spec.Template.ObjectMeta.Annotations[constants.TrustedCABundleHashName]
	if currentTrustedCAHash != desiredTrustedCAHash {
		return false
	}

	for _, secretName := range []string{"kibana", "kibana-proxy"} {
		hashKey := fmt.Sprintf("%s%s", constants.SecretHashPrefix, secretName)
		currentHash := current.Spec.Template.ObjectMeta.Annotations[hashKey]
		desiredHash := desired.Spec.Template.ObjectMeta.Annotations[hashKey]

		if currentHash != desiredHash {
			return false
		}
	}
	return true
}

func mutateDeployment(current *apps.Deployment, desired *apps.Deployment) {
	if !pod.ArePodTemplateSpecEqual(current.Spec.Template, desired.Spec.Template) {
		current.Spec.Template.Spec.NodeSelector = desired.Spec.Template.Spec.NodeSelector
		current.Spec.Template.Spec.Tolerations = desired.Spec.Template.Spec.Tolerations

		containers := current.Spec.Template.Spec.Containers

		for index, curr := range current.Spec.Template.Spec.Containers {
			for _, des := range desired.Spec.Template.Spec.Containers {
				// Only compare the images of containers with the same name
				if curr.Name == des.Name {
					if curr.Image != des.Image {
						containers[index].Image = des.Image
					}
					if !utils.EnvValueEqual(curr.Env, des.Env) {
						containers[index].Env = des.Env
					}
					containers[index].Args = des.Args
					containers[index].Resources = des.Resources
					containers[index].VolumeMounts = des.VolumeMounts
				}
			}
		}
	}

	if *current.Spec.Replicas != *desired.Spec.Replicas {
		*current.Spec.Replicas = *desired.Spec.Replicas
	}

	currentTrustedCAHash := current.Spec.Template.ObjectMeta.Annotations[constants.TrustedCABundleHashName]
	desiredTrustedCAHash := desired.Spec.Template.ObjectMeta.Annotations[constants.TrustedCABundleHashName]
	if currentTrustedCAHash != desiredTrustedCAHash {
		if current.Spec.Template.ObjectMeta.Annotations == nil {
			current.Spec.Template.ObjectMeta.Annotations = map[string]string{}
		}
		current.Spec.Template.ObjectMeta.Annotations[constants.TrustedCABundleHashName] = desiredTrustedCAHash
	}

	for _, secretName := range []string{"kibana", "kibana-proxy"} {
		hashKey := fmt.Sprintf("%s%s", constants.SecretHashPrefix, secretName)
		currentHash := current.Spec.Template.ObjectMeta.Annotations[hashKey]
		desiredHash := desired.Spec.Template.ObjectMeta.Annotations[hashKey]

		if currentHash != desiredHash {
			if current.Spec.Template.ObjectMeta.Annotations == nil {
				current.Spec.Template.ObjectMeta.Annotations = map[string]string{}
			}
			current.Spec.Template.ObjectMeta.Annotations[hashKey] = desiredHash
		}
	}
}

func (clusterRequest *KibanaRequest) createOrUpdateKibanaService() error {
	labels := map[string]string{
		"logging-infra": "support",
	}

	svc := service.New("kibana", clusterRequest.cluster.Namespace, labels).
		WithSelector(map[string]string{
			"component": "kibana",
			"provider":  "openshift",
		}).
		WithServicePorts(v1.ServicePort{
			Port:       443,
			TargetPort: intstr.FromString("oaproxy"),
		}).
		Build()

	utils.AddOwnerRefToObject(svc, getOwnerRef(clusterRequest.cluster))

	err := service.CreateOrUpdate(context.TODO(), clusterRequest.client, svc, service.Equal, service.Mutate)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update kibana service",
			"cluster", clusterRequest.cluster.Name,
			"namespace", clusterRequest.cluster.Namespace,
		)
	}

	return nil
}

func getImage() string {
	return utils.LookupEnvWithDefault("KIBANA_IMAGE", kibanaDefaultImage)
}

func getProxyImage() string {
	return utils.LookupEnvWithDefault("PROXY_IMAGE", kibanaProxyDefaultImage)
}

func newKibanaPodSpec(cluster *KibanaRequest, elasticsearchName string, proxyConfig *configv1.Proxy,
	trustedCABundleCM *v1.ConfigMap) v1.PodSpec {
	visSpec := kibana.KibanaSpec{}
	if cluster.cluster != nil {
		visSpec = cluster.cluster.Spec
	}
	kibanaResources := visSpec.Resources
	if kibanaResources == nil {
		kibanaResources = &v1.ResourceRequirements{
			Limits: v1.ResourceList{v1.ResourceMemory: defaultKibanaMemory},
			Requests: v1.ResourceList{
				v1.ResourceMemory: defaultKibanaMemory,
				v1.ResourceCPU:    defaultKibanaCPURequest,
			},
		}
	}

	kibanaImage := getImage()
	kibanaContainer := NewContainer(
		"kibana",
		kibanaImage,
		v1.PullIfNotPresent,
		*kibanaResources,
	)

	endpoints := fmt.Sprintf(`["https://%s:9200"]`, elasticsearchName)

	kibanaContainer.Env = []v1.EnvVar{
		{
			Name:  "ELASTICSEARCH_HOSTS",
			Value: endpoints,
		},
		{
			Name: "KIBANA_MEMORY_LIMIT",
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

	kibanaProxyResources := visSpec.ProxySpec.Resources
	if kibanaProxyResources == nil {
		kibanaProxyResources = &v1.ResourceRequirements{
			Limits: v1.ResourceList{v1.ResourceMemory: defaultKibanaProxyMemory},
			Requests: v1.ResourceList{
				v1.ResourceMemory: defaultKibanaProxyMemory,
				v1.ResourceCPU:    defaultKibanaProxyCPURequest,
			},
		}
	}

	proxyImage := getProxyImage()
	kibanaProxyContainer := NewContainer(
		"kibana-proxy",
		proxyImage,
		v1.PullIfNotPresent,
		*kibanaProxyResources,
	)

	kibanaProxyContainer.Args = []string{
		"--upstream-ca=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
		"--https-address=:3000",
		"-provider=openshift",
		fmt.Sprintf("-client-id=system:serviceaccount:%s:kibana", cluster.cluster.Namespace),
		"-client-secret-file=/var/run/secrets/kubernetes.io/serviceaccount/token",
		"-cookie-secret-file=/secret/session-secret",
		"-cookie-expire=24h",
		"-skip-provider-button",
		"-upstream=http://localhost:5601",
		"-scope=user:info user:check-access user:list-projects",
		"--tls-cert=/secret/server-cert",
		"-tls-key=/secret/server-key",
		"-pass-access-token",
	}

	kibanaProxyContainer.Env = []v1.EnvVar{
		{Name: "OAP_DEBUG", Value: "false"},
		{
			Name: "OCP_AUTH_PROXY_MEMORY_LIMIT",
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

	if hasTrustedCABundle(trustedCABundleCM) {
		addTrustedCAVolume = true
		kibanaProxyContainer.VolumeMounts = append(kibanaProxyContainer.VolumeMounts,
			v1.VolumeMount{
				Name:      constants.KibanaTrustedCAName,
				ReadOnly:  true,
				MountPath: constants.TrustedCABundleMountDir,
			})
	}

	kibanaPodSpec := pod.NewSpec(
		"kibana",
		[]v1.Container{kibanaContainer, kibanaProxyContainer},
		[]v1.Volume{
			{
				Name: "kibana", VolumeSource: v1.VolumeSource{
					Secret: &v1.SecretVolumeSource{
						SecretName: "kibana",
					},
				},
			},
			{
				Name: "kibana-proxy", VolumeSource: v1.VolumeSource{
					Secret: &v1.SecretVolumeSource{
						SecretName: "kibana-proxy",
					},
				},
			},
		},
	).
		WithNodeSelectors(visSpec.NodeSelector).
		WithTolerations(visSpec.Tolerations...).
		Build()

	if addTrustedCAVolume {
		kibanaPodSpec.Volumes = append(kibanaPodSpec.Volumes,
			v1.Volume{
				Name: constants.KibanaTrustedCAName,
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: constants.KibanaTrustedCAName,
						},
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
								{
									Key:      "logging-infra",
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

	return *kibanaPodSpec
}

func getOwnerRef(v *kibana.Kibana) metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: kibana.GroupVersion.String(),
		Kind:       "Kibana",
		Name:       v.Name,
		UID:        v.UID,
		Controller: &trueVar,
	}
}

func NewContainer(containerName string, imageName string, pullPolicy v1.PullPolicy, resources v1.ResourceRequirements) v1.Container {
	return v1.Container{
		Name:            containerName,
		Image:           imageName,
		ImagePullPolicy: pullPolicy,
		Resources:       resources,
	}
}
