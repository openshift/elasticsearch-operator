package service

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Builder represents the struct to build k8s services
type Builder struct {
	svc *corev1.Service
}

// New returns a new Builder instance with a default initialized service.
func New(serviceName, namespace string, labels map[string]string) *Builder {
	return &Builder{svc: newService(serviceName, namespace, labels)}
}

func newService(serviceName, namespace string, labels map[string]string) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{},
	}
}

// Build returns the final service.
func (b *Builder) Build() *corev1.Service { return b.svc }

// WithAnnotations set the object meta annotations.
func (b *Builder) WithAnnotations(a map[string]string) *Builder {
	b.svc.Annotations = a
	return b
}

// WithSelector sets the service selector.
func (b *Builder) WithSelector(s map[string]string) *Builder {
	b.svc.Spec.Selector = s
	return b
}

// WithServicePorts appends service ports to the service spec.
func (b *Builder) WithServicePorts(sp ...corev1.ServicePort) *Builder {
	b.svc.Spec.Ports = append(b.svc.Spec.Ports, sp...)
	return b
}

// WithPublishNotReady sets the spec PublishNotReadyAddresses flag.
func (b *Builder) WithPublishNotReady(val bool) *Builder {
	b.svc.Spec.PublishNotReadyAddresses = val
	return b
}
