package elasticsearch

import (
	"reflect"

	"github.com/openshift/elasticsearch-operator/pkg/utils/comparators"

	"github.com/sirupsen/logrus"

	"github.com/openshift/elasticsearch-operator/pkg/logger"

	v1 "k8s.io/api/core/v1"
)

func UpdatePodTemplateSpec(name string, actual, desired *v1.PodTemplateSpec) bool {

	changed := false

	// check the pod's nodeselector
	if !comparators.AreStringMapsSame(actual.Spec.NodeSelector, desired.Spec.NodeSelector) {
		logrus.Debugf("Resource '%s' has different nodeSelector than desired", name)
		actual.Spec.NodeSelector = desired.Spec.NodeSelector
		changed = true
	}

	// check the pod's tolerations
	if !comparators.AreTolerationsSame(actual.Spec.Tolerations, desired.Spec.Tolerations) {
		logrus.Debugf("Resource '%s' has different tolerations than desired", name)
		actual.Spec.Tolerations = desired.Spec.Tolerations
		changed = true
	}

	// Only Image and Resources (CPU & memory) differences trigger rolling restart
	for index, nodeContainer := range actual.Spec.Containers {
		for _, desiredContainer := range desired.Spec.Containers {

			if nodeContainer.Name == desiredContainer.Name {
				if nodeContainer.Image != desiredContainer.Image {
					logrus.Debugf("Resource '%s' has different container image than desired", name)
					nodeContainer.Image = desiredContainer.Image
					changed = true
				}

				if !comparators.EnvValueEqual(desiredContainer.Env, nodeContainer.Env) {
					nodeContainer.Env = desiredContainer.Env
					logger.Debugf("Container EnvVars are different between current and desired for %s", nodeContainer.Name)
					changed = true
				}

				if !reflect.DeepEqual(desiredContainer.Args, nodeContainer.Args) {
					nodeContainer.Args = desiredContainer.Args
					logger.Debugf("Container Args are different between current and desired for %s", nodeContainer.Name)
					changed = true
				}

				var updatedContainer v1.Container
				var resourceUpdated bool
				if updatedContainer, resourceUpdated = UpdateResources(name, nodeContainer, desiredContainer); resourceUpdated {
					changed = true
				}

				actual.Spec.Containers[index] = updatedContainer

			}

		}
	}

	return changed
}

//updateResources for the node; return true if an actual change is made
func UpdateResources(name string, nodeContainer, desiredContainer v1.Container) (v1.Container, bool) {
	changed := false
	if nodeContainer.Resources.Requests == nil {
		nodeContainer.Resources.Requests = v1.ResourceList{}
	}

	if nodeContainer.Resources.Limits == nil {
		nodeContainer.Resources.Limits = v1.ResourceList{}
	}

	// Check CPU limits
	if desiredContainer.Resources.Limits.Cpu().Cmp(*nodeContainer.Resources.Limits.Cpu()) != 0 {
		logrus.Debugf("Resource '%s' has different CPU (%+v) limit than desired (%+v)", name, *nodeContainer.Resources.Limits.Cpu(), desiredContainer.Resources.Limits.Cpu())
		nodeContainer.Resources.Limits[v1.ResourceCPU] = *desiredContainer.Resources.Limits.Cpu()
		if nodeContainer.Resources.Limits.Cpu().IsZero() {
			delete(nodeContainer.Resources.Limits, v1.ResourceCPU)
		}
		changed = true
	}
	// Check memory limits
	if desiredContainer.Resources.Limits.Memory().Cmp(*nodeContainer.Resources.Limits.Memory()) != 0 {
		logrus.Debugf("Resource '%s' has different Memory limit than desired", name)
		nodeContainer.Resources.Limits[v1.ResourceMemory] = *desiredContainer.Resources.Limits.Memory()
		if nodeContainer.Resources.Limits.Memory().IsZero() {
			delete(nodeContainer.Resources.Limits, v1.ResourceMemory)
		}
		changed = true
	}
	// Check CPU requests
	if desiredContainer.Resources.Requests.Cpu().Cmp(*nodeContainer.Resources.Requests.Cpu()) != 0 {
		logrus.Debugf("Resource '%s' has different CPU Request than desired", name)
		nodeContainer.Resources.Requests[v1.ResourceCPU] = *desiredContainer.Resources.Requests.Cpu()
		if nodeContainer.Resources.Requests.Cpu().IsZero() {
			delete(nodeContainer.Resources.Requests, v1.ResourceCPU)
		}
		changed = true
	}
	// Check memory requests
	if desiredContainer.Resources.Requests.Memory().Cmp(*nodeContainer.Resources.Requests.Memory()) != 0 {
		logrus.Debugf("Resource '%s' has different Memory Request than desired", name)
		nodeContainer.Resources.Requests[v1.ResourceMemory] = *desiredContainer.Resources.Requests.Memory()
		if nodeContainer.Resources.Requests.Memory().IsZero() {
			delete(nodeContainer.Resources.Requests, v1.ResourceMemory)
		}
		changed = true
	}

	return nodeContainer, changed
}
