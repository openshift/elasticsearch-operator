package kibana

import (
	"github.com/ViaQ/logerr/kverrors"
	"github.com/openshift/elasticsearch-operator/internal/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewPolicyRule stubs policy rule
func NewPolicyRule(apiGroups, resources, resourceNames, verbs []string) rbac.PolicyRule {
	return rbac.PolicyRule{
		APIGroups:     apiGroups,
		Resources:     resources,
		ResourceNames: resourceNames,
		Verbs:         verbs,
	}
}

// NewPolicyRules stubs policy rules
func NewPolicyRules(rules ...rbac.PolicyRule) []rbac.PolicyRule {
	return rules
}

// NewRole stubs a role
func NewRole(roleName, namespace string, rules []rbac.PolicyRule) *rbac.Role {
	return &rbac.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: namespace,
		},
		Rules: rules,
	}
}

// NewSubject stubs a new subject
func NewSubject(kind, name string) rbac.Subject {
	return rbac.Subject{
		Kind:     kind,
		Name:     name,
		APIGroup: rbac.GroupName,
	}
}

// NewSubjects stubs subjects
func NewSubjects(subjects ...rbac.Subject) []rbac.Subject {
	return subjects
}

// NewRoleBinding stubs a role binding
func NewRoleBinding(bindingName, namespace, roleName string, subjects []rbac.Subject) *rbac.RoleBinding {
	return &rbac.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName,
			Namespace: namespace,
		},
		RoleRef: rbac.RoleRef{
			Kind:     "Role",
			Name:     roleName,
			APIGroup: rbac.GroupName,
		},
		Subjects: subjects,
	}
}

// NewClusterRoleBinding stubs a cluster role binding
func NewClusterRoleBinding(bindingName, roleName string, subjects []rbac.Subject) *rbac.ClusterRoleBinding {
	return &rbac.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: bindingName,
		},
		RoleRef: rbac.RoleRef{
			Kind:     "ClusterRole",
			Name:     roleName,
			APIGroup: rbac.GroupName,
		},
		Subjects: subjects,
	}
}

// CreateClusterRole creates a cluster role or returns error
func (clusterRequest *KibanaRequest) CreateClusterRole(name string, rules []rbac.PolicyRule) (*rbac.ClusterRole, error) {
	clusterRole := &rbac.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Rules: rules,
	}

	utils.AddOwnerRefToObject(clusterRole, getOwnerRef(clusterRequest.cluster))

	err := clusterRequest.Create(clusterRole)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, kverrors.Wrap(err, "failed to create cluster role",
			"cluster_role", name,
		)
	}
	return clusterRole, nil
}

// RemoveClusterRoleBinding removes a cluster role binding
func (clusterRequest *KibanaRequest) RemoveClusterRoleBinding(name string) error {
	binding := NewClusterRoleBinding(
		name,
		"",
		[]rbac.Subject{},
	)

	err := clusterRequest.Delete(binding)
	if err != nil && !apierrors.IsNotFound(err) {
		return kverrors.Wrap(err, "failed to delete cluster role",
			"cluster_role", name,
		)
	}

	return nil
}
