package rbac

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewRole returns a new k8s role
func NewRole(roleName, namespace string, rules []rbacv1.PolicyRule) *rbacv1.Role {
	return &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: namespace,
		},
		Rules: rules,
	}
}

// NewRoleBinding returns a new role binding
func NewRoleBinding(bindingName, namespace, roleName string, subjects []rbacv1.Subject) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName,
			Namespace: namespace,
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     roleName,
			APIGroup: rbacv1.GroupName,
		},
		Subjects: subjects,
	}
}

// NewClusterRole returns a new clusterrole
func NewClusterRole(name string, rules []rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Rules: rules,
	}
}

// NewClusterRoleBinding returns a new clusterrolebinding
func NewClusterRoleBinding(bindingName, roleName string, subjects []rbacv1.Subject) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: bindingName,
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     roleName,
			APIGroup: rbacv1.GroupName,
		},
		Subjects: subjects,
	}
}

// NewPolicyRule returns a new policyrule
func NewPolicyRule(apiGroups, resources, resourceNames, verbs []string, urls []string) rbacv1.PolicyRule {
	return rbacv1.PolicyRule{
		APIGroups:       apiGroups,
		Resources:       resources,
		ResourceNames:   resourceNames,
		Verbs:           verbs,
		NonResourceURLs: urls,
	}
}

// NewPolicyRules returns a slice of policyrule objects
func NewPolicyRules(rules ...rbacv1.PolicyRule) []rbacv1.PolicyRule {
	return rules
}

// NewSubject returns a new subject
func NewSubject(kind, name, namespace string) rbacv1.Subject {
	return rbacv1.Subject{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
		APIGroup:  rbacv1.GroupName,
	}
}

// NewSubjects returns a slice of subject objects
func NewSubjects(subjects ...rbacv1.Subject) []rbacv1.Subject {
	return subjects
}
