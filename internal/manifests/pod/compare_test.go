package pod_test

import (
	"testing"

	"github.com/openshift/elasticsearch-operator/internal/manifests/pod"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestArePodTemplateSpecEqual(t *testing.T) {
	type table struct {
		desc string
		lhs  corev1.PodTemplateSpec
		rhs  corev1.PodTemplateSpec
		want bool
	}

	defaultContainer := corev1.Container{
		Name: "testContainer",
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("2Gi"),
				corev1.ResourceCPU:    resource.MustParse("600m"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("2Gi"),
				corev1.ResourceCPU:    resource.MustParse("600m"),
			},
		},
		Image: "image",
	}

	type mutateFunc func(*corev1.Container)
	diffContainer := func(fn mutateFunc) corev1.Container {
		c := defaultContainer.DeepCopy()
		fn(c)
		return *c
	}

	tests := []table{
		{
			desc: "no change",
			lhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{defaultContainer},
				},
			},
			rhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{defaultContainer},
				},
			},
			want: true,
		},
		{
			desc: "no volume change detection",
			lhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{defaultContainer},
					Volumes: []corev1.Volume{
						{
							Name: "configmapVolume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{},
							},
						},
						{
							Name: "secretVolume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "secret",
								},
							},
						},
					},
				},
			},
			rhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{defaultContainer},
					Volumes: []corev1.Volume{
						{
							Name: "configmapVolume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			desc: "different container args",
			lhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{defaultContainer},
				},
			},
			rhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						diffContainer(func(c *corev1.Container) {
							c.Args = []string{"this", "is", "a", "test"}
						}),
					},
				},
			},
			want: false,
		},
		{
			desc: "different container image",
			lhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{defaultContainer},
				},
			},
			rhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						diffContainer(func(c *corev1.Container) {
							c.Image = "other-image"
						}),
					},
				},
			},
			want: false,
		},
		{
			desc: "different container env var from literal",
			lhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{defaultContainer},
				},
			},
			rhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						diffContainer(func(c *corev1.Container) {
							envVar := corev1.EnvVar{
								Name:  "testVar",
								Value: "testValue",
							}
							c.Env = append(c.Env, envVar)
						}),
					},
				},
			},
			want: false,
		},
		{
			desc: "different container env var from ValueFrom",
			lhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{defaultContainer},
				},
			},
			rhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						diffContainer(func(c *corev1.Container) {
							envVar := corev1.EnvVar{
								Name: "testVar",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.name",
									},
								},
							}
							c.Env = append(c.Env, envVar)
						}),
					},
				},
			},
			want: false,
		},
		{
			desc: "different container volume mounts",
			lhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{defaultContainer},
				},
			},
			rhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						diffContainer(func(c *corev1.Container) {
							c.VolumeMounts = []corev1.VolumeMount{
								{
									Name:      "testMount",
									ReadOnly:  true,
									MountPath: "/dev/random",
								},
							}
						}),
					},
				},
			},
			want: false,
		},
		{
			desc: "different container ports",
			lhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{defaultContainer},
				},
			},
			rhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						diffContainer(func(c *corev1.Container) {
							c.Ports = []corev1.ContainerPort{
								{
									Name:          "testPort",
									ContainerPort: 9200,
									HostPort:      9200,
									Protocol:      corev1.ProtocolTCP,
								},
							}
						}),
					},
				},
			},
			want: false,
		},
		{
			desc: "container cpu request from zero to 500m",
			lhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						diffContainer(func(c *corev1.Container) {
							c.Resources.Requests = corev1.ResourceList{}
						}),
					},
				},
			},
			rhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						diffContainer(func(c *corev1.Container) {
							c.Resources.Requests = corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("500m"),
							}
						}),
					},
				},
			},
			want: false,
		},
		{
			desc: "container cpu limit from zero to 500m",
			lhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						diffContainer(func(c *corev1.Container) {
							c.Resources.Limits = corev1.ResourceList{}
						}),
					},
				},
			},
			rhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						diffContainer(func(c *corev1.Container) {
							c.Resources.Limits = corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("500m"),
							}
						}),
					},
				},
			},
			want: false,
		},
		{
			desc: "container cpu request from 600m to 500m",
			lhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{defaultContainer},
				},
			},
			rhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						diffContainer(func(c *corev1.Container) {
							c.Resources.Requests = corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("500m"),
							}
						}),
					},
				},
			},
			want: false,
		},
		{
			desc: "container cpu limit from 600m to 500m",
			lhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{defaultContainer},
				},
			},
			rhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						diffContainer(func(c *corev1.Container) {
							c.Resources.Limits = corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("500m"),
							}
						}),
					},
				},
			},
			want: false,
		},
		{
			desc: "container memory request from zero to 500m",
			lhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						diffContainer(func(c *corev1.Container) {
							c.Resources.Requests = corev1.ResourceList{}
						}),
					},
				},
			},
			rhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						diffContainer(func(c *corev1.Container) {
							c.Resources.Requests = corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							}
						}),
					},
				},
			},
			want: false,
		},
		{
			desc: "container memory limit from zero to 500m",
			lhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						diffContainer(func(c *corev1.Container) {
							c.Resources.Limits = corev1.ResourceList{}
						}),
					},
				},
			},
			rhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						diffContainer(func(c *corev1.Container) {
							c.Resources.Limits = corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							}
						}),
					},
				},
			},
			want: false,
		},
		{
			desc: "container memory request from 2Gi to 1Gi",
			lhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{defaultContainer},
				},
			},
			rhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						diffContainer(func(c *corev1.Container) {
							c.Resources.Requests = corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							}
						}),
					},
				},
			},
			want: false,
		},
		{
			desc: "container memory limit from 2Gi to 1Gi",
			lhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{defaultContainer},
				},
			},
			rhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						diffContainer(func(c *corev1.Container) {
							c.Resources.Limits = corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							}
						}),
					},
				},
			},
			want: false,
		},
		{
			desc: "different node selector",
			lhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{defaultContainer},
				},
			},
			rhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{defaultContainer},
					NodeSelector: map[string]string{
						"key": "value",
					},
				},
			},
			want: false,
		},
		{
			desc: "different tolerations",
			lhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{defaultContainer},
				},
			},
			rhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{defaultContainer},
					Tolerations: []corev1.Toleration{
						{
							Key:      "key",
							Operator: corev1.TolerationOpEqual,
							Value:    "value",
						},
					},
				},
			},
			want: false,
		},
		{
			desc: "different multiple containers",
			lhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						defaultContainer,
						defaultContainer,
					},
				},
			},
			rhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						diffContainer(func(c *corev1.Container) {
							c.Resources.Limits = corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							}
						}),
						diffContainer(func(c *corev1.Container) {
							c.Resources.Limits = corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							}
						}),
					},
				},
			},
			want: false,
		},
		{
			desc: "different containers len",
			lhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						defaultContainer,
					},
				},
			},
			rhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						defaultContainer,
						defaultContainer,
					},
				},
			},
			want: false,
		},
		{
			desc: "dropped container",
			lhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						defaultContainer,
						defaultContainer,
					},
				},
			},
			rhs: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						diffContainer(func(c *corev1.Container) {
							c.Name = "other"
						}),
					},
				},
			},
			want: false,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			got := pod.ArePodTemplateSpecEqual(test.lhs, test.rhs)
			if got != test.want {
				t.Errorf("got: %t, want: %t", got, test.want)
			}
		})
	}
}

