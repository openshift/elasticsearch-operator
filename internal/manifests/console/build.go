package console

import (
	"github.com/go-logr/logr"
	consolev1 "github.com/openshift/api/console/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var logger logr.Logger

// NewConsoleLink returns a new openshift api console link
func NewConsoleLink(name, href, text, icon, section string) *consolev1.ConsoleLink {
	return &consolev1.ConsoleLink{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: consolev1.ConsoleLinkSpec{
			Location: consolev1.ApplicationMenu,
			Link: consolev1.Link{
				Text: text,
				Href: href,
			},
			ApplicationMenu: &consolev1.ApplicationMenuSpec{
				ImageURL: icon,
				Section:  section,
			},
		},
	}
}

// NewConsoleExternalLogLink returns a new opensnfhit api ConsoleExternalLogLink
func NewConsoleExternalLogLink(resourceName, consoleText, hrefTemplate string, labels map[string]string) *consolev1.ConsoleExternalLogLink {
	return &consolev1.ConsoleExternalLogLink{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConsoleExternalLogLink",
			APIVersion: consolev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   resourceName,
			Labels: labels,
		},
		Spec: consolev1.ConsoleExternalLogLinkSpec{
			Text:         consoleText,
			HrefTemplate: hrefTemplate,
		},
	}
}
