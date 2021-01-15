package k8shandler

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	api "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/internal/utils"
	"github.com/openshift/elasticsearch-operator/test/helpers"
)

var (
	commonCPUValue = resource.MustParse("500m")
	commonMemValue = resource.MustParse("2Gi")

	nodeCPUValue = resource.MustParse("600m")
	nodeMemValue = resource.MustParse("3Gi")

	defaultTestCPURequest = resource.MustParse(defaultESCpuRequest)
	defaultTestMemLimit   = resource.MustParse(defaultESMemoryLimit)
	defaultTestMemRequest = resource.MustParse(defaultESMemoryRequest)
)

/*
  Resource scenarios:
  1. common has a limit and request
     node sets new request and limit
     -> use node settings

  2. common has limit and request
     node has no settings
     -> use common settings

  3. common has no settings
     node has no settings
     -> use defaults

  4. common has limit and request
     node sets request only
     -> use common limit and node request

  5. common has limit and request
     node sets limt only
     -> use common request and node limit

  6. common has request only
     node has no settings
     -> cpu/mem request and mem limit set to be same and use common settings

  7. common has limit only
     node has no settings
     -> request and limit set to be same and use common settings

  8. common has no settings
     node only has request
     -> cpu/mem request and mem limit set to be same and use node settings

  9. common has no settings
     node only has limit
     -> request and limit set to be same and use node settings

 10. common has limit
     node has request
     -> use common limit and node request

 11. common has request
     node has limit
     -> use common request and node limit
*/

// 1
func TestResourcesCommonAndNodeDefined(t *testing.T) {
	commonRequirements := buildResource(
		commonCPUValue,
		commonCPUValue,
		commonMemValue,
		commonMemValue,
	)

	nodeRequirements := buildResource(
		nodeCPUValue,
		nodeCPUValue,
		nodeMemValue,
		nodeMemValue,
	)

	expected := buildResource(
		nodeCPUValue,
		nodeCPUValue,
		nodeMemValue,
		nodeMemValue,
	)

	actual := newESResourceRequirements(nodeRequirements, commonRequirements)

	if !areResourcesSame(actual, expected) {
		t.Errorf("Expected %v but got %v", printResource(expected), printResource(actual))
	}
}

// 2
func TestResourcesNoNodeDefined(t *testing.T) {
	commonRequirements := buildResource(
		commonCPUValue,
		commonCPUValue,
		nodeMemValue,
		nodeMemValue,
	)

	nodeRequirements := v1.ResourceRequirements{}

	expected := buildResource(
		commonCPUValue,
		commonCPUValue,
		nodeMemValue,
		nodeMemValue,
	)

	actual := newESResourceRequirements(nodeRequirements, commonRequirements)

	if !areResourcesSame(actual, expected) {
		t.Errorf("Expected %v but got %v", printResource(expected), printResource(actual))
	}
}

// 3
func TestResourcesNoCommonNoNodeDefined(t *testing.T) {
	commonRequirements := v1.ResourceRequirements{}

	nodeRequirements := v1.ResourceRequirements{}

	expected := buildNoCPULimitResource(
		defaultTestCPURequest,
		defaultTestMemLimit,
		defaultTestMemRequest,
	)

	actual := newESResourceRequirements(nodeRequirements, commonRequirements)

	if !areResourcesSame(actual, expected) {
		t.Errorf("Expected %v but got %v", printResource(expected), printResource(actual))
	}
}

// 4
func TestResourcesCommonAndNodeRequestDefined(t *testing.T) {
	commonRequirements := buildResource(
		commonCPUValue,
		commonCPUValue,
		commonMemValue,
		commonMemValue,
	)

	nodeRequirements := buildResourceOnlyRequests(
		nodeCPUValue,
		nodeMemValue,
	)

	expected := buildResource(
		commonCPUValue,
		nodeCPUValue,
		commonMemValue,
		nodeMemValue,
	)

	actual := newESResourceRequirements(nodeRequirements, commonRequirements)

	if !areResourcesSame(actual, expected) {
		t.Errorf("Expected %v but got %v", printResource(expected), printResource(actual))
	}
}