func TestPodSpecEqual_NonStrictTolerations(t *testing.T) {
	type table struct {
		desc   string
		lhs    corev1.PodSpec
		rhs    corev1.PodSpec
		strict bool
		want   bool
	}

	tests := []table{
		{
			desc: "contains exact tolerations",
			lhs: corev1.PodSpec{
				Tolerations: []corev1.Toleration{
					{
						Key:      "key",
						Operator: corev1.TolerationOpEqual,
						Value:    "value",
					},
				},
			},
			rhs: corev1.PodSpec{
				Tolerations: []corev1.Toleration{
					{
						Key:      "key",
						Operator: corev1.TolerationOpEqual,
						Value:    "value",
					},
				},
			},
			strict: true,
			want:   true,
		},
		{
			desc: "contains same tolerations",
			lhs: corev1.PodSpec{
				Tolerations: []corev1.Toleration{
					{
						Key:      "key",
						Operator: corev1.TolerationOpEqual,
						Value:    "value",
					},
				},
			},
			rhs: corev1.PodSpec{
				Tolerations: []corev1.Toleration{
					{
						Key:      "key",
						Operator: corev1.TolerationOpEqual,
						Value:    "value",
					},
					{
						Key:      "key1",
						Operator: corev1.TolerationOpEqual,
						Value:    "value1",
					},
				},
			},
			strict: false,
			want:   false,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			got := pod.ArePodSpecEqual(test.lhs, test.rhs, test.strict)
			if got != test.want {
				t.Errorf("got: %t, want: %t", got, test.want)
			}
		})
	}
}
