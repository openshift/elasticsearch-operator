package kibana

import (
	"context"
	"fmt"

	"github.com/ViaQ/logerr/v2/log"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/console/v1"
	imagev1 "github.com/openshift/api/image/v1"
	routev1 "github.com/openshift/api/route/v1"
	loggingv1 "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/internal/constants"
	"github.com/openshift/elasticsearch-operator/internal/elasticsearch/esclient"
	"github.com/openshift/elasticsearch-operator/test/helpers"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
	_ = imagev1.AddToScheme(scheme.Scheme)

	var (
		logger  = log.NewLogger("kibana-testing")
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
		proxySourceImage = &imagev1.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "oauth-proxy",
				Namespace: "openshift",
			},
			Spec: imagev1.ImageStreamSpec{
				Tags: []imagev1.TagReference{
					{
						Name:            "v4.4",
						ReferencePolicy: imagev1.TagReferencePolicy{Type: imagev1.SourceTagReferencePolicy},
					},
				},
			},
			Status: imagev1.ImageStreamStatus{
				Tags: []imagev1.NamedTagEventList{
					{
						Tag: "v4.4",
						Items: []imagev1.TagEvent{
							{
								DockerImageReference: "image-ref",
							},
						},
					},
				},
			},
		}
		proxyLocalImage = &imagev1.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "oauth-proxy",
				Namespace: "openshift",
			},
			Spec: imagev1.ImageStreamSpec{
				Tags: []imagev1.TagReference{
					{
						Name:            "v4.4",
						ReferencePolicy: imagev1.TagReferencePolicy{Type: imagev1.LocalTagReferencePolicy},
					},
				},
			},
			Status: imagev1.ImageStreamStatus{
				DockerImageRepository: "image-registry",
				Tags: []imagev1.NamedTagEventList{
					{
						Tag: "v4.4",
						Items: []imagev1.TagEvent{
							{
								Image: "sha256:abcdef",
							},
						},
					},
				},
			},
		}
	)

	Describe("Kibana", func() {
		var client client.Client
		var esClient esclient.Client

		var (
			consoleLink = &consolev1.ConsoleLink{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConsoleLink",
					APIVersion: "console.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            KibanaConsoleLinkName,
					ResourceVersion: "1",
				},
				Spec: consolev1.ConsoleLinkSpec{
					Location: consolev1.ApplicationMenu,
					Link: consolev1.Link{
						Text: "Logging",
						Href: "https://",
					},
					ApplicationMenu: &consolev1.ApplicationMenuSpec{
						Section:  "Observability",
						ImageURL: icon,
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
					proxySourceImage,
				)
				esClient = newFakeEsClient(client, fakeResponses)
			})

			It("should create one new console link for the Kibana route", func() {
				Expect(Reconcile(logger, cluster, client, esClient, proxy, false, metav1.OwnerReference{})).Should(Succeed())

				key := types.NamespacedName{Name: KibanaConsoleLinkName}
				got := &consolev1.ConsoleLink{}

				err := client.Get(context.TODO(), key, got)
				Expect(err).To(BeNil())
				Expect(got).To(Equal(consoleLink))
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
					proxySourceImage,
				)
				esClient = newFakeEsClient(client, fakeResponses)
			})

			It("should use the default CA bundle in kibana proxy", func() {
				// Reconcile w/o custom CA bundle
				Expect(Reconcile(logger, cluster, client, esClient, proxy, false, metav1.OwnerReference{})).Should(Succeed())

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
				Expect(Reconcile(logger, cluster, client, esClient, proxy, false, metav1.OwnerReference{})).Should(Succeed())

				// Inject custom CA bundle into kibana config map
				injectedCABundle := kibanaCABundle.DeepCopy()
				injectedCABundle.Data[constants.TrustedCABundleKey] = customCABundle
				Expect(client.Update(context.TODO(), injectedCABundle)).Should(Succeed())

				// Reconcile with injected custom CA bundle
				esClient = newFakeEsClient(client, fakeResponses)
				Expect(Reconcile(logger, cluster, client, esClient, proxy, false, metav1.OwnerReference{})).Should(Succeed())

				key := types.NamespacedName{Name: cluster.GetName(), Namespace: cluster.GetNamespace()}
				dpl := &appsv1.Deployment{}
				err := client.Get(context.TODO(), key, dpl)
				Expect(err).Should(Succeed())

				trustedCABundleHash := dpl.Spec.Template.ObjectMeta.Annotations[constants.TrustedCABundleHashName]
				Expect(calcTrustedCAHashValue(injectedCABundle)).To(Equal(trustedCABundleHash))
				Expect(dpl.Spec.Template.Spec.Volumes).To(ContainElement(trustedCABundleVolume))
				Expect(dpl.Spec.Template.Spec.Containers[1].VolumeMounts).To(ContainElement(trustedCABundleVolumeMount))
			})

			It("should create a deployment with the source kibana proxy image", func() {
				Expect(Reconcile(logger, cluster, client, esClient, proxy, false, metav1.OwnerReference{})).Should(Succeed())

				key := types.NamespacedName{Name: "kibana", Namespace: cluster.GetNamespace()}
				depl := &appsv1.Deployment{}

				err := client.Get(context.TODO(), key, depl)
				Expect(err).To(BeNil())
				Expect(depl.Spec.Template.Spec.Containers[1].Image).To(Equal(proxySourceImage.Status.Tags[0].Items[0].DockerImageReference))
			})

			It("should create a deployment with the local kibana proxy image", func() {
				client = fake.NewFakeClient(
					cluster,
					kibanaCABundle,
					kibanaSecret,
					kibanaProxySecret,
					proxyLocalImage,
				)
				esClient = newFakeEsClient(client, fakeResponses)

				Expect(Reconcile(logger, cluster, client, esClient, proxy, false, metav1.OwnerReference{})).Should(Succeed())

				key := types.NamespacedName{Name: "kibana", Namespace: cluster.GetNamespace()}
				depl := &appsv1.Deployment{}

				err := client.Get(context.TODO(), key, depl)
				Expect(err).To(BeNil())
				Expect(depl.Spec.Template.Spec.Containers[1].Image).To(Equal(fmt.Sprintf("%s@%s", proxyLocalImage.Status.DockerImageRepository, proxyLocalImage.Status.Tags[0].Items[0].Image)))
			})
		})
	})
})

func newFakeEsClient(k8sClient client.Client, responses map[string]helpers.FakeElasticsearchResponses) esclient.Client {
	esChatter := helpers.NewFakeElasticsearchChatter(responses)
	return helpers.NewFakeElasticsearchClient("elasticsearch", "test-namespace", k8sClient, esChatter)
}
