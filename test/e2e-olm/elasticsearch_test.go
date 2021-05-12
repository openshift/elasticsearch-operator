package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	loggingv1 "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/test/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
)

func TestElasticsearchCluster(t *testing.T) {
	setupK8sClient(t)
	t.Run("Single node", singleNodeTest)
	t.Run("Multiple nodes", multipleNodesTest)
	t.Run("Scale up nodes", scaleUpNodesTest)
	t.Run("Multiple nodes with a single non-data node", multipleNodesWithNonDataNodeTest)
	t.Run("Full cluster redeploy", fullClusterRedeployTest)
	t.Run("Rolling restart", rollingRestartTest)
	t.Run("Invalid master count", invalidMasterCountTest)
}

func singleNodeTest(t *testing.T) {
	namespace := operatorNamespace
	t.Logf("Found namespace: %v", namespace)

	esUUID := utils.GenerateUUID()
	t.Logf("Using UUID for elasticsearch CR: %v", esUUID)

	dataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for data nodes: %v", dataUUID)

	// Create CR with a single node with client, data and master roles
	_, err := createElasticsearchCR(t, k8sClient, esUUID, dataUUID, 1)
	if err != nil {
		t.Fatalf("could not create exampleElasticsearch: %v", err)
	}

	dplName := fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	err = utils.WaitForDeployment(t, k8sClient, namespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for first node deployment %v: %v", dplName, err)
	}

	cleanupEsTest(t, k8sClient, namespace, esUUID)
	t.Log("SingleNodeTest finished successfully")
}

func cleanupEsTest(t *testing.T, cli client.Client, namespace string, esUUID string) {
	esSecret := &corev1.Secret{}
	key := types.NamespacedName{Name: elasticsearchNameFor(esUUID), Namespace: namespace}

	es := &loggingv1.Elasticsearch{}
	err := waitForDeleteObject(t, cli, key, es, retryInterval, timeout)
	if err != nil {
		t.Errorf("cannot remove elasticsearch CR: %v", err)
	}

	err = waitForDeleteObject(t, cli, key, esSecret, retryInterval, timeout)
	if err != nil {
		t.Errorf("cannot remove es secret: %v", err)
	}

}

func multipleNodesTest(t *testing.T) {
	esUUID := utils.GenerateUUID()
	t.Logf("Using UUID for elasticsearch CR: %v", esUUID)

	dataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for data nodes: %v", dataUUID)

	// Create CR with two nodes sharing client, data and master roles
	cr, err := createElasticsearchCR(t, k8sClient, esUUID, dataUUID, 2)
	if err != nil {
		t.Fatalf("could not create exampleElasticsearch: %v", err)
	}

	dplName := fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	err = utils.WaitForDeployment(t, k8sClient, operatorNamespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for first node deployment %v: %v", dplName, err)
	}

	dplName = fmt.Sprintf("elasticsearch-%v-cdm-%v-2", esUUID, dataUUID)
	err = utils.WaitForDeployment(t, k8sClient, operatorNamespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for second node deployment %v: %v", dplName, err)
	}

	cleanupEsTest(t, k8sClient, cr.Namespace, esUUID)
	t.Log("MultipleNodesTest finished successfully")
}

func scaleUpNodesTest(t *testing.T) {
	esUUID := utils.GenerateUUID()
	t.Logf("Using UUID for elasticsearch CR: %v", esUUID)

	dataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for data nodes: %v", dataUUID)

	// Create CR with one node sharing client, data and master roles
	cr, err := createElasticsearchCR(t, k8sClient, esUUID, dataUUID, 1)
	if err != nil {
		t.Fatalf("could not create exampleElasticsearch: %v", err)
	}

	dplName := fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	err = utils.WaitForDeployment(t, k8sClient, operatorNamespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for first node deployment %v: %v", dplName, err)
	}

	t.Log("Adding a new data node")
	cr.Spec.Nodes[0].NodeCount = int32(2)

	if err := updateElasticsearchSpec(t, k8sClient, cr); err != nil {
		t.Fatalf("could not update elasticsearch CR with an additional data node: %v", err)
	}

	dplName = fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	err = utils.WaitForDeployment(t, k8sClient, operatorNamespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for first node deployment %v: %v", dplName, err)
	}

	dplName = fmt.Sprintf("elasticsearch-%v-cdm-%v-2", esUUID, dataUUID)
	err = utils.WaitForDeployment(t, k8sClient, operatorNamespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for second node deployment %v: %v", dplName, err)
	}

	cleanupEsTest(t, k8sClient, cr.Namespace, esUUID)
	t.Log("ScaleUpNodesTest finished successfully")
}

