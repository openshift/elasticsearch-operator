package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	loggingv1 "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/test/utils"

	"github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestElasticsearchCluster(t *testing.T) {
	registerSchemes(t)
	t.Run("Single node", singleNodeTest)
	t.Run("Multiple nodes", multipleNodesTest)
	t.Run("Scale up nodes", scaleUpNodesTest)
	t.Run("Multiple nodes with a single non-data node", multipleNodesWithNonDataNodeTest)
	t.Run("Full cluster redeploy", fullClusterRedeployTest)
	t.Run("Rolling restart", rollingRestartTest)
	t.Run("Invalid master count", invalidMasterCountTest)
}

func singleNodeTest(t *testing.T) {
	f := test.Global

	ctx := test.NewContext(t)
	namespace, err := ctx.GetWatchNamespace()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Found namespace: %v", namespace)

	esUUID := utils.GenerateUUID()
	t.Logf("Using UUID for elasticsearch CR: %v", esUUID)

	if err = createElasticsearchSecret(t, f, ctx, esUUID); err != nil {
		t.Fatal(err)
	}

	dataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for data nodes: %v", dataUUID)

	// Create CR with a single node with client, data and master roles
	cr, err := createElasticsearchCR(t, f, ctx, esUUID, dataUUID, 1)
	if err != nil {
		t.Fatalf("could not create exampleElasticsearch: %v", err)
	}

	dplName := fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for first node deployment %v: %v", dplName, err)
	}

	ctx.Cleanup()
	e2eutil.WaitForDeletion(t, f.Client.Client, cr, cleanupRetryInterval, cleanupTimeout)
	t.Log("Finished successfully")
}

func multipleNodesTest(t *testing.T) {
	f := test.Global

	ctx := test.NewContext(t)
	namespace, err := ctx.GetWatchNamespace()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Found namespace: %v", namespace)

	esUUID := utils.GenerateUUID()
	t.Logf("Using UUID for elasticsearch CR: %v", esUUID)

	if err = createElasticsearchSecret(t, f, ctx, esUUID); err != nil {
		t.Fatal(err)
	}

	dataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for data nodes: %v", dataUUID)

	// Create CR with two nodes sharing client, data and master roles
	cr, err := createElasticsearchCR(t, f, ctx, esUUID, dataUUID, 2)
	if err != nil {
		t.Fatalf("could not create exampleElasticsearch: %v", err)
	}

	dplName := fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for first node deployment %v: %v", dplName, err)
	}

	dplName = fmt.Sprintf("elasticsearch-%v-cdm-%v-2", esUUID, dataUUID)
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for second node deployment %v: %v", dplName, err)
	}

	ctx.Cleanup()
	e2eutil.WaitForDeletion(t, f.Client.Client, cr, cleanupRetryInterval, cleanupTimeout)
	t.Log("Finished successfully")
}

func scaleUpNodesTest(t *testing.T) {
	f := test.Global

	ctx := test.NewContext(t)
	namespace, err := ctx.GetWatchNamespace()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Found namespace: %v", namespace)

	esUUID := utils.GenerateUUID()
	t.Logf("Using UUID for elasticsearch CR: %v", esUUID)

	if err = createElasticsearchSecret(t, f, ctx, esUUID); err != nil {
		t.Fatal(err)
	}

	dataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for data nodes: %v", dataUUID)

	// Create CR with one node sharing client, data and master roles
	cr, err := createElasticsearchCR(t, f, ctx, esUUID, dataUUID, 1)
	if err != nil {
		t.Fatalf("could not create exampleElasticsearch: %v", err)
	}

	dplName := fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for first node deployment %v: %v", dplName, err)
	}

	t.Log("Adding a new data node")
	cr.Spec.Nodes[0].NodeCount = int32(2)

	if err := updateElasticsearchSpec(t, f, cr); err != nil {
		t.Fatalf("could not update elasticsearch CR with an additional data node: %v", err)
	}

	dplName = fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for first node deployment %v: %v", dplName, err)
	}

	dplName = fmt.Sprintf("elasticsearch-%v-cdm-%v-2", esUUID, dataUUID)
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for second node deployment %v: %v", dplName, err)
	}

	ctx.Cleanup()
	e2eutil.WaitForDeletion(t, f.Client.Client, cr, cleanupRetryInterval, cleanupTimeout)
	t.Log("Finished successfully")
}

