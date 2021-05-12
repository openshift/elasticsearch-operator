package e2e

import (
	goctx "context"
	"fmt"
	"testing"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	loggingv1 "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/internal/k8shandler/kibana"
	"github.com/openshift/elasticsearch-operator/test/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestKibana(t *testing.T) {
	setupK8sClient(t)

	// wait for elasticsearch-operator to be ready
	err := utils.WaitForDeployment(t, k8sClient, operatorNamespace, "elasticsearch-operator", 1, retryInterval, timeout)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("kibana-group", func(t *testing.T) {
		t.Run("KibanaDeployment", KibanaDeployment)
	})
}

func KibanaDeployment(t *testing.T) {
	var err error
	esUUID := utils.GenerateUUID()
	t.Logf("Using UUID for elasticsearch CR: %v", esUUID)

	dataUUID := utils.GenerateUUID()
	t.Logf("Using GenUUID for data nodes: %v", dataUUID)

	esDeploymentName := fmt.Sprintf("elasticsearch-%v-cdm-%v-1", esUUID, dataUUID)
	_, err = createElasticsearchCR(t, k8sClient, esUUID, dataUUID, 1)
	if err != nil {
		t.Fatal(err)
	}

	err = utils.WaitForDeployment(t, k8sClient, operatorNamespace, esDeploymentName, 1, retryInterval, timeout)
	if err != nil {
		t.Errorf("timed out waiting for Deployment %q: %v", esDeploymentName, err)
	}

	kibanaCR := createKibanaCR(operatorNamespace)

	err = k8sClient.Create(goctx.TODO(), kibanaCR)
	if err != nil {
		t.Errorf("could not create kibanaCR: %v", err)
	}

	err = utils.WaitForDeployment(t, k8sClient, operatorNamespace, kibanaCRName, 1, retryInterval, timeout)
	if err != nil {
		t.Errorf("timed out waiting for Deployment kibana: %v", err)
	}

	// Test recovering route after deletion
	name := "kibana"
	routeInst := kibana.NewRoute(name, operatorNamespace, name)
	err = k8sClient.Delete(goctx.TODO(), routeInst)
	if err != nil {
		t.Errorf("could not delete Kibana route: %v", err)
	}

	route := &routev1.Route{}
	key := types.NamespacedName{Name: name, Namespace: operatorNamespace}
	err = waitForObject(t, k8sClient, key, route, retryInterval, timeout)
	if err != nil {
		t.Errorf("Kibana route not recovered: %v", err)
	}

	// Delete Kibana CR, elasticsearch CR, and secrets
	es := &loggingv1.Elasticsearch{}
	key = types.NamespacedName{Name: elasticsearchNameFor(esUUID), Namespace: operatorNamespace}
	err = waitForDeleteObject(t, k8sClient, key, es, retryInterval, timeout)
	if err != nil {
		t.Errorf("cannot remove elasticsearch CR: %v", err)
	}

	kibana := &loggingv1.Kibana{}
	key = types.NamespacedName{Name: kibanaCRName, Namespace: operatorNamespace}
	err = waitForDeleteObject(t, k8sClient, key, kibana, retryInterval, timeout)
	if err != nil {
		t.Errorf("cannot remove kibana CR: %v", err)
	}
}

func waitForObject(t *testing.T, client client.Client, key types.NamespacedName, obj runtime.Object, retryInterval, timout time.Duration) error {
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
