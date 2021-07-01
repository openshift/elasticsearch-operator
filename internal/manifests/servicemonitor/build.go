package servicemonitor

import (
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Builder represents the struct to build servicemonitors
type Builder struct {
	sm *monitoringv1.ServiceMonitor
}

// New returns a Builder for servicemonitors.
func New(smName, namespace string, labels map[string]string) *Builder {
	return &Builder{sm: newServiceMonitor(smName, namespace, labels)}
}

func newServiceMonitor(serviceMonitorName, namespace string, labels map[string]string) *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       monitoringv1.ServiceMonitorsKind,
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceMonitorName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: monitoringv1.ServiceMonitorSpec{},
	}
}

// Build returns the final servicemonitor
func (b *Builder) Build() *monitoringv1.ServiceMonitor { return b.sm }

// WithJobLabel sets the servicemonitor job label
func (b *Builder) WithJobLabel(l string) *Builder {
	b.sm.Spec.JobLabel = l
	return b
}

// WithSelector sets the servicemonitor selector
func (b *Builder) WithSelector(s metav1.LabelSelector) *Builder {
	b.sm.Spec.Selector = s
	return b
}

// WithNamespaceSelector sets ths servicemonitor namespace selector
func (b *Builder) WithNamespaceSelector(nss monitoringv1.NamespaceSelector) *Builder {
	b.sm.Spec.NamespaceSelector = nss
	return b
}

// WithEndpoints appends endpoints to the servicemonitor
func (b *Builder) WithEndpoints(ep ...monitoringv1.Endpoint) *Builder {
	b.sm.Spec.Endpoints = append(b.sm.Spec.Endpoints, ep...)
	return b
}
