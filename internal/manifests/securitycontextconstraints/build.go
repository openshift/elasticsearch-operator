package securitycontextconstraints

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	securityv1 "github.com/openshift/api/security/v1"
)

// Builder represents the struct to build security context constraints
type Builder struct {
	scc *securityv1.SecurityContextConstraints
}

// New returns a new Builder for security context constraints
func New(name string, allowPrivelegeContainer, allowHostDirVolumePlugin, readOnlyRootFilesystem bool) *Builder {
	return &Builder{scc: newConstraints(name, allowPrivelegeContainer, allowHostDirVolumePlugin, readOnlyRootFilesystem)}
}

func newConstraints(name string, allowPrivelegeContainer, allowHostDirVolumePlugin, readOnlyRootFilesystem bool) *securityv1.SecurityContextConstraints {
	return &securityv1.SecurityContextConstraints{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SecurityContextConstraints",
			APIVersion: securityv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		AllowPrivilegedContainer: allowPrivelegeContainer,
		AllowHostDirVolumePlugin: allowHostDirVolumePlugin,
		ReadOnlyRootFilesystem:   readOnlyRootFilesystem,
	}
}

// Build returns the final security context constraints
func (b *Builder) Build() *securityv1.SecurityContextConstraints { return b.scc }

// Sets the constraints volumes
func (b *Builder) WithVolumes(volumes []securityv1.FSType) *Builder {
	b.scc.Volumes = volumes
	return b
}

// Sets the constraints forbidden sysctls
func (b *Builder) WithForbiddenSysctls(forbiddenSysctls []string) *Builder {
	b.scc.ForbiddenSysctls = forbiddenSysctls
	return b
}

// Sets the constraints drop capabilities
func (b *Builder) WithRequiredDropCapabilities(capabilities []corev1.Capability) *Builder {
	b.scc.RequiredDropCapabilities = capabilities
	return b
}

// Sets the constraints user options
func (b *Builder) WithRunAsUserOptions(options securityv1.RunAsUserStrategyOptions) *Builder {
	b.scc.RunAsUser = options
	return b
}

// Sets the constraints selinuxcontext options
func (b *Builder) WithSELinuxContextOptions(options securityv1.SELinuxContextStrategyOptions) *Builder {
	b.scc.SELinuxContext = options
	return b
}

// Sets the constraints privelege escalation
func (b *Builder) WithAllowPrivilegeEscalation(value bool) *Builder {
	b.scc.AllowPrivilegeEscalation = &value
	return b
}

// Sets the constraints default privelege escalation
func (b *Builder) WithDefaultAllowPrivilegeEscalation(value bool) *Builder {
	b.scc.DefaultAllowPrivilegeEscalation = &value
	return b
}
