package serviceaccount

import (
	"context"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateOrUpdate attempts first to create the given serviceaccount. If the
// serviceaccount already exists and the provided comparison func detects any changes
// an update is attempted. Updates are retried with backoff (See retry.DefaultRetry).
// Returns on failure an non-nil error.
func CreateOrUpdate(ctx context.Context, c client.Client, sa *corev1.ServiceAccount) error {
	err := c.Create(ctx, sa)
	if err == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(kverrors.Root(err)) {
		return kverrors.Wrap(err, "failed to create serviceaccount",
			"name", sa.Name,
			"namespace", sa.Namespace,
		)
	}

	current := &corev1.ServiceAccount{}
	key := client.ObjectKey{Name: sa.Name, Namespace: sa.Namespace}
	err = c.Get(ctx, key, current)
	if err != nil {
		return kverrors.Wrap(err, "failed to get serviceaccount",
			"name", sa.Name,
			"namespace", sa.Namespace,
		)
	}

	if !equality.Semantic.DeepEqual(current, sa) {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := c.Get(ctx, key, current); err != nil {
				log.Error(err, "failed to get serviceaccount", sa.Name)
				return err
			}

			current = sa
			if err := c.Update(ctx, current); err != nil {
				log.Error(err, "failed to update serviceaccount", sa.Name)
				return err
			}
			return nil
		})
		if err != nil {
			return kverrors.Wrap(err, "failed to update serviceaccount",
				"name", sa.Name,
				"namespace", sa.Namespace,
			)
		}
		return nil
	}

	return nil
}
