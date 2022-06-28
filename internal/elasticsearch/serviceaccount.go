package elasticsearch

import (
	"context"

	"github.com/ViaQ/logerr/v2/kverrors"
	"github.com/openshift/elasticsearch-operator/internal/manifests/secret"
	"github.com/openshift/elasticsearch-operator/internal/manifests/serviceaccount"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// CreateOrUpdateServiceAccounts ensures the existence of the following serviceaccounts for Elasticsearch cluster:
// - elasticsearch: For mounting and using the custom SecurityContextConstraint to the elasticsearch pods
// - elasticsearch-metrics: For allowing Prometheus ServiceMonitor (ClusterMonitoring and User-Workload-Monitoring) to scrape logs
func (er *ElasticsearchRequest) CreateOrUpdateServiceAccounts() error {
	dpl := er.cluster

	sa := serviceaccount.New(dpl.Name, dpl.Namespace, map[string]string{})
	er.cluster.AddOwnerRefTo(sa)

	err := serviceaccount.CreateOrUpdate(context.TODO(), er.client, sa, serviceaccount.AnnotationsEqual, serviceaccount.MutateAnnotationsOnly)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update elasticsearch serviceaccount",
			"cluster", dpl.Name,
			"namespace", dpl.Namespace,
		)
	}

	sa = serviceaccount.New(serviceMonitorServiceAccountName(dpl.Name), dpl.Namespace, map[string]string{})
	er.cluster.AddOwnerRefTo(sa)

	err = serviceaccount.CreateOrUpdate(context.TODO(), er.client, sa, serviceaccount.AnnotationsEqual, serviceaccount.MutateAnnotationsOnly)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update elasticsearch-mertrics serviceaccount",
			"cluster", dpl.Name,
			"namespace", dpl.Namespace,
		)
	}

	return nil
}

// CreateOrUpdateServiceAccountTokenSecret ensures the existence of the following secrets
// of type `kubernetes.io/service-account-token` for the serviceaccount `elasticsearch-metrics`.
func (er *ElasticsearchRequest) CreateOrUpdateServiceAccountTokenSecret() error {
	dpl := er.cluster

	saName := serviceMonitorServiceAccountName(dpl.Name)

	key := client.ObjectKey{Name: saName, Namespace: dpl.Namespace}
	sa, err := serviceaccount.Get(context.TODO(), er.client, key)
	if err != nil {
		return kverrors.Wrap(err, "failed to get the service monitor serviceaccount",
			"cluster", dpl.Name,
			"namespace", dpl.Namespace,
		)
	}

	saTokenName := serviceMonitorServiceAccountTokenName(dpl.Name)
	s := secret.New(saTokenName, dpl.Namespace, nil)
	s.Annotations = map[string]string{
		corev1.ServiceAccountNameKey: sa.Name,
		corev1.ServiceAccountUIDKey:  string(sa.UID),
	}
	s.Type = corev1.SecretTypeServiceAccountToken
	er.cluster.AddOwnerRefTo(s)

	key = client.ObjectKeyFromObject(s)
	current, err := secret.Get(context.TODO(), er.client, key)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return kverrors.Wrap(err, "failed to get existing serviceaccount token secret for service monitor serviceaccount",
				"cluster", dpl.Name,
				"namespace", dpl.Namespace,
			)
		}
	}

	accountName := s.Annotations[corev1.ServiceAccountNameKey]
	accountUID := s.Annotations[corev1.ServiceAccountUIDKey]
	if accountName != sa.Name || accountUID != string(sa.UID) {
		key = client.ObjectKeyFromObject(current)

		if err := secret.Delete(context.TODO(), er.client, key); err != nil {
			return kverrors.Wrap(err, "failed to delete stale serviceaccount token secret for service monitor serviceaccount",
				"cluster", dpl.Name,
				"namespace", dpl.Namespace,
				"name", current.Name,
				"uid", current.UID,
			)
		}
	}

	err = secret.CreateOrUpdate(context.TODO(), er.client, s, secret.AnnotationsAndDataEqual, secret.MutateAnnotationsAndDataOnly)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update serviceacccount token secret for service monitor serviceaccount",
			"cluster", dpl.Name,
			"namespace", dpl.Namespace,
		)
	}

	return nil
}
