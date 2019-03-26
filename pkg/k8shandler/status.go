package k8shandler

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/util/retry"

	v1alpha1 "github.com/openshift/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const healthUnknown = "cluster health unknown"
const NOT_FOUND_INDEX = -1

var DISK_WATERMARK_LOW_PCT *float64
var DISK_WATERMARK_HIGH_PCT *float64
var DISK_WATERMARK_LOW_ABS *resource.Quantity
var DISK_WATERMARK_HIGH_ABS *resource.Quantity

func UpdateClusterStatus(cluster *v1alpha1.Elasticsearch) error {

	clusterStatus := cluster.Status.DeepCopy()

	health, err := GetClusterHealth(cluster.Name, cluster.Namespace)
	if err != nil {
		health = healthUnknown
	}
	clusterStatus.ClusterHealth = health

	allocation, err := GetShardAllocation(cluster.Name, cluster.Namespace)
	switch {
	case allocation == "none":
		clusterStatus.ShardAllocationEnabled = v1alpha1.ShardAllocationNone
	case err != nil:
		clusterStatus.ShardAllocationEnabled = v1alpha1.ShardAllocationUnknown
	default:
		clusterStatus.ShardAllocationEnabled = v1alpha1.ShardAllocationAll
	}

	clusterStatus.Pods = rolePodStateMap(cluster.Namespace, cluster.Name)
	updateStatusConditions(clusterStatus)
	updateNodeConditions(cluster.Name, cluster.Namespace, clusterStatus)

	if !reflect.DeepEqual(clusterStatus, cluster.Status) {
		nretries := -1
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			nretries++
			if getErr := sdk.Get(cluster); getErr != nil {
				logrus.Debugf("Could not get Elasticsearch %v: %v", cluster.Name, getErr)
				return getErr
			}

			cluster.Status.ClusterHealth = clusterStatus.ClusterHealth
			cluster.Status.Conditions = clusterStatus.Conditions
			cluster.Status.Pods = clusterStatus.Pods
			cluster.Status.ShardAllocationEnabled = clusterStatus.ShardAllocationEnabled
			cluster.Status.Nodes = clusterStatus.Nodes

			if updateErr := sdk.Update(cluster); updateErr != nil {
				logrus.Debugf("Failed to update Elasticsearch %s status. Reason: %v. Trying again...", cluster.Name, updateErr)
				return updateErr
			}
			return nil
		})

		if retryErr != nil {
			return fmt.Errorf("Error: could not update status for Elasticsearch %v after %v retries: %v", cluster.Name, nretries, retryErr)
		}
		logrus.Debugf("Updated Elasticsearch %v after %v retries", cluster.Name, nretries)
	}

	return nil
}

// if a status doesn't exist, provide a new one
func getNodeStatus(name string, status *v1alpha1.ElasticsearchStatus) (int, *v1alpha1.ElasticsearchNodeStatus) {
	for index, status := range status.Nodes {
		if status.DeploymentName == name || status.StatefulSetName == name {
			return index, &status
		}
	}

	return NOT_FOUND_INDEX, &v1alpha1.ElasticsearchNodeStatus{}
}

func rolePodStateMap(namespace string, clusterName string) map[v1alpha1.ElasticsearchNodeRole]v1alpha1.PodStateMap {

	baseSelector := fmt.Sprintf("component=%s", clusterName)
	clientList, _ := GetPodList(namespace, fmt.Sprintf("%s,%s", baseSelector, "es-node-client=true"))
	dataList, _ := GetPodList(namespace, fmt.Sprintf("%s,%s", baseSelector, "es-node-data=true"))
	masterList, _ := GetPodList(namespace, fmt.Sprintf("%s,%s", baseSelector, "es-node-master=true"))

	return map[v1alpha1.ElasticsearchNodeRole]v1alpha1.PodStateMap{
		v1alpha1.ElasticsearchRoleClient: podStateMap(clientList.Items),
		v1alpha1.ElasticsearchRoleData:   podStateMap(dataList.Items),
		v1alpha1.ElasticsearchRoleMaster: podStateMap(masterList.Items),
	}
}

