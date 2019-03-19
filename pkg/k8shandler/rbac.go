package k8shandler

import (
	"fmt"

	v1alpha1 "github.com/openshift/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	"github.com/openshift/elasticsearch-operator/pkg/utils"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	rbac "k8s.io/api/rbac/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
)

func CreateOrUpdateRBAC(dpl *v1alpha1.Elasticsearch) error {

	owner := asOwner(dpl)

	// elasticsearch RBAC
	elasticsearchRole := utils.NewClusterRole(
		"elasticsearch-metrics",
		utils.NewPolicyRules(
			utils.NewPolicyRule(
				[]string{""},
				[]string{"pods", "services", "endpoints"},
				[]string{},
				[]string{"list", "watch"},
				[]string{},
			),
			utils.NewPolicyRule(
				[]string{},
				[]string{},
				[]string{},
				[]string{"get"},
				[]string{"/metrics"},
			),
		),
	)

	addOwnerRefToObject(elasticsearchRole, owner)

	if err := createOrUpdateClusterRole(elasticsearchRole); err != nil {
		return err
	}

	subject := utils.NewSubject(
		"ServiceAccount",
		"prometheus-k8s",
		"openshift-monitoring",
	)
	subject.APIGroup = ""

	elasticsearchRoleBinding := utils.NewClusterRoleBinding(
		"elasticsearch-metrics",
		"elasticsearch-metrics",
		utils.NewSubjects(
			subject,
		),
	)

	addOwnerRefToObject(elasticsearchRoleBinding, owner)

	if err := createOrUpdateClusterRoleBinding(elasticsearchRoleBinding); err != nil {
		return err
	}

	// proxy RBAC
	proxyRole := utils.NewClusterRole(
		"elasticsearch-proxy",
		utils.NewPolicyRules(
			utils.NewPolicyRule(
				[]string{"authentication.k8s.io"},
				[]string{"tokenreviews"},
				[]string{},
				[]string{"create"},
				[]string{},
			),
			utils.NewPolicyRule(
				[]string{"authorization.k8s.io"},
				[]string{"subjectaccessreviews"},
				[]string{},
				[]string{"create"},
				[]string{},
			),
		),
	)

	addOwnerRefToObject(proxyRole, owner)

	if err := createOrUpdateClusterRole(proxyRole); err != nil {
		return err
	}

	subject = utils.NewSubject(
		"ServiceAccount",
		"elasticsearch",
		dpl.Namespace,
	)
	subject.APIGroup = ""

	proxyRoleBinding := utils.NewClusterRoleBinding(
		"elasticsearch-proxy",
		"elasticsearch-proxy",
		utils.NewSubjects(
			subject,
		),
	)

	addOwnerRefToObject(proxyRoleBinding, owner)

	return createOrUpdateClusterRoleBinding(proxyRoleBinding)
}

func createOrUpdateClusterRole(role *rbac.ClusterRole) error {
	if err := sdk.Create(role); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create ClusterRole %s: %v", role.Name, err)
		}
		existingRole := role.DeepCopy()
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if getErr := sdk.Get(existingRole); getErr != nil {
				logrus.Debugf("could not get ClusterRole %v: %v", existingRole.Name, getErr)
				return getErr
			}
			existingRole.Rules = role.Rules
			if updateErr := sdk.Update(existingRole); updateErr != nil {
				logrus.Debugf("failed to update ClusterRole %v status: %v", existingRole.Name, updateErr)
				return updateErr
			}
			return nil
		})
	}
	return nil
}

func createOrUpdateClusterRoleBinding(roleBinding *rbac.ClusterRoleBinding) error {
	if err := sdk.Create(roleBinding); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create ClusterRoleBindig %s: %v", roleBinding.Name, err)
		}
		existingRoleBinding := roleBinding.DeepCopy()
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if getErr := sdk.Get(existingRoleBinding); getErr != nil {
				return fmt.Errorf("could not get ClusterRole %v: %v", existingRoleBinding.Name, getErr)
			}
			existingRoleBinding.Subjects = roleBinding.Subjects
			if updateErr := sdk.Update(existingRoleBinding); updateErr != nil {
				return fmt.Errorf("failed to update ClusterRoleBinding %v status: %v", existingRoleBinding.Name, updateErr)
			}
			return nil
		})
	}
	return nil
}
