package rbac

import (
	"context"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateOrUpdateClusterRole attempts first to create the given clusterrole. If the
// clusterrole already exists and the provided comparison func detects any changes
// an update is attempted. Updates are retried with backoff (See retry.DefaultRetry).
// Returns on failure an non-nil error.
func CreateOrUpdateClusterRole(ctx context.Context, c client.Client, cr *rbacv1.ClusterRole) error {
	err := c.Create(ctx, cr)
	if err == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(kverrors.Root(err)) {
		return kverrors.Wrap(err, "failed to create clusterrole",
			"name", cr.Name,
		)
	}

	current := &rbacv1.ClusterRole{}
	key := client.ObjectKey{Name: cr.Name}
	err = c.Get(ctx, key, current)
	if err != nil {
		return kverrors.Wrap(err, "failed to get clusterrole",
			"name", cr.Name,
		)
	}

	if !equality.Semantic.DeepEqual(current, cr) {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := c.Get(ctx, key, current); err != nil {
				log.Error(err, "failed to get clusterrole", cr.Name)
				return err
			}

			current.Rules = cr.Rules
			if err := c.Update(ctx, current); err != nil {
				log.Error(err, "failed to update clusterrole", cr.Name)
				return err
			}
			return nil
		})
		if err != nil {
			return kverrors.Wrap(err, "failed to update clusterrole",
				"name", cr.Name,
			)
		}
		return nil
	}
	return nil
}
