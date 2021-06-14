package kibana

import (
	"github.com/openshift/elasticsearch-operator/internal/manifests/deployment"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewDeployment stubs an instance of a Deployment
func NewDeployment(deploymentName string, namespace string, loggingComponent string, component string, replicas int32, podSpec core.PodSpec) *apps.Deployment {
	labels := map[string]string{
		"provider":      "openshift",
		"component":     "kibana",
		"logging-infra": "kibana",
	}

	kibanaDeployment := deployment.New("kibana", namespace, labels, replicas).
		WithSelector(metav1.LabelSelector{
			MatchLabels: labels,
		}).
		WithStrategy(apps.RollingUpdateDeploymentStrategyType).
		WithTemplate(core.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "kibana",
				Labels: labels,
			},
			Spec: podSpec,
		}).
		Build()

	return kibanaDeployment
}
