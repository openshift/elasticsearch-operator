package k8shandler

import (
	"context"
	"fmt"
	"reflect"

	"github.com/openshift/elasticsearch-operator/pkg/log"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/pkg/types/k8s"
	rbac "k8s.io/api/rbac/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (er *ElasticsearchRequest) CreateOrUpdateRBAC() error {

	dpl := er.cluster

	owner := getOwnerRef(dpl)

	// elasticsearch RBAC
	elasticsearchRole := newClusterRole(
		"elasticsearch-metrics",
		newPolicyRules(
			newPolicyRule(
				[]string{""},
				[]string{"pods", "services", "endpoints"},
				[]string{},
				[]string{"list", "watch"},
				[]string{},
			),
			newPolicyRule(
				[]string{},
				[]string{},
				[]string{},
				[]string{"get"},
				[]string{"/metrics"},
			),
		),
	)

	addOwnerRefToObject(elasticsearchRole, owner)

	if err := createOrUpdateClusterRole(elasticsearchRole, er.client); err != nil {
		return err
	}

	subject := newSubject(
		"ServiceAccount",
		"prometheus-k8s",
		"openshift-monitoring",
	)
	subject.APIGroup = ""

	elasticsearchRoleBinding := newClusterRoleBinding(
		"elasticsearch-metrics",
		"elasticsearch-metrics",
		newSubjects(
			subject,
		),
	)

	addOwnerRefToObject(elasticsearchRoleBinding, owner)

	if err := createOrUpdateClusterRoleBinding(elasticsearchRoleBinding, er.client); err != nil {
		return err
	}

	// proxy RBAC
	proxyRole := newClusterRole(
		"elasticsearch-proxy",
		newPolicyRules(
			newPolicyRule(
				[]string{"authentication.k8s.io"},
				[]string{"tokenreviews"},
				[]string{},
				[]string{"create"},
				[]string{},
			),
			newPolicyRule(
				[]string{"authorization.k8s.io"},
				[]string{"subjectaccessreviews"},
				[]string{},
				[]string{"create"},
				[]string{},
			),
		),
	)

	addOwnerRefToObject(proxyRole, owner)

	if err := createOrUpdateClusterRole(proxyRole, er.client); err != nil {
		return err
	}

	// Cluster role elasticsearch-proxy has to contain subjects for all ES instances
	esList := &v1.ElasticsearchList{}
	err := er.client.List(context.TODO(), esList)
	if err != nil {
		return err
	}

	subjects := []rbac.Subject{}
	for _, es := range esList.Items {
		subject = newSubject(
			"ServiceAccount",
			es.Name,
			es.Namespace,
		)
		subject.APIGroup = ""
		subjects = append(subjects, subject)
	}

	proxyRoleBinding := newClusterRoleBinding(
		"elasticsearch-proxy",
		"elasticsearch-proxy",
		subjects,
	)

	addOwnerRefToObject(proxyRoleBinding, owner)

	if err := createOrUpdateClusterRoleBinding(proxyRoleBinding, er.client); err != nil {
		return err
	}
	return reconcileIndexManagmentRbac(dpl, owner, er.client)
}

func createOrUpdateClusterRole(role *rbac.ClusterRole, client client.Client) error {
	if err := client.Create(context.TODO(), role); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create ClusterRole %s: %v", role.Name, err)
		}
		existingRole := role.DeepCopy()
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := client.Get(context.TODO(), types.NamespacedName{Name: existingRole.Name, Namespace: existingRole.Namespace}, existingRole); err != nil {
				return err
			}
			existingRole.Rules = role.Rules
			if err := client.Update(context.TODO(), existingRole); err != nil {
				return err
			}
			return nil
		})
	}
	return nil
}

