package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/elasticsearch-operator/test/utils"
)

var (
	unauthorizedSaName string
	authorizedSaName   string
	clusterRoleName    string
)

func TestElasticsearchOperatorMetrics(t *testing.T) {
	if unauthorizedSaName = os.Getenv("UNAUTHORIZED_SA"); unauthorizedSaName == "" {
		t.Fatal("UNAUTHORIZED_SA is unset")
	}

	if authorizedSaName = os.Getenv("AUTHORIZED_SA"); authorizedSaName == "" {
		t.Fatal("AUTHORIZED_SA is unset")
	}

	if clusterRoleName = os.Getenv("CLUSTERROLE"); clusterRoleName == "" {
		t.Fatal("CLUSTERROLE is unset")
	}

	setupK8sClient(t)

	// wait for elasticsearch-operator to be ready
	err := utils.WaitForDeployment(t, k8sClient, operatorNamespace, "elasticsearch-operator", 1, retryInterval, timeout)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("Operator metrics", operatorMetricsTest)
}

func operatorMetricsTest(t *testing.T) {
	// Deploy a single node cluster, wait for success
	esUUID := utils.GenerateUUID()
	t.Logf("Using UUID for elasticsearch CR: %v", esUUID)

	dataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for data nodes: %v", dataUUID)

	cr, err := createElasticsearchCR(t, k8sClient, esUUID, dataUUID, 1)
	if err != nil {
		t.Fatalf("could not create exampleElasticsearch: %v", err)
	}

	dplName := fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	err = utils.WaitForDeployment(t, k8sClient, operatorNamespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for first node deployment %v: %v", dplName, err)
	}
	matchingLabels := map[string]string{
		"cluster-name": cr.GetName(),
		"component":    "elasticsearch",
	}
	pods, err := utils.WaitForPods(t, k8sClient, operatorNamespace, matchingLabels, retryInterval, timeout)
	if err != nil {
		t.Fatalf("failed to wait for pods: %v", err)
	}

	// create two service accounts
	authorizedSA, authorizedSASecret, err := newServiceAccount(t, k8sClient, operatorNamespace, authorizedSaName)
	if err != nil {
		t.Fatal(err)
	}

	unauthorizedSA, unauthorizedSASecret, err := newServiceAccount(t, k8sClient, operatorNamespace, unauthorizedSaName)
	if err != nil {
		t.Fatal(err)
	}

	// Creating RBAC for authorised serviceaccount to verify metrics
	newClusterRole(t, k8sClient, clusterRoleName)

	bindClusterRoleWithSA(t, k8sClient, clusterRoleName, clusterRoleName, authorizedSA)
	bindClusterRoleWithSA(t, k8sClient, "system:basic-user", "view-"+clusterRoleName, authorizedSA)

	//  Creating RBAC for unauthorised serviceaccount to verify metrics
	bindClusterRoleWithSA(t, k8sClient, "system:basic-user", "view-"+clusterRoleName+"-unauth", unauthorizedSA)

	// get serviceAccount token
	var getSaToken func(secret *corev1.Secret) string
	getSaToken = func(secret *corev1.Secret) string {
		s := &corev1.Secret{}
		key := client.ObjectKeyFromObject(secret)

		err := k8sClient.Get(context.TODO(), key, s)
		if err != nil {
			return ""
		}

		return string(s.Data["token"])
	}

	podName := pods.Items[0].GetName()
	token := getSaToken(authorizedSASecret)
	if token == "" {
		t.Errorf("secret token not exist for %s", authorizedSaName)
	}
	cmd := fmt.Sprintf("curl -ks -o /tmp/mymetrics.txt https://%s-metrics.%s.svc:60001/_prometheus/metrics -H Authorization:'Bearer %s' -w '%%{response_code}\\n'", cr.GetName(), operatorNamespace, token)
	code, _, err := ExecInPod(k8sConfig, operatorNamespace, podName, cmd, "elasticsearch")
	if code != "200" {
		t.Error("Authorized service account should have access to es metrics", "error", err)
	}
	token = getSaToken(unauthorizedSASecret)
	if token == "" {
		t.Errorf("secret token not exist for %s", unauthorizedSaName)
	}
	cmd = fmt.Sprintf("curl -ks -o /tmp/mymetrics.txt https://%s-metrics.%s.svc:60001/_prometheus/metrics -H Authorization:'Bearer %s' -w '%%{response_code}\\n'", cr.GetName(), operatorNamespace, token)
	code, _, err = ExecInPod(k8sConfig, operatorNamespace, podName, cmd, "elasticsearch")
	if code != "403" {
		t.Error("Unauthorized service account must not have access to es metrics", "error", err)
	}

	// cleanup
	cleanupEsTest(t, k8sClient, operatorNamespace, esUUID)
	// Delete authorizedSA, authorizedSAToken, unauthorizedSA, unauthorizedSAToken, 3 clusterRoleName
	cleanupSaRoles(t, k8sClient, operatorNamespace, esUUID)
	t.Log("Finished successfully")
}