func multipleNodesWithNonDataNodeTest(t *testing.T) {
	f := test.Global

	ctx := test.NewContext(t)
	namespace, err := ctx.GetWatchNamespace()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Found namespace: %v", namespace)

	esUUID := utils.GenerateUUID()
	t.Logf("Using UUID for elasticsearch CR: %v", esUUID)

	if err = createElasticsearchSecret(t, f, ctx, esUUID); err != nil {
		t.Fatal(err)
	}

	dataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for data nodes: %v", dataUUID)

	// Create CR with two nodes sharing client, data and master roles
	cr, err := createElasticsearchCR(t, f, ctx, esUUID, dataUUID, 2)
	if err != nil {
		t.Fatalf("could not create exampleElasticsearch: %v", err)
	}

	dplName := fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for first node deployment %v: %v", dplName, err)
	}

	dplName = fmt.Sprintf("elasticsearch-%v-cdm-%v-2", esUUID, dataUUID)
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for second node deployment %v: %v", dplName, err)
	}

	nonDataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for non data nodes: %v", nonDataUUID)

	storageClassName := "gp2"
	storageClassSize := resource.MustParse("2G")

	esNonDataNode := loggingv1.ElasticsearchNode{
		Roles: []loggingv1.ElasticsearchNodeRole{
			loggingv1.ElasticsearchRoleClient,
			loggingv1.ElasticsearchRoleMaster,
		},
		NodeCount: int32(1),
		Storage: loggingv1.ElasticsearchStorageSpec{
			StorageClassName: &storageClassName,
			Size:             &storageClassSize,
		},
		GenUUID: &nonDataUUID,
	}

	t.Log("Adding non-data node")
	cr.Spec.Nodes = append(cr.Spec.Nodes, esNonDataNode)

	if err := updateElasticsearchSpec(t, f, cr); err != nil {
		t.Fatalf("could not update elasticsearch CR with an additional non-data node: %v", err)
	}

	statefulSetName := fmt.Sprintf("elasticsearch-%v-cm-%v", esUUID, nonDataUUID)
	err = utils.WaitForStatefulset(t, f.KubeClient, namespace, statefulSetName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for non-data node %v: %v", statefulSetName, err)
	}
	t.Log("Created non-data statefulset")

	ctx.Cleanup()
	e2eutil.WaitForDeletion(t, f.Client.Client, cr, cleanupRetryInterval, cleanupTimeout)
	t.Log("Finished successfully")
}

func fullClusterRedeployTest(t *testing.T) {
	f := test.Global

	ctx := test.NewContext(t)
	namespace, err := ctx.GetWatchNamespace()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Found namespace: %v", namespace)

	esUUID := utils.GenerateUUID()
	t.Logf("Using UUID for elasticsearch CR: %v", esUUID)

	if err = createElasticsearchSecret(t, f, ctx, esUUID); err != nil {
		t.Fatal(err)
	}

	dataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for data nodes: %v", dataUUID)

	// Create CR with two nodes sharing client, data and master roles
	cr, err := createElasticsearchCR(t, f, ctx, esUUID, dataUUID, 2)
	if err != nil {
		t.Fatalf("could not create exampleElasticsearch: %v", err)
	}

	dplName := fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for first node deployment %v: %v", dplName, err)
	}

	dplName = fmt.Sprintf("elasticsearch-%v-cdm-%v-2", esUUID, dataUUID)
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for second node deployment %v: %v", dplName, err)
	}

	matchingLabels := map[string]string{
		"cluster-name": cr.GetName(),
		"component":    "elasticsearch",
	}

	initialPods, err := utils.WaitForPods(t, f, namespace, matchingLabels, retryInterval, timeout)
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
	if err := updateElasticsearchSpec(t, f, cr); err != nil {
		t.Fatalf("could not update elasticsearch CR to be SingleRedundancy: %v", err)
	}

	// Update the secret to force a full cluster redeploy
	err = updateElasticsearchSecret(t, f, ctx, esUUID)
	if err != nil {
		t.Fatalf("Unable to update secret")
	}

	t.Log("Waiting for redeployment after secret update")
	time.Sleep(time.Second * 10) // Let the operator do his thing

	// Increase redeploy timeout on full cluster redeploy until min masters available
	redeployTimeout := time.Second * 600

	dplName = fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	err = utils.WaitForReadyDeployment(t, f.KubeClient, namespace, dplName, 1, retryInterval, redeployTimeout)
	if err != nil {
		t.Fatalf("timed out waiting for first node deployment %v: %v", dplName, err)
	}

	dplName = fmt.Sprintf("elasticsearch-%v-cdm-%v-2", esUUID, dataUUID)
	err = utils.WaitForReadyDeployment(t, f.KubeClient, namespace, dplName, 1, retryInterval, redeployTimeout)
	if err != nil {
		t.Fatalf("timed out waiting for second node deployment %v: %v", dplName, err)
	}

	pods, err := utils.WaitForRolloutComplete(t, f, namespace, matchingLabels, initPodNames, 2, retryInterval, redeployTimeout)
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

	ctx.Cleanup()
	e2eutil.WaitForDeletion(t, f.Client.Client, cr, cleanupRetryInterval, cleanupTimeout)
	t.Log("Finished successfully")
}

