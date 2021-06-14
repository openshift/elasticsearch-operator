package service

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

// EqualityFunc is the type for functions that compare two services.
// Return true if two services are equal.
type EqualityFunc func(current, desired *corev1.Service) bool

// MutateFunc is the type for functions that mutate the current service
// by applying the values from the desired service.
type MutateFunc func(current, desired *corev1.Service)

// CreateOrUpdate attempts first to create the given service. If the
// service already exists and the provided comparison func detects any changes
// an update is attempted. Updates are retried with backoff (See retry.DefaultRetry).
// Returns on failure a non-nil error.
func CreateOrUpdate(ctx context.Context, c client.Client, svc *corev1.Service, equal EqualityFunc, mutate MutateFunc) error {
	err := c.Create(ctx, svc)
	if err == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(kverrors.Root(err)) {
		return kverrors.Wrap(err, "failed to create service",
			"name", svc.Name,
			"namespace", svc.Namespace,
		)
	}

	current := &corev1.Service{}
	key := client.ObjectKey{Name: svc.Name, Namespace: svc.Namespace}
	err = c.Get(ctx, key, current)
	if err != nil {
		return kverrors.Wrap(err, "failed to get service",
			"name", svc.Name,
			"namespace", svc.Namespace,
		)
	}

	if !equal(current, svc) {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := c.Get(ctx, key, current); err != nil {
				log.Error(err, "failed to get service", svc.Name)
				return err
			}

			mutate(current, svc)
			if err := c.Update(ctx, current); err != nil {
				log.Error(err, "failed to update service", svc.Name)
				return err
			}
			return nil
		})
		if err != nil {
			return kverrors.Wrap(err, "failed to update service",
				"name", svc.Name,
				"namespace", svc.Namespace,
			)
		}
		return nil
	}

	return nil
}

// Equal return only true if the service are equal
func Equal(current, desired *corev1.Service) bool {
	return equality.Semantic.DeepEqual(current, desired)
}

// Mutate is a default mutation function for services
// that copies only mutable fields from desired to current.
func Mutate(current, desired *corev1.Service) {
	current.Labels = desired.Labels
	current.Annotations = desired.Annotations
	current.Spec.Ports = desired.Spec.Ports
	current.Spec.Selector = desired.Spec.Selector
	current.Spec.PublishNotReadyAddresses = desired.Spec.PublishNotReadyAddresses
}
