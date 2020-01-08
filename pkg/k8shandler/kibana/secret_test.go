package kibana

import (
	"reflect"
	"testing"

	kibana "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestClusterKibanaRequest_CreateOrUpdateSecretNoop(t *testing.T) {
	// mimic ca.crt
	caCrtStr := `-----BEGIN CERTIFICATE-----
This is a ca cert.
-----END CERTIFICATE-----`

	// mimic ca.key
	caKeyStr := `-----BEGIN PRIVATE KEY-----
This is a private key.
-----END PRIVATE KEY-----`

	initSecret := NewSecret(
		"test-secret",
		"test-namespace",
		map[string][]byte{
			"testca":  []byte(caCrtStr),
			"testkey": []byte(caKeyStr),
		})

	clusterLoggingRequest := &ClusterKibanaRequest{
		client:  fake.NewFakeClient(initSecret),
		cluster: &kibana.Kibana{},
	}

	// create
	if err := clusterLoggingRequest.CreateOrUpdateSecret(initSecret); err != nil {
		t.Errorf("CreateOrUpdateSecret failed 1 [%v]", err)
	}

	firstSecret, err := clusterLoggingRequest.GetSecret("test-secret")
	if err != nil {
		t.Errorf("GetSecret failed 1 [%v]", err)
	}

	// update
	if err := clusterLoggingRequest.CreateOrUpdateSecret(firstSecret); err != nil {
		t.Errorf("CreateOrUpdateSecret failed 2 [%v]", err)
	}

	secondSecret, err := clusterLoggingRequest.GetSecret("test-secret")
	if err != nil {
		t.Errorf("GetSecret failed 2 [%v]", err)
	}

	if reflect.DeepEqual(firstSecret, secondSecret) {
		t.Logf("Initial secret [%p]\n%v", initSecret, initSecret)
		t.Logf("First secret [%p]\n%v", firstSecret, firstSecret)
		t.Logf("Second secret [%p]\n%v", secondSecret, secondSecret)
	} else {
		t.Errorf("First secret [%v] != Second secret [%v]", firstSecret, secondSecret)
	}
}

func TestClusterKibanaRequest_CreateOrUpdateSecretSameKeysNewValues(t *testing.T) {
	// mimic ca.crt
	caCrtStr := `-----BEGIN CERTIFICATE-----
This is a ca cert.
-----END CERTIFICATE-----`

	// mimic ca.key
	caKeyStr := `-----BEGIN PRIVATE KEY-----
This is a private key.
-----END PRIVATE KEY-----`

	initSecret := NewSecret(
		"test-secret",
		"test-namespace",
		map[string][]byte{
			"testca":  []byte(caCrtStr),
			"testkey": []byte(caKeyStr),
		})

	clusterLoggingRequest := &ClusterKibanaRequest{
		client:  fake.NewFakeClient(initSecret),
		cluster: &kibana.Kibana{},
	}

	// create
	if err := clusterLoggingRequest.CreateOrUpdateSecret(initSecret); err != nil {
		t.Errorf("CreateOrUpdateSecret failed 1 [%v]", err)
	}

	firstSecret, err := clusterLoggingRequest.GetSecret("test-secret")
	if err != nil {
		t.Errorf("GetSecret failed 1 [%v]", err)
	}

	caCrtStr = `-----BEGIN CERTIFICATE-----
This is a new ca cert.
-----END CERTIFICATE-----`

	caKeyStr = `-----BEGIN PRIVATE KEY-----
This is a new private key.
-----END PRIVATE KEY-----`

	newSecret := NewSecret(
		"test-secret",
		"test-namespace",
		map[string][]byte{
			"testca":  []byte(caCrtStr),
			"testkey": []byte(caKeyStr),
		})

	// update
	if err := clusterLoggingRequest.CreateOrUpdateSecret(newSecret); err != nil {
		t.Errorf("CreateOrUpdateSecret failed 2 [%v]", err)
	}

	secondSecret, err := clusterLoggingRequest.GetSecret("test-secret")
	if err != nil {
		t.Errorf("GetSecret failed 2 [%v]", err)
	}

	if reflect.DeepEqual(firstSecret, secondSecret) {
		t.Errorf("First secret [%v] == Second secret [%v]", firstSecret, secondSecret)
	} else {
		t.Logf("Initial secret [%p]\n%v", initSecret, initSecret)
		t.Logf("Initial secret [%p]\n%v", newSecret, newSecret)
		t.Logf("First secret [%p]\n%v", firstSecret, firstSecret)
		t.Logf("Second secret [%p]\n%v", secondSecret, secondSecret)
	}
}

