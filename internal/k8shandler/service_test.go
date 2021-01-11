package k8shandler

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	loggingv1 "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestCreateOrUpdateServices(t *testing.T) {
	isControlled := true

	tests := []struct {
		desc    string
		cluster *loggingv1.Elasticsearch
		objs    []runtime.Object
		wantSvc map[string]*corev1.Service
		wantErr bool
	}{
		{
			desc: "create services",
			cluster: &loggingv1.Elasticsearch{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "elasticsearch",
					Namespace: "openshift-logging",
				},
			},
			wantSvc: map[string]*corev1.Service{
				"elasticsearch-cluster": {
					TypeMeta: metav1.TypeMeta{
						Kind:       "Service",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "elasticsearch-cluster",
						Namespace:       "openshift-logging",
						ResourceVersion: "1",
						Labels:          map[string]string{"cluster-name": "elasticsearch"},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "logging.openshift.io/v1",
								Kind:       "Elasticsearch",
								Name:       "elasticsearch",
								Controller: &isControlled,
							},
						},
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name:       "elasticsearch",
								Protocol:   corev1.ProtocolTCP,
								Port:       9300,
								TargetPort: intstr.IntOrString{Type: 1, StrVal: "cluster"},
							},
						},
						Selector: map[string]string{
							"cluster-name":   "elasticsearch",
							"es-node-master": "true",
						},
						PublishNotReadyAddresses: true,
					},
				},
				"elasticsearch": {
					TypeMeta: metav1.TypeMeta{
						Kind:       "Service",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "elasticsearch",
						Namespace:       "openshift-logging",
						ResourceVersion: "1",
						Labels:          map[string]string{"cluster-name": "elasticsearch"},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "logging.openshift.io/v1",
								Kind:       "Elasticsearch",
								Name:       "elasticsearch",
								Controller: &isControlled,
							},
						},
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name:       "elasticsearch",
								Protocol:   corev1.ProtocolTCP,
								Port:       9200,
								TargetPort: intstr.IntOrString{Type: 1, StrVal: "restapi"},
							},
						},
						Selector: map[string]string{
							"cluster-name":   "elasticsearch",
							"es-node-client": "true",
						},
					},
				},
				"elasticsearch-metrics": {
					TypeMeta: metav1.TypeMeta{
						Kind:       "Service",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "elasticsearch-metrics",
						Namespace:       "openshift-logging",
						ResourceVersion: "1",
						Annotations: map[string]string{
							"service.beta.openshift.io/serving-cert-secret-name": "elasticsearch-metrics",
						},
						Labels: map[string]string{
							"cluster-name":   "elasticsearch",
							"scrape-metrics": "enabled",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "logging.openshift.io/v1",
								Kind:       "Elasticsearch",
								Name:       "elasticsearch",
								Controller: &isControlled,
							},
						},
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name:       "elasticsearch",
								Protocol:   corev1.ProtocolTCP,
								Port:       60001,
								TargetPort: intstr.IntOrString{Type: 1, StrVal: "metrics"},
							},
						},
						Selector: map[string]string{
							"cluster-name":   "elasticsearch",
							"es-node-client": "true",
						},
					},
				},
			},
		},
		{
			desc: "update services",
			cluster: &loggingv1.Elasticsearch{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "elasticsearch",
					Namespace: "openshift-logging",
				},
			},
			objs: []runtime.Object{
				&corev1.Service{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Service",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "elasticsearch-metrics",
						Namespace:       "openshift-logging",
						ResourceVersion: "1",
						Annotations: map[string]string{
							"service.alpha.openshift.io/serving-cert-secret-name": "elasticsearch-metrics",
						},
						Labels: map[string]string{
							"cluster-name":   "elasticsearch",
							"scrape-metrics": "enabled",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "logging.openshift.io/v1",
								Kind:       "Elasticsearch",
								Name:       "elasticsearch",
								Controller: &isControlled,
							},
						},
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name:       "elasticsearch",
								Protocol:   corev1.ProtocolTCP,
								Port:       60001,
								TargetPort: intstr.IntOrString{Type: 1, StrVal: "metrics"},
							},
						},
						Selector: map[string]string{
							"cluster-name":   "elasticsearch",
							"es-node-client": "true",
						},
					},
				},
			},
			wantSvc: map[string]*corev1.Service{
				"elasticsearch-cluster": {
					TypeMeta: metav1.TypeMeta{
						Kind:       "Service",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "elasticsearch-cluster",
						Namespace:       "openshift-logging",
						ResourceVersion: "1",
						Labels:          map[string]string{"cluster-name": "elasticsearch"},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "logging.openshift.io/v1",
								Kind:       "Elasticsearch",
								Name:       "elasticsearch",
								Controller: &isControlled,
							},
						},
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name:       "elasticsearch",
								Protocol:   corev1.ProtocolTCP,
								Port:       9300,
								TargetPort: intstr.IntOrString{Type: 1, StrVal: "cluster"},
							},
						},
						Selector: map[string]string{
							"cluster-name":   "elasticsearch",
							"es-node-master": "true",
						},
						PublishNotReadyAddresses: true,
					},
				},
				"elasticsearch": {
					TypeMeta: metav1.TypeMeta{
						Kind:       "Service",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "elasticsearch",
						Namespace:       "openshift-logging",
						ResourceVersion: "1",
						Labels:          map[string]string{"cluster-name": "elasticsearch"},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "logging.openshift.io/v1",
								Kind:       "Elasticsearch",
								Name:       "elasticsearch",
								Controller: &isControlled,
							},
						},
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name:       "elasticsearch",
								Protocol:   corev1.ProtocolTCP,
								Port:       9200,
								TargetPort: intstr.IntOrString{Type: 1, StrVal: "restapi"},
							},
						},
						Selector: map[string]string{
							"cluster-name":   "elasticsearch",
							"es-node-client": "true",
						},
					},
				},
				"elasticsearch-metrics": {
					TypeMeta: metav1.TypeMeta{
						Kind:       "Service",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "elasticsearch-metrics",
						Namespace:       "openshift-logging",
						ResourceVersion: "2",
						Annotations: map[string]string{
							"service.beta.openshift.io/serving-cert-secret-name": "elasticsearch-metrics",
						},
						Labels: map[string]string{
							"cluster-name":   "elasticsearch",
							"scrape-metrics": "enabled",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "logging.openshift.io/v1",
								Kind:       "Elasticsearch",
								Name:       "elasticsearch",
								Controller: &isControlled,
							},
						},
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name:       "elasticsearch",
								Protocol:   corev1.ProtocolTCP,
								Port:       60001,
								TargetPort: intstr.IntOrString{Type: 1, StrVal: "metrics"},
							},
						},
						Selector: map[string]string{
							"cluster-name":   "elasticsearch",
							"es-node-client": "true",
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		test := test

		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			client := fake.NewFakeClient(test.objs...)

			req := &ElasticsearchRequest{
				client:  client,
				cluster: test.cluster,
				ll:      log.Log.WithValues("cluster", "test-elasticsearch", "namespace", "test"),
			}

			err := req.CreateOrUpdateServices()
			if test.wantErr && err == nil {
				t.Error("missing error")
			}

			// Get elasticsearch-cluster SVC
			key := types.NamespacedName{
				Name:      "elasticsearch-cluster",
				Namespace: test.cluster.GetNamespace(),
			}
			got := &corev1.Service{}

			err = client.Get(context.TODO(), key, got)
			if err != nil {
				t.Errorf("failed with error: %s", err)
			}

			want := test.wantSvc["elasticsearch-cluster"]
			if diff := cmp.Diff(got, want); diff != "" {
				t.Errorf("diff: %s", diff)
			}

			// Get elasticsearch SVC
			key = types.NamespacedName{
				Name:      test.cluster.GetName(),
				Namespace: test.cluster.GetNamespace(),
			}
			got = &corev1.Service{}

			err = client.Get(context.TODO(), key, got)
			if err != nil {
				t.Errorf("failed with error: %s", err)
			}

			want = test.wantSvc["elasticsearch"]
			if diff := cmp.Diff(got, want); diff != "" {
				t.Errorf("diff: %s", diff)
			}

			// Get elasticsearch SVC
			key = types.NamespacedName{
				Name:      "elasticsearch-metrics",
				Namespace: test.cluster.GetNamespace(),
			}
			got = &corev1.Service{}

			err = client.Get(context.TODO(), key, got)
			if err != nil {
				t.Errorf("failed with error: %s", err)
			}

			want = test.wantSvc["elasticsearch-metrics"]
			if diff := cmp.Diff(got, want); diff != "" {
				t.Errorf("diff: %s", diff)
			}
		})
	}
}
