package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/openshift/elasticsearch-operator/test/utils"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	goctx "context"

	elasticsearch "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	v1 "k8s.io/api/core/v1"
)

func TestElasticsearch(t *testing.T) {
	registerSchemes(t)
	t.Run("elasticsearch-group", func(t *testing.T) {
		t.Run("Cluster", ElasticsearchCluster)
	})
}

func elasticsearchFullClusterTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		return fmt.Errorf("Could not get namespace: %v", err)
	}

	dataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for data nodes: %v", dataUUID)

	nonDataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for non data nodes: %v", nonDataUUID)

	storageClassName := "gp2"
	storageClassSize := resource.MustParse("2G")

	esNonDataNode := elasticsearch.ElasticsearchNode{
		Roles: []elasticsearch.ElasticsearchNodeRole{
			elasticsearch.ElasticsearchRoleClient,
			elasticsearch.ElasticsearchRoleMaster,
		},
		NodeCount: int32(1),
		Storage: elasticsearch.ElasticsearchStorageSpec{
			StorageClassName: &storageClassName,
			Size:             &storageClassSize,
		},
		GenUUID: &nonDataUUID,
	}

	exampleElasticsearch, err := createElasticsearchCR(t, f, ctx, dataUUID)
	if err != nil {
		return fmt.Errorf("could not create exampleElasticsearch: %v", err)
	}

	deploymentName := fmt.Sprintf("elasticsearch-cdm-%v-1", dataUUID)
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, deploymentName, 1, retryInterval, timeout)
	if err != nil {
		return fmt.Errorf("timed out waiting for initial Deployment %v: %v", deploymentName, err)
	}
	t.Log("Created initial deployment")

	// Scale up current node
	// then look for elasticsearch-cdm-0-2 and prior node
	t.Log("Scaling up the current node...")
	exampleName := types.NamespacedName{Name: elasticsearchCRName, Namespace: namespace}
	if err = f.Client.Get(goctx.TODO(), exampleName, exampleElasticsearch); err != nil {
		return fmt.Errorf("failed to get exampleElasticsearch: %v", err)
	}
	exampleElasticsearch.Spec.Nodes[0].NodeCount = int32(2)
	t.Logf("Updating Elasticsearch CR: %v", exampleElasticsearch)
	err = f.Client.Update(goctx.TODO(), exampleElasticsearch)
	if err != nil {
		return fmt.Errorf("could not update exampleElasticsearch with 2 replicas: %v", err)
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, fmt.Sprintf("elasticsearch-cdm-%v-1", dataUUID), 1, retryInterval, timeout)
	if err != nil {
		return fmt.Errorf("timed out waiting for Deployment %v: %v", fmt.Sprintf("elasticsearch-cdm-%v-1", dataUUID), err)
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, fmt.Sprintf("elasticsearch-cdm-%v-2", dataUUID), 1, retryInterval, timeout)
	if err != nil {
		return fmt.Errorf("timed out waiting for Deployment %v: %v", fmt.Sprintf("elasticsearch-cdm-%v-2", dataUUID), err)
	}
	t.Log("Created additional deployment")

	if err = f.Client.Get(goctx.TODO(), exampleName, exampleElasticsearch); err != nil {
		return fmt.Errorf("failed to get exampleElasticsearch: %v", err)
	}
	t.Log("Adding another node")
	exampleElasticsearch.Spec.Nodes = append(exampleElasticsearch.Spec.Nodes, esNonDataNode)
	err = f.Client.Update(goctx.TODO(), exampleElasticsearch)
	if err != nil {
		return fmt.Errorf("could not update exampleElasticsearch with an additional node: %v", err)
	}

	// Create another node
	// then look for elasticsearch-cdm-1-1 and prior nodes
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, fmt.Sprintf("elasticsearch-cdm-%v-1", dataUUID), 1, retryInterval, timeout)
	if err != nil {
		return fmt.Errorf("timed out waiting for Deployment %v: %v", fmt.Sprintf("elasticsearch-cdm-%v-1", dataUUID), err)
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, fmt.Sprintf("elasticsearch-cdm-%v-2", dataUUID), 1, retryInterval, timeout)
	if err != nil {
		return fmt.Errorf("timed out waiting for Deployment %v: %v", fmt.Sprintf("elasticsearch-cdm-%v-1", dataUUID), err)
	}

	err = utils.WaitForStatefulset(t, f.KubeClient, namespace, fmt.Sprintf("elasticsearch-cm-%v", nonDataUUID), 1, retryInterval, timeout)
	if err != nil {
		return fmt.Errorf("timed out waiting for Statefulset %v: %v", fmt.Sprintf("elasticsearch-cm-%v", nonDataUUID), err)
	}
	t.Log("Created non-data statefulset")

	// Scale up to SingleRedundancy
	if err = f.Client.Get(goctx.TODO(), exampleName, exampleElasticsearch); err != nil {
		return fmt.Errorf("failed to get exampleElasticsearch: %v", err)
	}

	exampleElasticsearch.Spec.RedundancyPolicy = elasticsearch.SingleRedundancy
	t.Logf("Updating redundancy policy to %v", exampleElasticsearch.Spec.RedundancyPolicy)
	err = f.Client.Update(goctx.TODO(), exampleElasticsearch)
	if err != nil {
		return fmt.Errorf("could not update exampleElasticsearch to be SingleRedundancy: %v", err)
	}

	/*
		FIXME: this is commented out as we currently do not run our e2e tests in a container on the test cluster
		 to be added back in as a follow up
		err = utils.WaitForIndexTemplateReplicas(t, f.KubeClient, namespace, "elasticsearch", 1, retryInterval, timeout)
		if err != nil {
			return fmt.Errorf("timed out waiting for all index templates to have correct replica count")
		}

		err = utils.WaitForIndexReplicas(t, f.KubeClient, namespace, "elasticsearch", 1, retryInterval, timeout)
		if err != nil {
			return fmt.Errorf("timed out waiting for all indices to have correct replica count")
		}
	*/

	// Update the secret to force a full cluster redeploy
	err = updateElasticsearchSecret(t, f, ctx)
	if err != nil {
		return fmt.Errorf("Unable to update secret")
	}

	//FIXME: Update the WaitForCondition methods

	// wait for pods to have "redeploy for certs" condition as true?
	//desiredCondition := elasticsearch.ElasticsearchNodeUpgradeStatus{
	//	ScheduledForCertRedeploy: v1.ConditionTrue,
	//}
	//
	//err = utils.WaitForNodeStatusCondition(t, f, namespace, elasticsearchCRName, desiredCondition, retryInterval, time.Second*300)
	//if err != nil {
	//	d, _ := yaml.Marshal(desiredCondition)
	//	t.Log("Desired condition", string(d))
	//	return fmt.Errorf("Timed out waiting for full cluster restart to begin")
	//}
	//
	//// then wait for conditions to be gone
	//desiredClusterCondition := elasticsearch.ClusterCondition{
	//	Type:   elasticsearch.Restarting,
	//	Status: v1.ConditionFalse,
	//}
	//
	//err = utils.WaitForClusterStatusCondition(t, f, namespace, elasticsearchCRName, desiredClusterCondition, retryInterval, time.Second*300)
	//if err != nil {
	//	d, _ := yaml.Marshal(desiredClusterCondition)
	//	t.Log("Desired condition", string(d))
	//	return fmt.Errorf("Timed out waiting for full cluster restart to complete")
	//}

	// ensure all prior nodes are ready again
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, fmt.Sprintf("elasticsearch-cdm-%v-1", dataUUID), 1, retryInterval, timeout)
	if err != nil {
		return fmt.Errorf("timed out waiting for Deployment %v: %v", fmt.Sprintf("elasticsearch-cdm-%v-1", dataUUID), err)
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, fmt.Sprintf("elasticsearch-cdm-%v-2", dataUUID), 1, retryInterval, timeout)
	if err != nil {
		return fmt.Errorf("timed out waiting for Deployment %v: %v", fmt.Sprintf("elasticsearch-cdm-%v-1", dataUUID), err)
	}

	err = utils.WaitForStatefulset(t, f.KubeClient, namespace, fmt.Sprintf("elasticsearch-cm-%v", nonDataUUID), 1, retryInterval, timeout)
	if err != nil {
		return fmt.Errorf("timed out waiting for Statefulset %v: %v", fmt.Sprintf("elasticsearch-cm-%v", nonDataUUID), err)
	}

	// Incorrect scale up and verify we don't see a 4th master created
	if err = f.Client.Get(goctx.TODO(), exampleName, exampleElasticsearch); err != nil {
		return fmt.Errorf("failed to get exampleElasticsearch: %v", err)
	}
	exampleElasticsearch.Spec.Nodes[1].NodeCount = int32(2)
	err = f.Client.Update(goctx.TODO(), exampleElasticsearch)
	if err != nil {
		return fmt.Errorf("could not update exampleElasticsearch with an additional statefulset replica: %v", err)
	}

	err = utils.WaitForStatefulset(t, f.KubeClient, namespace, fmt.Sprintf("elasticsearch-cm-%v", nonDataUUID), 2, retryInterval, time.Second*30)
	if err == nil {
		return fmt.Errorf("unexpected statefulset replica count for %v found", fmt.Sprintf("elasticsearch-cm-%v", nonDataUUID))
	}

	if err = f.Client.Get(goctx.TODO(), exampleName, exampleElasticsearch); err != nil {
		return fmt.Errorf("failed to get exampleElasticsearch: %v", err)
	}

	for _, condition := range exampleElasticsearch.Status.Conditions {
		if condition.Type == elasticsearch.InvalidMasters {
			if condition.Status == v1.ConditionFalse ||
				condition.Status == "" {
				return fmt.Errorf("unexpected status condition for elasticsearch found: %v", condition.Status)
			}
		}
	}

	// Update the resource spec for the cluster
	oldMemValue := exampleElasticsearch.Spec.Spec.Resources.Limits.Memory()

	memValue := resource.MustParse("1.5Gi")
	cpuValue := resource.MustParse("100m")
	exampleElasticsearch.Spec.Spec.Resources = v1.ResourceRequirements{
		Limits: v1.ResourceList{
			v1.ResourceMemory: memValue,
		},
		Requests: v1.ResourceList{
			v1.ResourceCPU:    cpuValue,
			v1.ResourceMemory: memValue,
		},
	}

	t.Log("Updating Limits.Memory and Requests.Memory to trigger a rolling restart")
	t.Logf("Updating from %s to %s", oldMemValue.String(), memValue.String())
	err = f.Client.Update(goctx.TODO(), exampleElasticsearch)
	if err != nil {
		return fmt.Errorf("could not update exampleElasticsearch with an additional statefulset replica: %v", err)
	}

	// wait for node to not be ready (restart is happening)
	/*desiredCondition := elasticsearch.ElasticsearchNodeUpgradeStatus{
		UnderUpgrade: v1.ConditionTrue,
	}*/

	// This doesn't work correctly because we don't update the cluster status until we've failed out
	// of our upgrade loop...
	/*err = utils.WaitForNodeStatusCondition(t, f, namespace, elasticsearchCRName, desiredCondition, retryInterval, time.Second*300)
	if err != nil {
		d, _ := yaml.Marshal(desiredCondition)
		t.Log("Desired condition", string(d))
		return fmt.Errorf("Timed out waiting for full cluster restart to begin")
	}*/

	// due to gap mentioned above -- pause here for a few seconds to let the operator do its thing?
	time.Sleep(10 * time.Second)

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, fmt.Sprintf("elasticsearch-cdm-%v-1", dataUUID), 1, retryInterval, timeout)
	if err != nil {
		return fmt.Errorf("timed out waiting for Deployment %v: %v", fmt.Sprintf("elasticsearch-cdm-%v-1", dataUUID), err)
	}

	// ensure all prior nodes are ready again
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, fmt.Sprintf("elasticsearch-cdm-%v-1", dataUUID), 1, retryInterval, timeout)
	if err != nil {
		return fmt.Errorf("timed out waiting for Deployment %v: %v", fmt.Sprintf("elasticsearch-cdm-%v-1", dataUUID), err)
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, fmt.Sprintf("elasticsearch-cdm-%v-2", dataUUID), 1, retryInterval, timeout)
	if err != nil {
		return fmt.Errorf("timed out waiting for Deployment %v: %v", fmt.Sprintf("elasticsearch-cdm-%v-1", dataUUID), err)
	}

	err = utils.WaitForStatefulset(t, f.KubeClient, namespace, fmt.Sprintf("elasticsearch-cm-%v", nonDataUUID), 1, retryInterval, timeout)
	if err != nil {
		return fmt.Errorf("timed out waiting for Statefulset %v: %v", fmt.Sprintf("elasticsearch-cm-%v", nonDataUUID), err)
	}

	ctx.Cleanup()
	t.Log("Finished successfully")
	return nil
}

func ElasticsearchCluster(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	/*
		err := ctx.InitializeClusterResources(&framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
		if err != nil {
			t.Fatalf("failed to initialize cluster resources: %v", err)
		}
		t.Log("Initialized cluster resources")
	*/
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Found namespace: %v", namespace)

	// get global framework variables
	f := framework.Global
	// wait for elasticsearch-operator to be ready
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "elasticsearch-operator", 1, retryInterval, timeout)
	if err != nil {
		t.Fatal(err)
	}

	if err = createElasticsearchSecret(t, f, ctx); err != nil {
		t.Fatal(err)
	}

	if err = elasticsearchFullClusterTest(t, f, ctx); err != nil {
		t.Fatal(err)
	}
}
