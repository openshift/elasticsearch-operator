package indexmanagement_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestIndexManagementHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IndexManagement Suite")
}
