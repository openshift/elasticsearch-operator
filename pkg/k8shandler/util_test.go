package k8shandler

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	api "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	v1 "k8s.io/api/core/v1"
)

var _ = Describe("util.go", func() {
	defer GinkgoRecover()
	Describe("#getLogConfig", func() {
		It("should return 'info' when annotation is missing", func() {
			Expect(getLogConfig(map[string]string{}).LogLevel).To(Equal("info"))
			Expect(getLogConfig(map[string]string{}).ServerLoglevel).To(Equal("info"))
			Expect(getLogConfig(map[string]string{}).ServerAppender).To(Equal("console"))
		})
		It("should return 'info' when annotation value is empty", func() {
			annotations := map[string]string{"elasticsearch.openshift.io/loglevel": "", "elasticsearch.openshift.io/develLogAppender": "", "elasticsearch.openshift.io/esloglevel": ""}
			Expect(getLogConfig(annotations).LogLevel).To(Equal("info"))
			Expect(getLogConfig(annotations).ServerLoglevel).To(Equal("info"))
			Expect(getLogConfig(annotations).ServerAppender).To(Equal("console"))
		})
		It("should return the value when annotation value is not empty", func() {
			annotations := map[string]string{"elasticsearch.openshift.io/loglevel": "foo", "elasticsearch.openshift.io/develLogAppender": "bar", "elasticsearch.openshift.io/esloglevel": "xyz"}
			Expect(getLogConfig(annotations).LogLevel).To(Equal("foo"))
			Expect(getLogConfig(annotations).ServerLoglevel).To(Equal("xyz"))
			Expect(getLogConfig(annotations).ServerAppender).To(Equal("bar"))
		})
	})
})

func TestSelectorsBothUndefined(t *testing.T) {

	commonSelector := map[string]string{}

	nodeSelector := map[string]string{}

	expected := map[string]string{}

	actual := mergeSelectors(nodeSelector, commonSelector)

	if !areSelectorsSame(actual, expected) {
		t.Errorf("Expected %v but got %v", expected, actual)
	}
}

func TestSelectorsCommonDefined(t *testing.T) {

	commonSelector := map[string]string{
		"common": "test",
	}

	nodeSelector := map[string]string{}

	expected := map[string]string{
		"common": "test",
	}

	actual := mergeSelectors(nodeSelector, commonSelector)

	if !areSelectorsSame(actual, expected) {
		t.Errorf("Expected %v but got %v", expected, actual)
	}
}

func TestSelectorsNodeDefined(t *testing.T) {

	commonSelector := map[string]string{}

	nodeSelector := map[string]string{
		"node": "test",
	}

	expected := map[string]string{
		"node": "test",
	}

	actual := mergeSelectors(nodeSelector, commonSelector)

	if !areSelectorsSame(actual, expected) {
		t.Errorf("Expected %v but got %v", expected, actual)
	}
}

func TestSelectorsCommonAndNodeDefined(t *testing.T) {

	commonSelector := map[string]string{
		"common": "test",
	}

	nodeSelector := map[string]string{
		"node": "test",
	}

	expected := map[string]string{
		"common": "test",
		"node":   "test",
	}

	actual := mergeSelectors(nodeSelector, commonSelector)

	if !areSelectorsSame(actual, expected) {
		t.Errorf("Expected %v but got %v", expected, actual)
	}
}

func TestSelectorsCommonOverwritten(t *testing.T) {

	commonSelector := map[string]string{
		"common": "test",
		"node":   "test",
		"test":   "common",
	}

	nodeSelector := map[string]string{
		"common": "node",
		"test":   "node",
	}

	expected := map[string]string{
		"common": "node",
		"node":   "test",
		"test":   "node",
	}

	actual := mergeSelectors(nodeSelector, commonSelector)

	if !areSelectorsSame(actual, expected) {
		t.Errorf("Expected %v but got %v", expected, actual)
	}
}

func TestInvalidRedundancyPolicySpecified(t *testing.T) {

	esNode := api.ElasticsearchNode{
		Roles:     []api.ElasticsearchNodeRole{"data"},
		NodeCount: int32(1),
	}

	esCR := &api.Elasticsearch{
		Spec: api.ElasticsearchSpec{
			RedundancyPolicy: api.SingleRedundancy,
			Nodes:            []api.ElasticsearchNode{esNode},
		},
	}

	//replicaCount := calculateReplicaCount(esCR)
	isValid := isValidRedundancyPolicy(esCR)

	if isValid {
		t.Error("Expected SingleRedundancy with one data node to be invalid, flagged as valid")
	}

	esCR = &api.Elasticsearch{
		Spec: api.ElasticsearchSpec{
			RedundancyPolicy: api.MultipleRedundancy,
			Nodes:            []api.ElasticsearchNode{esNode},
		},
	}

	isValid = isValidRedundancyPolicy(esCR)

	if isValid {
		t.Error("Expected MultipleRedundancy with two data nodes to be invalid, flagged as valid")
	}

	esCR = &api.Elasticsearch{
		Spec: api.ElasticsearchSpec{
			RedundancyPolicy: api.FullRedundancy,
			Nodes:            []api.ElasticsearchNode{esNode},
		},
	}

	isValid = isValidRedundancyPolicy(esCR)

	if isValid {
		t.Error("Expected FullRedundancy with two data nodes to be invalid, flagged as valid")
	}
}