func multipleNodesWithNonDataNodeTest(t *testing.T) {
	esUUID := utils.GenerateUUID()
	t.Logf("Using UUID for elasticsearch CR: %v", esUUID)

	dataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for data nodes: %v", dataUUID)

	// Create CR with two nodes sharing client, data and master roles
	cr, err := createElasticsearchCR(t, k8sClient, esUUID, dataUUID, 2)
	if err != nil {
		t.Fatalf("could not create exampleElasticsearch: %v", err)
	}

	dplName := fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	err = utils.WaitForDeployment(t, k8sClient, operatorNamespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for first node deployment %v: %v", dplName, err)
	}

	dplName = fmt.Sprintf("elasticsearch-%v-cdm-%v-2", esUUID, dataUUID)
	err = utils.WaitForDeployment(t, k8sClient, operatorNamespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for second node deployment %v: %v", dplName, err)
	}

	nonDataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for non data nodes: %v", nonDataUUID)

	storageClassSize := resource.MustParse("2G")

	esNonDataNode := loggingv1.ElasticsearchNode{
		Roles: []loggingv1.ElasticsearchNodeRole{
			loggingv1.ElasticsearchRoleClient,
			loggingv1.ElasticsearchRoleMaster,
		},
		NodeCount: int32(1),
		Storage: loggingv1.ElasticsearchStorageSpec{
			Size: &storageClassSize,
		},
		GenUUID: &nonDataUUID,
	}

	t.Log("Adding non-data node")
	cr.Spec.Nodes = append(cr.Spec.Nodes, esNonDataNode)

	if err := updateElasticsearchSpec(t, k8sClient, cr); err != nil {
		t.Fatalf("could not update elasticsearch CR with an additional non-data node: %v", err)
	}

	statefulSetName := fmt.Sprintf("elasticsearch-%v-cm-%v", esUUID, nonDataUUID)
	err = utils.WaitForStatefulset(t, k8sClient, operatorNamespace, statefulSetName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for non-data node %v: %v", statefulSetName, err)
	}
	t.Log("Created non-data statefulset")

	cleanupEsTest(t, k8sClient, cr.Namespace, esUUID)
	t.Log("MultipleNodesWithNonDataNode test finished successfully")
}