func podStateMap(podList []v1.Pod) v1alpha1.PodStateMap {
	stateMap := map[v1alpha1.PodStateType][]string{
		v1alpha1.PodStateTypeReady:    []string{},
		v1alpha1.PodStateTypeNotReady: []string{},
		v1alpha1.PodStateTypeFailed:   []string{},
	}

	for _, pod := range podList {
		switch pod.Status.Phase {
		case v1.PodPending:
			stateMap[v1alpha1.PodStateTypeNotReady] = append(stateMap[v1alpha1.PodStateTypeNotReady], pod.Name)
		case v1.PodRunning:
			if isPodReady(pod) {
				stateMap[v1alpha1.PodStateTypeReady] = append(stateMap[v1alpha1.PodStateTypeReady], pod.Name)
			} else {
				stateMap[v1alpha1.PodStateTypeNotReady] = append(stateMap[v1alpha1.PodStateTypeNotReady], pod.Name)
			}
		case v1.PodFailed:
			stateMap[v1alpha1.PodStateTypeFailed] = append(stateMap[v1alpha1.PodStateTypeFailed], pod.Name)
		}
	}

	return stateMap
}

func isPodReady(pod v1.Pod) bool {

	for _, container := range pod.Status.ContainerStatuses {
		if !container.Ready {
			return false
		}
	}

	return true
}

func updateNodeConditions(clusterName, namespace string, status *v1alpha1.ElasticsearchStatus) {
	// Get all pods based on status.Nodes[] and check their conditions
	// get pod with label 'node-name=node.getName()'
	baseSelector := fmt.Sprintf("component=%s", clusterName)

	thresholdEnabled, err := GetThresholdEnabled(clusterName, namespace)
	if err != nil {
		logrus.Debugf("Unable to check if threshold is enabled for %v", clusterName)
	}

	if thresholdEnabled {
		// refresh value of thresholds in case they changed...
		refreshDiskWatermarkThresholds(clusterName, namespace)
	}

	for nodeIndex, _ := range status.Nodes {
		node := &status.Nodes[nodeIndex]

		nodeName := "unknown name"
		if node.DeploymentName != "" {
			nodeName = node.DeploymentName
		} else {
			if node.StatefulSetName != "" {
				nodeName = node.StatefulSetName
			}
		}
		nodeNameSelector := fmt.Sprintf("node-name=%v", nodeName)

		nodePodList, _ := GetPodList(namespace, fmt.Sprintf("%s,%s", baseSelector, nodeNameSelector))
		for _, nodePod := range nodePodList.Items {

			isUnschedulable := false
			for _, podCondition := range nodePod.Status.Conditions {
				if podCondition.Type == v1.PodReasonUnschedulable {
					updatePodUnschedulableCondition(node, podCondition)
					isUnschedulable = true
				}
			}

			if isUnschedulable {
				return
			}

			updatePodUnschedulableCondition(node, v1.PodCondition{
				Status: v1.ConditionFalse,
			})

			// if the pod can't be scheduled we shouldn't enter here
			for _, containerStatus := range nodePod.Status.ContainerStatuses {
				if containerStatus.Name == "elasticsearch" {
					if containerStatus.State.Waiting != nil {
						updatePodNotReadyCondition(
							node,
							v1alpha1.ESContainerWaiting,
							containerStatus.State.Waiting.Reason,
							containerStatus.State.Waiting.Message,
						)
					} else {
						updatePodNotReadyCondition(
							node,
							v1alpha1.ESContainerWaiting,
							"",
							"",
						)
					}
					if containerStatus.State.Terminated != nil {
						updatePodNotReadyCondition(
							node,
							v1alpha1.ESContainerTerminated,
							containerStatus.State.Terminated.Reason,
							containerStatus.State.Terminated.Message,
						)
					} else {
						updatePodNotReadyCondition(
							node,
							v1alpha1.ESContainerTerminated,
							"",
							"",
						)
					}
				}
				if containerStatus.Name == "proxy" {
					if containerStatus.State.Waiting != nil {
						updatePodNotReadyCondition(
							node,
							v1alpha1.ProxyContainerWaiting,
							containerStatus.State.Waiting.Reason,
							containerStatus.State.Waiting.Message,
						)
					} else {
						updatePodNotReadyCondition(
							node,
							v1alpha1.ProxyContainerWaiting,
							"",
							"",
						)
					}
					if containerStatus.State.Terminated != nil {
						updatePodNotReadyCondition(
							node,
							v1alpha1.ProxyContainerTerminated,
							containerStatus.State.Terminated.Reason,
							containerStatus.State.Terminated.Message,
						)
					} else {
						updatePodNotReadyCondition(
							node,
							v1alpha1.ProxyContainerTerminated,
							"",
							"",
						)
					}
				}
			}

			if !thresholdEnabled {
				// disk threshold is not enabled, just return
				return
			}

			usage, percent, err := GetNodeDiskUsage(clusterName, namespace, nodeName)
			if err != nil {
				logrus.Debugf("Unable to get disk usage for %v", nodeName)
				return
			}

			if exceedsLowWatermark(usage, percent) {
				if exceedsHighWatermark(usage, percent) {
					updatePodNodeStorageCondition(
						node,
						"Disk Watermark High",
						fmt.Sprintf("Disk storage usage for node is %vb (%v%%). Shards will be relocated from this node.", usage, percent),
					)
				} else {
					updatePodNodeStorageCondition(
						node,
						"Disk Watermark Low",
						fmt.Sprintf("Disk storage usage for node is %vb (%v%%). Shards will be not be allocated on this node.", usage, percent),
					)
				}
			} else {
				if percent > float64(0.0) {
					// if we were able to pull the usage but it isn't above the thresholds -- clear the status message
					updatePodNodeStorageCondition(node, "", "")
				}
			}

		}
	}
}

