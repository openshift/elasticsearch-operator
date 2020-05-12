package k8shandler

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	loggingv1 "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
)

var (
	deployment                      apps.Deployment
	nodeContainer, desiredContainer v1.Container
	node                            *deploymentNode
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
	node = &deploymentNode{
		self: deployment,
	}
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

	actual, changed := updateResources(node, nodeContainer, desiredContainer)

	if !changed {
		t.Error("Expected updating the resources would recognized as changed, but it was not")
	}
	if !areResourcesSame(actual.Resources, desiredContainer.Resources) {
		t.Errorf("Expected %v but got %v", printResource(desiredContainer.Resources), printResource(actual.Resources))
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
	actual, changed := updateResources(node, nodeContainer, desiredContainer)

	if !changed {
		t.Error("Expected updating the resources would recognized as changed, but it was not")
	}
	if !areResourcesSame(actual.Resources, desiredContainer.Resources) {
		t.Errorf("Expected %v but got %v", printResource(desiredContainer.Resources), printResource(actual.Resources))
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

	actual, changed := updateResources(node, nodeContainer, desiredContainer)

	if !changed {
		t.Error("Expected updating the resources would recognized as changed, but it was not")
	}
	if !areResourcesSame(actual.Resources, desiredContainer.Resources) {
		t.Errorf("Expected %v but got %v", printResource(desiredContainer.Resources), printResource(actual.Resources))
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
	actual, changed := updateResources(node, nodeContainer, desiredContainer)

	if !changed {
		t.Error("Expected updating the resources would recognized as changed, but it was not")
	}
	if !areResourcesSame(actual.Resources, desiredContainer.Resources) {
		t.Errorf("Expected %v but got %v", printResource(desiredContainer.Resources), printResource(actual.Resources))
	}
}

var _ = Describe("deployment", func() {
	defer GinkgoRecover()

	var (
		current *deploymentNode = &deploymentNode{
			self: apps.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "aName",
					Namespace: "aNamespace",
				},
				Spec: apps.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								v1.Container{},
							},
						},
					},
				},
			},
		}
		client = fake.NewFakeClient(&current.self)

		elasticsearch = newElasticsearchContainer("someImage",
			newEnvVars("mynodename", "clustername", "", map[loggingv1.ElasticsearchNodeRole]bool{}),
			v1.ResourceRequirements{
				Limits: v1.ResourceList{},
			})

		newDesired = func(elasticsearch v1.Container) *deploymentNode {
			return &deploymentNode{
				client: client,
				self: apps.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      current.self.Name,
						Namespace: current.self.Namespace,
					},
					Spec: apps.DeploymentSpec{
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									elasticsearch,
								},
							},
						},
					},
				},
			}
		}

		desired = newDesired(elasticsearch)
	)

	Context("isChanged()", func() {

		//strange nameing in the method IMO.  The object to be updated is the node
		//which in this case is "desired" since the spec is loaded from current and then
		//reset to the value from "desired" if appropriate
		It("should recognize container EnvVars when they change", func() {
			Expect(desired.isChanged()).To(BeTrue())
			Expect(desired.self.Spec.Template.Spec.Containers[0].Env).To(Equal(elasticsearch.Env))
		})

		It("should recognize container args when they change", func() {
			elasticsearch.Args = []string{"list", "of", "args"}
			desired = newDesired(elasticsearch)
			Expect(desired.isChanged()).To(BeTrue())
			Expect(desired.self.Spec.Template.Spec.Containers[0].Args).To(Equal(elasticsearch.Args))
		})

	})
})
