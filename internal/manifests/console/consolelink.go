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

// ConsoleLinkEqualityFunc is the type for functions that compare two consolelinks.
// Return true if two consolelinks are equal.
type ConsoleLinkEqualityFunc func(current, desired *consolev1.ConsoleLink) bool

// MutateConsoleLinkFunc is the type for functions that mutate the current consolelink
// by applying the values from the desired consolelink.
type MutateConsoleLinkFunc func(current, desired *consolev1.ConsoleLink)

// CreateOrUpdateConsoleLink attempts first to create the given consolelink. If the
// consolelink already exists and the provided comparison func detects any changes
// an update is attempted. Updates are retried with backoff (See retry.DefaultRetry).
// Returns on failure an non-nil error.
func CreateOrUpdateConsoleLink(ctx context.Context, c client.Client, cl *consolev1.ConsoleLink, equal ConsoleLinkEqualityFunc, mutate MutateConsoleLinkFunc) error {
	err := c.Create(ctx, cl)
	if err == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(kverrors.Root(err)) {
		return kverrors.Wrap(err, "failed to create consolelink",
			"name", cl.Name,
		)
	}

	current := &consolev1.ConsoleLink{}
	key := client.ObjectKey{Name: cl.Name}
	err = c.Get(ctx, key, current)
	if err != nil {
		return kverrors.Wrap(err, "failed to get consolelink",
			"name", cl.Name,
		)
	}

	if !equal(current, cl) {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := c.Get(ctx, key, current); err != nil {
				log.Error(err, "failed to get consolelink", cl.Name)
				return err
			}

			mutate(current, cl)
			if err := c.Update(ctx, current); err != nil {
				log.Error(err, "failed to update consolelink", cl.Name)
				return err
			}
			return nil
		})
		if err != nil {
			return kverrors.Wrap(err, "failed to update consolelink",
				"name", cl.Name,
			)
		}
		return nil
	}

	return nil
}

// ConsoleLinksEqual returns true all of the following are equal:
// - location
// - link text
// - link href
// - application menu section
func ConsoleLinksEqual(current, desired *consolev1.ConsoleLink) bool {
	if current.Spec.Location != desired.Spec.Location {
		return false
	}

	if current.Spec.Link.Text != desired.Spec.Link.Text {
		return false
	}

	if current.Spec.Link.Href != desired.Spec.Link.Href {
		return false
	}

	if current.Spec.ApplicationMenu.Section != desired.Spec.ApplicationMenu.Section {
		return false
	}

	return true
}

// MutateSpecOnly is a default mutate implementation that copies
// only the spec from desired to current consolelink.
func MutateConsoleLinkSpecOnly(current, desired *consolev1.ConsoleLink) {
	current.Spec = desired.Spec
}
