package k8shandler

import (
	api "github.com/ViaQ/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	defaultMasterCPULimit   = "100m"
	defaultMasterCPURequest = "100m"
	defaultCPULimit         = "4000m"
	defaultCPURequest       = "100m"
	defaultMemoryLimit      = "4Gi"
	defaultMemoryRequest    = "1Gi"
)

// addOwnerRefToObject appends the desired OwnerReference to the object
func addOwnerRefToObject(o metav1.Object, r metav1.OwnerReference) {
	if (metav1.OwnerReference{}) != r {
		o.SetOwnerReferences(append(o.GetOwnerReferences(), r))
	}
}

func selectorForES(nodeRole string, clusterName string) map[string]string {

	return map[string]string{
		nodeRole:  "true",
		"cluster": clusterName,
	}
}

func LabelsForESCluster(clusterName string) map[string]string {

	return map[string]string{
		"cluster": clusterName,
	}
}

// asOwner returns an owner reference set as the vault cluster CR
func asOwner(v *api.Elasticsearch) metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: api.SchemeGroupVersion.String(),
		Kind:       v.Kind,
		Name:       v.Name,
		UID:        v.UID,
		Controller: &trueVar,
	}
}

func getReadinessProbe() v1.Probe {
	return v1.Probe{
		TimeoutSeconds:      30,
		InitialDelaySeconds: 10,
		FailureThreshold:    15,
		Handler: v1.Handler{
			TCPSocket: &v1.TCPSocketAction{
				Port: intstr.FromInt(9300),
			},
		},
	}
}

func getResourceRequirements(commonResRequirements, nodeResRequirements v1.ResourceRequirements) v1.ResourceRequirements {
	limitCPU := nodeResRequirements.Limits.Cpu()
	if limitCPU.IsZero() {
		if commonResRequirements.Limits.Cpu().IsZero() {
			CPU, _ := resource.ParseQuantity(defaultCPULimit)
			limitCPU = &CPU
		} else {
			limitCPU = commonResRequirements.Limits.Cpu()
		}
	}
	limitMem := nodeResRequirements.Limits.Memory()
	if limitMem.IsZero() {
		if commonResRequirements.Limits.Memory().IsZero() {
			Mem, _ := resource.ParseQuantity(defaultMemoryLimit)
			limitMem = &Mem
		} else {
			limitMem = commonResRequirements.Limits.Memory()
		}

	}
	requestCPU := nodeResRequirements.Requests.Cpu()
	if requestCPU.IsZero() {
		if commonResRequirements.Requests.Cpu().IsZero() {
			CPU, _ := resource.ParseQuantity(defaultCPURequest)
			requestCPU = &CPU
		} else {
			requestCPU = commonResRequirements.Requests.Cpu()
		}
	}
	requestMem := nodeResRequirements.Requests.Memory()
	if requestMem.IsZero() {
		if commonResRequirements.Requests.Memory().IsZero() {
			Mem, _ := resource.ParseQuantity(defaultMemoryRequest)
			requestMem = &Mem
		} else {
			requestMem = commonResRequirements.Requests.Memory()
		}
	}

	return v1.ResourceRequirements{
		Limits: v1.ResourceList{
			"cpu":    *limitCPU,
			"memory": *limitMem,
		},
		Requests: v1.ResourceList{
			"cpu":    *requestCPU,
			"memory": *requestMem,
		},
	}

}
