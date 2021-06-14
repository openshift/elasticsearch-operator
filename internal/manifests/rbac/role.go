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

// CreateOrUpdateRole attempts first to create the given role. If the
// role already exists and the provided comparison func detects any changes
// an update is attempted. Updates are retried with backoff (See retry.DefaultRetry).
// Returns on failure an non-nil error.
func CreateOrUpdateRole(ctx context.Context, c client.Client, r *rbacv1.Role) error {
	err := c.Create(ctx, r)
	if err == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(kverrors.Root(err)) {
		return kverrors.Wrap(err, "failed to create role",
			"name", r.Name,
			"namespace", r.Namespace,
		)
	}

	current := &rbacv1.Role{}
	key := client.ObjectKey{Name: r.Name, Namespace: r.Namespace}
	err = c.Get(ctx, key, current)
	if err != nil {
		return kverrors.Wrap(err, "failed to get role",
			"name", r.Name,
			"namespace", r.Namespace,
		)
	}

	if !equality.Semantic.DeepEqual(current, r) {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := c.Get(ctx, key, current); err != nil {
				log.Error(err, "failed to get role", r.Name)
				return err
			}

			current.Rules = r.Rules
			if err := c.Update(ctx, current); err != nil {
				log.Error(err, "failed to update role", r.Name)
				return err
			}
			return nil
		})
		if err != nil {
			return kverrors.Wrap(err, "failed to update role",
				"name", r.Name,
				"namespace", r.Namespace,
			)
		}
		return nil
	}
	return nil
}
