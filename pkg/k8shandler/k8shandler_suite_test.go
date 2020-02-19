package k8shandler_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestK8sHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "k8shandler Suite")
}
