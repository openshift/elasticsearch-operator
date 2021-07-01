package pod

import (
	"reflect"

	"github.com/openshift/elasticsearch-operator/internal/utils/comparators"

	corev1 "k8s.io/api/core/v1"
)

// ArePodTemplateSpecEqual compares two corev1.PodTemplateSpec objects
// and returns true only if pod spec are equal and tolerations are strictly the same
func ArePodTemplateSpecEqual(lhs, rhs corev1.PodTemplateSpec) bool {
	return ArePodSpecEqual(lhs.Spec, rhs.Spec, true)
}

// ArePodSpecEqual compares two corev1.PodSpec objects and returns true
// only if they are equal in any of the following:
// - Length of containers slice
// - Node selectors
// - Tolerations, if strict they need to be the same, non-strict for superset check
// - Containers: Name, Image, VolumeMounts, EnvVar, Args, Ports, ResourceRequirements
func ArePodSpecEqual(lhs, rhs corev1.PodSpec, strictTolerations bool) bool {
	equal := true

	if len(lhs.Containers) != len(rhs.Containers) {
		equal = false
	}

	// check nodeselectors
	if !comparators.AreSelectorsSame(lhs.NodeSelector, rhs.NodeSelector) {
		equal = false
	}

	// strictTolerations are for when we compare from the deployments or statefulsets
	// if we are seeing if rolled out pods contain changes we don't want strictTolerations
	//   since k8s may add additional tolerations to pods
	if strictTolerations {
		// check tolerations
		if !comparators.AreTolerationsSame(lhs.Tolerations, rhs.Tolerations) {
			equal = false
		}
	} else {
		// check tolerations
		if !comparators.ContainsSameTolerations(lhs.Tolerations, rhs.Tolerations) {
			equal = false
		}
	}

	// check container fields
	for _, lContainer := range lhs.Containers {
		found := false

		for _, rContainer := range rhs.Containers {
			// Only compare the images of containers with the same name
			if lContainer.Name != rContainer.Name {
				continue
			}

			found = true

			// can't use reflect.DeepEqual here, due to k8s adding token mounts
			// check that rContainer is all found within lContainer and that they match by name
			if !comparators.ContainsSameVolumeMounts(lContainer.VolumeMounts, rContainer.VolumeMounts) {
				equal = false
			}

			if lContainer.Image != rContainer.Image {
				equal = false
			}

			if !comparators.EnvValueEqual(lContainer.Env, rContainer.Env) {
				equal = false
			}

			if !reflect.DeepEqual(lContainer.Args, rContainer.Args) {
				equal = false
			}

			if !reflect.DeepEqual(lContainer.Ports, rContainer.Ports) {
				equal = false
			}

			if !comparators.AreResourceRequementsSame(lContainer.Resources, rContainer.Resources) {
				equal = false
			}
		}

		if !found {
			equal = false
		}
	}

	return equal
}
