package deployment

import (
	"context"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EqualityFunc is the type for functions that compare two deployments.
// Return true if two deployment are equal.
type EqualityFunc func(current, desired *appsv1.Deployment) bool

// MutateFunc is the type for functions that mutate the current deployment
// by applying the values from the desired deployment.
type MutateFunc func(current, desired *appsv1.Deployment)

// Get returns the k8s deployment for the given object key or an error.
func Get(ctx context.Context, c client.Client, key client.ObjectKey) (*appsv1.Deployment, error) {
	dpl := New(key.Name, key.Namespace, nil, 1).Build()

	if err := c.Get(ctx, key, dpl); err != nil {
		return dpl, kverrors.Wrap(err, "failed to get deployment",
			"name", dpl.Name,
			"namespace", dpl.Namespace,
		)
	}

	return dpl, nil
}

// Create will create the given deployment on the api server or return an error on failure
func Create(ctx context.Context, c client.Client, dpl *appsv1.Deployment) error {
	err := c.Create(ctx, dpl)
	if err != nil {
		return kverrors.Wrap(err, "failed to create deployment",
			"name", dpl.Name,
			"namespace", dpl.Namespace,
		)
	}
	return nil
}

// Update will update an existing deployment if compare func returns true or else leave it unchanged
func Update(ctx context.Context, c client.Client, dpl *appsv1.Deployment, equal EqualityFunc, mutate MutateFunc) error {
	current := &appsv1.Deployment{}
	key := client.ObjectKey{Name: dpl.Name, Namespace: dpl.Namespace}
	err := c.Get(ctx, key, current)
	if err != nil {
		return kverrors.Wrap(err, "failed to get deployment",
			"name", dpl.Name,
			"namespace", dpl.Namespace,
		)
	}

	if !equal(current, dpl) {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := c.Get(ctx, key, current); err != nil {
				log.Error(err, "failed to get deployment", dpl.Name)
				return err
			}

			mutate(current, dpl)
			if err := c.Update(ctx, current); err != nil {
				log.Error(err, "failed to update deployment", dpl.Name)
				return err
			}
			return nil
		})
		if err != nil {
			return kverrors.Wrap(err, "failed to update deployment",
				"name", dpl.Name,
				"namespace", dpl.Namespace,
			)
		}
		return nil
	}

	return nil
}

// CreateOrUpdate attempts first to create the given deployment. If the
// deployment already exists and the provided comparison func detects any changes
// an update is attempted. Updates are retried with backoff (See retry.DefaultRetry).
// Returns on failure an non-nil error.
func CreateOrUpdate(ctx context.Context, c client.Client, dpl *appsv1.Deployment, equal EqualityFunc, mutate MutateFunc) error {
	err := Create(ctx, c, dpl)
	if err == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(kverrors.Root(err)) {
		return kverrors.Wrap(err, "failed to create deployment",
			"name", dpl.Name,
			"namespace", dpl.Namespace,
		)
	}

	return Update(ctx, c, dpl, equal, mutate)
}

// Delete attempts to delete a k8s deployment if existing or returns an error.
func Delete(ctx context.Context, c client.Client, key client.ObjectKey) error {
	dpl := New(key.Name, key.Namespace, nil, 1).Build()

	if err := c.Delete(ctx, dpl, &client.DeleteOptions{}); err != nil {
		return kverrors.Wrap(err, "failed to delete deployment",
			"name", dpl.Name,
			"namespace", dpl.Namespace,
		)
	}

	return nil
}

// List returns a list of deployments that match the given selector.
func List(ctx context.Context, c client.Client, namespace string, selector map[string]string) ([]appsv1.Deployment, error) {
	list := &appsv1.DeploymentList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(selector),
	}
	if err := c.List(ctx, list, opts...); err != nil {
		return nil, kverrors.Wrap(err, "failed to list deployments",
			"namespace", namespace,
		)
	}

	return list.Items, nil
}

// ListReplicaSets returns the replica sets for a deployment and given selector.
func ListReplicaSets(ctx context.Context, c client.Client, name, namespace string, selector map[string]string) ([]appsv1.ReplicaSet, error) {
	selector["component"] = name

	list := &appsv1.ReplicaSetList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(selector),
	}
	if err := c.List(ctx, list, opts...); err != nil {
		return nil, kverrors.Wrap(err, "failed to list deployment replica sets",
			"name", name,
			"namespace", namespace,
		)
	}

	return list.Items, nil
}

// ListPods returns the replica sets for a deployment and given selector.
func ListPods(ctx context.Context, c client.Client, name, namespace string, selector map[string]string) ([]corev1.Pod, error) {
	selector["component"] = name

	list := &corev1.PodList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(selector),
	}
	if err := c.List(ctx, list, opts...); err != nil {
		return nil, kverrors.Wrap(err, "failed to list deployment pods",
			"name", name,
			"namespace", namespace,
		)
	}

	return list.Items, nil
}
