package k8shandler

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/openshift/elasticsearch-operator/internal/constants"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getSecret(secretName, namespace string, client client.Client) (*v1.Secret, error) {
	secret := v1.Secret{}

	err := client.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: namespace}, &secret)

	return &secret, err
}

func getSecretDataHash(secretName, namespace string, client client.Client) string {
	hash := ""

	secret, err := getSecret(secretName, namespace, client)
	if err != nil {
		return hash
	}

	dataHashes := make(map[string][32]byte)

	for key, data := range secret.Data {
		dataHashes[key] = sha256.Sum256([]byte(data))
	}

	sortedKeys := sortDataHashKeys(dataHashes)

	for _, key := range sortedKeys {
		hash = fmt.Sprintf("%s%s", hash, dataHashes[key])
	}

	return hash
}

// hasRequiredSecrets will check that all secrets that we expect for EO to be able to communicate
// with the ES cluster it manages exist.
// It will return true if all required secrets/keys exist.
// Otherwise, it will return false and the message will be populated with what is missing.
func (er ElasticsearchRequest) hasRequiredSecrets() (bool, string) {

	message := ""
	hasRequired := true

	secret, err := getSecret(er.cluster.Name, er.cluster.Namespace, er.client)

	// check that the secret is there
	if apierrors.IsNotFound(err) {
		return false, fmt.Sprintf("Expected secret %q in namespace %q is missing", er.cluster.Name, er.cluster.Namespace)
	}

	var missingCerts []string
	var secretKeys []string

	for key, data := range secret.Data {
		// check that the fields aren't blank
		if string(data) == "" {
			missingCerts = append(missingCerts, key)
		}

		secretKeys = append(secretKeys, key)
	}

	// check the fields are there
	for _, key := range constants.ExpectedSecretKeys {
		if !sliceContainsString(secretKeys, key) {
			missingCerts = append(missingCerts, key)
		}
	}

	if len(missingCerts) > 0 {
		message = fmt.Sprintf("Secret %q fields are either missing or empty: [%s]", er.cluster.Name, strings.Join(missingCerts, ", "))
		hasRequired = false
	}

	return hasRequired, message
}
