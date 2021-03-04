package indexmanagement

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	batch "k8s.io/api/batch/v1beta1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apis "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	fakeruntime "github.com/openshift/elasticsearch-operator/test/helpers/runtime"
)

var _ = Describe("Index Management", func() {
	defer GinkgoRecover()

	var (
		primaryShards = int32(1)
		apiclient     client.Client
		testclient    *fakeruntime.FakeClient
		cluster       *apis.Elasticsearch
		policy        apis.IndexManagementPolicySpec
		mapping       apis.IndexManagementPolicyMappingSpec
		cronjob       *batch.CronJob
	)
	BeforeEach(func() {
		apiclient = fake.NewFakeClient()
		cluster = &apis.Elasticsearch{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mycluster",
				Namespace: "somenamespace",
			},
			Spec: apis.ElasticsearchSpec{
				Spec: apis.ElasticsearchNodeSpec{
					Image: "anImage",
				},
			},
		}
		policy = apis.IndexManagementPolicySpec{
			Name:         "foo",
			PollInterval: "5m",
		}
		mapping = apis.IndexManagementPolicyMappingSpec{
			Name: "foo",
		}
		selector := map[string]string{}
		tolerations := []core.Toleration{}
		name := fmt.Sprintf("%s-im-%s", cluster.Name, mapping.Name)
		cronjob = newCronJob(cluster.Name, cluster.Namespace, name, "*/5 * * * *", "", selector, tolerations, []core.EnvVar{})
	})
	Describe("#formatCmd", func() {
		Context("with no policies", func() {
			It("should return an empty command", func() {
				Expect(formatCmd(apis.IndexManagementPolicySpec{})).To(BeEmpty())
			})
		})
		Context("with delete phase", func() {
			It("should format the command for delete", func() {
				policy.Phases.Delete = &apis.IndexManagementDeletePhaseSpec{}
				Expect(formatCmd(policy)).To(Equal("./delete;delete_rc=$?;$(exit $delete_rc)"))
			})
		})
		Context("with rollover phase", func() {
			It("should format the command for rollover", func() {
				policy.Phases.Hot = &apis.IndexManagementHotPhaseSpec{}
				Expect(formatCmd(policy)).To(Equal("./rollover;rollover_rc=$?;$(exit $rollover_rc)"))
			})
		})
		Context("with delete and rollover phases", func() {
			It("should format the command for all phases", func() {
				policy.Phases.Delete = &apis.IndexManagementDeletePhaseSpec{}
				policy.Phases.Hot = &apis.IndexManagementHotPhaseSpec{}
				Expect(formatCmd(policy)).To(Equal("./delete;delete_rc=$?;./rollover;rollover_rc=$?;$(exit $delete_rc&&exit $rollover_rc)"))
			})
		})
	})
	Describe("#reconcileCronJob", func() {
		fnCronsAreSame := func(lhs, rhs *batch.CronJob) bool {
			return true
		}
		Describe("when trying to create the cronjob", func() {
			Context("and does not error", func() {
				It("should return without error", func() {
					apiclient = fake.NewFakeClient(cronjob)
					err := reconcileCronJob(apiclient, cluster, cronjob, fnCronsAreSame)
					Expect(err).To(BeNil(), fmt.Sprintf("Error: %v", err))
				})
			})
			Context("and errors for reasons other then already existing", func() {
				It("should return the error", func() {
					err := reconcileCronJob(apiclient, cluster, cronjob, fnCronsAreSame)
					Expect(err).To(BeNil())
				})
			})
			Context("and errors because it already exists", func() {
				Context("and the current is the same as desired", func() {
					It("should not try to update the cronjob", func() {
						apiclient = fake.NewFakeClient(cronjob)
						testclient = fakeruntime.NewFakeClient(apiclient, fakeruntime.NewAlreadyExistsException())
						apiclient = testclient
						err := reconcileCronJob(apiclient, cluster, cronjob, fnCronsAreSame)
						Expect(err).To(BeNil(), fmt.Sprintf("Error: %v", err))
						Expect(testclient.WasUpdated(cronjob.Name)).To(BeFalse(), "Exp. to not try and update the cronjob")
					})
				})
				Context("when the current is different from the desired", func() {
					It("should update the cronjob", func() {
						cronjob.Spec.Schedule = "*/5 10 * * * *"
						apiclient = fake.NewFakeClient(cronjob)
						testclient = fakeruntime.NewFakeClient(apiclient, fakeruntime.NewAlreadyExistsException())
						apiclient = testclient
						err := reconcileCronJob(apiclient, cluster, cronjob, func(lhs, rhs *batch.CronJob) bool {
							return false
						})
						Expect(err).To(BeNil(), fmt.Sprintf("Error: %v", err))
						Expect(testclient.WasUpdated(cronjob.Name)).To(BeTrue(), "Exp. to update the cronjob")
					})
				})
			})
		})
	})
	Describe("#ReconcileIndexManagementCronjob", func() {
		BeforeEach(func() {
			selector := map[string]string{}
			tolerations := []core.Toleration{}
			name := fmt.Sprintf("%s-rollover-%s", cluster.Name, policy.Name)
			cronjob = newCronJob(cluster.Name, cluster.Namespace, name, "*/5 * * * *", "", selector, tolerations, []core.EnvVar{})
			policy.Phases.Hot = &apis.IndexManagementHotPhaseSpec{
				Actions: apis.IndexManagementActionsSpec{
					Rollover: &apis.IndexManagementActionSpec{
						MaxAge: "3d",
					},
				},
			}
			policy.Phases.Delete = &apis.IndexManagementDeletePhaseSpec{
				MinAge: "7d",
			}
		})
		Describe("for invalid poll interval", func() {
			It("should not create the cronjob and return the error", func() {
				policy.PollInterval = "notavalue"
				Expect(ReconcileIndexManagementCronjob(apiclient, cluster, policy, mapping, primaryShards)).To(Not(Succeed()))
			})
		})
		Describe("when trying to create the cronjob", func() {
			Context("and no phases exist", func() {
				It("should return without error", func() {
					policy.Phases.Delete = nil
					policy.Phases.Hot = nil
					apiclient = fake.NewFakeClient(cronjob)
					err := ReconcileIndexManagementCronjob(apiclient, cluster, policy, mapping, primaryShards)
					Expect(err).To(BeNil(), fmt.Sprintf("Error: %v", err))
				})
			})
			Context("and no delete phase exists", func() {
				It("should return without error", func() {
					policy.Phases.Delete = nil
					apiclient = fake.NewFakeClient(cronjob)
					err := ReconcileIndexManagementCronjob(apiclient, cluster, policy, mapping, primaryShards)
					Expect(err).To(BeNil(), fmt.Sprintf("Error: %v", err))
				})
			})
			Context("and no hot phase exists", func() {
				It("should return without error", func() {
					policy.Phases.Hot = nil
					apiclient = fake.NewFakeClient(cronjob)
					err := ReconcileIndexManagementCronjob(apiclient, cluster, policy, mapping, primaryShards)
					Expect(err).To(BeNil(), fmt.Sprintf("Error: %v", err))
				})
			})
			Context("and does not error", func() {
				It("should return without error", func() {
					apiclient = fake.NewFakeClient(cronjob)
					err := ReconcileIndexManagementCronjob(apiclient, cluster, policy, mapping, primaryShards)
					Expect(err).To(BeNil(), fmt.Sprintf("Error: %v", err))
				})
			})
			Context("and errors for reasons other then already existing", func() {
				It("should return the error", func() {
					err := ReconcileIndexManagementCronjob(apiclient, cluster, policy, mapping, primaryShards)
					Expect(err).To(BeNil())
				})
			})
			Context("and errors because it already exists", func() {
				Context("when the current is different from the desired", func() {
					It("should update the cronjob", func() {
						newSchedule := "*/5 10 * * * *"
						cronjob.Spec.Schedule = newSchedule
						apiclient = fake.NewFakeClient(cronjob)
						err := ReconcileIndexManagementCronjob(apiclient, cluster, policy, mapping, primaryShards)
						Expect(err).To(BeNil(), fmt.Sprintf("Error: %v", err))
						Expect(cronjob.Spec.Schedule).To(Equal(newSchedule), "Exp. to update the cronjob")
					})
				})
			})
		})
	})
})
