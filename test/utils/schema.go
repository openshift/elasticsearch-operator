package utils

import (
	"testing"

	consolev1 "github.com/openshift/api/console/v1"
	loggingv1 "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"

	"github.com/operator-framework/operator-sdk/pkg/test"
)

func RegisterSchemes(t *testing.T) {
	elasticsearchList := &loggingv1.ElasticsearchList{}
	err := test.AddToFrameworkScheme(loggingv1.SchemeBuilder.AddToScheme, elasticsearchList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	kibanaList := &loggingv1.KibanaList{}
	err = test.AddToFrameworkScheme(loggingv1.SchemeBuilder.AddToScheme, kibanaList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	consoleLinkList := &consolev1.ConsoleLinkList{}
	err = test.AddToFrameworkScheme(consolev1.Install, consoleLinkList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}
}
