package deployment

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Builder represents the struct to build k8s deployments
type Builder struct {
	dpl *appsv1.Deployment
}

// New returns a new Builder instance with a default initialized deployment.
func New(deploymentName, namespace string, labels map[string]string, replicas int32) *Builder {
	return &Builder{dpl: newDeployment(deploymentName, namespace, labels, replicas)}
}

func newDeployment(deploymentName, namespace string, labels map[string]string, replicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}
}

// Build returns the final deployment.
func (b *Builder) Build() *appsv1.Deployment { return b.dpl }

// WithSelector sets the deployment pod selector.
func (b *Builder) WithSelector(s metav1.LabelSelector) *Builder {
	b.dpl.Spec.Selector = &s
	return b
}

// WithStrategy sets the deployment strategy
func (b *Builder) WithStrategy(s appsv1.DeploymentStrategyType) *Builder {
	b.dpl.Spec.Strategy = appsv1.DeploymentStrategy{Type: s}
	return b
}

// WithTemplate sets the deployment pod template spec
func (b *Builder) WithTemplate(t corev1.PodTemplateSpec) *Builder {
	b.dpl.Spec.Template = t
	return b
}

// WithPaused sets the deployment spec paused flag
func (b *Builder) WithPaused(p bool) *Builder {
	b.dpl.Spec.Paused = p
	return b
}

// WithProgressDeadlineSeconds sets the deployment ProgressDeadlineSeconds
func (b *Builder) WithProgressDeadlineSeconds(pds int32) *Builder {
	b.dpl.Spec.ProgressDeadlineSeconds = &pds
	return b
}
