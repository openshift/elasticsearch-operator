package k8shandler

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestK8shandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "K8sHandler Suite")
}
