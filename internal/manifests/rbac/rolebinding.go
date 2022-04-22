package rbac

import (
	"context"

	"github.com/ViaQ/logerr/v2/kverrors"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateOrUpdateRoleBinding attempts first to get the given rolebinding. If the
// rolebinding does not exist, the rolebinding will be created. Otherwise,
// if the rolebinding exists and the provided comparison func detects any changes
// an update is attempted. Updates are retried with backoff (See retry.DefaultRetry).
// Returns on failure an non-nil error.
func CreateOrUpdateRoleBinding(ctx context.Context, c client.Client, rb *rbacv1.RoleBinding) error {
	current := &rbacv1.RoleBinding{}
	key := client.ObjectKey{Name: rb.Name, Namespace: rb.Namespace}
	err := c.Get(ctx, key, current)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = c.Create(ctx, rb)

			if err == nil {
				return nil
			}

			return kverrors.Wrap(err, "failed to create rolebinding",
				"name", rb.Name,
				"namespace", rb.Namespace,
			)
		}

		return kverrors.Wrap(err, "failed to get rolebinding",
			"name", rb.Name,
			"namespace", rb.Namespace,
		)
	}

	if !equality.Semantic.DeepEqual(current, rb) {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := c.Get(ctx, key, current); err != nil {
				return kverrors.Wrap(err, "failed to get rolebinding",
					"name", rb.Name,
					"namespace", rb.Namespace,
				)
			}

			current.Subjects = rb.Subjects
			if err := c.Update(ctx, current); err != nil {
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