func refreshDiskWatermarkThresholds(clusterName, namespace string) {
	//quantity, err := resource.ParseQuantity(string)
	low, high, err := GetDiskWatermarks(clusterName, namespace)
	if err != nil {
		logrus.Debugf("Unable to refresh disk watermarks from cluster, using defaults")
	}

	switch low.(type) {
	case float64:
		value := low.(float64)
		DISK_WATERMARK_LOW_PCT = &value
		DISK_WATERMARK_LOW_ABS = nil
	case string:
		value, err := resource.ParseQuantity(strings.ToUpper(low.(string)))
		if err != nil {
			logrus.Warnf("Unable to parse %v: %v", low.(string), err)
		}
		DISK_WATERMARK_LOW_ABS = &value
		DISK_WATERMARK_LOW_PCT = nil
	default:
		// error
		logrus.Warnf("Unknown type for low: %T", low)
	}

	switch high.(type) {
	case float64:
		value := high.(float64)
		DISK_WATERMARK_HIGH_PCT = &value
		DISK_WATERMARK_HIGH_ABS = nil
	case string:
		value, err := resource.ParseQuantity(strings.ToUpper(high.(string)))
		if err != nil {
			logrus.Warnf("Unable to parse %v: %v", high.(string), err)
		}
		DISK_WATERMARK_HIGH_ABS = &value
		DISK_WATERMARK_HIGH_PCT = nil
	default:
		// error
		logrus.Warnf("Unknown type for high: %T", high)
	}

}

func exceedsLowWatermark(usage string, percent float64) bool {

	return exceedsWatermarks(usage, percent, DISK_WATERMARK_LOW_ABS, DISK_WATERMARK_LOW_PCT)
}

func exceedsHighWatermark(usage string, percent float64) bool {

	return exceedsWatermarks(usage, percent, DISK_WATERMARK_HIGH_ABS, DISK_WATERMARK_HIGH_PCT)
}

func exceedsWatermarks(usage string, percent float64, watermarkUsage *resource.Quantity, watermarkPercent *float64) bool {

	if usage == "" || percent < float64(0) {
		return false
	}

	quantity, err := resource.ParseQuantity(usage)
	if err != nil {
		logrus.Warnf("Unable to parse usage quantity %v: %v", usage, err)
		return false
	}

	// if quantity is > watermarkUsage and is used
	if watermarkUsage != nil && quantity.Cmp(*watermarkUsage) == 1 {
		return true
	}

	if watermarkPercent != nil && percent > *watermarkPercent {
		return true
	}

	return false
}

