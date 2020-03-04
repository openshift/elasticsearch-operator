package kibana

import (
	consolev1 "github.com/openshift/api/console/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewConsoleLink(name, namespace, location string) *consolev1.ConsoleLink {
	return &consolev1.ConsoleLink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: consolev1.ConsoleLinkSpec{
			Location: consolev1.ConsoleLinkLocation(location),
			ApplicationMenu: &consolev1.ApplicationMenuSpec{
				Section: "Monitoring",
			},
		},
	}
}

func isConsoleLinkDifferent(current, desired *consolev1.ConsoleLink) (*consolev1.ConsoleLink, bool) {
	different := false

	if current.Spec.Location != desired.Spec.Location {
		logrus.Debugf("Location change detected, updating %q", current.Name)
		different = true
	}

	if current.Spec.ApplicationMenu.Section != desired.Spec.ApplicationMenu.Section {
		logrus.Debugf("Application Manu section change detected, updating %q", current.Name)
		different = true
	}

	return current, different
}
