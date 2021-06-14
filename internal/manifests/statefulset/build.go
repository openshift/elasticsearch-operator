package statefulset

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Builder represents the struct to build k8s statefulsets
type Builder struct {
	sts *appsv1.StatefulSet
}

// New returns a new Builder instance with a default initialized statefulset.
func New(statefulSetName, namespace string, labels map[string]string, replicas int32) *Builder {
	return &Builder{sts: newStatefulSet(statefulSetName, namespace, labels, replicas)}
}

func newStatefulSet(statefulSetName, namespace string, labels map[string]string, replicas int32) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      statefulSetName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
		},
	}
}

// Build returns the final statefulset.
func (b *Builder) Build() *appsv1.StatefulSet { return b.sts }

// WithSelector sets the statefulset pod selector.
func (b *Builder) WithSelector(s metav1.LabelSelector) *Builder {
	b.sts.Spec.Selector = &s
	return b
}

// WithStrategy sets the statefulset spec update strategy
func (b *Builder) WithUpdateStrategy(s appsv1.StatefulSetUpdateStrategy) *Builder {
	b.sts.Spec.UpdateStrategy = s
	return b
}

// WithTemplate sets the statefulset spec pod template spec
func (b *Builder) WithTemplate(t corev1.PodTemplateSpec) *Builder {
	b.sts.Spec.Template = t
	return b
}