func updatePodCondition(node *v1alpha1.ElasticsearchNodeStatus, condition *v1alpha1.ClusterCondition) bool {
	if node.Conditions == nil {
		node.Conditions = make([]v1alpha1.ClusterCondition, 0, 4)
	}

	// Try to find this node condition.
	conditionIndex, oldCondition := getPodCondition(node, condition.Type)

	if condition.Status == v1.ConditionFalse {
		if oldCondition != nil {
			node.Conditions = append(node.Conditions[:conditionIndex], node.Conditions[conditionIndex+1:]...)
			return true
		}

		return false
	}

	if oldCondition == nil {
		// We are adding new node condition.
		node.Conditions = append(node.Conditions, *condition)
		return true
	}

	isEqual := condition.Status == oldCondition.Status &&
		condition.Reason == oldCondition.Reason &&
		condition.Message == oldCondition.Message

	node.Conditions[conditionIndex] = *condition
	return !isEqual
}

func getPodCondition(node *v1alpha1.ElasticsearchNodeStatus, conditionType v1alpha1.ClusterConditionType) (int, *v1alpha1.ClusterCondition) {
	if node == nil {
		return -1, nil
	}
	for i := range node.Conditions {
		if node.Conditions[i].Type == conditionType {
			return i, &node.Conditions[i]
		}
	}
	return -1, nil
}

func updatePodUnschedulableCondition(node *v1alpha1.ElasticsearchNodeStatus, podCondition v1.PodCondition) bool {
	return updatePodCondition(node, &v1alpha1.ClusterCondition{
		Type:               v1alpha1.Unschedulable,
		Status:             podCondition.Status,
		Reason:             podCondition.Reason,
		Message:            podCondition.Message,
		LastTransitionTime: podCondition.LastTransitionTime,
	})
}

func updatePodNotReadyCondition(node *v1alpha1.ElasticsearchNodeStatus, conditionType v1alpha1.ClusterConditionType, reason, message string) bool {

	var status v1.ConditionStatus
	if message == "" && reason == "" {
		status = v1.ConditionFalse
	} else {
		status = v1.ConditionTrue
	}

	return updatePodCondition(node, &v1alpha1.ClusterCondition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
}

func updatePodNodeStorageCondition(node *v1alpha1.ElasticsearchNodeStatus, reason, message string) bool {

	var status v1.ConditionStatus
	if message == "" && reason == "" {
		status = v1.ConditionFalse
	} else {
		status = v1.ConditionTrue
	}

	return updatePodCondition(node, &v1alpha1.ClusterCondition{
		Type:               v1alpha1.NodeStorage,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
}

func updateStatusConditions(status *v1alpha1.ElasticsearchStatus) {
	if status.Conditions == nil {
		status.Conditions = make([]v1alpha1.ClusterCondition, 0, 4)
	}
	if _, condition := getESNodeCondition(status, v1alpha1.UpdatingSettings); condition == nil {
		updateUpdatingSettingsCondition(status, v1.ConditionFalse)
	}
	if _, condition := getESNodeCondition(status, v1alpha1.ScalingUp); condition == nil {
		updateScalingUpCondition(status, v1.ConditionFalse)
	}
	if _, condition := getESNodeCondition(status, v1alpha1.ScalingDown); condition == nil {
		updateScalingDownCondition(status, v1.ConditionFalse)
	}
	if _, condition := getESNodeCondition(status, v1alpha1.Restarting); condition == nil {
		updateRestartingCondition(status, v1.ConditionFalse)
	}
}

func getESNodeCondition(status *v1alpha1.ElasticsearchStatus, conditionType v1alpha1.ClusterConditionType) (int, *v1alpha1.ClusterCondition) {
	if status == nil {
		return -1, nil
	}
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return i, &status.Conditions[i]
		}
	}
	return -1, nil
}

func updateESNodeCondition(status *v1alpha1.ElasticsearchStatus, condition *v1alpha1.ClusterCondition) bool {
	condition.LastTransitionTime = metav1.Now()
	// Try to find this node condition.
	conditionIndex, oldCondition := getESNodeCondition(status, condition.Type)

	if condition.Status == v1.ConditionFalse {
		if oldCondition != nil {
			status.Conditions = append(status.Conditions[:conditionIndex], status.Conditions[conditionIndex+1:]...)
			return true
		}

		return false
	}

	if oldCondition == nil {
		// We are adding new node condition.
		status.Conditions = append(status.Conditions, *condition)
		return true
	}
	// We are updating an existing condition, so we need to check if it has changed.
	if condition.Status == oldCondition.Status {
		condition.LastTransitionTime = oldCondition.LastTransitionTime
	}

	isEqual := condition.Status == oldCondition.Status &&
		condition.Reason == oldCondition.Reason &&
		condition.Message == oldCondition.Message &&
		condition.LastTransitionTime.Equal(&oldCondition.LastTransitionTime)

	status.Conditions[conditionIndex] = *condition
	// Return true if one of the fields have changed.
	return !isEqual
}

func updateConditionWithRetry(dpl *v1alpha1.Elasticsearch, value v1.ConditionStatus,
	executeUpdateCondition func(*v1alpha1.ElasticsearchStatus, v1.ConditionStatus) bool) error {
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if getErr := sdk.Get(dpl); getErr != nil {
			logrus.Debugf("Could not get Elasticsearch %v: %v", dpl.Name, getErr)
			return getErr
		}

		executeUpdateCondition(&dpl.Status, value)

		if updateErr := sdk.Update(dpl); updateErr != nil {
			logrus.Debugf("Failed to update Elasticsearch %v status: %v", dpl.Name, updateErr)
			return updateErr
		}
		return nil
	})
	return retryErr
}

