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

// CreateOrUpdateClusterRoleBinding attempts first to create the given clusterrolebinding. If the
// clusterrolebinding already exists and the provided comparison func detects any changes
// an update is attempted. Updates are retried with backoff (See retry.DefaultRetry).
// Returns on failure an non-nil error.
func CreateOrUpdateClusterRoleBinding(ctx context.Context, c client.Client, crb *rbacv1.ClusterRoleBinding) error {
	err := c.Create(ctx, crb)
	if err == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(kverrors.Root(err)) {
		return kverrors.Wrap(err, "failed to create clusterrolebinding",
			"name", crb.Name,
		)
	}

	current := &rbacv1.ClusterRoleBinding{}
	key := client.ObjectKey{Name: crb.Name}
	err = c.Get(ctx, key, current)
	if err != nil {
		return kverrors.Wrap(err, "failed to get clusterrolebinding",
			"name", crb.Name,
		)
	}

	if !equality.Semantic.DeepEqual(current, crb) {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := c.Get(ctx, key, current); err != nil {
				log.Error(err, "failed to get clusterrolebinding", crb.Name)
				return err
			}

			current.Subjects = crb.Subjects
			if err := c.Update(ctx, current); err != nil {
				log.Error(err, "failed to update clusterrolebinding", crb.Name)
				return err
			}
			return nil
		})
		if err != nil {
			return kverrors.Wrap(err, "failed to update clusterrolebinding",
				"name", crb.Name,
			)
		}
		return nil
	}
	return nil
}
