package route

import (
	"context"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"

	routev1 "github.com/openshift/api/route/v1"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EqualityFunc is the type for functions that compare two routes.
// Return true if two route are equal.
type EqualityFunc func(current, desired *routev1.Route) bool

// MutateFunc is the type for functions that mutate the current route
// by applying the values from the desired route.
type MutateFunc func(current, desired *routev1.Route)

// Get returns the openshift route for the given object key or an error.
func Get(ctx context.Context, c client.Client, key client.ObjectKey) (*routev1.Route, error) {
	r := New(key.Name, key.Namespace, "", nil).Build()

	if err := c.Get(ctx, key, r); err != nil {
		return r, kverrors.Wrap(err, "failed to get route",
			"name", r.Name,
			"namespace", r.Namespace,
		)
	}

	return r, nil
}

// CreateOrUpdate attempts first to create the given route. If the
// route already exists and the provided comparison func detects any changes
// an update is attempted. Updates are retried with backoff (See retry.DefaultRetry).
// Returns on failure an non-nil error.
func CreateOrUpdate(ctx context.Context, c client.Client, r *routev1.Route, equal EqualityFunc, mutate MutateFunc) error {
	err := c.Create(ctx, r)
	if err == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(kverrors.Root(err)) {
		return kverrors.Wrap(err, "failed to create route",
			"name", r.Name,
			"namespace", r.Namespace,
		)
	}

	current := &routev1.Route{}
	key := client.ObjectKey{Name: r.Name, Namespace: r.Namespace}
	err = c.Get(ctx, key, current)
	if err != nil {
		return kverrors.Wrap(err, "failed to get route",
			"name", r.Name,
			"namespace", r.Namespace,
		)
	}

	if !equal(current, r) {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := c.Get(ctx, key, current); err != nil {
				log.Error(err, "failed to get route", r.Name)
				return err
			}

			mutate(current, r)
			if err := c.Update(ctx, current); err != nil {
				log.Error(err, "failed to update route", r.Name)
				return err
			}
			return nil
		})
		if err != nil {
			return kverrors.Wrap(err, "failed to update route",
				"name", r.Name,
				"namespace", r.Namespace,
			)
		}
		return nil
	}

	return nil
}

// RouteTLSConfigEqual returns true only if the routes are equal in tls configs.
func RouteTLSConfigEqual(current, desired *routev1.Route) bool {
	return equality.Semantic.DeepEqual(current.Spec.TLS, desired.Spec.TLS)
}

// MutateTLSConfigOnly is a default mutate implementation that copies
// only the route's tls config from desired to current.
func MutateTLSConfigOnly(current, desired *routev1.Route) {
	current.Spec.TLS = desired.Spec.TLS
}