func updateInvalidMasterCountCondition(status *v1alpha1.ElasticsearchStatus, value v1.ConditionStatus) bool {
	var message string
	var reason string
	if value == v1.ConditionTrue {
		message = fmt.Sprintf("Invalid master nodes count. Please ensure there are no more than %v total nodes with master roles", maxMasterCount)
		reason = "Invalid Settings"
	} else {
		message = ""
		reason = ""
	}
	return updateESNodeCondition(status, &v1alpha1.ClusterCondition{
		Type:    v1alpha1.InvalidMasters,
		Status:  value,
		Reason:  reason,
		Message: message,
	})
}

func updateInvalidDataCountCondition(status *v1alpha1.ElasticsearchStatus, value v1.ConditionStatus) bool {
	var message string
	var reason string
	if value == v1.ConditionTrue {
		message = "No data nodes requested. Please ensure there is at least 1 node with data roles"
		reason = "Invalid Settings"
	} else {
		message = ""
		reason = ""
	}
	return updateESNodeCondition(status, &v1alpha1.ClusterCondition{
		Type:    v1alpha1.InvalidData,
		Status:  value,
		Reason:  reason,
		Message: message,
	})
}

func updateInvalidReplicationCondition(status *v1alpha1.ElasticsearchStatus, value v1.ConditionStatus) bool {
	var message string
	var reason string
	if value == v1.ConditionTrue {
		message = "Wrong RedundancyPolicy selected. Choose different RedundancyPolicy or add more nodes with data roles"
		reason = "Invalid Settings"
	} else {
		message = ""
		reason = ""
	}
	return updateESNodeCondition(status, &v1alpha1.ClusterCondition{
		Type:    v1alpha1.InvalidRedundancy,
		Status:  value,
		Reason:  reason,
		Message: message,
	})
}

func updateUpdatingSettingsCondition(status *v1alpha1.ElasticsearchStatus, value v1.ConditionStatus) bool {
	var message string
	if value == v1.ConditionTrue {
		message = "Config Map is different"
	} else {
		message = "Config Map is up to date"
	}
	return updateESNodeCondition(status, &v1alpha1.ClusterCondition{
		Type:    v1alpha1.UpdatingSettings,
		Status:  value,
		Reason:  "ConfigChange",
		Message: message,
	})
}

func updateScalingUpCondition(status *v1alpha1.ElasticsearchStatus, value v1.ConditionStatus) bool {
	return updateESNodeCondition(status, &v1alpha1.ClusterCondition{
		Type:   v1alpha1.ScalingUp,
		Status: value,
	})
}

func updateScalingDownCondition(status *v1alpha1.ElasticsearchStatus, value v1.ConditionStatus) bool {
	return updateESNodeCondition(status, &v1alpha1.ClusterCondition{
		Type:   v1alpha1.ScalingDown,
		Status: value,
	})
}

func updateRestartingCondition(status *v1alpha1.ElasticsearchStatus, value v1.ConditionStatus) bool {
	return updateESNodeCondition(status, &v1alpha1.ClusterCondition{
		Type:   v1alpha1.Restarting,
		Status: value,
	})
}
