package servicemonitor

import (
	"context"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EqualityFunc is the type for functions that compare two servicemonitors.
// Return true if two service are equal.
type EqualityFunc func(current, desired *monitoringv1.ServiceMonitor) bool

// MutateFunc is the type for functions that mutate the current servicemonitor
// by applying the values from the desired service.
type MutateFunc func(current, desired *monitoringv1.ServiceMonitor)

// CreateOrUpdate attempts first to create the given servicemonitor. If the
// servicemonitor already exists and the provided comparison func detects any changes
// an update is attempted. Updates are retried with backoff (See retry.DefaultRetry).
// Returns on failure a non-nil error.
func CreateOrUpdate(ctx context.Context, c client.Client, sm *monitoringv1.ServiceMonitor, equal EqualityFunc, mutate MutateFunc) error {
	err := c.Create(ctx, sm)
	if err == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(kverrors.Root(err)) {
		return kverrors.Wrap(err, "failed to create servicemonitor",
			"name", sm.Name,
			"namespace", sm.Namespace,
		)
	}

	current := &monitoringv1.ServiceMonitor{}
	key := client.ObjectKey{Name: sm.Name, Namespace: sm.Namespace}
	err = c.Get(ctx, key, current)
	if err != nil {
		return kverrors.Wrap(err, "failed to get servicemonitor",
			"name", sm.Name,
			"namespace", sm.Namespace,
		)
	}

	if !equal(current, sm) {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := c.Get(ctx, key, current); err != nil {
				log.Error(err, "failed to get servicemonitor", sm.Name)
				return err
			}

			mutate(current, sm)
			if err := c.Update(ctx, current); err != nil {
				log.Error(err, "failed to update servicemonitor", sm.Name)
				return err
			}
			return nil
		})
		if err != nil {
			return kverrors.Wrap(err, "failed to update servicemonitor",
				"name", sm.Name,
				"namespace", sm.Namespace,
			)
		}
		return nil
	}

	return nil
}

// Equal return only true if the service monitors are equal
func Equal(current, desired *monitoringv1.ServiceMonitor) bool {
	return equality.Semantic.DeepEqual(current, desired)
}

// Mutate is a default mutation function for servicemonitors
// that copies only mutable fields from desired to current.
func Mutate(current, desired *monitoringv1.ServiceMonitor) {
	current.Labels = desired.Labels
	current.Spec.JobLabel = desired.Spec.JobLabel
	current.Spec.Endpoints = desired.Spec.Endpoints
}
