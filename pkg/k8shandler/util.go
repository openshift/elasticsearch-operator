package k8shandler

import (
	"context"
	"fmt"
	"strings"

	"github.com/ViaQ/logerr/kverrors"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/openshift/elasticsearch-operator/apis/logging/v1"
)

const (
	loglevelAnnotation          = "elasticsearch.openshift.io/loglevel"
	serverLogAppenderAnnotation = "elasticsearch.openshift.io/develLogAppender"
	serverLoglevelAnnotation    = "elasticsearch.openshift.io/esloglevel"
)

type LogConfig struct {
	// LogLevel of the proxy and server security
	LogLevel string
	// ServerLoglevel of the remainder of Elasticsearch
	ServerLoglevel string
	// ServerAppender where to log messages
	ServerAppender string
}

func getLogConfig(annotations map[string]string) LogConfig {
	config := LogConfig{"info", "info", "console"}
	if value, found := annotations[loglevelAnnotation]; found {
		if strings.TrimSpace(value) != "" {
			config.LogLevel = value
		}
	}
	if value, found := annotations[serverLoglevelAnnotation]; found {
		if strings.TrimSpace(value) != "" {
			config.ServerLoglevel = value
		}
	}
	if value, found := annotations[serverLogAppenderAnnotation]; found {
		if strings.TrimSpace(value) != "" {
			config.ServerAppender = value
		}
	}
	return config
}

func selectorForES(nodeRole string, clusterName string) map[string]string {
	return map[string]string{
		nodeRole:       "true",
		"cluster-name": clusterName,
	}
}

func appendDefaultLabel(clusterName string, labels map[string]string) map[string]string {
	if _, ok := labels["cluster-name"]; ok {
		return labels
	}
	if labels == nil {
		labels = map[string]string{}
	}
	labels["cluster-name"] = clusterName
	return labels
}

func areSelectorsSame(lhs, rhs map[string]string) bool {
	if len(lhs) != len(rhs) {
		return false
	}

	for lhsKey, lhsVal := range lhs {
		rhsVal, ok := rhs[lhsKey]
		if !ok || lhsVal != rhsVal {
			return false
		}
	}

	return true
}

func mergeSelectors(nodeSelectors, commonSelectors map[string]string) map[string]string {
	if commonSelectors == nil {
		commonSelectors = make(map[string]string)
	}

	for k, v := range nodeSelectors {
		commonSelectors[k] = v
	}

	return commonSelectors
}

func areTolerationsSame(lhs, rhs []v1.Toleration) bool {
	// if we are checking this as a part of pod spec comparison during a rollout we can't check this
	// if we are comparing the deployment specs we can...
	if len(lhs) != len(rhs) {
		return false
	}

	return containsSameTolerations(lhs, rhs)
}

// containsSameTolerations checks that the tolerations in rhs are all contained within lhs
// this follows our other patterns of "current, desired"
func containsSameTolerations(lhs, rhs []v1.Toleration) bool {
	for _, rhsToleration := range rhs {
		if !containsToleration(rhsToleration, lhs) {
			return false
		}
	}

	return true
}

func containsToleration(toleration v1.Toleration, tolerations []v1.Toleration) bool {
	for _, t := range tolerations {
		if isTolerationSame(t, toleration) {
			return true
		}
	}

	return false
}

func isTolerationSame(lhs, rhs v1.Toleration) bool {
	tolerationSecondsBool := false
	// check that both are either null or not null
	if (lhs.TolerationSeconds == nil) == (rhs.TolerationSeconds == nil) {
		if lhs.TolerationSeconds != nil {
			// only compare values (attempt to dereference) if pointers aren't nil
			tolerationSecondsBool = *lhs.TolerationSeconds == *rhs.TolerationSeconds
		} else {
			tolerationSecondsBool = true
		}
	}

	return (lhs.Key == rhs.Key) &&
		(lhs.Operator == rhs.Operator) &&
		(lhs.Value == rhs.Value) &&
		(lhs.Effect == rhs.Effect) &&
		tolerationSecondsBool
}

func appendTolerations(nodeTolerations, commonTolerations []v1.Toleration) []v1.Toleration {
	if commonTolerations == nil {
		commonTolerations = []v1.Toleration{}
	}

	return append(commonTolerations, nodeTolerations...)
}

func getMasterCount(dpl *api.Elasticsearch) int32 {
	masterCount := int32(0)
	for _, node := range dpl.Spec.Nodes {
		if isMasterNode(node) {
			masterCount += node.NodeCount
		}
	}

	return masterCount
}

func getDataCount(dpl *api.Elasticsearch) int32 {
	dataCount := int32(0)
	for _, node := range dpl.Spec.Nodes {
		if isDataNode(node) {
			dataCount = dataCount + node.NodeCount
		}
	}
	return dataCount
}

func isValidMasterCount(dpl *api.Elasticsearch) bool {
	if len(dpl.Spec.Nodes) == 0 {
		return true
	}

	masterCount := int(getMasterCount(dpl))
	return masterCount <= maxMasterCount && masterCount > 0
}

func isValidDataCount(dpl *api.Elasticsearch) bool {
	if len(dpl.Spec.Nodes) == 0 {
		return true
	}

	dataCount := int(getDataCount(dpl))
	return dataCount > 0
}

func isValidRedundancyPolicy(dpl *api.Elasticsearch) bool {
	dataCount := int(getDataCount(dpl))

	switch dpl.Spec.RedundancyPolicy {
	case api.ZeroRedundancy:
		return true
	case api.SingleRedundancy, api.MultipleRedundancy, api.FullRedundancy:
		return dataCount > 1
	default:
		return false
	}
}