// 5
func TestResourcesCommonAndNodeLimitDefined(t *testing.T) {
	commonRequirements := buildResource(
		commonCPUValue,
		commonCPUValue,
		commonMemValue,
		commonMemValue,
	)

	nodeRequirements := buildResourceOnlyLimits(
		nodeCPUValue,
		nodeMemValue,
	)

	expected := buildResource(
		nodeCPUValue,
		commonCPUValue,
		nodeMemValue,
		commonMemValue,
	)

	actual := newESResourceRequirements(nodeRequirements, commonRequirements)

	if !areResourcesSame(actual, expected) {
		t.Errorf("Expected %v but got %v", printResource(expected), printResource(actual))
	}
}

// 6
func TestResourcesCommonRequestAndNoNodeDefined(t *testing.T) {
	commonRequirements := buildResourceOnlyRequests(
		commonCPUValue,
		commonMemValue,
	)

	nodeRequirements := v1.ResourceRequirements{}

	expected := buildNoCPULimitResource(
		commonCPUValue,
		commonMemValue,
		commonMemValue,
	)

	actual := newESResourceRequirements(nodeRequirements, commonRequirements)

	if !areResourcesSame(actual, expected) {
		t.Errorf("Expected %v but got %v", printResource(expected), printResource(actual))
	}
}

// 7
func TestResourcesCommonLimitAndNoNodeDefined(t *testing.T) {
	commonRequirements := buildResourceOnlyLimits(
		commonCPUValue,
		commonMemValue,
	)

	nodeRequirements := v1.ResourceRequirements{}

	expected := buildResource(
		commonCPUValue,
		commonCPUValue,
		commonMemValue,
		commonMemValue,
	)

	actual := newESResourceRequirements(nodeRequirements, commonRequirements)

	if !areResourcesSame(actual, expected) {
		t.Errorf("Expected %v but got %v", printResource(expected), printResource(actual))
	}
}

// 8
func TestResourcesNoCommonAndNodeRequestDefined(t *testing.T) {
	commonRequirements := v1.ResourceRequirements{}

	nodeRequirements := buildResourceOnlyRequests(
		nodeCPUValue,
		nodeMemValue,
	)

	expected := buildNoCPULimitResource(
		nodeCPUValue,
		nodeMemValue,
		nodeMemValue,
	)

	actual := newESResourceRequirements(nodeRequirements, commonRequirements)

	if !areResourcesSame(actual, expected) {
		t.Errorf("Expected %v but got %v", printResource(expected), printResource(actual))
	}
}

// 9
func TestResourcesNoCommonAndNodeLimitDefined(t *testing.T) {
	commonRequirements := v1.ResourceRequirements{}

	nodeRequirements := buildResourceOnlyLimits(
		nodeCPUValue,
		nodeMemValue,
	)

	expected := buildResource(
		nodeCPUValue,
		nodeCPUValue,
		nodeMemValue,
		nodeMemValue,
	)

	actual := newESResourceRequirements(nodeRequirements, commonRequirements)

	if !areResourcesSame(actual, expected) {
		t.Errorf("Expected %v but got %v", printResource(expected), printResource(actual))
	}
}

// 10
func TestResourcesCommonLimitAndNodeResourceDefined(t *testing.T) {
	commonRequirements := buildResourceOnlyLimits(
		commonCPUValue,
		commonMemValue,
	)

	nodeRequirements := buildResourceOnlyRequests(
		nodeCPUValue,
		nodeMemValue,
	)

	expected := buildResource(
		commonCPUValue,
		nodeCPUValue,
		commonMemValue,
		nodeMemValue,
	)

	actual := newESResourceRequirements(nodeRequirements, commonRequirements)

	if !areResourcesSame(actual, expected) {
		t.Errorf("Expected %v but got %v", printResource(expected), printResource(actual))
	}
}

