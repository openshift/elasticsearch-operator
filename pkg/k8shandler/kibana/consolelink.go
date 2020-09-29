package kibana

import (
	consolev1 "github.com/openshift/api/console/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
