package elasticsearch_test

import (
	"testing"

	"github.com/openshift/elasticsearch-operator/pkg/k8shandler/elasticsearch"
	"github.com/openshift/elasticsearch-operator/pkg/utils/comparators"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/elasticsearch-operator/test/utils"
)

var (
	deployment                      apps.Deployment
	nodeContainer, desiredContainer v1.Container
	node                            string
)

func setUp() {
	nodeContainer = v1.Container{
		Resources: v1.ResourceRequirements{
			Limits: v1.ResourceList{
				v1.ResourceMemory: resource.MustParse("2Gi"),
				v1.ResourceCPU:    resource.MustParse("600m"),
			},
			Requests: v1.ResourceList{
				v1.ResourceMemory: resource.MustParse("2Gi"),
				v1.ResourceCPU:    resource.MustParse("600m"),
			},
		},
	}

	desiredContainer = v1.Container{
		Resources: v1.ResourceRequirements{
			Limits:   v1.ResourceList{},
			Requests: v1.ResourceList{},
		},
	}
	deployment = apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: apps.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
				},
			},
		},
	}
	node = "anesnode"
}

func TestUpdateResourcesWhenDesiredCPULimitIsZero(t *testing.T) {
	setUp()
	desiredContainer.Resources.Limits = v1.ResourceList{
		v1.ResourceMemory: resource.MustParse("2Gi"),
	}

	desiredContainer.Resources.Requests = v1.ResourceList{
		v1.ResourceMemory: resource.MustParse("2Gi"),
		v1.ResourceCPU:    resource.MustParse("600m"),
	}

	actual, changed := elasticsearch.UpdateResources(node, nodeContainer, desiredContainer)

	if !changed {
		t.Error("Expected updating the resources would recognized as changed, but it was not")
	}
	if !comparators.AreResourceRequementsSame(actual.Resources, desiredContainer.Resources) {
		t.Errorf("Expected %v but got %v", utils.PrintResource(desiredContainer.Resources), utils.PrintResource(actual.Resources))
	}
}
func TestUpdateResourcesWhenDesiredMemoryLimitIsZero(t *testing.T) {
	setUp()
	desiredContainer.Resources.Limits = v1.ResourceList{
		v1.ResourceCPU: resource.MustParse("600m"),
	}

	desiredContainer.Resources.Requests = v1.ResourceList{
		v1.ResourceMemory: resource.MustParse("2Gi"),
		v1.ResourceCPU:    resource.MustParse("600m"),
	}
	actual, changed := elasticsearch.UpdateResources(node, nodeContainer, desiredContainer)

	if !changed {
		t.Error("Expected updating the resources would recognized as changed, but it was not")
	}
	if !comparators.AreResourceRequementsSame(actual.Resources, desiredContainer.Resources) {
		t.Errorf("Expected %v but got %v", utils.PrintResource(desiredContainer.Resources), utils.PrintResource(actual.Resources))
	}
}
func TestUpdateResourcesWhenDesiredCPURequestIsZero(t *testing.T) {
	setUp()
	desiredContainer.Resources.Limits = v1.ResourceList{
		v1.ResourceMemory: resource.MustParse("2Gi"),
		v1.ResourceCPU:    resource.MustParse("600m"),
	}

	desiredContainer.Resources.Requests = v1.ResourceList{
		v1.ResourceMemory: resource.MustParse("2Gi"),
	}

	actual, changed := elasticsearch.UpdateResources(node, nodeContainer, desiredContainer)

	if !changed {
		t.Error("Expected updating the resources would recognized as changed, but it was not")
	}
	if !comparators.AreResourceRequementsSame(actual.Resources, desiredContainer.Resources) {
		t.Errorf("Expected %v but got %v", utils.PrintResource(desiredContainer.Resources), utils.PrintResource(actual.Resources))
	}
}
func TestUpdateResourcesWhenDesiredMemoryRequestIsZero(t *testing.T) {
	setUp()
	desiredContainer.Resources.Limits = v1.ResourceList{
		v1.ResourceCPU:    resource.MustParse("600m"),
		v1.ResourceMemory: resource.MustParse("2Gi"),
	}

	desiredContainer.Resources.Requests = v1.ResourceList{
		v1.ResourceCPU: resource.MustParse("600m"),
	}
	actual, changed := elasticsearch.UpdateResources(node, nodeContainer, desiredContainer)

	if !changed {
		t.Error("Expected updating the resources would recognized as changed, but it was not")
	}
	if !comparators.AreResourceRequementsSame(actual.Resources, desiredContainer.Resources) {
		t.Errorf("Expected %v but got %v", utils.PrintResource(desiredContainer.Resources), utils.PrintResource(actual.Resources))
	}
}
