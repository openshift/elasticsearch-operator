package e2e

import (
	"fmt"
	"strings"
	"testing"

	"github.com/openshift/elasticsearch-operator/test/utils"
)

func TestElasticsearchWrite(t *testing.T) {
	setupK8sClient(t)
	t.Run("elasticsearch write", esWriteTest)
}

func esWriteTest(t *testing.T) {
	namespace := operatorNamespace

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
	pods, err := utils.WaitForPods(t, k8sClient, namespace, matchingLabels, retryInterval, timeout)
	if err != nil {
		t.Fatalf("failed to wait for pods: %v", err)
	}
	podName := pods.Items[0].GetName()

	var cmd string
	var execExpect func(text string)
	execExpect = func(text string) {
		code, _, _ := ExecInPod(k8sConfig, namespace, podName, cmd, "elasticsearch")
		if strings.Index(code, text) < 0 {
			t.Errorf("cmd [%s] output does not contain expected text %s", cmd, text)
		}
	}

	cmd = "es_util --query=foo/_doc/7 -d '{\"key\":\"value\"}' -XPUT -w %{http_code}"
	execExpect("201")

	cmd = "es_util --query=foo-write/_doc/8 -d '{\"key\":\"value\"}' -XPUT -w %{http_code}"
	execExpect("404")

	cmd = "es_util --query=foo-write -XPUT -w %{http_code}"
	execExpect("200")

	cmd = "es_util --query=foo-write/_doc/1 -d '{\"key\":\"value\"}' -XPUT  -w %{http_code}"
	execExpect("201")

	cleanupEsTest(t, k8sClient, operatorNamespace, esUUID)
	t.Log("Finished successfully")
}