// 11
func TestResourcesCommonResourceAndNodeLimitDefined(t *testing.T) {
	commonRequirements := buildResourceOnlyRequests(
		commonCPUValue,
		commonMemValue,
	)

	nodeRequirements := buildResourceOnlyLimits(
		nodeCPUValue,
		nodeMemValue,
	)

	expected := buildResource(
		nodeCPUValue,
		commonCPUValue,
		nodeMemValue,
		commonMemValue,
	)

	actual := newESResourceRequirements(nodeRequirements, commonRequirements)

	if !areResourcesSame(actual, expected) {
		t.Errorf("Expected %v but got %v", printResource(expected), printResource(actual))
	}
}

func TestProxyContainerResourcesDefined(t *testing.T) {
	expectedCPU := resource.MustParse("100m")
	expectedMemory := resource.MustParse("256Mi")

	empty := v1.ResourceRequirements{}
	proxyResources := newESProxyResourceRequirements(empty, empty)

	if memoryLimit, ok := proxyResources.Limits["memory"]; ok {
		if memoryLimit.Cmp(expectedMemory) != 0 {
			t.Errorf("Expected Memory limit %s but got %s", expectedMemory.String(), memoryLimit.String())
		}
	} else {
		t.Errorf("Proxy container is missing Memory limit. Expected limit %s", expectedMemory.String())
	}

	if cpuRequest, ok := proxyResources.Requests["cpu"]; ok {
		if cpuRequest.Cmp(expectedCPU) != 0 {
			t.Errorf("Expected CPU request %s but got %s", expectedCPU.String(), cpuRequest.String())
		}
	} else {
		t.Errorf("Proxy container is missing CPU request. Expected request %s", expectedCPU.String())
	}

	if memoryRequest, ok := proxyResources.Requests["memory"]; ok {
		if memoryRequest.Cmp(expectedMemory) != 0 {
			t.Errorf("Expected memory request %s but got %s", expectedMemory.String(), memoryRequest.String())
		}
	} else {
		t.Errorf("Proxy container is missing memory request. Expected request %s", expectedMemory.String())
	}
}

func TestProxyContainerResourcesDefinedFromCommonSpec(t *testing.T) {
	empty := v1.ResourceRequirements{}

	expectedCPU := resource.MustParse("200m")
	expectedMemoryLimit := resource.MustParse("256Mi")
	expectedMemoryRequest := resource.MustParse("128Mi")

	nodeProxySpec := buildNoCPULimitResource(expectedCPU, expectedMemoryLimit, expectedMemoryRequest)
	proxyResources := newESProxyResourceRequirements(empty, nodeProxySpec)

	if memoryLimit, ok := proxyResources.Limits["memory"]; ok {
		if memoryLimit.Cmp(expectedMemoryLimit) != 0 {
			t.Errorf("Expected Memory limit %s but got %s", expectedMemoryLimit.String(), memoryLimit.String())
		}
	} else {
		t.Errorf("Proxy container is missing Memory limit. Expected limit %s", expectedMemoryLimit.String())
	}

	if cpuRequest, ok := proxyResources.Requests["cpu"]; ok {
		if cpuRequest.Cmp(expectedCPU) != 0 {
			t.Errorf("Expected CPU request %s but got %s", expectedCPU.String(), cpuRequest.String())
		}
	} else {
		t.Errorf("Proxy container is missing CPU request. Expected request %s", expectedCPU.String())
	}

	if memoryRequest, ok := proxyResources.Requests["memory"]; ok {
		if memoryRequest.Cmp(expectedMemoryRequest) != 0 {
			t.Errorf("Expected memory request %s but got %s", expectedMemoryRequest.String(), memoryRequest.String())
		}
	} else {
		t.Errorf("Proxy container is missing memory request. Expected request %s", expectedMemoryRequest.String())
	}
}

