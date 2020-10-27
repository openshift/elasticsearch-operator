package kibana

import (
	"os"
	"path/filepath"

	"github.com/ViaQ/logerr/kverrors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sYAML "k8s.io/apimachinery/pkg/util/yaml"
)

func NewPrometheusRule(ruleName, namespace string) *monitoringv1.PrometheusRule {
	return &monitoringv1.PrometheusRule{
		TypeMeta: metav1.TypeMeta{
			Kind:       monitoringv1.PrometheusRuleKind,
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ruleName,
			Namespace: namespace,
		},
	}
}

func (clusterRequest *KibanaRequest) RemovePrometheusRule(ruleName string) error {
	promRule := NewPrometheusRule(ruleName, clusterRequest.cluster.Namespace)

	err := clusterRequest.Delete(promRule)
	if err != nil && !apierrors.IsNotFound(err) {
		return kverrors.Wrap(err, "failed to delete prometheus rule",
			"rule", promRule,
		)
	}

	return nil
}

func NewPrometheusRuleSpecFrom(filePath string) (*monitoringv1.PrometheusRuleSpec, error) {
	f, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return nil, kverrors.Wrap(err, "failed to read prometheus spec file", "filePath", filePath)
	}
	defer f.Close()
	ruleSpec := monitoringv1.PrometheusRuleSpec{}
	if err := k8sYAML.NewYAMLOrJSONDecoder(f, 1000).Decode(&ruleSpec); err != nil {
		return nil, kverrors.Wrap(err, "failed to read prometheus spec from file", "filePath", filePath)
	}
	return &ruleSpec, nil
}
