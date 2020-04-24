package kibana

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/console/v1"
	routev1 "github.com/openshift/api/route/v1"
	kibana "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	loggingv1 "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/pkg/constants"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Kibana", func() {
	defer GinkgoRecover()

	_ = routev1.AddToScheme(scheme.Scheme)
	_ = consolev1.AddToScheme(scheme.Scheme)
	_ = loggingv1.SchemeBuilder.AddToScheme(scheme.Scheme)

	var (
		cluster = &kibana.Kibana{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kibana",
				Namespace: "test-namespace",
			},
			Spec: kibana.KibanaSpec{
				Image:           "hub/openshift/kibana:latest",
				ManagementState: kibana.ManagementStateManaged,
				Replicas:        2,
			},
		}
		caBundle        = fmt.Sprint("-----BEGIN CERTIFICATE-----\n<PEM_ENCODED_CERT>\n-----END CERTIFICATE-----\n")
		trustedCABundle = &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.KibanaTrustedCAName,
				Namespace: cluster.GetNamespace(),
				Labels: map[string]string{
					constants.InjectTrustedCABundleLabel: "true",
				},
			},
			Data: map[string]string{
				constants.TrustedCABundleKey: caBundle,
			},
		}
		kibanaSecret = &core.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kibana",
				Namespace: cluster.GetNamespace(),
			},
		}
		kibanaProxySecret = &core.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kibana-proxy",
				Namespace: cluster.GetNamespace(),
			},
		}
		proxy = &configv1.Proxy{
			Spec: configv1.ProxySpec{
				TrustedCA: configv1.ConfigMapNameReference{
					Name: constants.KibanaTrustedCAName,
				},
			},
		}
		consoleAppLogsLink = &consolev1.ConsoleLink{
			ObjectMeta: metav1.ObjectMeta{
				Name: AppLogsConsoleLinkName,
				OwnerReferences: []metav1.OwnerReference{
					getOwnerRef(cluster),
				},
			},
			Spec: consolev1.ConsoleLinkSpec{
				Location: consolev1.ApplicationMenu,
				Link: consolev1.Link{
					Text: "Logging",
					Href: "https://",
				},
				ApplicationMenu: &consolev1.ApplicationMenuSpec{
					Section: "Monitoring",
				},
			},
		}
		consoleInfraLogsLink = &consolev1.ConsoleLink{
			ObjectMeta: metav1.ObjectMeta{
				Name: InfraLogsConsoleLinkName,
				OwnerReferences: []metav1.OwnerReference{
					getOwnerRef(cluster),
				},
			},
			Spec: consolev1.ConsoleLinkSpec{
				Location: consolev1.ApplicationMenu,
				Link: consolev1.Link{
					Text: "Logging",
					Href: "https://",
				},
				ApplicationMenu: &consolev1.ApplicationMenuSpec{
					Section: "Monitoring",
				},
			},
		}
	)

	Describe("#CreateOrUpdateKibana", func() {
		var client client.Client

		Context("when creating Kibana for the first time on a new cluster", func() {
			BeforeEach(func() {
				client = fake.NewFakeClient(
					cluster,
					trustedCABundle,
					kibanaSecret,
					kibanaProxySecret,
				)
			})
			It("should create two new console links for the Kibana route", func() {
				Expect(ReconcileKibana(cluster, client, proxy)).Should(Succeed())

				key := types.NamespacedName{Name: AppLogsConsoleLinkName}
				got := &consolev1.ConsoleLink{}

				err := client.Get(context.TODO(), key, got)
				Expect(err).To(BeNil())
				Expect(got).To(Equal(consoleAppLogsLink))

				key = types.NamespacedName{Name: InfraLogsConsoleLinkName}
				got = &consolev1.ConsoleLink{}

				err = client.Get(context.TODO(), key, got)
				Expect(err).To(BeNil())
				Expect(got).To(Equal(consoleInfraLogsLink))
			})
		})

		Context("when updating kibana on an existing cluster", func() {
			var (
				sharingConfigMap = NewConfigMap(
					"sharing-config",
					cluster.GetNamespace(),
					map[string]string{
						"kibanaAppURL":   "https://",
						"kibanaInfraURL": "https://",
					},
				)
				sharingConfigReader = NewRole(
					"sharing-config-reader",
					cluster.GetNamespace(),
					NewPolicyRules(
						NewPolicyRule(
							[]string{""},
							[]string{"configmaps"},
							[]string{"sharing-config"},
							[]string{"get"},
						),
					),
				)
				sharingConfigReaderBinding = NewRoleBinding(
					"openshift-logging-sharing-config-reader-binding",
					cluster.GetNamespace(),
					"sharing-config-reader",
					NewSubjects(
						NewSubject(
							"Group",
							"system:authenticated",
						),
					),
				)
			)

			BeforeEach(func() {
				client = fake.NewFakeClient(
					cluster,
					trustedCABundle,
					kibanaSecret,
					kibanaProxySecret,
					sharingConfigMap,
					sharingConfigReader,
					sharingConfigReaderBinding,
					consoleAppLogsLink,
					consoleInfraLogsLink,
				)
			})

			It("should replace existing sharing confimap links with two console links", func() {
				Expect(ReconcileKibana(cluster, client, nil)).Should(Succeed())

				key := types.NamespacedName{Name: AppLogsConsoleLinkName}
				got := &consolev1.ConsoleLink{}

				err := client.Get(context.TODO(), key, got)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(got).To(Equal(consoleAppLogsLink))

				key = types.NamespacedName{Name: InfraLogsConsoleLinkName}
				got = &consolev1.ConsoleLink{}

				Expect(client.Get(context.TODO(), key, got)).Should(Succeed())
				Expect(got).To(Equal(consoleInfraLogsLink))

				// Check old shared config map is deleted
				key = types.NamespacedName{Name: "sharing-config", Namespace: cluster.GetNamespace()}
				gotCmPre44x := &v1.ConfigMap{}
				Expect(errors.IsNotFound(client.Get(context.TODO(), key, gotCmPre44x))).To(BeTrue())

				// Check old role to access the shared config map is deleted
				key = types.NamespacedName{Name: "sharing-config-reader", Namespace: cluster.GetNamespace()}
				gotRolePre45x := &rbac.Role{}
				Expect(errors.IsNotFound(client.Get(context.TODO(), key, gotRolePre45x))).To(BeTrue())

				// Check old rolebinding for group system:autheticated is deleted
				key = types.NamespacedName{Name: "openshift-logging-sharing-config-reader-binding", Namespace: cluster.GetNamespace()}
				gotRoleBindingPre45x := &rbac.RoleBinding{}
				Expect(errors.IsNotFound(client.Get(context.TODO(), key, gotRoleBindingPre45x))).To(BeTrue())
			})
		})
	})
})