func TestProxyContainerResourcesDefinedFromNodeSpec(t *testing.T) {
	expectedCPU := resource.MustParse("150m")
	expectedMemoryLimit := resource.MustParse("200Mi")
	expectedMemoryRequest := resource.MustParse("100Mi")
	nodeProxySpec := buildNoCPULimitResource(expectedCPU, expectedMemoryLimit, expectedMemoryRequest)

	commonCPU := resource.MustParse("200m")
	commonMemoryLimit := resource.MustParse("256Mi")
	commonMemoryRequest := resource.MustParse("128Mi")
	commonProxySpec := buildNoCPULimitResource(commonCPU, commonMemoryLimit, commonMemoryRequest)

	proxyResources := newESProxyResourceRequirements(nodeProxySpec, commonProxySpec)

	if memoryLimit, ok := proxyResources.Limits["memory"]; ok {
		if memoryLimit.Cmp(expectedMemoryLimit) != 0 {
			t.Errorf("Expected Memory limit %s but got %s", expectedMemoryLimit.String(), memoryLimit.String())
		}
	} else {
		t.Errorf("Proxy container is missing Memory limit. Expected limit %s", expectedMemoryLimit.String())
	}

	if cpuRequest, ok := proxyResources.Requests["cpu"]; ok {
		if cpuRequest.Cmp(expectedCPU) != 0 {
			t.Errorf("Expected CPU request %s but got %s", expectedCPU.String(), cpuRequest.String())
		}
	} else {
		t.Errorf("Proxy container is missing CPU request. Expected request %s", expectedCPU.String())
	}

	if memoryRequest, ok := proxyResources.Requests["memory"]; ok {
		if memoryRequest.Cmp(expectedMemoryRequest) != 0 {
			t.Errorf("Expected memory request %s but got %s", expectedMemoryRequest.String(), memoryRequest.String())
		}
	} else {
		t.Errorf("Proxy container is missing memory request. Expected request %s", expectedMemoryRequest.String())
	}
}

func TestProxyContainerTLSClientAuthDefined(t *testing.T) {
	imageName := "openshift/elasticsearch-proxy:1.1"
	clusterName := "elasticsearch"

	empty := v1.ResourceRequirements{}
	proxyResources := newESProxyResourceRequirements(empty, empty)
	proxyContainer := newProxyContainer(imageName, clusterName, "openshift-logging", LogConfig{}, proxyResources)

	want := []string{
		"--tls-cert=/etc/proxy/elasticsearch/logging-es.crt",
		"--tls-key=/etc/proxy/elasticsearch/logging-es.key",
		"--tls-client-ca=/etc/proxy/elasticsearch/admin-ca",
	}

	for _, arg := range want {
		if !sliceContainsString(proxyContainer.Args, arg) {
			t.Errorf("Missing tls client auth argument: %s", arg)
		}
	}

	wantVolumeMount := v1.VolumeMount{Name: "certificates", MountPath: "/etc/proxy/elasticsearch"}

	hasMount := false
	for _, vm := range proxyContainer.VolumeMounts {
		if reflect.DeepEqual(vm, wantVolumeMount) {
			hasMount = true
		}
	}

	if !hasMount {
		t.Errorf("Missing volume mount for tls client auth PKI: %#v", wantVolumeMount)
	}
}

func TestProxyContainerSpec(t *testing.T) {
	imageName := "openshift/elasticsearch-proxy:1.1"
	clusterName := "elasticsearch"

	empty := v1.ResourceRequirements{}
	proxyResources := newESProxyResourceRequirements(empty, empty)
	proxyContainer := newProxyContainer(imageName, clusterName, "openshift-logging", LogConfig{}, proxyResources)

	wantArgs := []string{
		"--metrics-listening-address=:60001",
		"--metrics-tls-cert=/etc/proxy/secrets/tls.crt",
		"--metrics-tls-key=/etc/proxy/secrets/tls.key",
		"--auth-admin-role=admin_reader",
		"--auth-default-role=project_user",
	}

	for _, arg := range wantArgs {
		if !sliceContainsString(proxyContainer.Args, arg) {
			t.Errorf("Missing tls client auth argument: %s", arg)
		}
	}

	wantPort := v1.ContainerPort{
		Name:          "metrics",
		ContainerPort: 60001,
		Protocol:      v1.ProtocolTCP,
	}

	hasPort := false
	for _, port := range proxyContainer.Ports {
		if port.Name == wantPort.Name && port.ContainerPort == wantPort.ContainerPort && port.Protocol == wantPort.Protocol {
			hasPort = true
		}
	}
	if !hasPort {
		t.Errorf("Missing container port for tls metrics: %#v", wantPort)
	}

	wantVolumeMount := v1.VolumeMount{
		Name:      "elasticsearch-metrics",
		MountPath: "/etc/proxy/secrets",
	}

	hasMount := false
	for _, vm := range proxyContainer.VolumeMounts {
		if reflect.DeepEqual(vm, wantVolumeMount) {
			hasMount = true
		}
	}

	if !hasMount {
		t.Errorf("Missing volume mount for tls metrics PKI: %#v", wantVolumeMount)
	}
}

