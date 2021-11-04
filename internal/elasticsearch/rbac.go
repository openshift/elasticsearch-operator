package elasticsearch

import (
	"context"

	"github.com/ViaQ/logerr/kverrors"

	v1 "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/internal/manifests/rbac"
	rbacv1 "k8s.io/api/rbac/v1"
)

func (er *ElasticsearchRequest) CreateOrUpdateRBAC() error {
	dpl := er.cluster

	// elasticsearch RBAC
	elasticsearchRole := rbac.NewClusterRole(
		"elasticsearch-metrics",
		rbac.NewPolicyRules(
			rbac.NewPolicyRule(
				[]string{""},
				[]string{"pods", "services", "endpoints"},
				[]string{},
				[]string{"list", "watch"},
				[]string{},
			),
			rbac.NewPolicyRule(
				[]string{},
				[]string{},
				[]string{},
				[]string{"get"},
				[]string{"/metrics"},
			),
		),
	)

	err := rbac.CreateOrUpdateClusterRole(context.TODO(), er.client, elasticsearchRole)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update elasticsearch clusterrole",
			"cluster", dpl.Name,
		)
	}

	subject := rbac.NewSubject(
		"ServiceAccount",
		"prometheus-k8s",
		"openshift-monitoring",
	)
	subject.APIGroup = ""

	elasticsearchRoleBinding := rbac.NewClusterRoleBinding(
		"elasticsearch-metrics",
		"elasticsearch-metrics",
		rbac.NewSubjects(subject),
	)

	err = rbac.CreateOrUpdateClusterRoleBinding(context.TODO(), er.client, elasticsearchRoleBinding)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update elasticsearch clusterrolebinding",
			"cluster_role_binding_name", elasticsearchRoleBinding.Name,
		)
	}

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

	err = rbac.CreateOrUpdateClusterRole(context.TODO(), er.client, proxyRole)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update elasticsearch proxy clusterrole",
			"cluster", dpl.Name,
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
		subject = rbac.NewSubject(
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

	sccRole := rbac.NewRole(
		"elasticsearch-restricted",
		dpl.Namespace,
		rbac.NewPolicyRules(
			rbac.NewPolicyRule(
				[]string{"security.openshift.io"},
				[]string{"securitycontextconstraints"},
				[]string{"restricted"},
				[]string{"use"},
				[]string{},
			),
		),
	)

	err = rbac.CreateOrUpdateRole(context.TODO(), er.client, sccRole)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update elasticsearch restricted role",
			"cluster", dpl.Name,
			"namespace", dpl.Namespace,
		)
	}

	sccRoleBinding := rbac.NewRoleBinding(
		"elasticsearch-restricted",
		dpl.Namespace,
		"elasticsearch-restricted",
		subjects,
	)

	err = rbac.CreateOrUpdateRoleBinding(context.TODO(), er.client, sccRoleBinding)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update elasticsearch restricted rolebinding",
			"role_binding_name", sccRoleBinding.Name,
			"namespace", dpl.Namespace,
		)
	}

	return nil
}
