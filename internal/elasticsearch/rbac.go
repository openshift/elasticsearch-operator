package elasticsearch

import (
	"context"

	"github.com/ViaQ/logerr/kverrors"

	v1 "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/internal/manifests/rbac"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (er *ElasticsearchRequest) CreateOrUpdateRBAC() error {
	dpl := er.cluster

	// proxy RBAC
	proxyRole := rbac.NewClusterRole(
		"elasticsearch-proxy",
		rbac.NewPolicyRules(
			rbac.NewPolicyRule(
				[]string{"authentication.k8s.io"},
				[]string{"tokenreviews"},
				[]string{},
				[]string{"create"},
				[]string{},
			),
			rbac.NewPolicyRule(
				[]string{"authorization.k8s.io"},
				[]string{"subjectaccessreviews"},
				[]string{},
				[]string{"create"},
				[]string{},
			),
		),
	)

	err := rbac.CreateOrUpdateClusterRole(context.TODO(), er.client, proxyRole)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update elasticsearch proxy clusterrole",
			"cluster", dpl.Name,
			"namespace", dpl.Namespace,
		)
	}

	// Cluster role elasticsearch-proxy has to contain subjects for all ES instances
	esList := &v1.ElasticsearchList{}
	err = er.client.List(context.TODO(), esList)
	if err != nil {
		return err
	}

	subjects := []rbacv1.Subject{}
	for _, es := range esList.Items {
		subject := rbac.NewSubject(
			"ServiceAccount",
			es.Name,
			es.Namespace,
		)
		subject.APIGroup = ""
		subjects = append(subjects, subject)
	}

	proxyRoleBinding := rbac.NewClusterRoleBinding(
		"elasticsearch-proxy",
		"elasticsearch-proxy",
		subjects,
	)

	err = rbac.CreateOrUpdateClusterRoleBinding(context.TODO(), er.client, proxyRoleBinding)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update elasticsearch proxy clusterrolebinding",
			"cluster_role_binding_name", proxyRoleBinding.Name,
		)
	}

	// metrics RBAC
	metricsRole := rbac.NewRole(
		"elasticsearch-metrics",
		dpl.Namespace,
		rbac.NewPolicyRules(
			rbac.NewPolicyRule(
				[]string{""},
				[]string{"pods", "services", "endpoints"},
				[]string{},
				[]string{"list", "watch"},
				[]string{},
			),
		),
	)

	err = rbac.CreateOrUpdateRole(context.TODO(), er.client, metricsRole)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update elasticsearch metrics role",
			"cluster", dpl.Name,
			"namespace", dpl.Namespace,
		)
	}

	err = rbac.DeleteClusterRole(context.TODO(), er.client, client.ObjectKey{Name: metricsRole.Name})
	if err != nil {
		return kverrors.Wrap(err, "failed to delete elasticsearch metrics clusterrole",
			"cluster", dpl.Name,
		)
	}

	subject := rbac.NewSubject(
		"ServiceAccount",
		"prometheus-k8s",
		"openshift-monitoring",
	)
	subject.APIGroup = ""

	metricsRoleBinding := rbac.NewRoleBinding(
		"elasticsearch-metrics",
		dpl.Namespace,
		"elasticsearch-metrics",
		rbac.NewSubjects(subject),
	)

	err = rbac.CreateOrUpdateRoleBinding(context.TODO(), er.client, metricsRoleBinding)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update elasticsearch metrics rolebinding",
			"role_binding_name", metricsRoleBinding.Name,
			"namespace", dpl.Namespace,
		)
	}

	err = rbac.DeleteClusterRoleBinding(context.TODO(), er.client, client.ObjectKey{Name: metricsRoleBinding.Name})
	if err != nil {
		return kverrors.Wrap(err, "failed to delete elasticsearch metrics clusterrolebinding",
			"cluster", dpl.Name,
		)
	}

	return nil
}
