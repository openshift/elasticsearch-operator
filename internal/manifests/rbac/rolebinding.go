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

// CreateOrUpdateRoleBinding attempts first to create the given rolebinding. If the
// rolebinding already exists and the provided comparison func detects any changes
// an update is attempted. Updates are retried with backoff (See retry.DefaultRetry).
// Returns on failure an non-nil error.
func CreateOrUpdateRoleBinding(ctx context.Context, c client.Client, rb *rbacv1.RoleBinding) error {
	err := c.Create(ctx, rb)
	if err == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(kverrors.Root(err)) {
		return kverrors.Wrap(err, "failed to create rolebinding",
			"name", rb.Name,
			"namespace", rb.Namespace,
		)
	}

	current := &rbacv1.RoleBinding{}
	key := client.ObjectKey{Name: rb.Name, Namespace: rb.Namespace}
	err = c.Get(ctx, key, current)
	if err != nil {
		return kverrors.Wrap(err, "failed to get rolebinding",
			"name", rb.Name,
			"namespace", rb.Namespace,
		)
	}

	if !equality.Semantic.DeepEqual(current, rb) {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := c.Get(ctx, key, current); err != nil {
				log.Error(err, "failed to get rolebinding", rb.Name)
				return err
			}

			current.Subjects = rb.Subjects
			if err := c.Update(ctx, current); err != nil {
				log.Error(err, "failed to update rolebinding", rb.Name)
				return err
			}
			return nil
		})
		if err != nil {
			return kverrors.Wrap(err, "failed to update rolebinding",
				"name", rb.Name,
				"namespace", rb.Namespace,
			)
		}
		return nil
	}
	return nil
}
