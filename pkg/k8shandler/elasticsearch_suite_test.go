package k8shandler_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func Testelasticsearch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "elasticsearch Suite")
}
