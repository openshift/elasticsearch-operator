package elasticsearch_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"

	"github.com/openshift/elasticsearch-operator/pkg/k8shandler/elasticsearch"
)

const nodeName = "testNode"

var _ = Describe("UpdatePodTemplateSpec", func() {
	defer GinkgoRecover()

	var (
		actual = &v1.PodTemplateSpec{
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					v1.Container{},
				},
			},
		}
		desired = &v1.PodTemplateSpec{
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					v1.Container{},
				},
			},
		}
	)
	BeforeEach(func() {
	})
	Context("when evaluating containers", func() {

		Context("and their args", func() {

			BeforeEach(func() {
				actual.Spec.Containers[0].Args = []string{"abc", "123"}
				desired.Spec.Containers[0].Args = actual.Spec.Containers[0].Args
			})

			It("should indicate changed when actual does not equal desired", func() {
				desired.Spec.Containers[0].Args = []string{"change", "me"}
				Expect(elasticsearch.UpdatePodTemplateSpec(nodeName, actual, desired)).To(BeTrue())
				Expect(actual.Spec.Containers[0].Args).To(Equal(actual.Spec.Containers[0].Args))
			})

			It("should indicate no change when actual equal desired", func() {
				exp := actual.Spec.Containers[0].Args
				Expect(elasticsearch.UpdatePodTemplateSpec(nodeName, actual, desired)).To(BeFalse())
				Expect(actual.Spec.Containers[0].Args).To(Equal(exp))
			})

		})
	})
})
