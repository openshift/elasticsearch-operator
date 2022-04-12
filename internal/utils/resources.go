package utils

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func CompareResources(current, desired v1.ResourceRequirements) (bool, v1.ResourceRequirements) {
	changed := false
	desiredResources := *current.DeepCopy()
	if desiredResources.Limits == nil {
		desiredResources.Limits = map[v1.ResourceName]resource.Quantity{}
	}
	if desiredResources.Requests == nil {
		desiredResources.Requests = map[v1.ResourceName]resource.Quantity{}
	}

	if desired.Limits.Cpu().Cmp(*current.Limits.Cpu()) != 0 {
		desiredResources.Limits[v1.ResourceCPU] = *desired.Limits.Cpu()
		changed = true
	}
	// Check memory limits
	if desired.Limits.Memory().Cmp(*current.Limits.Memory()) != 0 {
		desiredResources.Limits[v1.ResourceMemory] = *desired.Limits.Memory()
		changed = true
	}
	// Check CPU requests
	if desired.Requests.Cpu().Cmp(*current.Requests.Cpu()) != 0 {
		desiredResources.Requests[v1.ResourceCPU] = *desired.Requests.Cpu()
		changed = true
	}
	// Check memory requests
	if desired.Requests.Memory().Cmp(*current.Requests.Memory()) != 0 {
		desiredResources.Requests[v1.ResourceMemory] = *desired.Requests.Memory()
		changed = true
	}

	return changed, desiredResources
}
