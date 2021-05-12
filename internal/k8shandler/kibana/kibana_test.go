package kibana

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/console/v1"
	routev1 "github.com/openshift/api/route/v1"
	loggingv1 "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/internal/constants"
	"github.com/openshift/elasticsearch-operator/internal/elasticsearch"
	"github.com/openshift/elasticsearch-operator/test/helpers"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Reconciling", func() {
	defer GinkgoRecover()

	_ = routev1.AddToScheme(scheme.Scheme)
	_ = consolev1.AddToScheme(scheme.Scheme)
	_ = loggingv1.SchemeBuilder.AddToScheme(scheme.Scheme)

	var (
		cluster = &loggingv1.Kibana{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kibana",
				Namespace: "test-namespace",
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind: expectedCLOKind,
						Name: expectedCLOName,
					},
				},
			},
			Spec: loggingv1.KibanaSpec{
				ManagementState: loggingv1.ManagementStateManaged,
				Replicas:        2,
			},
		}

		kibanaCABundle = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.KibanaTrustedCAName,
				Namespace: cluster.GetNamespace(),
				Labels: map[string]string{
					constants.InjectTrustedCABundleLabel: "true",
				},
			},
			Data: map[string]string{
				constants.TrustedCABundleKey: `
                  -----BEGIN CERTIFICATE-----
                  <PEM_ENCODED_CERT>
                  -----END CERTIFICATE-------
                `,
			},
		}
		kibanaSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kibana",
				Namespace: cluster.GetNamespace(),
			},
		}
		kibanaProxySecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kibana-proxy",
				Namespace: cluster.GetNamespace(),
			},
		}
		proxy = &configv1.Proxy{
			Spec: configv1.ProxySpec{
				TrustedCA: configv1.ConfigMapNameReference{
					Name: "custom-ca-bundle",
				},
			},
		}
	)

	Describe("Kibana", func() {
		var client client.Client
		var esClient elasticsearch.Client

		var (
			consoleLink = &consolev1.ConsoleLink{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConsoleLink",
					APIVersion: "console.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            KibanaConsoleLinkName,
					ResourceVersion: "1",
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
						Section: "Observability",
					},
				},
			}

			fakeResponses = map[string]helpers.FakeElasticsearchResponses{
				"_cat/indices/.kibana?format=json": {
					{
						StatusCode: 200,
						Body:       `[{"health":"green","status":"open","index":".kibana","uuid":"KNegGDiRSs6dxWzdxWqkaQ","pri":"1","rep":"1","docs.count":"1","docs.deleted":"0","store.size":"6.4kb","pri.store.size":"3.2kb"}]`,
					},
				},
				"_cluster/stats": {
					{
						StatusCode: 200,
						Body:       `{"nodes": {"versions": ["6.8.1"]}}`,
					},
				},
				"_alias/.kibana": {
					// Set migration completed
					{
						StatusCode: 200,
						Body:       `{".kibana-6": {"aliases": []}}`,
					},
				},
			}
		)

		Context("when creating Kibana for the first time on a new cluster", func() {
			BeforeEach(func() {
				client = fake.NewFakeClient(
					cluster,
					kibanaCABundle,
					kibanaSecret,
					kibanaProxySecret,
				)
				esClient = newFakeEsClient(client, fakeResponses)
			})

			It("should create one new console link for the Kibana route", func() {
				Expect(Reconcile(cluster, client, esClient, proxy, false, metav1.OwnerReference{})).Should(Succeed())

				key := types.NamespacedName{Name: KibanaConsoleLinkName}
				got := &consolev1.ConsoleLink{}

				err := client.Get(context.TODO(), key, got)
				Expect(err).To(BeNil())
				Expect(got).To(Equal(consoleLink))
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
					kibanaCABundle,
					kibanaSecret,
					kibanaProxySecret,
					sharingConfigMap,
					sharingConfigReader,
					sharingConfigReaderBinding,
					consoleLink,
				)
				esClient = newFakeEsClient(client, fakeResponses)
			})

			It("should replace existing sharing confimap links with one console link", func() {
				Expect(Reconcile(cluster, client, esClient, nil, false, metav1.OwnerReference{})).Should(Succeed())

				key := types.NamespacedName{Name: KibanaConsoleLinkName}
				got := &consolev1.ConsoleLink{}

				err := client.Get(context.TODO(), key, got)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(got).To(Equal(consoleLink))

				// Check old shared config map is deleted
				key = types.NamespacedName{Name: "sharing-config", Namespace: cluster.GetNamespace()}
				gotCmPre44x := &corev1.ConfigMap{}
				Expect(errors.IsNotFound(client.Get(context.TODO(), key, gotCmPre44x))).To(BeTrue())

				// Check old role to access the shared config map is deleted
				key = types.NamespacedName{Name: "sharing-config-reader", Namespace: cluster.GetNamespace()}
				gotRolePre45x := &rbacv1.Role{}
				Expect(errors.IsNotFound(client.Get(context.TODO(), key, gotRolePre45x))).To(BeTrue())

				// Check old rolebinding for group system:autheticated is deleted
				key = types.NamespacedName{Name: "openshift-logging-sharing-config-reader-binding", Namespace: cluster.GetNamespace()}
				gotRoleBindingPre45x := &rbacv1.RoleBinding{}
				Expect(errors.IsNotFound(client.Get(context.TODO(), key, gotRoleBindingPre45x))).To(BeTrue())
			})
		})

		Context("when cluster proxy present", func() {
			var (
				customCABundle = `
                  -----BEGIN CERTIFICATE-----
                  <PEM_ENCODED_CERT1>
                  -----END CERTIFICATE-------
                  -----BEGIN CERTIFICATE-----
                  <PEM_ENCODED_CERT2>
                  -----END CERTIFICATE-------
                `
				trustedCABundleVolume = corev1.Volume{
					Name: constants.KibanaTrustedCAName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: constants.KibanaTrustedCAName,
							},
							Items: []corev1.KeyToPath{
								{
									Key:  constants.TrustedCABundleKey,
									Path: constants.TrustedCABundleMountFile,
								},
							},
						},
					},
				}
				trustedCABundleVolumeMount = corev1.VolumeMount{
					Name:      constants.KibanaTrustedCAName,
					ReadOnly:  true,
					MountPath: constants.TrustedCABundleMountDir,
				}
			)

			BeforeEach(func() {
				client = fake.NewFakeClient(
					cluster,
					kibanaCABundle,
					kibanaSecret,
					kibanaProxySecret,
				)
				esClient = newFakeEsClient(client, fakeResponses)
			})

			It("should use the default CA bundle in kibana proxy", func() {
				// Reconcile w/o custom CA bundle
				Expect(Reconcile(cluster, client, esClient, proxy, false, metav1.OwnerReference{})).Should(Succeed())

				key := types.NamespacedName{Name: constants.KibanaTrustedCAName, Namespace: cluster.GetNamespace()}
				kibanaCaBundle := &corev1.ConfigMap{}
				err := client.Get(context.TODO(), key, kibanaCaBundle)
				Expect(err).Should(Succeed())
				Expect(kibanaCABundle.Data).To(Equal(kibanaCaBundle.Data))

				key = types.NamespacedName{Name: cluster.GetName(), Namespace: cluster.GetNamespace()}
				dpl := &appsv1.Deployment{}
				err = client.Get(context.TODO(), key, dpl)
				Expect(err).Should(Succeed())

				trustedCABundleHash := dpl.Spec.Template.ObjectMeta.Annotations[constants.TrustedCABundleHashName]
				Expect(calcTrustedCAHashValue(kibanaCABundle)).To(Equal(trustedCABundleHash))
				Expect(dpl.Spec.Template.Spec.Volumes).To(ContainElement(trustedCABundleVolume))
				Expect(dpl.Spec.Template.Spec.Containers[1].VolumeMounts).To(ContainElement(trustedCABundleVolumeMount))
			})

			It("should use the injected custom CA bundle in kibana proxy", func() {
				// Reconcile w/o custom CA bundle
				Expect(Reconcile(cluster, client, esClient, proxy, false, metav1.OwnerReference{})).Should(Succeed())

				// Inject custom CA bundle into kibana config map
				injectedCABundle := kibanaCABundle.DeepCopy()
				injectedCABundle.Data[constants.TrustedCABundleKey] = customCABundle
				Expect(client.Update(context.TODO(), injectedCABundle)).Should(Succeed())

				// Reconcile with injected custom CA bundle
				esClient = newFakeEsClient(client, fakeResponses)
				Expect(Reconcile(cluster, client, esClient, proxy, false, metav1.OwnerReference{})).Should(Succeed())

				key := types.NamespacedName{Name: cluster.GetName(), Namespace: cluster.GetNamespace()}
				dpl := &appsv1.Deployment{}
				err := client.Get(context.TODO(), key, dpl)
				Expect(err).Should(Succeed())

				trustedCABundleHash := dpl.Spec.Template.ObjectMeta.Annotations[constants.TrustedCABundleHashName]
				Expect(calcTrustedCAHashValue(injectedCABundle)).To(Equal(trustedCABundleHash))
				Expect(dpl.Spec.Template.Spec.Volumes).To(ContainElement(trustedCABundleVolume))
				Expect(dpl.Spec.Template.Spec.Containers[1].VolumeMounts).To(ContainElement(trustedCABundleVolumeMount))
			})
		})
	})
})

func newFakeEsClient(k8sClient client.Client, responses map[string]helpers.FakeElasticsearchResponses) elasticsearch.Client {
	esChatter := helpers.NewFakeElasticsearchChatter(responses)
	return helpers.NewFakeElasticsearchClient("elasticsearch", "test-namespace", k8sClient, esChatter)
}
