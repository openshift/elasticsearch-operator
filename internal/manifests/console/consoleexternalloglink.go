package console

import (
	"context"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"

	consolev1 "github.com/openshift/api/console/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EqualityFunc is the type for functions that compare two consoleexternalloglinks.
// Return true if two consoleexternalloglinks are not not equal.
type ConsoleExternalLogLinkEqualityFunc func(current, desired *consolev1.ConsoleExternalLogLink) bool

// MutateFunc is the type for functions that mutate the current consoleexternalloglink
// by applying the values from the desired consoleexternalloglink.
type MutateConsoleExternalLogLinkFunc func(current, desired *consolev1.ConsoleExternalLogLink)

// CreateOrUpdateConsoleExternalLogLink attempts first to create the given consoleexternalloglink. If the
// consoleexternalloglink already exists and the provided comparison func detects any changes
// an update is attempted. Updates are retried with backoff (See retry.DefaultRetry).
// Returns on failure an non-nil error.
func CreateOrUpdateConsoleExternalLogLink(ctx context.Context, c client.Client, cll *consolev1.ConsoleExternalLogLink, equal ConsoleExternalLogLinkEqualityFunc, mutate MutateConsoleExternalLogLinkFunc) error {
	err := c.Create(ctx, cll)
	if err == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(kverrors.Root(err)) {
		return kverrors.Wrap(err, "failed to create consoleexternalloglink",
			"name", cll.Name,
		)
	}

	current := &consolev1.ConsoleExternalLogLink{}
	key := client.ObjectKey{Name: cll.Name}
	err = c.Get(ctx, key, current)
	if err != nil {
		return kverrors.Wrap(err, "failed to get consoleexternalloglink",
			"name", cll.Name,
		)
	}

	if !equal(current, cll) {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := c.Get(ctx, key, current); err != nil {
				log.Error(err, "failed to get consoleexternalloglink", cll.Name)
				return err
			}

			mutate(current, cll)
			if err := c.Update(ctx, current); err != nil {
				log.Error(err, "failed to update consoleexternalloglink", cll.Name)
				return err
			}
			return nil
		})
		if err != nil {
			return kverrors.Wrap(err, "failed to update consoleexternalloglink",
				"name", cll.Name,
			)
		}
		return nil
	}

	return nil
}

// ConsoleExternalLogLinkEqual returns true href template and text are equal.
func ConsoleExternalLogLinkEqual(current, desired *consolev1.ConsoleExternalLogLink) bool {
	if current.Spec.HrefTemplate != desired.Spec.HrefTemplate {
		return false
	}

	if current.Spec.Text != desired.Spec.Text {
		return false
	}

	return true
}

// MutateConsoleExternalLogLink is a default mutate implementation that copies
// only the href template and text from desired to current consoleexternalloglink.
func MutateConsoleExternalLogLink(current, desired *consolev1.ConsoleExternalLogLink) {
	current.Spec.HrefTemplate = desired.Spec.HrefTemplate
	current.Spec.Text = desired.Spec.Text
}
