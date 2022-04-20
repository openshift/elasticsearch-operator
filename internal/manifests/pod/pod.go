package pod

import (
	"context"

	"github.com/ViaQ/logerr/v2/kverrors"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// List returns a list of pods in a namespace that match the given selector
func List(ctx context.Context, c client.Client, namespace string, selector map[string]string) ([]corev1.Pod, error) {
	list := &corev1.PodList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(selector),
	}
	if err := c.List(ctx, list, opts...); err != nil {
		return nil, kverrors.Wrap(err, "failed to list pods",
			"namespace", namespace,
		)
	}

	return list.Items, nil
}
