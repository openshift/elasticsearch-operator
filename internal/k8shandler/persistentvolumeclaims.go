package k8shandler

import (
	"context"
	"reflect"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createOrUpdatePersistentVolumeClaim(pvc v1.PersistentVolumeClaimSpec, newName, namespace, clusterName string, client client.Client) error {
	// for some reason if the PVC already exists but creating it again would violate
	// quota we get an error regarding quota not that it already exists
	// so check to see if it already exists
	claim := createPersistentVolumeClaim(newName, namespace, clusterName, pvc)

	current := &v1.PersistentVolumeClaim{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: claim.Name, Namespace: claim.Namespace}, current)
	if err == nil {
		return updatePersistentVolumeClaim(claim, client)
	}

	if !apierrors.IsNotFound(err) {
		log.Error(err, "Could not get PVC", "pvc", newName)
		return err
	}

	err = client.Create(context.TODO(), claim)
	if err == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(err) {
		return kverrors.Wrap(err, "unable to create PVC")
	}

	return updatePersistentVolumeClaim(claim, client)
}

func updatePersistentVolumeClaim(claim *v1.PersistentVolumeClaim, client client.Client) error {
	current := &v1.PersistentVolumeClaim{}

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := client.Get(context.TODO(), types.NamespacedName{Name: claim.Name, Namespace: claim.Namespace}, current); err != nil {
			if apierrors.IsNotFound(err) {
				// the object doesn't exist -- it was likely culled
				// recreate it on the next time through if necessary
				return nil
			}
			return kverrors.Wrap(err, "failed to get PVC",
				"claim", claim.Name,
			)
		}

		if !reflect.DeepEqual(current.ObjectMeta.Labels, claim.ObjectMeta.Labels) {
			current.ObjectMeta.Labels = claim.ObjectMeta.Labels

			if err := client.Update(context.TODO(), current); err != nil {
				return err
			}
		}

		return nil
	})
	if retryErr != nil {
		return retryErr
	}

	return nil
}

func createPersistentVolumeClaim(pvcName, namespace, clusterName string, volSpec v1.PersistentVolumeClaimSpec) *v1.PersistentVolumeClaim {
	pvc := persistentVolumeClaim(pvcName, namespace, clusterName)
	pvc.Spec = volSpec
	return pvc
}

func persistentVolumeClaim(pvcName, namespace, clusterName string) *v1.PersistentVolumeClaim {
	pvcLabels := map[string]string{
		"logging-cluster": clusterName,
	}

	return &v1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: namespace,
			Labels:    pvcLabels,
		},
	}
}
