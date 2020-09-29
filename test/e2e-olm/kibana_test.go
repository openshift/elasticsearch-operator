package e2e

import (
	goctx "context"
	"fmt"
	"testing"
	"time"

	consolev1 "github.com/openshift/api/console/v1"
	"github.com/openshift/elasticsearch-operator/pkg/k8shandler/kibana"
	"github.com/openshift/elasticsearch-operator/test/utils"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

func TestKibana(t *testing.T) {
	registerSchemes(t)
	t.Run("kibana-group", func(t *testing.T) {
		t.Run("KibanaDeployment", KibanaDeployment)
	})
}

func KibanaDeployment(t *testing.T) {
	ctx := framework.NewTestCtx(t)

	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Found namespace: %q", namespace)

	// get global framework variables
	f := framework.Global
	// wait for elasticsearch-operator to be ready
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "elasticsearch-operator", 1, retryInterval, timeout)
	if err != nil {
		t.Fatal(err)
	}

	esUUID := utils.GenerateUUID()
	t.Logf("Using UUID for elasticsearch CR: %v", esUUID)

	dataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for data nodes: %v", dataUUID)

	if err = createElasticsearchSecret(t, f, ctx, esUUID); err != nil {
		t.Fatal(err)
	}

	esDeploymentName := fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	_, err = createElasticsearchCR(t, f, ctx, esUUID, dataUUID, 1)
	if err != nil {
		t.Fatal(err)
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, esDeploymentName, 1, retryInterval, timeout)
	if err != nil {
		t.Errorf("timed out waiting for Deployment %q: %v", esDeploymentName, err)
	}

	if err = createKibanaSecret(f, ctx, esUUID); err != nil {
		t.Fatal(err)
	}

	if err = createKibanaProxySecret(f, ctx, esUUID); err != nil {
		t.Fatal(err)
	}

	kibanaCR := createKibanaCR(namespace)
	cleanupOpts := &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval}
	err = f.Client.Create(goctx.TODO(), kibanaCR, cleanupOpts)
	if err != nil {
		t.Errorf("could not create kibanaCR: %v", err)
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, kibanaCRName, 1, retryInterval, timeout)
	if err != nil {
		t.Errorf("timed out waiting for Deployment kibana: %v", err)
	}

	consoleLink := &consolev1.ConsoleLink{}
	key := types.NamespacedName{Name: kibana.KibanaConsoleLinkName}
	err = waitForObject(t, f.Client, key, consoleLink, retryInterval, timeout)
	if err != nil {
		t.Errorf("Kibana console link missing: %v", err)
	}

	ctx.Cleanup()
}

func waitForObject(t *testing.T, client framework.FrameworkClient, key types.NamespacedName, obj runtime.Object, retryInterval, timout time.Duration) error {
	return wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		err = client.Get(goctx.TODO(), key, obj)
		if err != nil {
			if errors.IsNotFound(err) {
				return true, err
			}
			return false, nil
		}
		t.Logf("Found object %s", key.String())
		return true, nil
	})
}
