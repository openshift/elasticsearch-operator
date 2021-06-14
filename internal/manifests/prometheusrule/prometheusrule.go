package prometheusrule

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

// CreateOrUpdate attempts first to create the given prometheusrule. If the
// prometheusrule already exists and the provided comparison func detects any changes
// an update is attempted. Updates are retried with backoff (See retry.DefaultRetry).
// Returns on failure an non-nil error.
func CreateOrUpdate(ctx context.Context, c client.Client, pr *monitoringv1.PrometheusRule) error {
	err := c.Create(ctx, pr)
	if err == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(kverrors.Root(err)) {
		return kverrors.Wrap(err, "failed to create prometheusrule",
			"name", pr.Name,
			"namespace", pr.Namespace,
		)
	}

	current := &monitoringv1.PrometheusRule{}
	key := client.ObjectKey{Name: pr.Name, Namespace: pr.Namespace}
	err = c.Get(ctx, key, current)
	if err != nil {
		return kverrors.Wrap(err, "failed to get prometheusrule",
			"name", pr.Name,
			"namespace", pr.Namespace,
		)
	}

	if !equality.Semantic.DeepEqual(current, pr) {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := c.Get(ctx, key, current); err != nil {
				log.Error(err, "failed to get prometheusrule", pr.Name)
				return err
			}

			current.Spec = pr.Spec
			if err := c.Update(ctx, current); err != nil {
				log.Error(err, "failed to update prometheusrule", pr.Name)
				return err
			}
			return nil
		})
		if err != nil {
			return kverrors.Wrap(err, "failed to update prometheusrule",
				"name", pr.Name,
				"namespace", pr.Namespace,
			)
		}
		return nil
	}

	return nil
}
