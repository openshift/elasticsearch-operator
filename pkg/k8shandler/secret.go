package k8shandler

import (
	"crypto/sha256"
	"fmt"
	"context"

	"k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/apimachinery/pkg/types"
)

func getSecret(secretName, namespace string, client client.Client) *v1.Secret {
	secret := v1.Secret{}

	err := client.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: namespace}, &secret)

	if err != nil {
		// check if doesn't exist
	}

	return &secret
}

func getSecretDataHash(secretName, namespace string, client client.Client) string {
	hash := ""

	secret := getSecret(secretName, namespace, client)

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
