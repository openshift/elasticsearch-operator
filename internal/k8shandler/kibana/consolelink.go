package kibana

import (
	"context"

	"github.com/ViaQ/logerr/kverrors"
	consolev1 "github.com/openshift/api/console/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewConsoleLink(name, href string) *consolev1.ConsoleLink {
	return &consolev1.ConsoleLink{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: consolev1.ConsoleLinkSpec{
			Location: consolev1.ApplicationMenu,
			Link: consolev1.Link{
				Text: "Logging",
				Href: href,
			},
			ApplicationMenu: &consolev1.ApplicationMenuSpec{
				Section: "Observability",
			},
		},
	}
}

func DeleteKibanaConsoleLink(ctx context.Context, c client.Client) error {

	current := NewConsoleLink(KibanaConsoleLinkName, "")

	if err := c.Delete(ctx, current); err != nil {
		if !apierrors.IsNotFound(err) {
			return kverrors.Wrap(err, "failed to delete consolelink",
				"name", KibanaConsoleLinkName,
			)
		}
	}

	return nil
}

func consoleLinksEqual(current, desired *consolev1.ConsoleLink) bool {
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
