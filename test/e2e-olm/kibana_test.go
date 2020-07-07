package e2e

import (
	"context"
	"fmt"
	"testing"

	consolev1 "github.com/openshift/api/console/v1"
	"github.com/openshift/elasticsearch-operator/pkg/k8shandler/kibana"
	"github.com/openshift/elasticsearch-operator/test/utils"

	"github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"

	"k8s.io/apimachinery/pkg/types"
)

func TestKibana(t *testing.T) {
	utils.RegisterSchemes(t)
	t.Run("KibanaDeployment", kibanaDeploymentTest)
}

func kibanaDeploymentTest(t *testing.T) {
	f := test.Global

	ctx := test.NewContext(t)

	namespace, err := ctx.GetWatchNamespace()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Found namespace: %q", namespace)

	// wait for elasticsearch-operator to be ready
	err = utils.WaitForReadyDeployment(t, f.KubeClient, namespace, "elasticsearch-operator", 1, utils.DefaultRetryInterval, utils.DefaultTimeout)
	if err != nil {
		t.Fatal(err)
	}

	esUUID := utils.GenerateUUID()
	t.Logf("Using UUID for elasticsearch CR: %v", esUUID)

	dataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for data nodes: %v", dataUUID)

	if err = utils.CreateElasticsearchSecret(t, f, ctx, esUUID); err != nil {
		t.Fatal(err)
	}

	esDeploymentName := fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	_, err = utils.CreateElasticsearchCR(t, f, ctx, esUUID, dataUUID, 1)
	if err != nil {
		t.Fatal(err)
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, esDeploymentName, 1, utils.DefaultRetryInterval, utils.DefaultTimeout)
	if err != nil {
		t.Errorf("timed out waiting for Deployment %q: %v", esDeploymentName, err)
	}

	if err = utils.CreateKibanaSecret(f, ctx, esUUID); err != nil {
		t.Fatal(err)
	}

	if err = utils.CreateKibanaProxySecret(f, ctx, esUUID); err != nil {
		t.Fatal(err)
	}

	kibanaCR := utils.CreateKibanaCR(namespace)

	cleanupOpts := &test.CleanupOptions{
		TestContext:   ctx,
		Timeout:       utils.DefaultCleanupTimeout,
		RetryInterval: utils.DefaultCleanupRetryInterval,
	}

	err = f.Client.Create(context.TODO(), kibanaCR, cleanupOpts)
	if err != nil {
		t.Errorf("could not create kibanaCR: %v", err)
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, utils.KibanaCRName, 1, utils.DefaultRetryInterval, utils.DefaultTimeout)
	if err != nil {
		t.Errorf("timed out waiting for Deployment kibana: %v", err)
	}

	consoleLink := &consolev1.ConsoleLink{}
	key := types.NamespacedName{Name: kibana.KibanaConsoleLinkName}

	err = utils.WaitForObject(t, f.Client, key, consoleLink, utils.DefaultRetryInterval, utils.DefaultTimeout)
	if err != nil {
		t.Errorf("Kibana console link missing: %v", err)
	}

	ctx.Cleanup()
}
