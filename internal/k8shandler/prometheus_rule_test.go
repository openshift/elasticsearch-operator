package k8shandler

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	rulePath  = "../../files/prometheus_recording_rules.yml"
	alertPath = "../../files/prometheus_alerts.yml"
)

var _ = Describe("prometheusrules", func() {
	defer GinkgoRecover()

	Context("rules", func() {
		It("should build without errors", func() {
			_, err := ruleSpec("prometheus_recording_rules.yml", rulePath)

			Expect(err).To(BeNil())
		})
	})

	Context("alerts", func() {
		It("should build without errors", func() {
			_, err := ruleSpec("prometheus_alerts.yml", alertPath)

			Expect(err).To(BeNil())
		})
	})
})