func TestPodSpecHasTaintTolerations(t *testing.T) {
	expectedTolerations := []v1.Toleration{
		{
			Key:      "node.kubernetes.io/disk-pressure",
			Operator: v1.TolerationOpExists,
			Effect:   v1.TaintEffectNoSchedule,
		},
	}

	podTemplateSpec := newPodTemplateSpec("test-node-name", "test-cluster-name", "test-namespace-name", api.ElasticsearchNode{}, api.ElasticsearchNodeSpec{}, map[string]string{}, map[api.ElasticsearchNodeRole]bool{}, nil, LogConfig{})

	if !reflect.DeepEqual(podTemplateSpec.Spec.Tolerations, expectedTolerations) {
		t.Errorf("Exp. the tolerations to be %v but was %v", expectedTolerations, podTemplateSpec.Spec.Tolerations)
	}
}

// All pods created by Elasticsearch operator needs to be allocated to linux nodes.
// See LOG-411
func TestPodNodeSelectors(t *testing.T) {
	var podSpec v1.PodSpec

	// Create podSpecTemplate providing nil/empty node selectors, we expect the PodTemplateSpec.Spec selectors
	// will contain only the linux allocation selector.
	podSpec = preparePodTemplateSpecProvidingNodeSelectors(nil).Spec

	if podSpec.NodeSelector == nil {
		t.Errorf("Exp. the nodeSelector to contains the linux allocation selector but was %T", podSpec.NodeSelector)
	}
	if len(podSpec.NodeSelector) != 1 {
		t.Errorf("Exp. single nodeSelector but %d were found", len(podSpec.NodeSelector))
	}
	if podSpec.NodeSelector[utils.OsNodeLabel] != utils.LinuxValue {
		t.Errorf("Exp. the nodeSelector to contains %s: %s pair", utils.OsNodeLabel, utils.LinuxValue)
	}

	// Create podSpecTemplate providing some custom node selectors, we expect the PodTemplateSpec.Spec selectors
	// will add linux node selector.
	podSpec = preparePodTemplateSpecProvidingNodeSelectors(map[string]string{"foo": "bar", "baz": "foo"}).Spec

	if podSpec.NodeSelector == nil {
		t.Errorf("Exp. the nodeSelector to contains the linux allocation selector but was %T", podSpec.NodeSelector)
	}
	if len(podSpec.NodeSelector) != 3 {
		t.Errorf("Exp. single nodeSelector but %d were found", len(podSpec.NodeSelector))
	}
	if podSpec.NodeSelector[utils.OsNodeLabel] != utils.LinuxValue {
		t.Errorf("Exp. the nodeSelector to contains %s: %s pair", utils.OsNodeLabel, utils.LinuxValue)
	}

	// Create podSpecTemplate providing node selector with some custom value, we expect the PodTemplateSpec.Spec selector
	// will override the node selector to linux one.
	podSpec = preparePodTemplateSpecProvidingNodeSelectors(map[string]string{utils.OsNodeLabel: "foo"}).Spec

	if podSpec.NodeSelector == nil {
		t.Errorf("Exp. the nodeSelector to contains the linux allocation selector but was %T", podSpec.NodeSelector)
	}
	if len(podSpec.NodeSelector) != 1 {
		t.Errorf("Exp. single nodeSelector but %d were found", len(podSpec.NodeSelector))
	}
	if podSpec.NodeSelector[utils.OsNodeLabel] != utils.LinuxValue {
		t.Errorf("Exp. the nodeSelector to contains %s: %s pair", utils.OsNodeLabel, utils.LinuxValue)
	}
}

