package k8shandler

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	lhs, rhs                                              v1.PodTemplateSpec
	emptyVolume, configmapVolume, secretVolume, pvcVolume v1.Volume
)

const (
	expectedImageName  = "testImage"
	differentImageName = "testImage2"
)

var _ = Describe("podtemplate", func() {
	defer GinkgoRecover()

	BeforeEach(func() {
		nodeContainer = v1.Container{
			Name: "testContainer",
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
			Image: expectedImageName,
		}

		emptyVolume = v1.Volume{
			Name: "emptyVolume",
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		}

		configmapVolume = v1.Volume{
			Name: "configmapVolume",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{},
			},
		}

		secretVolume = v1.Volume{
			Name: "secretVolume",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: "secret",
				},
			},
		}

		pvcVolume = v1.Volume{
			Name: "pvcVolume",
			VolumeSource: v1.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
					ClaimName: "pvc",
				},
			},
		}

		lhs = v1.PodTemplateSpec{
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					nodeContainer,
				},
			},
		}
	})

	Context("no change", func() {
		JustBeforeEach(func() {
			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
				},
			}
		})

		It("should recognize podtemplates as the same", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeFalse())
		})
	})

	Context("different image", func() {
		JustBeforeEach(func() {
			nodeContainer.Image = differentImageName

			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
				},
			}
		})

		It("should recognize an image name change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeTrue())
		})
	})

	Context("different nodeselector", func() {
		JustBeforeEach(func() {
			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
					NodeSelector: map[string]string{
						"key": "value",
					},
				},
			}
		})

		It("should recognize a nodeSelector change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeTrue())
		})
	})

	Context("different emptyDir volumes declared", func() {
		JustBeforeEach(func() {
			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
					Volumes: []v1.Volume{
						emptyVolume,
					},
				},
			}
		})

		It("shouldn't recognize a volume change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeFalse())
		})
	})

	Context("different emptyDir volumes sizes declared", func() {
		JustBeforeEach(func() {
			lhs.Spec.Volumes = []v1.Volume{
				emptyVolume,
			}

			size := resource.MustParse("10G")
			emptyVolume2 := emptyVolume.DeepCopy()

			emptyVolume2.EmptyDir.SizeLimit = &size

			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
					Volumes: []v1.Volume{
						*emptyVolume2,
					},
				},
			}
		})

		It("shouldn't recognize a volume change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeFalse())
		})
	})

	Context("different emptyDir volumes sizes", func() {
		JustBeforeEach(func() {
			size := resource.MustParse("10G")
			emptyVolume.EmptyDir.SizeLimit = &size

			lhs.Spec.Volumes = []v1.Volume{
				emptyVolume,
			}

			size2 := resource.MustParse("20G")
			emptyVolume2 := emptyVolume.DeepCopy()
			emptyVolume2.EmptyDir.SizeLimit = &size2

			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
					Volumes: []v1.Volume{
						*emptyVolume2,
					},
				},
			}
		})

		It("shouldn't recognize a volume change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeFalse())
		})
	})

	Context("different configmap volumes declared", func() {
		JustBeforeEach(func() {
			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
					Volumes: []v1.Volume{
						configmapVolume,
					},
				},
			}
		})

		It("shouldn't recognize a volume change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeFalse())
		})
	})

	Context("different configmap volumes names", func() {
		JustBeforeEach(func() {
			configmapVolume.ConfigMap.Name = "configmap"
			lhs.Spec.Volumes = []v1.Volume{
				configmapVolume,
			}

			configmapVolume2 := configmapVolume.DeepCopy()
			configmapVolume2.ConfigMap.Name = "configmap2"

			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
					Volumes: []v1.Volume{
						*configmapVolume2,
					},
				},
			}
		})

		It("shouldn't recognize a volume change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeFalse())
		})
	})

	Context("different secret volumes declared", func() {
		JustBeforeEach(func() {
			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
					Volumes: []v1.Volume{
						secretVolume,
					},
				},
			}
		})

		It("shouldn't recognize a volume change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeFalse())
		})
	})

	Context("different secret volumes names", func() {
		JustBeforeEach(func() {
			lhs.Spec.Volumes = []v1.Volume{
				secretVolume,
			}

			secretVolume2 := secretVolume.DeepCopy()
			secretVolume2.Secret.SecretName = "secret2"

			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
					Volumes: []v1.Volume{
						*secretVolume2,
					},
				},
			}
		})

		It("shouldn't recognize a volume change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeFalse())
		})
	})

	Context("different pvc volumes declared", func() {
		JustBeforeEach(func() {
			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
					Volumes: []v1.Volume{
						pvcVolume,
					},
				},
			}
		})

		It("shouldn't recognize a volume change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeFalse())
		})
	})

	Context("different pvc volumes names", func() {
		JustBeforeEach(func() {
			lhs.Spec.Volumes = []v1.Volume{
				pvcVolume,
			}

			pvcVolume2 := pvcVolume.DeepCopy()
			pvcVolume2.PersistentVolumeClaim.ClaimName = "pvc2"

			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
					Volumes: []v1.Volume{
						*pvcVolume2,
					},
				},
			}
		})

		It("shouldn't recognize a volume change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeFalse())
		})
	})

	Context("different toleration", func() {
		JustBeforeEach(func() {
			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
					Tolerations: []v1.Toleration{
						{
							Key:      "key",
							Operator: v1.TolerationOpEqual,
							Value:    "value",
						},
					},
				},
			}
		})

		It("should recognize a toleration change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeTrue())
		})
	})

	Context("different env var literal", func() {
		JustBeforeEach(func() {
			envVar := v1.EnvVar{
				Name:  "testVar",
				Value: "testValue",
			}
			nodeContainer.Env = append(nodeContainer.Env, envVar)

			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
				},
			}
		})

		It("should recognize an env var literal change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeTrue())
		})
	})

	Context("different env var fromValue", func() {
		JustBeforeEach(func() {
			envVar := v1.EnvVar{
				Name: "testVar",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			}
			nodeContainer.Env = append(nodeContainer.Env, envVar)

			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
				},
			}
		})

		It("should recognize an env var fromValue change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeTrue())
		})
	})

	Context("different args", func() {
		JustBeforeEach(func() {
			nodeContainer.Args = []string{"this", "is", "a", "test"}

			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
				},
			}
		})

		It("should recognize an args change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeTrue())
		})
	})

	Context("different volumemounts", func() {
		JustBeforeEach(func() {
			nodeContainer.VolumeMounts = []v1.VolumeMount{
				{
					Name:      "testMount",
					ReadOnly:  true,
					MountPath: "/dev/random",
				},
			}

			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
				},
			}
		})

		It("should recognize an volumemounts change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeTrue())
		})
	})

	Context("different ports", func() {
		JustBeforeEach(func() {
			nodeContainer.Ports = []v1.ContainerPort{
				{
					Name:          "testPort",
					ContainerPort: 9200,
					HostPort:      9200,
					Protocol:      v1.ProtocolTCP,
				},
			}

			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
				},
			}
		})

		It("should recognize a ports change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeTrue())
		})
	})

	Context("cpu limit zero", func() {
		JustBeforeEach(func() {
			nodeContainer.Resources.Limits = v1.ResourceList{
				v1.ResourceMemory: resource.MustParse("2Gi"),
			}

			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
				},
			}
		})

		It("should recognize a cpu limits change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeTrue())
		})
	})

	Context("cpu request zero", func() {
		JustBeforeEach(func() {
			nodeContainer.Resources.Requests = v1.ResourceList{
				v1.ResourceMemory: resource.MustParse("2Gi"),
			}

			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
				},
			}
		})

		It("should recognize a cpu requests change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeTrue())
		})
	})

	Context("cpu limit changed", func() {
		JustBeforeEach(func() {
			nodeContainer.Resources.Limits = v1.ResourceList{
				v1.ResourceMemory: resource.MustParse("2Gi"),
				v1.ResourceCPU:    resource.MustParse("500m"),
			}

			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
				},
			}
		})

		It("should recognize a cpu limits change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeTrue())
		})
	})

	Context("cpu request changed", func() {
		JustBeforeEach(func() {
			nodeContainer.Resources.Requests = v1.ResourceList{
				v1.ResourceMemory: resource.MustParse("2Gi"),
				v1.ResourceCPU:    resource.MustParse("500m"),
			}

			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
				},
			}
		})

		It("should recognize a cpu requests change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeTrue())
		})
	})

	Context("memory limit zero", func() {
		JustBeforeEach(func() {
			nodeContainer.Resources.Limits = v1.ResourceList{
				v1.ResourceCPU: resource.MustParse("600m"),
			}

			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
				},
			}
		})

		It("should recognize a memory limits change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeTrue())
		})
	})

	Context("memory request zero", func() {
		JustBeforeEach(func() {
			nodeContainer.Resources.Requests = v1.ResourceList{
				v1.ResourceCPU: resource.MustParse("600m"),
			}

			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
				},
			}
		})

		It("should recognize a memory requests change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeTrue())
		})
	})

	Context("memory limit changed", func() {
		JustBeforeEach(func() {
			nodeContainer.Resources.Limits = v1.ResourceList{
				v1.ResourceMemory: resource.MustParse("3Gi"),
				v1.ResourceCPU:    resource.MustParse("600m"),
			}

			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
				},
			}
		})

		It("should recognize a memory limits change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeTrue())
		})
	})

	Context("memory request changed", func() {
		JustBeforeEach(func() {
			nodeContainer.Resources.Requests = v1.ResourceList{
				v1.ResourceMemory: resource.MustParse("3Gi"),
				v1.ResourceCPU:    resource.MustParse("600m"),
			}

			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
					},
				},
			}
		})

		It("should recognize a memory requests change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeTrue())
		})
	})

	Context("proxy memory request resource change", func() {
		JustBeforeEach(func() {
			proxyContainer := v1.Container{
				Name: "testProxyContainer",
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceMemory: resource.MustParse("1Gi"),
						v1.ResourceCPU:    resource.MustParse("600m"),
					},
					Requests: v1.ResourceList{
						v1.ResourceMemory: resource.MustParse("1Gi"),
						v1.ResourceCPU:    resource.MustParse("600m"),
					},
				},
				Image: expectedImageName,
			}

			lhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
						proxyContainer,
					},
				},
			}

			proxyContainer2 := v1.Container{
				Name: "testProxyContainer",
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceMemory: resource.MustParse("1Gi"),
						v1.ResourceCPU:    resource.MustParse("600m"),
					},
					Requests: v1.ResourceList{
						v1.ResourceMemory: resource.MustParse("2Gi"),
						v1.ResourceCPU:    resource.MustParse("600m"),
					},
				},
				Image: expectedImageName,
			}

			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
						proxyContainer2,
					},
				},
			}
		})

		It("should recognize a request memory change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeTrue())
		})
	})

	Context("proxy memory limit resource change", func() {
		JustBeforeEach(func() {
			proxyContainer := v1.Container{
				Name: "testProxyContainer",
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceMemory: resource.MustParse("1Gi"),
						v1.ResourceCPU:    resource.MustParse("600m"),
					},
					Requests: v1.ResourceList{
						v1.ResourceMemory: resource.MustParse("1Gi"),
						v1.ResourceCPU:    resource.MustParse("600m"),
					},
				},
				Image: expectedImageName,
			}

			lhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
						proxyContainer,
					},
				},
			}

			proxyContainer2 := v1.Container{
				Name: "testProxyContainer",
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceMemory: resource.MustParse("2Gi"),
						v1.ResourceCPU:    resource.MustParse("600m"),
					},
					Requests: v1.ResourceList{
						v1.ResourceMemory: resource.MustParse("1Gi"),
						v1.ResourceCPU:    resource.MustParse("600m"),
					},
				},
				Image: expectedImageName,
			}

			rhs = v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						nodeContainer,
						proxyContainer2,
					},
				},
			}
		})

		It("should recognize a limit memory change", func() {
			Expect(ArePodTemplateSpecDifferent(lhs, rhs)).To(BeTrue())
		})
	})
})
