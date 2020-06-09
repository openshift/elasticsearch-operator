package kibana

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestKibanaSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Kibana Suite")
}
