package cronjob

import (
	"context"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EqualityFunc is the type for functions that compare two cronjobs.
// Return true if two cronjobs are equal.
type EqualityFunc func(current, desired *batchv1.CronJob) bool

// MutateFunc is the type for functions that mutate the current cronjob
// by applying the values from the desired cronjob.
type MutateFunc func(current, desired *batchv1.CronJob)

// CreateOrUpdate attempts first to get the given cronjob. If the
// cronjob does not exist, the cronjob will be created. Otherwise,
// if the cronjob exists and the provided comparison func detects any changes
// an update is attempted. Updates are retried with backoff (See retry.DefaultRetry).
// Returns on failure an non-nil error.
func CreateOrUpdate(ctx context.Context, c client.Client, cj *batchv1.CronJob, equal EqualityFunc, mutate MutateFunc) error {
	current := &batchv1.CronJob{}
	key := client.ObjectKey{Name: cj.Name, Namespace: cj.Namespace}
	err := c.Get(ctx, key, current)
	if err != nil {
		if apierrors.IsNotFound(kverrors.Root(err)) {
			err = c.Create(ctx, cj)

			if err == nil {
				return nil
			}

			return kverrors.Wrap(err, "failed to create cronjob",
				"name", cj.Name,
				"namespace", cj.Namespace,
			)
		}

		return kverrors.Wrap(err, "failed to get cronjob",
			"name", cj.Name,
			"namespace", cj.Namespace,
		)
	}

	if !equal(current, cj) {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := c.Get(ctx, key, current); err != nil {
				log.Error(err, "failed to get cronjob", cj.Name)
				return err
			}

			mutate(current, cj)
			if err := c.Update(ctx, current); err != nil {
				log.Error(err, "failed to update cronjob", cj.Name)
				return err
			}
			return nil
		})
		if err != nil {
			return kverrors.Wrap(err, "failed to update cronjob",
				"name", cj.Name,
				"namespace", cj.Namespace,
			)
		}
		return nil
	}

	return nil
}

// Delete attempts to delete a k8s deployment if existing or returns an error.
func Delete(ctx context.Context, c client.Client, key client.ObjectKey) error {
	cj := New(key.Name, key.Namespace, nil).Build()

	if err := c.Delete(ctx, cj, &client.DeleteOptions{}); err != nil {
		return kverrors.Wrap(err, "failed to delete cronjob",
			"name", cj.Name,
			"namespace", cj.Namespace,
		)
	}

	return nil
}

// List returns a list of deployments that match the given selector.
func List(ctx context.Context, c client.Client, namespace string, selector map[string]string) ([]batchv1.CronJob, error) {
	list := &batchv1.CronJobList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(selector),
	}
	if err := c.List(ctx, list, opts...); err != nil {
		return nil, kverrors.Wrap(err, "failed to list cronjobs",
			"namespace", namespace,
		)
	}

	return list.Items, nil
}

// Equal return only true if the cronjob are equal
func Equal(current, desired *batchv1.CronJob) bool {
	return equality.Semantic.DeepEqual(current, desired)
}

// Mutate is a default mutation function for cronjobs
// that copies only mutable fields from desired to current.
func Mutate(current, desired *batchv1.CronJob) {
	current.Spec = desired.Spec
}
