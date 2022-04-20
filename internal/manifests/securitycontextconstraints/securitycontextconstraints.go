package securitycontextconstraints

import (
	"context"

	"github.com/ViaQ/logerr/v2/kverrors"
	securityv1 "github.com/openshift/api/security/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EqualityFunc is the type for functions that compare two ecuritycontextconstraints.
// Return true if two ecuritycontextconstraints are equal.
type EqualityFunc func(current, desired *securityv1.SecurityContextConstraints) bool

// MutateFunc is the type for functions that mutate the current securitycontextconstraints
// by applying the values from the desired route.
type MutateFunc func(current, desired *securityv1.SecurityContextConstraints)

// CreateOrUpdate attempts first to get the given securitycontextconstraints. If the
// securitycontextconstraints does not exist, the securitycontextconstraints will be created. Otherwise,
// if the securitycontextconstraints exists and the provided comparison func detects any changes
// an update is attempted. Updates are retried with backoff (See retry.DefaultRetry).
// Returns on failure an non-nil error.
func CreateOrUpdate(ctx context.Context, c client.Client, scc *securityv1.SecurityContextConstraints, equal EqualityFunc, mutate MutateFunc) error {
	current := &securityv1.SecurityContextConstraints{}
	key := client.ObjectKey{Name: scc.Name}
	err := c.Get(ctx, key, current)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = c.Create(ctx, scc)

			if err == nil {
				return nil
			}

			return kverrors.Wrap(err, "failed to create security context constraints",
				"name", scc.Name,
			)
		}

		return kverrors.Wrap(err, "failed to get security context constraints",
			"name", scc.Name,
		)
	}

	if !equal(current, scc) {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := c.Get(ctx, key, current); err != nil {
				return kverrors.Wrap(err, "failed to get security context constraints",
					"name", scc.Name,
				)
			}

			mutate(current, scc)
			if err := c.Update(ctx, current); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return kverrors.Wrap(err, "failed to update security context constraints",
				"name", scc.Name,
			)
		}
		return nil
	}

	return nil
}

// Equal return only true if the securitycontextconstraints are equal
func Equal(current, desired *securityv1.SecurityContextConstraints) bool {
	return equality.Semantic.DeepEqual(current, desired)
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