func TestPodDiskToleration(t *testing.T) {
	expectedToleration := []v1.Toleration{
		{
			Key:      "node.kubernetes.io/disk-pressure",
			Operator: v1.TolerationOpExists,
			Effect:   v1.TaintEffectNoSchedule,
		},
	}

	// Create podSpecTemplate providing nil/empty tolerations, we expect the PodTemplateSpec.Spec tolerations
	// will contain only the disk usage toleration
	podSpec := preparePodTemplateSpecProvidingNodeSelectors(nil).Spec

	if podSpec.Tolerations == nil {
		t.Errorf("Exp. the toleration to contains the disk usage toleration but was %T", podSpec.Tolerations)
	}
	if len(podSpec.Tolerations) != 1 {
		t.Errorf("Exp. single toleration but %d were found", len(podSpec.Tolerations))
	}
	if !areTolerationsSame(podSpec.Tolerations, expectedToleration) {
		t.Errorf("Exp. the tolerations to contain %v", expectedToleration)
	}
}

func TestNewVolumeSource(t *testing.T) {
	const (
		clusterName = "elastisearch"
		nodeName    = "elasticsearch-cdm-1"
		namespace   = "openshift-logging"
	)

	var (
		claimName   = fmt.Sprintf("%s-%s", clusterName, nodeName)
		gp2SCName   = "gp2"
		storageSize = resource.MustParse("2Gi")
	)

	tests := []struct {
		desc string
		node api.ElasticsearchNode
		vs   v1.VolumeSource
		pvc  *v1.PersistentVolumeClaim
	}{
		{
			desc: "ephemeral storage on empty storage spec",
			node: api.ElasticsearchNode{},
			vs: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		},
		{
			desc: "persistent storage with default storage class",
			node: api.ElasticsearchNode{
				Storage: api.ElasticsearchStorageSpec{
					Size: &storageSize,
				},
			},
			vs: v1.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
					ClaimName: claimName,
				},
			},
			pvc: &v1.PersistentVolumeClaim{
				TypeMeta: metav1.TypeMeta{
					Kind:       "PersistentVolumeClaim",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      claimName,
					Namespace: namespace,
					Labels: map[string]string{
						"logging-cluster": clusterName,
					},
					ResourceVersion: "1",
				},
				Spec: v1.PersistentVolumeClaimSpec{
					AccessModes: []v1.PersistentVolumeAccessMode{
						v1.ReadWriteOnce,
					},
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceStorage: storageSize,
						},
					},
				},
			},
		},
		{
			desc: "persistent storage with custom storage class",
			node: api.ElasticsearchNode{
				Storage: api.ElasticsearchStorageSpec{
					StorageClassName: &gp2SCName,
					Size:             &storageSize,
				},
			},
			vs: v1.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
					ClaimName: claimName,
				},
			},
			pvc: &v1.PersistentVolumeClaim{
				TypeMeta: metav1.TypeMeta{
					Kind:       "PersistentVolumeClaim",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      claimName,
					Namespace: namespace,
					Labels: map[string]string{
						"logging-cluster": clusterName,
					},
					ResourceVersion: "1",
				},
				Spec: v1.PersistentVolumeClaimSpec{
					AccessModes: []v1.PersistentVolumeAccessMode{
						v1.ReadWriteOnce,
					},
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceStorage: storageSize,
						},
					},
					StorageClassName: &gp2SCName,
				},
			},
		},
		{
			desc: "persistent storage without storage size",
			node: api.ElasticsearchNode{
				Storage: api.ElasticsearchStorageSpec{
					StorageClassName: &gp2SCName,
				},
			},
			vs: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		},
	}
	for _, test := range tests {
		test := test
		client := fake.NewFakeClient()

		vs := newVolumeSource(clusterName, nodeName, namespace, test.node, client)
		if diff := cmp.Diff(test.vs, vs); diff != "" {
			t.Errorf("diff: %s", diff)
		}

		if test.pvc != nil {
			key := types.NamespacedName{Name: claimName, Namespace: namespace}
			pvc := &v1.PersistentVolumeClaim{}
			if err := client.Get(context.TODO(), key, pvc); err != nil {
				t.Errorf("got err: %s, want nil", err)
			}

			if diff := cmp.Diff(test.pvc, pvc); diff != "" {
				t.Errorf("diff: %s", diff)
			}
		}
	}
}

