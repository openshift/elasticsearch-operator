package securitycontextconstraints

import (
	"context"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"

	securityv1 "github.com/openshift/api/security/v1"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MutateFunc is the type for functions that mutate the current constraints
// by applying the values from the desired route.
type MutateFunc func(current, desired *securityv1.SecurityContextConstraints)

// CreateOrUpdate attempts first to create the given constraints. If the
// constraints already exists and the provided comparison func detects any changes
// an update is attempted. Updates are retried with backoff (See retry.DefaultRetry).
// Returns on failure an non-nil error.
func CreateOrUpdate(ctx context.Context, c client.Client, s *securityv1.SecurityContextConstraints, mutate MutateFunc) error {
	err := c.Create(ctx, s)
	if err == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(kverrors.Root(err)) {
		return kverrors.Wrap(err, "failed to create security context constraints",
			"name", s.Name,
		)
	}

	current := &securityv1.SecurityContextConstraints{}
	key := client.ObjectKey{Name: s.Name}
	err = c.Get(ctx, key, current)
	if err != nil {
		return kverrors.Wrap(err, "failed to get security context constraints",
			"name", s.Name,
		)
	}

	if !equality.Semantic.DeepEqual(current, s) {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := c.Get(ctx, key, current); err != nil {
				log.Error(err, "failed to get security context constraints", s.Name)
				return err
			}

			mutate(current, s)
			if err := c.Update(ctx, current); err != nil {
				log.Error(err, "failed to update security context constraints", s.Name)
				return err
			}
			return nil
		})
		if err != nil {
			return kverrors.Wrap(err, "failed to update security context constraints",
				"name", s.Name,
			)
		}
		return nil
	}

	return nil
}

// Mutate is a default mutate functions for securitycontextconstraints.
// It overrides the values used by the cluster to maintain security.
func Mutate(current, desired *securityv1.SecurityContextConstraints) {
	current.Priority = desired.Priority
	current.AllowPrivilegedContainer = desired.AllowPrivilegedContainer
	current.DefaultAddCapabilities = desired.DefaultAddCapabilities
	current.RequiredDropCapabilities = desired.RequiredDropCapabilities
	current.AllowedCapabilities = desired.AllowedCapabilities
	current.AllowHostDirVolumePlugin = desired.AllowHostDirVolumePlugin
	current.Volumes = desired.Volumes
	current.AllowedFlexVolumes = desired.AllowedFlexVolumes
	current.AllowHostNetwork = desired.AllowHostNetwork
	current.AllowHostPorts = desired.AllowHostPorts
	current.AllowHostPID = desired.AllowHostPID
	current.AllowHostIPC = desired.AllowHostIPC
	current.DefaultAllowPrivilegeEscalation = desired.DefaultAllowPrivilegeEscalation
	current.AllowPrivilegeEscalation = desired.AllowPrivilegeEscalation
	current.SELinuxContext = desired.SELinuxContext
	current.RunAsUser = desired.RunAsUser
	current.SupplementalGroups = desired.SupplementalGroups
	current.FSGroup = desired.FSGroup
	current.ReadOnlyRootFilesystem = desired.ReadOnlyRootFilesystem
	current.Users = desired.Users
	current.Groups = desired.Groups
	current.SeccompProfiles = desired.SeccompProfiles
	current.AllowedUnsafeSysctls = desired.AllowedUnsafeSysctls
	current.ForbiddenSysctls = desired.ForbiddenSysctls
}