func (er *ElasticsearchRequest) isValidConf() error {
	dpl := er.cluster

	if !isValidMasterCount(dpl) {
		if err := updateConditionWithRetry(dpl, v1.ConditionTrue, updateInvalidMasterCountCondition, er.client); err != nil {
			return err
		}
		return kverrors.New("invalid master nodes count. Please ensure the total nodes with master roles is less than the maximum",
			"maximum", maxMasterCount)
	} else {
		if err := updateConditionWithRetry(dpl, v1.ConditionFalse, updateInvalidMasterCountCondition, er.client); err != nil {
			return kverrors.Wrap(err, "failed to set master count status")
		}
	}

	if !isValidDataCount(dpl) {
		if err := updateConditionWithRetry(dpl, v1.ConditionTrue, updateInvalidDataCountCondition, er.client); err != nil {
			return kverrors.Wrap(err, "failed to set data count status")
		}
		return kverrors.New("no data nodes requested. Please ensure there is at least 1 node with data roles")
	} else {
		if err := updateConditionWithRetry(dpl, v1.ConditionFalse, updateInvalidDataCountCondition, er.client); err != nil {
			return kverrors.Wrap(err, "failed to set data count status")
		}
	}

	if !isValidRedundancyPolicy(dpl) {
		if err := updateConditionWithRetry(dpl, v1.ConditionTrue, updateInvalidReplicationCondition, er.client); err != nil {
			return kverrors.Wrap(err, "failed to set replication status")
		}
		return kverrors.New("wrong RedundancyPolicy selected. Choose different RedundancyPolicy or add more nodes with data roles",
			"policy", dpl.Spec.RedundancyPolicy)
	} else {
		if err := updateConditionWithRetry(dpl, v1.ConditionFalse, updateInvalidReplicationCondition, er.client); err != nil {
			return kverrors.Wrap(err, "failed to set replication status")
		}
	}

	// TODO: replace this with a validating web hook to ensure field is immutable
	if err := validateUUIDs(dpl); err != nil {
		if err := updateInvalidUUIDChangeCondition(dpl, v1.ConditionTrue, err.Error(), er.client); err != nil {
			return kverrors.Wrap(err, "failed to set UUID change status")
		}
		return kverrors.Wrap(err, "unsupported change to UUIDs made")
	} else {
		if err := updateInvalidUUIDChangeCondition(dpl, v1.ConditionFalse, "", er.client); err != nil {
			return kverrors.Wrap(err, "failed to set UUID change status")
		}
	}

	return nil
}

func validateUUIDs(dpl *api.Elasticsearch) error {
	// TODO:
	// check that someone didn't update a uuid
	// check status.nodes[*].deploymentName for list of used uuids
	// deploymentName should match pattern {cluster.Name}-{uuid}[-replica]
	// if any in that list aren't found in spec.Nodes[*].GenUUID then someone did something bad...
	// somehow rollback the cluster object change and update message?
	// no way to rollback, but maybe maintain a last known "good state" and update SPEC to that?
	// update status message to be very descriptive of this

	prefix := fmt.Sprintf("%s-", dpl.Name)

	var knownUUIDs []string
	for _, node := range dpl.Status.Nodes {

		var nodeName string
		if node.DeploymentName != "" {
			nodeName = node.DeploymentName
		}

		if node.StatefulSetName != "" {
			nodeName = node.StatefulSetName
		}

		parts := strings.Split(strings.TrimPrefix(nodeName, prefix), "-")

		if len(parts) < 2 {
			return kverrors.New("invalid name found for node",
				"node", nodeName)
		}

		uuid := parts[1]

		if !sliceContainsString(knownUUIDs, uuid) {
			knownUUIDs = append(knownUUIDs, uuid)
		}
	}

	// make sure all known UUIDs are found amongst spec.nodes[*].genuuid
	for _, uuid := range knownUUIDs {
		if !isUUIDFound(uuid, dpl.Spec.Nodes) {
			return kverrors.New("previously used GenUUID is no longer found in Spec.Nodes",
				"uuid", uuid)
		}
	}

	return nil
}

func isUUIDFound(uuid string, nodes []api.ElasticsearchNode) bool {
	for _, node := range nodes {
		if node.GenUUID != nil {
			if *node.GenUUID == uuid {
				return true
			}
		}
	}

	return false
}

func sliceContainsString(slice []string, value string) bool {
	for _, s := range slice {
		if value == s {
			return true
		}
	}

	return false
}

func GetPodList(namespace string, selector map[string]string, sdkClient client.Client) (*v1.PodList, error) {
	list := &v1.PodList{}

	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(selector),
	}

	err := sdkClient.List(
		context.TODO(),
		list,
		listOpts...,
	)

	return list, err
}

func GetDeploymentList(namespace string, selector map[string]string, sdkClient client.Client) (*appsv1.DeploymentList, error) {
	list := &appsv1.DeploymentList{}

	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(selector),
	}

	err := sdkClient.List(context.TODO(), list, listOpts...)

	return list, err
}

func GetStatefulSetList(namespace string, selector map[string]string, sdkClient client.Client) (*appsv1.StatefulSetList, error) {
	list := &appsv1.StatefulSetList{}

	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(selector),
	}

	err := sdkClient.List(context.TODO(), list, listOpts...)

	return list, err
}

func GetPVCList(namespace string, selector map[string]string, sdkClient client.Client) (*v1.PersistentVolumeClaimList, error) {
	list := &v1.PersistentVolumeClaimList{}

	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(selector),
	}

	err := sdkClient.List(context.TODO(), list, listOpts...)

	return list, err
}
