package k8shandler

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	loggingv1 "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var nodeContainer v1.Container

var _ = Describe("deployment", func() {
	defer GinkgoRecover()

	var (
		current = &deploymentNode{
			self: apps.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "aName",
					Namespace: "aNamespace",
				},
				Spec: apps.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{},
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
		// strange nameing in the method IMO.  The object to be updated is the node
		// which in this case is "desired" since the spec is loaded from current and then
		// reset to the value from "desired" if appropriate
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
		It("should recognize container ports when they change", func() {
			elasticsearch.Ports = []v1.ContainerPort{
				{
					Name:          "someotherport",
					ContainerPort: 60000,
					Protocol:      v1.ProtocolTCP,
				},
			}
			desired = newDesired(elasticsearch)
			Expect(desired.isChanged()).To(BeTrue())
			Expect(desired.self.Spec.Template.Spec.Containers[0].Ports).To(Equal(elasticsearch.Ports))
		})
	})
})