// Return a fresh new PodTemplateSpec using provided node selectors.
// Resulting selectors set always contains also the node selector with value of "linux", see LOG-411
// This function wraps the call to newPodTempalteSpec in case its signature changes in the future
// so that keeping unit tests up to date will be easier.
func preparePodTemplateSpecProvidingNodeSelectors(selectors map[string]string) v1.PodTemplateSpec {
	return newPodTemplateSpec(
		"test-node-name",
		"test-cluster-name",
		"test-namespace-name",
		api.ElasticsearchNode{NodeSelector: selectors},
		api.ElasticsearchNodeSpec{},
		map[string]string{},
		map[api.ElasticsearchNodeRole]bool{},
		nil,
		LogConfig{})
}

func buildResource(cpuLimit, cpuRequest, memLimit, memRequest resource.Quantity) v1.ResourceRequirements {
	return v1.ResourceRequirements{
		Limits: v1.ResourceList{
			v1.ResourceCPU:    cpuLimit,
			v1.ResourceMemory: memLimit,
		},
		Requests: v1.ResourceList{
			v1.ResourceCPU:    cpuRequest,
			v1.ResourceMemory: memRequest,
		},
	}
}

func buildResourceOnlyRequests(cpuRequest, memRequest resource.Quantity) v1.ResourceRequirements {
	return v1.ResourceRequirements{
		Requests: v1.ResourceList{
			v1.ResourceCPU:    cpuRequest,
			v1.ResourceMemory: memRequest,
		},
	}
}

func buildResourceOnlyLimits(cpuLimit, memLimit resource.Quantity) v1.ResourceRequirements {
	return v1.ResourceRequirements{
		Limits: v1.ResourceList{
			v1.ResourceCPU:    cpuLimit,
			v1.ResourceMemory: memLimit,
		},
	}
}

func buildNoCPULimitResource(cpuRequest, memLimit, memRequest resource.Quantity) v1.ResourceRequirements {
	return v1.ResourceRequirements{
		Limits: v1.ResourceList{
			v1.ResourceMemory: memLimit,
		},
		Requests: v1.ResourceList{
			v1.ResourceCPU:    cpuRequest,
			v1.ResourceMemory: memRequest,
		},
	}
}

func areResourcesSame(lhs, rhs v1.ResourceRequirements) bool {
	return reflect.DeepEqual(lhs, rhs)
}

func printResource(resource v1.ResourceRequirements) string {
	pretty, err := json.MarshalIndent(resource, "", "  ")
	if err != nil {
		fmt.Printf("Error marshalling to json: %v", pretty)
	}
	return string(pretty)
}

var _ = Describe("common.go", func() {
	defer GinkgoRecover()

	Describe("#newEnvVars", func() {
		var envVars []v1.EnvVar
		BeforeEach(func() {
			envVars = newEnvVars("theNodeName", "theClusterName", "theInstanceRam", map[api.ElasticsearchNodeRole]bool{})
		})

		It("should define POD_IP so IPV4 or IPV6 deployments are possible", func() {
			helpers.ExpectEnvVars(envVars).ToIncludeName("POD_IP").WithFieldRefPath("status.podIP")
		})
	})
})
