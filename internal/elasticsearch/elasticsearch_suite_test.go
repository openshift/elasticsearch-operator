package elasticsearch_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestElasticsearchSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Elasticsearch Suite")
}