func TestClusterKibanaRequest_CreateOrUpdateSecretNewKeysSameValues(t *testing.T) {
	// mimic ca.crt
	caCrtStr := `-----BEGIN CERTIFICATE-----
This is a ca cert.
-----END CERTIFICATE-----`

	// mimic ca.key
	caKeyStr := `-----BEGIN PRIVATE KEY-----
This is a private key.
-----END PRIVATE KEY-----`

	initSecret := NewSecret(
		"test-secret",
		"test-namespace",
		map[string][]byte{
			"testca":  []byte(caCrtStr),
			"testkey": []byte(caKeyStr),
		})

	clusterLoggingRequest := &ClusterKibanaRequest{
		client:  fake.NewFakeClient(initSecret),
		cluster: &kibana.Kibana{},
	}

	// create
	if err := clusterLoggingRequest.CreateOrUpdateSecret(initSecret); err != nil {
		t.Errorf("CreateOrUpdateSecret failed 1 [%v]", err)
	}

	firstSecret, err := clusterLoggingRequest.GetSecret("test-secret")
	if err != nil {
		t.Errorf("GetSecret failed 1 [%v]", err)
	}

	newSecret := NewSecret(
		"test-secret",
		"test-namespace",
		map[string][]byte{
			"newtestca":  []byte(caCrtStr),
			"newtestkey": []byte(caKeyStr),
		})

	// update
	if err := clusterLoggingRequest.CreateOrUpdateSecret(newSecret); err != nil {
		t.Errorf("CreateOrUpdateSecret failed 2 [%v]", err)
	}

	secondSecret, err := clusterLoggingRequest.GetSecret("test-secret")
	if err != nil {
		t.Errorf("GetSecret failed 2 [%v]", err)
	}

	if reflect.DeepEqual(firstSecret, secondSecret) {
		t.Errorf("First secret [%v] == Second secret [%v]", firstSecret, secondSecret)
	} else {
		t.Logf("Initial secret [%p]\n%v", initSecret, initSecret)
		t.Logf("Initial secret [%p]\n%v", newSecret, newSecret)
		t.Logf("First secret [%p]\n%v", firstSecret, firstSecret)
		t.Logf("Second secret [%p]\n%v", secondSecret, secondSecret)
	}
}

func TestClusterKibanaRequest_CreateOrUpdateSecretRemovingKey(t *testing.T) {
	// mimic ca.crt
	caCrtStr := `-----BEGIN CERTIFICATE-----
This is a ca cert.
-----END CERTIFICATE-----`

	// mimic ca.key
	caKeyStr := `-----BEGIN PRIVATE KEY-----
This is a private key.
-----END PRIVATE KEY-----`

	initSecret := NewSecret(
		"test-secret",
		"test-namespace",
		map[string][]byte{
			"testca":  []byte(caCrtStr),
			"testkey": []byte(caKeyStr),
		})

	clusterLoggingRequest := &ClusterKibanaRequest{
		client:  fake.NewFakeClient(initSecret),
		cluster: &kibana.Kibana{},
	}

	// create
	if err := clusterLoggingRequest.CreateOrUpdateSecret(initSecret); err != nil {
		t.Errorf("CreateOrUpdateSecret failed 1 [%v]", err)
	}

	firstSecret, err := clusterLoggingRequest.GetSecret("test-secret")
	if err != nil {
		t.Errorf("GetSecret failed 1 [%v]", err)
	}

	newSecret := NewSecret(
		"test-secret",
		"test-namespace",
		map[string][]byte{
			"testca": []byte(caCrtStr),
		})

	// update
	if err := clusterLoggingRequest.CreateOrUpdateSecret(newSecret); err != nil {
		t.Errorf("CreateOrUpdateSecret failed 2 [%v]", err)
	}

	secondSecret, err := clusterLoggingRequest.GetSecret("test-secret")
	if err != nil {
		t.Errorf("GetSecret failed 2 [%v]", err)
	}

	if reflect.DeepEqual(firstSecret, secondSecret) {
		t.Errorf("First secret [%v] == Second secret [%v]", firstSecret, secondSecret)
	} else {
		t.Logf("Initial secret [%p]\n%v", initSecret, initSecret)
		t.Logf("Initial secret [%p]\n%v", newSecret, newSecret)
		t.Logf("First secret [%p]\n%v", firstSecret, firstSecret)
		t.Logf("Second secret [%p]\n%v", secondSecret, secondSecret)
	}
}