func reconcileIndexManagmentRbac(cluster *v1.Elasticsearch, owner metav1.OwnerReference, client client.Client) error {
	role := k8s.NewRole(
		"elasticsearch-index-management",
		cluster.Namespace,
		newPolicyRules(
			newPolicyRule(
				[]string{"elasticsearch.openshift.io"},
				[]string{"indices"},
				[]string{},
				[]string{"*"},
				[]string{},
			),
		),
	)
	addOwnerRefToObject(role, owner)
	if err := reconcileRole(role, client); err != nil {
		return err
	}

	subject := newSubject(
		"ServiceAccount",
		cluster.Name,
		cluster.Namespace,
	)
	subject.APIGroup = ""
	rolebinding := k8s.NewRoleBinding(
		role.Name,
		role.Namespace,
		role.Name,
		newSubjects(subject),
	)
	addOwnerRefToObject(rolebinding, owner)
	return reconcileRoleBinding(rolebinding, client)
}

func reconcileRole(role *rbac.Role, client client.Client) error {
	err := client.Create(context.TODO(), role)
	if err == nil {
		return nil
	}
	if !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create Role %s/%s: %v", role.Namespace, role.Name, err)
	}
	current := &rbac.Role{}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: role.Namespace}, current); err != nil {
			log.Info("failed to get Role", "error", err)
			return err
		}
		if !reflect.DeepEqual(current.Rules, role.Rules) {
			if err := client.Update(context.TODO(), current); err != nil {
				log.Info("failed to update Role", "error", err)
				return err
			}
		}
		return nil
	})
}
func reconcileRoleBinding(rb *rbac.RoleBinding, client client.Client) error {
	ll := log.WithValues("namespace", rb.Namespace, "name", rb.Name)
	err := client.Create(context.TODO(), rb)
	if err == nil {
		return nil
	}
	if !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create RoleBinding %s/%s: %v", rb.Namespace, rb.Name, err)
	}
	current := &rbac.RoleBinding{}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := client.Get(context.TODO(), types.NamespacedName{Name: rb.Name, Namespace: rb.Namespace}, current); err != nil {
			ll.Info("could not get RoleBindng", "error", err)
			return err
		}
		if !reflect.DeepEqual(current.Subjects, rb.Subjects) {
			if err := client.Update(context.TODO(), current); err != nil {
				ll.Info("failed to update RoleBinding", "error", err)
				return err
			}
		}
		return nil
	})
}

func createOrUpdateClusterRoleBinding(roleBinding *rbac.ClusterRoleBinding, client client.Client) error {
	if err := client.Create(context.TODO(), roleBinding); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create ClusterRoleBindig %s: %v", roleBinding.Name, err)
		}
		existingRoleBinding := roleBinding.DeepCopy()
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if getErr := client.Get(context.TODO(), types.NamespacedName{Name: existingRoleBinding.Name, Namespace: existingRoleBinding.Namespace}, existingRoleBinding); getErr != nil {
				return fmt.Errorf("could not get ClusterRole %v: %v", existingRoleBinding.Name, getErr)
			}
			existingRoleBinding.Subjects = roleBinding.Subjects
			if updateErr := client.Update(context.TODO(), existingRoleBinding); updateErr != nil {
				return fmt.Errorf("failed to update ClusterRoleBinding %v status: %v", existingRoleBinding.Name, updateErr)
			}
			return nil
		})
	}
	return nil
}

func newPolicyRule(apiGroups, resources, resourceNames, verbs, urls []string) rbac.PolicyRule {
	return rbac.PolicyRule{
		APIGroups:       apiGroups,
		Resources:       resources,
		ResourceNames:   resourceNames,
		Verbs:           verbs,
		NonResourceURLs: urls,
	}
}

func newPolicyRules(rules ...rbac.PolicyRule) []rbac.PolicyRule {
	return rules
}

func newClusterRole(roleName string, rules []rbac.PolicyRule) *rbac.ClusterRole {
	return &rbac.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
		},
		Rules: rules,
	}
}

func newSubject(kind, name, namespace string) rbac.Subject {
	return rbac.Subject{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
		APIGroup:  rbac.GroupName,
	}
}

func newSubjects(subjects ...rbac.Subject) []rbac.Subject {
	return subjects
}

func newClusterRoleBinding(bindingName, roleName string, subjects []rbac.Subject) *rbac.ClusterRoleBinding {
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
