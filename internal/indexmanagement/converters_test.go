package indexmanagement

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apis "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/internal/constants"
)

var _ = Describe("Index Management", func() {
	defer GinkgoRecover()

	Describe("#crontabScheduleFor", func() {
		It("should error if the timeunit is not convertible", func() {
			_, err := crontabScheduleFor(apis.TimeUnit("15wk"))
			Expect(err).To(Not(BeNil()), "Invalid time units should fail")
		})

		It("should error if the timeunit is unsupported", func() {
			for _, unit := range []string{"Y", "M", "w", "d", "h", "H", "s"} {
				_, err := crontabScheduleFor(apis.TimeUnit(fmt.Sprintf("15%s", unit)))
				Expect(err).To(Not(BeNil()), fmt.Sprintf("Unsupported time units should fail: %q", unit))
			}
		})

		It("should convert minutes correctly", func() {
			Expect(crontabScheduleFor(apis.TimeUnit("8m"))).To(Equal("*/8 * * * *"))
		})
	})

	Describe("#calculateConditions", func() {
		Context("the default strategy", func() {
			var (
				conditions    rolloverConditions
				policy        apis.IndexManagementPolicySpec
				primaryShards = int32(3)
			)
			BeforeEach(func() {
				policy = apis.IndexManagementPolicySpec{
					Phases: apis.IndexManagementPhasesSpec{
						Hot: &apis.IndexManagementHotPhaseSpec{
							Actions: apis.IndexManagementActionsSpec{
								Rollover: &apis.IndexManagementActionSpec{
									MaxAge: "3d",
								},
							},
						},
					},
				}
				conditions = calculateConditions(policy, primaryShards)
			})
			It("should restrict the index to 40Gb per shard", func() {
				Expect(conditions.MaxSize).To(Equal("120gb"))
			})
			It("should restrict the index to (TheoreticalShardMaxSizeInMB * 1000) docs per primary shard", func() {
				Expect(conditions.MaxDocs).To(Equal(constants.TheoreticalShardMaxSizeInMB * 1000 * primaryShards))
			})
			It("should restrict the age to that defined by policy management", func() {
				Expect(conditions.MaxAge).To(Equal(string(policy.Phases.Hot.Actions.Rollover.MaxAge)))
			})
			It("should ignore the age if the hot phase is not defined", func() {
				policy.Phases.Hot = nil
				conditions = calculateConditions(policy, primaryShards)
				Expect(conditions.MaxAge).To(Equal(""))
			})
			It("should ignore the age if the hot phase rollover action is not defined", func() {
				policy.Phases.Hot.Actions.Rollover = nil
				conditions = calculateConditions(policy, primaryShards)
				Expect(conditions.MaxAge).To(Equal(""))
			})
		})
	})

	Describe("#calculateMillisForTimeUnit", func() {
		It("should error for an invalid value", func() {
			_, err := calculateMillisForTimeUnit(apis.TimeUnit("www5s"))
			Expect(err).ToNot(BeNil())
		})
		Context("with unsupported units", func() {
			It("should fail for 'years'", func() {
				value, err := calculateMillisForTimeUnit(apis.TimeUnit("12y"))
				Expect(value).To(BeEquivalentTo(0))
				Expect(err).ToNot(BeNil())
			})
			It("should fail for 'months'", func() {
				_, err := calculateMillisForTimeUnit(apis.TimeUnit("12M"))
				Expect(err).ToNot(BeNil())
			})
		})

		Context("with supported units", func() {
			It("should succeed for 'weeks'", func() {
				value, err := calculateMillisForTimeUnit(apis.TimeUnit("12w"))
				Expect(err).To(BeNil(), fmt.Sprintf("Error: %v", err))
				Expect(value).To(BeEquivalentTo(uint64(7257600000)))
			})
			It("should succeed for 'days'", func() {
				value, err := calculateMillisForTimeUnit(apis.TimeUnit("12d"))
				Expect(err).To(BeNil(), fmt.Sprintf("Error: %v", err))
				Expect(value).To(BeEquivalentTo(uint64(1036800000)))
			})
			It("should succeed for 'hours'", func() {
				for _, unit := range []string{"12h", "12H"} {
					value, err := calculateMillisForTimeUnit(apis.TimeUnit(unit))
					Expect(err).To(BeNil(), fmt.Sprintf("Error: %v", err))
					Expect(value).To(BeEquivalentTo(uint64(43200000)))
				}
			})
			It("should succeed for 'minutes'", func() {
				value, err := calculateMillisForTimeUnit(apis.TimeUnit("12m"))
				Expect(err).To(BeNil(), fmt.Sprintf("Error: %v", err))
				Expect(value).To(BeEquivalentTo(uint64(720000)))
			})
			It("should succeed for 'seconds'", func() {
				value, err := calculateMillisForTimeUnit(apis.TimeUnit("12s"))
				Expect(err).To(BeNil(), fmt.Sprintf("Error: %v", err))
				Expect(value).To(BeEquivalentTo(uint64(12000)))
			})
		})
	})
})
