package configmap

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EqualityFunc is the type for functions that compare two configmaps.
// Return true if two configmaps are equal.
type EqualityFunc func(current, desired *corev1.ConfigMap) bool

// MutateFunc is the type for functions that mutate the current configmap
// by applying the values from the desired configmap.
type MutateFunc func(current, desired *corev1.ConfigMap)

// Get returns the k8s configmap for the given object key or an error.
func Get(ctx context.Context, c client.Client, key client.ObjectKey) (*corev1.ConfigMap, error) {
	cm := New(key.Name, key.Namespace, nil, nil)

	if err := c.Get(ctx, key, cm); err != nil {
		return cm, kverrors.Wrap(err, "failed to get configmap",
			"name", cm.Name,
			"namespace", cm.Namespace,
		)
	}

	return cm, nil
}

// GetDataSHA256 returns the sha256 checksum of the confimap data keys
func GetDataSHA256(ctx context.Context, c client.Client, key client.ObjectKey, excludeKeys []string) string {
	hash := ""

	cm, err := Get(ctx, c, key)
	if err != nil {
		return hash
	}

	dataHashes := make(map[string][32]byte)
outer:
	for key, data := range cm.Data {
		for _, excludeKey := range excludeKeys {
			if key == excludeKey {
				continue outer
			}
		}
		dataHashes[key] = sha256.Sum256([]byte(data))
	}

	sortedKeys := []string{}

	for key := range dataHashes {
		sortedKeys = append(sortedKeys, key)
	}

	sort.Strings(sortedKeys)

	for _, key := range sortedKeys {
		hash = fmt.Sprintf("%s%s", hash, dataHashes[key])
	}

	return hash
}

// Create will create the given configmap on the api server or return an error on failure
func Create(ctx context.Context, c client.Client, cm *corev1.ConfigMap) error {
	err := c.Create(ctx, cm)
	if err != nil {
		return kverrors.Wrap(err, "failed to create configmap",
			"name", cm.Name,
			"namespace", cm.Namespace,
		)
	}
	return nil
}

// CreateOrUpdate attempts first to create the given configmap. If the
// configmap already exists and the provided comparison func detects any changes
// an update is attempted. Updates are retried with backoff (See retry.DefaultRetry).
// Returns on failure an non-nil error.
func CreateOrUpdate(ctx context.Context, c client.Client, cm *corev1.ConfigMap, equal EqualityFunc, mutate MutateFunc) (bool, error) {
	err := Create(ctx, c, cm)
	if err == nil {
		return false, nil
	}

	if !apierrors.IsAlreadyExists(kverrors.Root(err)) {
		return false, kverrors.Wrap(err, "failed to create configmap",
			"name", cm.Name,
			"namespace", cm.Namespace,
		)
	}

	current := &corev1.ConfigMap{}
	key := client.ObjectKey{Name: cm.Name, Namespace: cm.Namespace}
	err = c.Get(ctx, key, current)
	if err != nil {
		return false, kverrors.Wrap(err, "failed to get configmap",
			"name", cm.Name,
			"namespace", cm.Namespace,
		)
	}

	if !equal(current, cm) {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := c.Get(ctx, key, current); err != nil {
				log.Error(err, "failed to get configmap", cm.Name)
				return err
			}

			mutate(current, cm)
			if err := c.Update(ctx, current); err != nil {
				log.Error(err, "failed to update configmap", cm.Name)
				return err
			}
			return nil
		})
		if err != nil {
			return false, kverrors.Wrap(err, "failed to update configmap",
				"name", cm.Name,
				"namespace", cm.Namespace,
			)
		}
		return true, nil
	}

	return false, nil
}

// Delete attempts to delete a k8s configmap if existing or returns an error.
func Delete(ctx context.Context, c client.Client, key client.ObjectKey) error {
	cm := New(key.Name, key.Namespace, nil, nil)

	if err := c.Delete(ctx, cm, &client.DeleteOptions{}); err != nil {
		return kverrors.Wrap(err, "failed to delete configmap",
			"name", cm.Name,
			"namespace", cm.Namespace,
		)
	}

	return nil
}

// DataEqual return only true if the configmaps have equal data sections only.
func DataEqual(current, desired *corev1.ConfigMap) bool {
	return equality.Semantic.DeepEqual(current.Data, desired.Data)
}

// MutateDataOnly is a default mutate function implementation
// that copies only the data section from desired to current
// configmap.
func MutateDataOnly(current, desired *corev1.ConfigMap) {
	current.Data = desired.Data
}
