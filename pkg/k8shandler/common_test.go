package k8shandler

import (
	"fmt"
	"reflect"
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	api "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
)

var (
	commonCPUValue = resource.MustParse("500m")
	commonMemValue = resource.MustParse("2Gi")

	nodeCPUValue = resource.MustParse("600m")
	nodeMemValue = resource.MustParse("3Gi")

	defaultTestCPULimit   = resource.MustParse("1")
	defaultTestCPURequest = resource.MustParse(defaultCPURequest)
	defaultTestMemLimit   = resource.MustParse(defaultMemoryLimit)
	defaultTestMemRequest = resource.MustParse(defaultMemoryRequest)
)

/*
  Resource scenarios:
  1. common has no settings
     node has no settings
     -> use defaults

  2. common resource requirements are set
     node has no settings
     -> use common settings

  3. common has no settings
     node resource requirements are set
     -> use node resource requirements

  4. common resource requirements are set
     node resource requirements are set
	 -> merge common and node, when values colide, node value takes precedence

  5. common resource requirements are set
     node resource requirements are partialy set
	 -> merge common and node, when values colide, node value takes precedence
*/

// 1
func TestResourcesNoCommonNoNodeDefined(t *testing.T) {

	commonRequirements := v1.ResourceRequirements{}

	nodeRequirements := v1.ResourceRequirements{}

	expected := buildResourceNoCPULimits(
		defaultTestCPURequest,
		defaultTestMemLimit,
		defaultTestMemRequest,
	)

	actual := newResourceRequirements(&nodeRequirements, &commonRequirements)

	if !areResourcesSame(*actual, expected) {
		t.Errorf("Expected:\n%v\nBut got:\n%v", resourcesToString(&expected), resourcesToString(actual))
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

	actual := newResourceRequirements(&nodeRequirements, &commonRequirements)

	if !areResourcesSame(*actual, expected) {
		t.Errorf("Expected:\n%v\nBut got:\n%v", resourcesToString(&expected), resourcesToString(actual))
	}
}

// 3
func TestResourcesNoCommonDefined(t *testing.T) {

	commonRequirements := v1.ResourceRequirements{}

	nodeRequirements := buildResourceOnlyRequests(
		nodeCPUValue,
		nodeMemValue,
	)

	expected := buildResourceNoCPULimits(
		nodeCPUValue,
		defaultTestMemLimit,
		nodeMemValue,
	)

	actual := newResourceRequirements(&nodeRequirements, &commonRequirements)

	if val, ok := actual.Limits[v1.ResourceCPU]; ok {
		t.Errorf("No CPU limit expected, but got %s", val.String())
	}

	if !areResourcesSame(*actual, expected) {
		t.Errorf("Expected:\n%v\nBut got:\n%v", resourcesToString(&expected), resourcesToString(actual))
	}

	// check that changes don't propagate to the original map
	// deep copy is expected
	if _, ok := nodeRequirements.Limits[v1.ResourceCPU]; ok {
		t.Error("Original NodeRequirements map modified")
	}
}

// 4
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

	actual := newResourceRequirements(&nodeRequirements, &commonRequirements)

	if !areResourcesSame(*actual, expected) {
		t.Errorf("Expected:\n%v\nBut got:\n%v", resourcesToString(&expected), resourcesToString(actual))
	}
}

// 5
func TestResourcesRequestsNodeOverride(t *testing.T) {

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

	actual := newResourceRequirements(&nodeRequirements, &commonRequirements)

	if !areResourcesSame(*actual, expected) {
		t.Errorf("Expected:\n%v\nBut got:\n%v", resourcesToString(&expected), resourcesToString(actual))
	}
}

func TestProxyContainerResourcesDefined(t *testing.T) {

	imageName := "openshift/oauth-proxy:1.1"
	clusterName := "elasticsearch"

	expectedCPU := resource.MustParse("100m")
	expectedMemory := resource.MustParse("64Mi")

	proxyContainer, err := newProxyContainer(imageName, clusterName)
	if err != nil {
		t.Errorf("Failed to populate Proxy container: %v", err)
	}

	if memoryLimit, ok := proxyContainer.Resources.Limits["memory"]; ok {
		if memoryLimit.Cmp(expectedMemory) != 0 {
			t.Errorf("Expected CPU limit %s but got %s", expectedMemory.String(), memoryLimit.String())
		}
	} else {
		t.Errorf("Proxy container is missing CPU limit. Expected limit %s", expectedMemory.String())
	}

	if cpuRequest, ok := proxyContainer.Resources.Requests["cpu"]; ok {
		if cpuRequest.Cmp(expectedCPU) != 0 {
			t.Errorf("Expected CPU request %s but got %s", expectedCPU.String(), cpuRequest.String())
		}
	} else {
		t.Errorf("Proxy container is missing CPU request. Expected request %s", expectedCPU.String())
	}

	if memoryLimit, ok := proxyContainer.Resources.Limits["memory"]; ok {
		if memoryLimit.Cmp(expectedMemory) != 0 {
			t.Errorf("Expected memory limit %s but got %s", expectedMemory.String(), memoryLimit.String())
		}
	} else {
		t.Errorf("Proxy container is missing memory limit. Expected limit %s", expectedMemory.String())
	}
}

func TestPodSpecHasTaintTolerations(t *testing.T) {

	expectedTolerations := []v1.Toleration{
		v1.Toleration{
			Key:      "node.kubernetes.io/disk-pressure",
			Operator: v1.TolerationOpExists,
			Effect:   v1.TaintEffectNoSchedule,
		},
	}

	podTemplateSpec := newPodTemplateSpec("test-node-name", "test-cluster-name", "test-namespace-name", api.ElasticsearchNode{}, api.ElasticsearchNodeSpec{}, map[string]string{}, map[api.ElasticsearchNodeRole]bool{}, nil)

	if !reflect.DeepEqual(podTemplateSpec.Spec.Tolerations, expectedTolerations) {
		t.Errorf("Exp. the tolerations to be %q but was %q", expectedTolerations, podTemplateSpec.Spec.Tolerations)
	}
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

func buildResourceNoCPULimits(cpuRequest, memLimit, memRequest resource.Quantity) v1.ResourceRequirements {
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

	if !areQuantitiesSame(lhs.Limits.Cpu(), rhs.Limits.Cpu()) {
		return false
	}

	if !areQuantitiesSame(lhs.Requests.Cpu(), rhs.Requests.Cpu()) {
		return false
	}

	if !areQuantitiesSame(lhs.Limits.Memory(), rhs.Limits.Memory()) {
		return false
	}

	if !areQuantitiesSame(lhs.Requests.Memory(), rhs.Requests.Memory()) {
		return false
	}

	return true
}

func areQuantitiesSame(lhs, rhs *resource.Quantity) bool {
	return lhs.Cmp(*rhs) == 0
}

func resourcesToString(r *v1.ResourceRequirements) string {
	return fmt.Sprintf("limits:\n  cpu: %s\n  memory: %s\nrequests:\n  cpu: %s\n  memory: %s\n",
		r.Limits.Cpu().String(),
		r.Limits.Memory().String(),
		r.Requests.Cpu().String(),
		r.Requests.Memory().String(),
	)
}