func rollingRestartTest(t *testing.T) {
	f := test.Global

	ctx := test.NewContext(t)
	namespace, err := ctx.GetWatchNamespace()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Found namespace: %v", namespace)

	esUUID := utils.GenerateUUID()
	t.Logf("Using UUID for elasticsearch CR: %v", esUUID)

	if err = createElasticsearchSecret(t, f, ctx, esUUID); err != nil {
		t.Fatal(err)
	}

	dataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for data nodes: %v", dataUUID)

	// Create CR with two nodes sharing client, data and master roles
	cr, err := createElasticsearchCR(t, f, ctx, esUUID, dataUUID, 2)
	if err != nil {
		t.Fatalf("could not create exampleElasticsearch: %v", err)
	}

	dplName := fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for first node deployment %v: %v", dplName, err)
	}

	dplName = fmt.Sprintf("elasticsearch-%v-cdm-%v-2", esUUID, dataUUID)
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, dplName, 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("timed out waiting for second node deployment %v: %v", dplName, err)
	}

	matchingLabels := map[string]string{
		"cluster-name": cr.GetName(),
		"component":    "elasticsearch",
	}

	initialPods, err := utils.WaitForPods(t, f, namespace, matchingLabels, retryInterval, timeout)
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
	if err := updateElasticsearchSpec(t, f, cr); err != nil {
		t.Fatalf("could not update elasticsearch CR to be SingleRedundancy: %v", err)
	}

	t.Log("Waiting for restart after resource requests/limits update")

	// Increase restart timeout on full cluster redeploy until min masters available
	restartTimeout := time.Second * 600

	dplName = fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	err = utils.WaitForReadyDeployment(t, f.KubeClient, namespace, dplName, 1, retryInterval, restartTimeout)
	if err != nil {
		t.Fatalf("timed out waiting for first ready node deployment  %v: %v", dplName, err)
	}

	dplName = fmt.Sprintf("elasticsearch-%v-cdm-%v-2", esUUID, dataUUID)
	err = utils.WaitForReadyDeployment(t, f.KubeClient, namespace, dplName, 1, retryInterval, restartTimeout)
	if err != nil {
		t.Fatalf("timed out waiting for second ready node deployment %v: %v", dplName, err)
	}

	// tests are failing here --
	// it can't be the function because its also used for fullcluster restart...
	pods, err := utils.WaitForRolloutComplete(t, f, namespace, matchingLabels, initPodNames, 2, retryInterval, restartTimeout)

	t.Logf("pods returned while waiting for rollout: %v", pods)
	t.Logf("received error while waiting for rollout: %v", err)

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

	ctx.Cleanup()
	e2eutil.WaitForDeletion(t, f.Client.Client, cr, cleanupRetryInterval, cleanupTimeout)
	t.Log("Finished successfully")
}

func invalidMasterCountTest(t *testing.T) {
	f := test.Global

	ctx := test.NewContext(t)
	namespace, err := ctx.GetWatchNamespace()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Found namespace: %v", namespace)

	esUUID := utils.GenerateUUID()
	t.Logf("Using UUID for elasticsearch CR: %v", esUUID)

	if err = createElasticsearchSecret(t, f, ctx, esUUID); err != nil {
		t.Fatal(err)
	}

	dataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for data nodes: %v", dataUUID)

	// Create CR with invalid case: four nodes all sharing client, data and master roles
	cr, err := createElasticsearchCR(t, f, ctx, esUUID, dataUUID, 4)
	if err != nil {
		t.Fatalf("could not create exampleElasticsearch: %v", err)
	}

	key := client.ObjectKey{Name: cr.GetName(), Namespace: cr.GetNamespace()}
	if err := f.Client.Get(context.TODO(), key, cr); err != nil {
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

	ctx.Cleanup()
	e2eutil.WaitForDeletion(t, f.Client.Client, cr, cleanupRetryInterval, cleanupTimeout)
	t.Log("Finished successfully")
}
