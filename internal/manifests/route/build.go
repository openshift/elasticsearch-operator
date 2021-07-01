package route

import (
	routev1 "github.com/openshift/api/route/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Builder represents the struct to build openshift api route objects
type Builder struct {
	r *routev1.Route
}

// New returns a new Builder for openshift api route objects.
func New(routeName, namespace, serviceName string, labels map[string]string) *Builder {
	return &Builder{r: newRoute(routeName, namespace, serviceName, labels)}
}

func newRoute(routeName, namespace, serviceName string, labels map[string]string) *routev1.Route {
	return &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Route",
			APIVersion: routev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Name: serviceName,
				Kind: "Service",
			},
		},
	}
}

// Build returns the final route object
func (b *Builder) Build() *routev1.Route { return b.r }

// WithTLSConfig sets the route TLS configuration
func (b *Builder) WithTLSConfig(tc *routev1.TLSConfig) *Builder {
	b.r.Spec.TLS = tc
	return b
}

// WithCA sets the certificate authority to the TLS config if present
func (b *Builder) WithCA(caCert []byte) *Builder {
	if b.r.Spec.TLS != nil {
		b.r.Spec.TLS.CACertificate = string(caCert)
		b.r.Spec.TLS.DestinationCACertificate = string(caCert)
	}
	return b
}