func TestValidRedundancyPolicySpecified(t *testing.T) {

	esNode := api.ElasticsearchNode{
		Roles:     []api.ElasticsearchNodeRole{"data"},
		NodeCount: int32(1),
	}

	esCR := &api.Elasticsearch{
		Spec: api.ElasticsearchSpec{
			RedundancyPolicy: api.ZeroRedundancy,
			Nodes:            []api.ElasticsearchNode{esNode},
		},
	}

	isValid := isValidRedundancyPolicy(esCR)

	if !isValid {
		t.Error("Expected ZeroRedundancy with one data node to be valid, flagged as invalid")
	}

	esNode = api.ElasticsearchNode{
		Roles:     []api.ElasticsearchNodeRole{"data"},
		NodeCount: int32(2),
	}

	esCR = &api.Elasticsearch{
		Spec: api.ElasticsearchSpec{
			RedundancyPolicy: api.SingleRedundancy,
			Nodes:            []api.ElasticsearchNode{esNode},
		},
	}

	isValid = isValidRedundancyPolicy(esCR)

	if !isValid {
		t.Error("Expected SingleRedundancy with two data nodes to be valid, flagged as invalid")
	}

	esCR = &api.Elasticsearch{
		Spec: api.ElasticsearchSpec{
			RedundancyPolicy: api.MultipleRedundancy,
			Nodes:            []api.ElasticsearchNode{esNode},
		},
	}

	isValid = isValidRedundancyPolicy(esCR)

	if !isValid {
		t.Error("Expected MultipleRedundancy with two data nodes to be valid, flagged as invalid")
	}

	esCR = &api.Elasticsearch{
		Spec: api.ElasticsearchSpec{
			RedundancyPolicy: api.FullRedundancy,
			Nodes:            []api.ElasticsearchNode{esNode},
		},
	}

	isValid = isValidRedundancyPolicy(esCR)

	if !isValid {
		t.Error("Expected FullRedundancy with two data nodes to be valid, flagged as invalid")
	}
}

func TestValidNoNodesSpecified(t *testing.T) {

	esCR := &api.Elasticsearch{
		Spec: api.ElasticsearchSpec{
			Nodes:            []api.ElasticsearchNode{},
			RedundancyPolicy: api.ZeroRedundancy,
		},
	}

	isValid := isValidMasterCount(esCR)

	if !isValid {
		t.Error("Expected no nodes defined to be flagged as valid, was found to be invalid master count")
	}

	isValid = isValidDataCount(esCR)

	if !isValid {
		t.Error("Expected no nodes defined to be flagged as valid, was found to be invalid data count")
	}

	isValid = isValidRedundancyPolicy(esCR)

	if !isValid {
		t.Error("Expected no nodes defined to be flagged as valid, was found to be invalid redundancy policy")
	}

	if err := validateUUIDs(esCR); err != nil {
		t.Errorf("Expected no nodes defined to be flagged as valid, was found to be invalid UUIDs: %v", err)
	}
}

func TestValidReplicaCount(t *testing.T) {

	dataNodeCount := 5

	esNode := api.ElasticsearchNode{
		Roles:     []api.ElasticsearchNodeRole{"data"},
		NodeCount: int32(dataNodeCount),
	}

	esCR := &api.Elasticsearch{
		Spec: api.ElasticsearchSpec{
			RedundancyPolicy: api.FullRedundancy,
			Nodes:            []api.ElasticsearchNode{esNode},
		},
	}

	rc := calculateReplicaCount(esCR)

	// FullRedundancy = dataNodeCount - 1
	if rc != dataNodeCount-1 {
		t.Errorf("Expected 4 replica shards for 5 data nodes and FullRedundancy policy, got %d", rc)
	}

}

