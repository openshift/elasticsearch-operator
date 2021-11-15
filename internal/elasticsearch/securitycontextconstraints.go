package elasticsearch

import (
	"context"

	securityv1 "github.com/openshift/api/security/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/openshift/elasticsearch-operator/internal/manifests/securitycontextconstraints"
)

// CreateOrUpdateSecurityContextConstraints ensures the existence of the securitycontextconstraints for Elasticsearch cluster
func (er *ElasticsearchRequest) CreateOrUpdateSecurityContextConstraints() error {
	dpl := er.cluster

	// This scc prevents a container from running as privileged
	// It allows the pod access to the hostPath volume type and marks it as not read-only
	builder := securitycontextconstraints.New("elasticsearch-scc", false, true, false)

	// Disasbles all sysctls from being executed in the pod
	builder.WithForbiddenSysctls([]string{
		"*",
	})
	// Allows the pod to be able to use the requested volume types
	builder.WithVolumes([]securityv1.FSType{
		"configMap",
		"secret",
		"emptyDir",
		"persistentVolumeClaim",
	})
	// Drops these capabilities and prevents them from being added to the pod
	builder.WithRequiredDropCapabilities([]corev1.Capability{
		"CHOWN",
		"DAC_OVERRIDE",
		"FSETID",
		"FOWNER",
		"SETGID",
		"SETUID",
		"SETPCAP",
		"NET_BIND_SERVICE",
		"KILL",
	})
	// Prevents the processes and pod from gaining more privileges than it is allowed
	builder.WithAllowPrivilegeEscalation(false)
	builder.WithDefaultAllowPrivilegeEscalation(false)
	// Does not set a default user or selinuxcontext value
	// These values can be added from the pod specification
	builder.WithRunAsUserOptions(securityv1.RunAsUserStrategyOptions{
		Type: securityv1.RunAsUserStrategyRunAsAny,
	})
	builder.WithSELinuxContextOptions(securityv1.SELinuxContextStrategyOptions{
		Type: securityv1.SELinuxStrategyRunAsAny,
	})

	scc := builder.Build()

	err := securitycontextconstraints.CreateOrUpdate(context.TODO(), er.client, scc, securitycontextconstraints.Mutate)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update elasticsearch securitycontextconstraints",
			"cluster", dpl.Name,
		)
	}

	return nil
}
