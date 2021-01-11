package kibana

import (
	"github.com/ViaQ/logerr/kverrors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewServiceMonitor(serviceMonitorName, namespace string) *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       monitoringv1.ServiceMonitorsKind,
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceMonitorName,
			Namespace: namespace,
		},
	}
}

func (clusterRequest *KibanaRequest) RemoveServiceMonitor(smName string) error {
	serviceMonitor := NewServiceMonitor(smName, clusterRequest.cluster.Namespace)

	err := clusterRequest.Delete(serviceMonitor)
	if err != nil && !apierrors.IsNotFound(kverrors.Root(err)) {
		return kverrors.Wrap(err, "failed to delete service monitor",
			"service_monitor", serviceMonitor,
		)
	}

	return nil
}