func fullClusterRedeployTest(t *testing.T) {
	esUUID := utils.GenerateUUID()
	t.Logf("Using UUID for elasticsearch CR: %v", esUUID)

	dataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for data nodes: %v", dataUUID)

	// Create CR with two nodes sharing client, data and master roles
	cr, err := createElasticsearchCR(t, k8sClient, esUUID, dataUUID, 2)
	if err != nil {
		t.Fatalf("could not create exampleElasticsearch: %v", err)
	}

	dplName := fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	err = utils.WaitForDeployment(t, k8sClient, operatorNamespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for first node deployment %v: %v", dplName, err)
	}

	dplName = fmt.Sprintf("elasticsearch-%v-cdm-%v-2", esUUID, dataUUID)
	err = utils.WaitForDeployment(t, k8sClient, operatorNamespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for second node deployment %v: %v", dplName, err)
	}

	matchingLabels := map[string]string{
		"cluster-name": cr.GetName(),
		"component":    "elasticsearch",
	}

	initialPods, err := utils.WaitForPods(t, k8sClient, operatorNamespace, matchingLabels, retryInterval, timeout)
	if err != nil {
		t.Fatalf("failed to wait for pods: %v", err)
	}

	var initPodNames []string
	for _, pod := range initialPods.Items {
		initPodNames = append(initPodNames, pod.GetName())
	}
	t.Logf("Cluster pods before full cluster redeploy: %v", initPodNames)

	// Scale up to SingleRedundancy
	cr.Spec.RedundancyPolicy = loggingv1.SingleRedundancy

	t.Logf("Updating redundancy policy to %v", cr.Spec.RedundancyPolicy)
	if err := updateElasticsearchSpec(t, k8sClient, cr); err != nil {
		t.Fatalf("could not update elasticsearch CR to be SingleRedundancy: %v", err)
	}

	// Update the secret to force a full cluster redeploy
	err = updateElasticsearchSecret(t, k8sClient, esUUID)
	if err != nil {
		t.Fatalf("Unable to update secret")
	}

	t.Log("Waiting for redeployment after secret update")
	time.Sleep(time.Second * 60) // Let the operator do his thing

	// Increase redeploy timeout on full cluster redeploy until min masters available
	redeployTimeout := time.Second * 600

	dplName = fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	err = utils.WaitForReadyDeployment(t, k8sClient, operatorNamespace, dplName, 1, retryInterval, redeployTimeout)
	if err != nil {
		t.Fatalf("timed out waiting for first node deployment %v: %v", dplName, err)
	}

	dplName = fmt.Sprintf("elasticsearch-%v-cdm-%v-2", esUUID, dataUUID)
	err = utils.WaitForReadyDeployment(t, k8sClient, operatorNamespace, dplName, 1, retryInterval, redeployTimeout)
	if err != nil {
		t.Fatalf("timed out waiting for second node deployment %v: %v", dplName, err)
	}

	pods, err := utils.WaitForRolloutComplete(t, k8sClient, operatorNamespace, matchingLabels, initPodNames, 2, retryInterval, redeployTimeout)
	if err != nil {
		t.Fatal(err)
	}

	var podNames []string
	for _, pod := range pods.Items {
		podNames = append(podNames, pod.GetName())
	}
	t.Logf("Cluster pods after full cluster redeploy: %v", podNames)

	if len(pods.Items) != 2 {
		t.Fatalf("No matching pods found for labels: %#v", matchingLabels)
	}

	cleanupEsTest(t, k8sClient, cr.Namespace, esUUID)
	t.Log("fullClusterRedeploy test finished successfully")
}