func TestNoReplicaCount(t *testing.T) {

	dataNodeCount := 5

	esNode := api.ElasticsearchNode{
		Roles:     []api.ElasticsearchNodeRole{"data"},
		NodeCount: int32(dataNodeCount),
	}

	esCR := &api.Elasticsearch{
		Spec: api.ElasticsearchSpec{
			Nodes: []api.ElasticsearchNode{esNode},
		},
	}

	rc := calculateReplicaCount(esCR)

	// we default to 1
	if rc != 1 {
		t.Errorf("Expected SingleRedundancy, when no policy is specified and cluster has 2 or more data nodes, got %d replica shards", rc)
	}

}

func TestSingleNodeNoReplicaCount(t *testing.T) {

	dataNodeCount := 1

	esNode := api.ElasticsearchNode{
		Roles:     []api.ElasticsearchNodeRole{"data"},
		NodeCount: int32(dataNodeCount),
	}

	esCR := &api.Elasticsearch{
		Spec: api.ElasticsearchSpec{
			Nodes: []api.ElasticsearchNode{esNode},
		},
	}

	rc := calculateReplicaCount(esCR)

	if rc != 0 {
		t.Errorf("Expected ZeroRedundancy, when no policy is specified and cluster has only 1 data node, got %d replica shards", rc)
	}

}

func TestNoTolerations(t *testing.T) {
	commonTolerations := []v1.Toleration{}

	nodeTolerations := []v1.Toleration{}

	expected := []v1.Toleration{}

	actual := appendTolerations(nodeTolerations, commonTolerations)

	if !areTolerationsSame(actual, expected) {
		t.Errorf("Expected %v but got %v", expected, actual)
	}
}

func TestNoNodeTolerations(t *testing.T) {
	commonTolerations := []v1.Toleration{
		{
			Key:      "node.kubernetes.io/disk-pressure",
			Operator: v1.TolerationOpExists,
			Effect:   v1.TaintEffectNoSchedule,
		},
	}

	nodeTolerations := []v1.Toleration{}

	expected := []v1.Toleration{
		{
			Key:      "node.kubernetes.io/disk-pressure",
			Operator: v1.TolerationOpExists,
			Effect:   v1.TaintEffectNoSchedule,
		},
	}

	actual := appendTolerations(nodeTolerations, commonTolerations)

	if !areTolerationsSame(actual, expected) {
		t.Errorf("Expected %v but got %v", expected, actual)
	}
}

func TestNoCommonTolerations(t *testing.T) {
	commonTolerations := []v1.Toleration{}

	nodeTolerations := []v1.Toleration{
		{
			Key:      "node.kubernetes.io/disk-pressure",
			Operator: v1.TolerationOpExists,
			Effect:   v1.TaintEffectNoSchedule,
		},
	}

	expected := []v1.Toleration{
		{
			Key:      "node.kubernetes.io/disk-pressure",
			Operator: v1.TolerationOpExists,
			Effect:   v1.TaintEffectNoSchedule,
		},
	}

	actual := appendTolerations(nodeTolerations, commonTolerations)

	if !areTolerationsSame(actual, expected) {
		t.Errorf("Expected %v but got %v", expected, actual)
	}
}

func TestTolerations(t *testing.T) {
	commonTolerations := []v1.Toleration{
		{
			Key:      "node.kubernetes.io/disk-pressure",
			Operator: v1.TolerationOpExists,
			Effect:   v1.TaintEffectNoSchedule,
		},
	}

	nodeTolerations := []v1.Toleration{
		{
			Key:      "node.kubernetes.io/memory-pressure",
			Operator: v1.TolerationOpExists,
			Effect:   v1.TaintEffectNoSchedule,
		},
	}

	expected := []v1.Toleration{
		{
			Key:      "node.kubernetes.io/disk-pressure",
			Operator: v1.TolerationOpExists,
			Effect:   v1.TaintEffectNoSchedule,
		},
		{
			Key:      "node.kubernetes.io/memory-pressure",
			Operator: v1.TolerationOpExists,
			Effect:   v1.TaintEffectNoSchedule,
		},
	}

	actual := appendTolerations(nodeTolerations, commonTolerations)

	if !areTolerationsSame(actual, expected) {
		t.Errorf("Expected %v but got %v", expected, actual)
	}

	// ensure that ordering does not make a difference
	expected = []v1.Toleration{
		{
			Key:      "node.kubernetes.io/memory-pressure",
			Operator: v1.TolerationOpExists,
			Effect:   v1.TaintEffectNoSchedule,
		},
		{
			Key:      "node.kubernetes.io/disk-pressure",
			Operator: v1.TolerationOpExists,
			Effect:   v1.TaintEffectNoSchedule,
		},
	}

	if !areTolerationsSame(actual, expected) {
		t.Errorf("Expected %v but got %v", expected, actual)
	}
}
