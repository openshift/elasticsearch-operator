package pod

import (
	"testing"

	"github.com/openshift/elasticsearch-operator/internal/utils"

	corev1 "k8s.io/api/core/v1"
)

func TestNodeAllocationLabelsForPod(t *testing.T) {
	// Create pod with nil selectors, we expect a new selectors map will be created
	// and it will contain only linux allocation selector.
	podSpec := NewSpec(
		"Foo",
		[]corev1.Container{},
		[]corev1.Volume{},
	).Build()

	if podSpec.NodeSelector == nil {
		t.Errorf("Exp. the nodeSelector to contains the linux allocation selector but was %T", podSpec.NodeSelector)
	}
	if len(podSpec.NodeSelector) != 1 {
		t.Errorf("Exp. single nodeSelector but %d were found", len(podSpec.NodeSelector))
	}
	if podSpec.NodeSelector[utils.OsNodeLabel] != utils.LinuxValue {
		t.Errorf("Exp. the nodeSelector to contains %s: %s pair", utils.OsNodeLabel, utils.LinuxValue)
	}

	// Create pod with some "foo" selector, we expect a new linux box selector will be added
	// while existing selectors will be left intact.
	podSpec = NewSpec(
		"Foo",
		[]corev1.Container{},
		[]corev1.Volume{},
	).WithNodeSelectors(map[string]string{"foo": "bar"}).Build()

	if podSpec.NodeSelector == nil {
		t.Errorf("Exp. the nodeSelector to contains the linux allocation selector but was %T", podSpec.NodeSelector)
	}
	if len(podSpec.NodeSelector) != 2 {
		t.Errorf("Exp. single nodeSelector but %d were found", len(podSpec.NodeSelector))
	}
	if podSpec.NodeSelector["foo"] != "bar" {
		t.Errorf("Exp. the nodeSelector to contains %s: %s pair", "foo", "bar")
	}
	if podSpec.NodeSelector[utils.OsNodeLabel] != utils.LinuxValue {
		t.Errorf("Exp. the nodeSelector to contains %s: %s pair", utils.OsNodeLabel, utils.LinuxValue)
	}

	// Create pod with "linux" selector, we expect it stays unchanged.
	podSpec = NewSpec(
		"Foo",
		[]corev1.Container{},
		[]corev1.Volume{},
	).WithNodeSelectors(map[string]string{utils.OsNodeLabel: utils.LinuxValue}).Build()

	if podSpec.NodeSelector == nil {
		t.Errorf("Exp. the nodeSelector to contains the linux allocation selector but was %T", podSpec.NodeSelector)
	}
	if len(podSpec.NodeSelector) != 1 {
		t.Errorf("Exp. single nodeSelector but %d were found", len(podSpec.NodeSelector))
	}
	if podSpec.NodeSelector[utils.OsNodeLabel] != utils.LinuxValue {
		t.Errorf("Exp. the nodeSelector to contains %s: %s pair", utils.OsNodeLabel, utils.LinuxValue)
	}

	// Create pod with some "non-linux" selector, we expect it is overridden.
	podSpec = NewSpec(
		"Foo",
		[]corev1.Container{},
		[]corev1.Volume{},
	).WithNodeSelectors(map[string]string{utils.OsNodeLabel: "Donald Duck"}).Build()

	if podSpec.NodeSelector == nil {
		t.Errorf("Exp. the nodeSelector to contains the linux allocation selector but was %T", podSpec.NodeSelector)
	}
	if len(podSpec.NodeSelector) != 1 {
		t.Errorf("Exp. single nodeSelector but %d were found", len(podSpec.NodeSelector))
	}
	if custom := podSpec.NodeSelector[utils.OsNodeLabel]; custom != utils.LinuxValue {
		t.Errorf("Exp. the nodeSelector was overridden from %s: %s pair to %s", utils.OsNodeLabel, utils.LinuxValue, custom)
	}
}
