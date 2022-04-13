package rbac

import (
	"context"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateOrUpdateClusterRole attempts first to get the given clusterrole. If the
// clusterrole does not exist, the clusterrole will be created. Otherwise,
// if the clusterrole exists and the provided comparison func detects any changes
// an update is attempted. Updates are retried with backoff (See retry.DefaultRetry).
// Returns on failure an non-nil error.
func CreateOrUpdateClusterRole(ctx context.Context, log logr.Logger, c client.Client, cr *rbacv1.ClusterRole) error {
	current := &rbacv1.ClusterRole{}
	key := client.ObjectKey{Name: cr.Name}
	err := c.Get(ctx, key, current)
	if err != nil {
		if apierrors.IsNotFound(kverrors.Root(err)) {
			err = c.Create(ctx, cr)

			if err == nil {
				return nil
			}

			return kverrors.Wrap(err, "failed to create clusterrole",
				"name", cr.Name,
			)
		}

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

// DeleteClusterRole attempts to delete a k8s cluster role if existing or returns an error.
func DeleteClusterRole(ctx context.Context, c client.Client, key client.ObjectKey) error {
	cr := NewClusterRole(key.Name, []rbacv1.PolicyRule{})

	if err := c.Delete(ctx, cr, &client.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(kverrors.Root(err)) {
			return kverrors.Wrap(err, "failed to delete clusterrole",
				"name", cr.Name,
			)
		}
	}

	return nil
}