func rollingRestartTest(t *testing.T) {
	esUUID := utils.GenerateUUID()
	t.Logf("Using UUID for elasticsearch CR: %v", esUUID)

	dataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for data nodes: %v", dataUUID)

	// Create CR with two nodes sharing client, data and master roles
	cr, err := createElasticsearchCR(t, k8sClient, esUUID, dataUUID, 2)
	if err != nil {
		t.Fatalf("could not create exampleElasticsearch: %v", err)
	}

	dplName := fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	err = utils.WaitForDeployment(t, k8sClient, operatorNamespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for first node deployment %v: %v", dplName, err)
	}

	dplName = fmt.Sprintf("elasticsearch-%v-cdm-%v-2", esUUID, dataUUID)
	err = utils.WaitForDeployment(t, k8sClient, operatorNamespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for second node deployment %v: %v", dplName, err)
	}

	matchingLabels := map[string]string{
		"cluster-name": cr.GetName(),
		"component":    "elasticsearch",
	}

	initialPods, err := utils.WaitForPods(t, k8sClient, operatorNamespace, matchingLabels, retryInterval, timeout)
	if err != nil {
		t.Fatalf("failed to wait for pods: %v", err)
	}

	var initPodNames []string
	for _, pod := range initialPods.Items {
		initPodNames = append(initPodNames, pod.GetName())
	}
	t.Logf("Cluster pods before rolling restart: %v", initPodNames)

	// Update the resource spec for the cluster
	oldMemValue := cr.Spec.Spec.Resources.Limits.Memory()

	memValue := cr.Spec.Spec.Resources.Requests.Memory().DeepCopy()
	memValue.Add(resource.MustParse("1Mi"))
	cpuValue := cr.Spec.Spec.Resources.Requests.Cpu().DeepCopy()
	cpuValue.Add(resource.MustParse("1m"))

	desiredResources := corev1.ResourceRequirements{
		Limits: cr.Spec.Spec.Resources.Limits,
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    cpuValue,
			corev1.ResourceMemory: memValue,
		},
	}

	t.Log("Updating Limits.Memory and Requests.Memory to trigger a rolling restart")
	t.Logf("Updating from %s to %s", oldMemValue.String(), memValue.String())

	cr.Spec.Spec.Resources = desiredResources
	if err := updateElasticsearchSpec(t, k8sClient, cr); err != nil {
		t.Fatalf("could not update elasticsearch CR to be SingleRedundancy: %v", err)
	}

	t.Log("Waiting for restart after resource requests/limits update")

	// Increase restart timeout on full cluster redeploy until min masters available
	restartTimeout := time.Second * 600

	dplName = fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	err = utils.WaitForReadyDeployment(t, k8sClient, operatorNamespace, dplName, 1, retryInterval, restartTimeout)
	if err != nil {
		t.Fatalf("timed out waiting for first ready node deployment  %v: %v", dplName, err)
	}

	dplName = fmt.Sprintf("elasticsearch-%v-cdm-%v-2", esUUID, dataUUID)
	err = utils.WaitForReadyDeployment(t, k8sClient, operatorNamespace, dplName, 1, retryInterval, restartTimeout)
	if err != nil {
		t.Fatalf("timed out waiting for second ready node deployment %v: %v", dplName, err)
	}

	pods, err := utils.WaitForRolloutComplete(t, k8sClient, operatorNamespace, matchingLabels, initPodNames, 2, retryInterval, restartTimeout)
	if err != nil {
		t.Fatal(err)
	}

	var podNames []string
	for _, pod := range pods.Items {
		podNames = append(podNames, pod.GetName())
	}
	t.Logf("Cluster pods after rolling restart: %v", podNames)

	if len(pods.Items) != 2 {
		t.Fatalf("No matching pods found for labels: %#v", matchingLabels)
	}

	for _, pod := range pods.Items {
		if diff := cmp.Diff(pod.Spec.Containers[0].Resources, desiredResources); diff != "" {
			t.Errorf("failed to match pods with resources:\n%s", diff)
		}
	}

	cleanupEsTest(t, k8sClient, cr.Namespace, esUUID)
	t.Log("RollingRestart test finished successfully")
}

func invalidMasterCountTest(t *testing.T) {
	esUUID := utils.GenerateUUID()
	t.Logf("Using UUID for elasticsearch CR: %v", esUUID)

	dataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for data nodes: %v", dataUUID)

	// Create CR with invalid case: four nodes all sharing client, data and master roles
	cr, err := createElasticsearchCR(t, k8sClient, esUUID, dataUUID, 4)
	if err != nil {
		t.Fatalf("could not create exampleElasticsearch: %v", err)
	}

	key := client.ObjectKey{Name: cr.GetName(), Namespace: cr.GetNamespace()}
	if err := k8sClient.Get(context.TODO(), key, cr); err != nil {
		t.Fatalf("failed to get updated CR: %s", key)
	}

	for _, condition := range cr.Status.Conditions {
		if condition.Type == loggingv1.InvalidMasters {
			if condition.Status == corev1.ConditionFalse ||
				condition.Status == "" {
				t.Errorf("unexpected status condition for elasticsearch found: %v", condition.Status)
			}
		}
	}

	cleanupEsTest(t, k8sClient, cr.Namespace, esUUID)
	t.Log("invalidMasterCount test finished successfully")
}
