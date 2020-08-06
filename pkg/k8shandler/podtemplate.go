package k8shandler

import (
	"reflect"

	"github.com/openshift/elasticsearch-operator/pkg/utils"
	"github.com/openshift/elasticsearch-operator/pkg/utils/comparators"
	v1 "k8s.io/api/core/v1"
)

// ArePodTemplateSpecDifferent compares two v1.PodTemplateSpecs
// and returns True or False
func ArePodTemplateSpecDifferent(lhs, rhs v1.PodTemplateSpec) bool {

	return ArePodSpecDifferent(lhs.Spec, rhs.Spec, true)
}

// Abstracted logic into comparing pod specs so that we can check if our change has been rolled out
// yet or not
func ArePodSpecDifferent(lhs, rhs v1.PodSpec, strictTolerations bool) bool {
	changed := false

	if len(lhs.Containers) != len(rhs.Containers) {
		changed = true
	}

	// check nodeselectors
	if !areSelectorsSame(lhs.NodeSelector, rhs.NodeSelector) {
		changed = true
	}

	// strictTolerations are for when we compare from the deployments or statefulsets
	// if we are seeing if rolled out pods contain changes we don't want strictTolerations
	//   since k8s may add additional tolerations to pods
	if strictTolerations {
		// check tolerations
		if !areTolerationsSame(lhs.Tolerations, rhs.Tolerations) {
			changed = true
		}
	} else {
		// check tolerations
		if !containsSameTolerations(lhs.Tolerations, rhs.Tolerations) {
			changed = true
		}
	}

	// check container fields
	for _, lContainer := range lhs.Containers {
		found := false

		for _, rContainer := range rhs.Containers {
			// Only compare the images of containers with the same name
			if lContainer.Name == rContainer.Name {
				found = true

				if lContainer.Image != rContainer.Image {
					changed = true
				}

				if !comparators.EnvValueEqual(lContainer.Env, rContainer.Env) {
					changed = true
				}

				if !reflect.DeepEqual(lContainer.Args, rContainer.Args) {
					//logger.Debugf("Container Args are different between current and desired for %s", nodeContainer.Name)
					changed = true
				}

				if !reflect.DeepEqual(lContainer.Ports, rContainer.Ports) {
					//logger.Debugf("Container Ports are different between current and desired for %s", nodeContainer.Name)
					changed = true
				}

				if different, _ := utils.CompareResources(lContainer.Resources, rContainer.Resources); different {
					changed = true
				}
			}
		}

		if !found {
			changed = true
		}
	}
	return changed
}
