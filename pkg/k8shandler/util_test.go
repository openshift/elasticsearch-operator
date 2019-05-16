package k8shandler

import (
	"testing"

	api "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
)

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

func TestNoRedundancyPolicySpecified(t *testing.T) {

	esCR := &api.Elasticsearch{
		Spec: api.ElasticsearchSpec{},
	}

	isValid := isValidRedundancyPolicy(esCR)

	if !isValid {
		t.Errorf("Expected default policy of SingleRedundancy to be used, incorrectly flagged as invalid")
	}

}

func TestValidRedundancyPolicySpecified(t *testing.T) {

	esCR := &api.Elasticsearch{
		Spec: api.ElasticsearchSpec{
			RedundancyPolicy: api.FullRedundancy,
		},
	}

	isValid := isValidRedundancyPolicy(esCR)

	if !isValid {
		t.Error("Expected FullRedundancy to be valid, flagged as invalid")
	}

}

func TestInvalidRedundancyPolicySpecified(t *testing.T) {

	esCR := &api.Elasticsearch{
		Spec: api.ElasticsearchSpec{
			RedundancyPolicy: "NoRedundancy",
		},
	}

	isValid := isValidRedundancyPolicy(esCR)

	if isValid {
		t.Error("Expected NoRedundancy to be flagged as invalid, was found to be valid")
	}

}

func TestValidNoNodesSpecified(t *testing.T) {

	esCR := &api.Elasticsearch{
		Spec: api.ElasticsearchSpec{
			Nodes: []api.ElasticsearchNode{},
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

	if ok, msg := hasValidUUIDs(esCR); !ok {
		t.Errorf("Expected no nodes defined to be flagged as valid, was found to be invalid UUIDs: %v", msg)
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
