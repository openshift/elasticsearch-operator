package e2e

import (
	goctx "context"
	"fmt"
	"testing"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/elasticsearch-operator/internal/k8shandler/kibana"
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

	// Test recovering route after deletion
	name := "kibana"
	routeInst := kibana.NewRoute(name, namespace, name)
	err = f.Client.Delete(goctx.TODO(), routeInst)
	if err != nil {
		t.Errorf("could not delete Kibana route: %v", err)
	}

	route := &routev1.Route{}
	key := types.NamespacedName{Name: name, Namespace: namespace}
	err = waitForObject(t, f.Client, key, route, retryInterval, timeout)
	if err != nil {
		t.Errorf("Kibana route not recovered: %v", err)
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
