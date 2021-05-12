package k8shandler

import (
	"context"
	"crypto/sha256"
	"fmt"
	"reflect"
	"strings"

	"github.com/openshift/elasticsearch-operator/internal/constants"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newSecret(secretName, namespace string, data map[string][]byte) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: data,
	}
}

func createOrUpdateSecret(secret *v1.Secret, client client.Client) error {
	err := client.Create(context.TODO(), secret)
	if err == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(err) {
		// FIXME: we should wrap this instead...
		return err
	}

	// get and update with new data
	currentSecret, err := getSecret(secret.Name, secret.Namespace, client)
	if err != nil {
		return err
	}

	if !isContentSame(currentSecret.Data, secret.Data) {
		currentSecret.Data = secret.Data
		return client.Update(context.TODO(), currentSecret)
	}

	return nil
}

func CreateOrUpdateSecretWithOwnerRef(secretName, namespace string, data map[string][]byte, client client.Client, ownerRef metav1.OwnerReference) error {
	secret := newSecret(secretName, namespace, data)

	// add owner ref to secret
	secret.OwnerReferences = append(secret.OwnerReferences, ownerRef)

	return createOrUpdateSecret(secret, client)
}

func CreateOrUpdateSecret(secretName, namespace string, data map[string][]byte, client client.Client) error {
	secret := newSecret(secretName, namespace, data)
	return createOrUpdateSecret(secret, client)
}

func isContentSame(lhs, rhs map[string][]byte) bool {
	if len(lhs) != len(rhs) {
		return false
	}

	for lKey, lVal := range lhs {
		keyFound := false
		for rKey, rVal := range rhs {
			if lKey == rKey {
				keyFound = true

				if !reflect.DeepEqual(lVal, rVal) {
					return false
				}
			}
		}

		if !keyFound {
			return false
		}
	}

	return true
}

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