func cleanupSaRoles(t *testing.T, cli client.Client, namespace string, esUUID string) {
	// remove role bindings
	for _, cbrName := range []string{clusterRoleName, "view-" + clusterRoleName, "view-" + clusterRoleName + "-unauth"} {
		cbr := &rbac.ClusterRoleBinding{}
		key := types.NamespacedName{Name: cbrName}
		err := waitForDeleteObject(t, cli, key, cbr, retryInterval, timeout)
		if err != nil {
			t.Errorf("cannot remove ClusterRoleBinding %s: %v", cbrName, err)
		}
	}

	// remove cluster roles
	cr := &rbac.ClusterRole{}
	key := types.NamespacedName{Name: clusterRoleName}
	err := waitForDeleteObject(t, cli, key, cr, retryInterval, timeout)
	if err != nil {
		t.Errorf("cannot remove ClusterRole: %v", err)
	}

	for _, saName := range []string{authorizedSaName, unauthorizedSaName} {
		// remove service account
		sa := &corev1.ServiceAccount{}
		key = types.NamespacedName{Name: saName, Namespace: operatorNamespace}
		err := waitForDeleteObject(t, cli, key, sa, retryInterval, timeout)
		if err != nil {
			t.Errorf("cannot remove service account %s: %v", saName, err)
		}

		// remove service account token secret
		s := &corev1.Secret{}
		name := fmt.Sprintf("%s-token", saName)
		key := client.ObjectKey{Name: name, Namespace: operatorNamespace}
		err = waitForDeleteObject(t, cli, key, s, retryInterval, timeout)
		if err != nil {
			t.Errorf("cannot remove service account token secret %s: %v", name, err)
		}
	}
}

func newServiceAccount(t *testing.T, c client.Client, namespace, name string) (*corev1.ServiceAccount, *corev1.Secret, error) {
	sa := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	err := c.Create(context.TODO(), sa)
	if err != nil {
		return nil, nil, err
	}

	key := client.ObjectKeyFromObject(sa)
	err = c.Get(context.TODO(), key, sa)
	if err != nil {
		return nil, nil, err
	}

	saTokenSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-token", name),
			Namespace: namespace,
			Annotations: map[string]string{
				corev1.ServiceAccountNameKey: sa.Name,
				corev1.ServiceAccountUIDKey:  string(sa.UID),
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}

	err = c.Create(context.TODO(), saTokenSecret)
	if err != nil {
		return nil, nil, err
	}

	return sa, saTokenSecret, nil
}

func newClusterRole(t *testing.T, client client.Client, name string) {
	cr := &rbac.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{"elasticsearch.openshift.io"},
				Resources: []string{"metrics"},
				Verbs:     []string{"get"},
			},
		},
	}

	err := client.Create(context.TODO(), cr)
	if err != nil {
		t.Logf("Unable to create cluster role due to error: %v", err)
	}
}

func bindClusterRoleWithSA(t *testing.T, client client.Client, roleName, name string, sa *corev1.ServiceAccount) {
	crb := &rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Subjects: []rbac.Subject{{
			Kind:      "ServiceAccount",
			Name:      sa.GetName(),
			Namespace: sa.GetNamespace(),
		}},
		RoleRef: rbac.RoleRef{
			Kind: "ClusterRole",
			Name: roleName,
		},
	}

	err := client.Create(context.TODO(), crb)
	if err != nil {
		t.Logf("Unable to create cluster role binding due to error: %v", err)
	}
}
