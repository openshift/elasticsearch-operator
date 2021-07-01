package pod

import (
	"time"

	"github.com/openshift/elasticsearch-operator/internal/utils"

	corev1 "k8s.io/api/core/v1"
)

// Builder represents the struct to build k9s podspecs
type Builder struct {
	spec *corev1.PodSpec
}

// NewSpec returns a new Builder instance with a default initialized podspec
func NewSpec(serviceAccountName string, containers []corev1.Container, volumes []corev1.Volume) *Builder {
	return &Builder{spec: newPodSpec(serviceAccountName, containers, volumes)}
}

func newPodSpec(serviceAccountName string, containers []corev1.Container, volumes []corev1.Volume) *corev1.PodSpec {
	return &corev1.PodSpec{
		ServiceAccountName: serviceAccountName,
		Containers:         containers,
		Volumes:            volumes,
		NodeSelector:       utils.EnsureLinuxNodeSelector(map[string]string{}),
	}
}

// Build returns the final podspec
func (b *Builder) Build() *corev1.PodSpec { return b.spec }

// WithNodeSelectors sets the podsec selectors and ensures that the
// default linux node selector is always present.
func (b *Builder) WithNodeSelectors(s map[string]string) *Builder {
	b.spec.NodeSelector = utils.EnsureLinuxNodeSelector(s)
	return b
}

// WithTolerations appends tolerations to the podspec
func (b *Builder) WithTolerations(t ...corev1.Toleration) *Builder {
	b.spec.Tolerations = append(b.spec.Tolerations, t...)
	return b
}

// WithAffinity sets the affinity rule for the podspec
func (b *Builder) WithAffinity(a *corev1.Affinity) *Builder {
	b.spec.Affinity = a
	return b
}

// WithRestartPolicy sets the restart policy for the podspec
func (b *Builder) WithRestartPolicy(rp corev1.RestartPolicy) *Builder {
	b.spec.RestartPolicy = rp
	return b
}

// WithTerminationGracePeriodSeconds sets the termination grace period for the podspec
func (b *Builder) WithTerminationGracePeriodSeconds(p time.Duration) *Builder {
	d := int64(p.Seconds())
	b.spec.TerminationGracePeriodSeconds = &d
	return b
}
