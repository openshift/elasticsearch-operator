package statefulset

import (
	"context"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EqualityFunc is the type for functions that compare two statefulsets.
// Return true if two statefulset are not not equal.
type EqualityFunc func(current, desired *appsv1.StatefulSet) bool

// MutateFunc is the type for functions that mutate the current statefulset
// by applying the values from the desired statefulset.
type MutateFunc func(current, desired *appsv1.StatefulSet)

// Get returns the k8s statefulset for the given object key or an error.
func Get(ctx context.Context, c client.Client, key client.ObjectKey) (*appsv1.StatefulSet, error) {
	sts := New(key.Name, key.Namespace, nil, 1).Build()

	if err := c.Get(ctx, key, sts); err != nil {
		return sts, kverrors.Wrap(err, "failed to get statefulset",
			"name", sts.Name,
			"namespace", sts.Namespace,
		)
	}

	return sts, nil
}

// Create will create the given statefulset on the api server or return an error on failure
func Create(ctx context.Context, c client.Client, sts *appsv1.StatefulSet) error {
	err := c.Create(ctx, sts)
	if err == nil {
		return kverrors.Wrap(err, "failed to create statefulset",
			"name", sts.Name,
			"namespace", sts.Namespace,
		)
	}
	return nil
}

// Update will update an existing statefulset if compare func returns true or else leave it unchanged. Updates are retried with backoff (See retry.DefaultRetry).
// Returns on failure an non-nil error.
func Update(ctx context.Context, c client.Client, sts *appsv1.StatefulSet, equal EqualityFunc, mutate MutateFunc) error {
	current := &appsv1.StatefulSet{}
	key := client.ObjectKey{Name: sts.Name, Namespace: sts.Namespace}
	err := c.Get(ctx, key, current)
	if err != nil {
		return kverrors.Wrap(err, "failed to get statefulset",
			"name", sts.Name,
			"namespace", sts.Namespace,
		)
	}

	if !equal(current, sts) {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := c.Get(ctx, key, current); err != nil {
				log.Error(err, "failed to get statefulset", sts.Name)
				return err
			}

			mutate(current, sts)
			if err := c.Update(ctx, current); err != nil {
				log.Error(err, "failed to update statefulset", sts.Name)
				return err
			}
			return nil
		})
		if err != nil {
			return kverrors.Wrap(err, "failed to update statefulset",
				"name", sts.Name,
				"namespace", sts.Namespace,
			)
		}
		return nil
	}

	return nil
}

// Delete attempts to delete a k8s statefulset if existing or returns an error.
func Delete(ctx context.Context, c client.Client, key client.ObjectKey) error {
	dpl := New(key.Name, key.Namespace, nil, 1).Build()

	if err := c.Delete(ctx, dpl, &client.DeleteOptions{}); err != nil {
		return kverrors.Wrap(err, "failed to delete statefulset",
			"name", dpl.Name,
			"namespace", dpl.Namespace,
		)
	}

	return nil
}

// List returns a list of statefulsets that match the given selector.
func List(ctx context.Context, c client.Client, namespace string, selector map[string]string) ([]appsv1.StatefulSet, error) {
	list := &appsv1.StatefulSetList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(selector),
	}
	if err := c.List(ctx, list, opts...); err != nil {
		return nil, kverrors.Wrap(err, "failed to list statefulsets")
	}

	return list.Items, nil
}
