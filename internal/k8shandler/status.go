package k8shandler

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	healthUnknown = "cluster health unknown"
	NotFoundIndex = -1
)

var (
	DiskWatermarkLowPct   *float64
	DiskWatermarkHighPct  *float64
	DiskWatermarkFloodPct *float64
	DiskWatermarkLowAbs   *resource.Quantity
	DiskWatermarkHighAbs  *resource.Quantity
	DiskWatermarkFloodAbs *resource.Quantity
)

func (er *ElasticsearchRequest) UpdateClusterStatus() error {
	cluster := er.cluster
	esClient := er.esClient

	clusterStatus := cluster.Status.DeepCopy()

	health := api.ClusterHealth{
		Status: healthUnknown,
	}

	// if the cluster isn't ready don't both to try to curl it
	if er.AnyNodeReady() {
		health, _ = esClient.GetClusterHealth()
	}

	clusterStatus.Cluster = health
	clusterStatus.ShardAllocationEnabled = api.ShardAllocationUnknown

	// if the cluster isn't ready don't both to try to curl it
	if er.AnyNodeReady() {
		allocation, _ := esClient.GetShardAllocation()
		switch {
		case allocation == "none":
			clusterStatus.ShardAllocationEnabled = api.ShardAllocationNone
		case allocation == "primaries":
			clusterStatus.ShardAllocationEnabled = api.ShardAllocationPrimaries
		case allocation == "all":
			clusterStatus.ShardAllocationEnabled = api.ShardAllocationAll
		default:
			clusterStatus.ShardAllocationEnabled = api.ShardAllocationUnknown
		}
	}

	clusterStatus.Pods = rolePodStateMap(cluster.Namespace, cluster.Name, er.client)
	updateStatusConditions(clusterStatus)
	if err := er.updateNodeConditions(clusterStatus); err != nil {
		return err
	}

	if err := er.updateStorageConditions(clusterStatus); err != nil {
		return err
	}

	if !reflect.DeepEqual(clusterStatus, cluster.Status) {
		nretries := -1
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			nretries++
			if err := er.client.Get(context.TODO(), types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster); err != nil {
				return err
			}

			cluster.Status.Cluster = clusterStatus.Cluster
			cluster.Status.Conditions = clusterStatus.Conditions
			cluster.Status.Pods = clusterStatus.Pods
			cluster.Status.ShardAllocationEnabled = clusterStatus.ShardAllocationEnabled
			cluster.Status.Nodes = clusterStatus.Nodes

			if err := er.client.Status().Update(context.TODO(), cluster); err != nil {
				return err
			}
			return nil
		})

		if retryErr != nil {
			return kverrors.Wrap(retryErr, "failed to update status for cluster",
				"cluster", cluster.Name,
				"retries", nretries)
		}
	}

	return nil
}

func (er *ElasticsearchRequest) GetCurrentPodStateMap() map[api.ElasticsearchNodeRole]api.PodStateMap {
	return rolePodStateMap(er.cluster.Namespace, er.cluster.Name, er.client)
}

func (er *ElasticsearchRequest) setNodeStatus(node NodeTypeInterface, nodeStatus *api.ElasticsearchNodeStatus, clusterStatus *api.ElasticsearchStatus) error {
	index, _ := getNodeStatus(node.name(), clusterStatus)

	if index == NotFoundIndex {
		clusterStatus.Nodes = append(clusterStatus.Nodes, *nodeStatus)
	} else {
		clusterStatus.Nodes[index] = *nodeStatus
	}

	return er.updateNodeStatus(*clusterStatus)
}

func (er *ElasticsearchRequest) updateNodeStatus(status api.ElasticsearchStatus) error {
	cluster := er.cluster
	// if there is nothing to update, don't
	if reflect.DeepEqual(cluster.Status, status) {
		return nil
	}

	nretries := -1
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		nretries++
		if err := er.client.Get(context.TODO(), types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster); err != nil {
			return err
		}

		cluster.Status = status

		if err := er.client.Status().Update(context.TODO(), cluster); err != nil {
			return err
		}

		return nil
	})

	if retryErr != nil {
		return kverrors.Wrap(retryErr, "failed to update status for cluster",
			"cluster", cluster.Name,
			"retries", nretries)
	}

	return nil
}

func containsClusterCondition(condition api.ClusterConditionType, status v1.ConditionStatus, elasticsearchStatus *api.ElasticsearchStatus) bool {
	// if we're looking for a status of v1.ConditionTrue then we want to see if the
	// condition is present and the status is the same
	//
	// if we're looking for a status of v1.ConditionFalse then we want the condition
	// to either be present with status of false or to not find the condition
	defaultValue := status != v1.ConditionTrue

	for _, clusterCondition := range elasticsearchStatus.Conditions {
		if clusterCondition.Type == condition {
			return clusterCondition.Status == status
		}
	}

	return defaultValue
}

// if a status doesn't exist, provide a new one
func getNodeStatus(name string, status *api.ElasticsearchStatus) (int, *api.ElasticsearchNodeStatus) {
	for index, status := range status.Nodes {
		if status.DeploymentName == name || status.StatefulSetName == name {
			return index, &status
		}
	}

	return NotFoundIndex, &api.ElasticsearchNodeStatus{}
}

func rolePodStateMap(namespace, clusterName string, client client.Client) map[api.ElasticsearchNodeRole]api.PodStateMap {
	clientList, _ := GetPodList(
		namespace,
		map[string]string{
			"component":      "elasticsearch",
			"cluster-name":   clusterName,
			"es-node-client": "true",
		},
		client,
	)
	dataList, _ := GetPodList(
		namespace,
		map[string]string{
			"component":    "elasticsearch",
			"cluster-name": clusterName,
			"es-node-data": "true",
		},
		client,
	)
	masterList, _ := GetPodList(
		namespace,
		map[string]string{
			"component":      "elasticsearch",
			"cluster-name":   clusterName,
			"es-node-master": "true",
		},
		client,
	)

	return map[api.ElasticsearchNodeRole]api.PodStateMap{
		api.ElasticsearchRoleClient: podStateMap(clientList.Items),
		api.ElasticsearchRoleData:   podStateMap(dataList.Items),
		api.ElasticsearchRoleMaster: podStateMap(masterList.Items),
	}
}

func podStateMap(podList []v1.Pod) api.PodStateMap {
	stateMap := map[api.PodStateType][]string{
		api.PodStateTypeReady:    {},
		api.PodStateTypeNotReady: {},
		api.PodStateTypeFailed:   {},
	}

	for _, pod := range podList {
		switch pod.Status.Phase {
		case v1.PodPending:
			stateMap[api.PodStateTypeNotReady] = append(stateMap[api.PodStateTypeNotReady], pod.Name)
		case v1.PodRunning:
			if isPodReady(pod) {
				stateMap[api.PodStateTypeReady] = append(stateMap[api.PodStateTypeReady], pod.Name)
			} else {
				stateMap[api.PodStateTypeNotReady] = append(stateMap[api.PodStateTypeNotReady], pod.Name)
			}
		case v1.PodFailed:
			stateMap[api.PodStateTypeFailed] = append(stateMap[api.PodStateTypeFailed], pod.Name)
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

func (er *ElasticsearchRequest) updateNodeConditions(status *api.ElasticsearchStatus) error {
	esClient := er.esClient

	// Get all pods based on status.Nodes[] and check their conditions
	// get pod with label 'node-name=node.getName()'
	thresholdEnabled := false
	if er.AnyNodeReady() {
		var err error

		thresholdEnabled, err = esClient.GetThresholdEnabled()
		if err != nil {
			er.L().Info("Unable to check if threshold is enabled", "error", err)
		}
	}

	if thresholdEnabled {
		// refresh value of thresholds in case they changed...
		er.refreshDiskWatermarkThresholds()
	}

	if err := er.pruneMissingNodes(status); err != nil {
		return err
	}

	if err := er.updatePodNodeConditions(status, thresholdEnabled); err != nil {
		return err
	}

	return nil
}

func (er *ElasticsearchRequest) updateStorageConditions(status *api.ElasticsearchStatus) error {
	ll := er.L()

	emptySpecVol := api.ElasticsearchStorageSpec{}
	structureStatus, nameStatus, sizeStatus := v1.ConditionFalse, v1.ConditionFalse, v1.ConditionFalse

	nodeNames := []string{}
	clusterNodes := nodes[nodeMapKey(er.cluster.GetName(), er.cluster.GetNamespace())]

	for _, node := range clusterNodes {
		nodeNames = append(nodeNames, node.name())
	}

	for _, node := range er.cluster.Spec.Nodes {
		nodeName := nodeNameContains(node.GenUUID, nodeNames)

		if nodeName == "" {
			ll.Info("Unable to find appropriate node to compare storage spec.")
			continue
		}

		specVol := node.Storage
		current := &v1.PersistentVolumeClaim{}
		claimName := fmt.Sprintf("%s-%s", er.cluster.Name, nodeName)

		isUsingPVCStorageSpec := true
		isEphemeralStorageSpec := reflect.DeepEqual(specVol, emptySpecVol) || specVol.Size == nil

		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := er.client.Get(context.TODO(), types.NamespacedName{Name: claimName, Namespace: er.cluster.Namespace}, current); err != nil {
				if !apierrors.IsNotFound(err) {
					return kverrors.Wrap(err, "failed to get PVC", "claim", claimName)
				}
				isUsingPVCStorageSpec = false
			}

			if isEphemeralStorageSpec && !isUsingPVCStorageSpec {
				return nil
			}

			if !isEphemeralStorageSpec != isUsingPVCStorageSpec {
				structureStatus = v1.ConditionTrue
				return nil
			}

			// Generally, this won't last very long with the exception of the case of
			// moving from ephermeral storage to a persistent one. Since the volume is
			// never created after the intial create, a PVC should never remain in the
			// the pending state for a long period of time with the exception of this case.
			isPVCPending := reflect.DeepEqual(current.Status.Phase, v1.ClaimPending)
			if isUsingPVCStorageSpec && isPVCPending {
				structureStatus = v1.ConditionTrue
				return nil
			}

			isDefaultName := specVol.StorageClassName == nil && current.Spec.StorageClassName != nil
			if !isDefaultName && !reflect.DeepEqual(current.Spec.StorageClassName, specVol.StorageClassName) {
				nameStatus = v1.ConditionTrue
			}

			currentSize := current.Spec.Resources.Requests.Storage()
			if currentSize != nil && specVol.Size != nil {
				if !currentSize.Equal(*specVol.Size) {
					sizeStatus = v1.ConditionTrue
				}
			} else if currentSize != nil || specVol.Size != nil {
				sizeStatus = v1.ConditionTrue
			}

			return nil
		})

		if retryErr != nil {
			ll.Error(retryErr, "Unable to get PVC")
		}
	}

	updateESNodeCondition(status, &api.ClusterCondition{
		Type:               api.StorageStructure,
		Status:             structureStatus,
		LastTransitionTime: metav1.Now(),
		Reason:             "StorageStructureChangeIgnored",
		Message:            "Changing the storage structure for a custom resource is not supported",
	})

	updateESNodeCondition(status, &api.ClusterCondition{
		Type:               api.StorageClassName,
		Status:             nameStatus,
		LastTransitionTime: metav1.Now(),
		Reason:             "StorageClassNameChangeIgnored",
		Message:            "Changing the storage class name for a custom resource is not supported",
	})

	updateESNodeCondition(status, &api.ClusterCondition{
		Type:               api.StorageSize,
		Status:             sizeStatus,
		LastTransitionTime: metav1.Now(),
		Reason:             "StorageSizeChangeIgnored",
		Message:            "Resizing the storage for a custom resource is not supported",
	})

	return nil
}

func nodeNameContains(uuid *string, names []string) string {
	nodeName := ""

	if uuid == nil {
		return nodeName
	}

	for _, name := range names {
		if strings.Contains(name, *uuid) {
			nodeName = name
			break
		}
	}

	return nodeName
}

func (er *ElasticsearchRequest) pruneMissingNodes(status *api.ElasticsearchStatus) error {
	cluster := er.cluster
	ll := er.L()

	clusterNodes := nodes[nodeMapKey(er.cluster.GetName(), er.cluster.GetNamespace())]

	ns := status.Nodes[:0]
	for _, nodeStatus := range status.Nodes {
		ll = ll.WithValues("node", nodeStatus)

		nodeName := "unknown name"
		if nodeStatus.DeploymentName != "" {
			nodeName = nodeStatus.DeploymentName
		} else {
			if nodeStatus.StatefulSetName != "" {
				nodeName = nodeStatus.StatefulSetName
			}
		}

		matchingLabels := map[string]string{
			"component":    "elasticsearch",
			"cluster-name": cluster.GetName(),
			"node-name":    nodeName,
		}

		nodePodList, err := GetPodList(cluster.GetNamespace(), matchingLabels, er.client)
		if err != nil {
			return err
		}

		for _, node := range clusterNodes {
			if nodeName != node.name() {
				continue
			}

			// Filter all existing nodes in status.Nodes
			if !node.isMissing() {
				ns = append(ns, nodeStatus)
				break
			}

			// Filter all pod status for missing node
			if len(nodePodList.Items) == 0 {
				for nodeRole, podStateMap := range status.Pods {
					for podState, podNames := range podStateMap {
						for idx, podName := range podNames {
							if strings.HasPrefix(podName, nodeName) {
								// Prune pod state maps for non existing pods
								status.Pods[nodeRole][podState] = append(podNames[:idx], podNames[idx+1:]...)
							}
						}
					}
				}
			}
		}
	}

	// Set pruned node status to their zero value
	// to allow garbage collection.
	for i := len(ns); i < len(status.Nodes); i++ {
		status.Nodes[i] = api.ElasticsearchNodeStatus{} // or the zero value of T
	}
	status.Nodes = ns

	return nil
}

func (er *ElasticsearchRequest) updatePodNodeConditions(status *api.ElasticsearchStatus, thresholdEnabled bool) error {
	cluster := er.cluster
	ll := er.L()

	for nodeIndex, node := range status.Nodes {
		ll = ll.WithValues("node", node)

		nodeName := "unknown name"
		if node.DeploymentName != "" {
			nodeName = node.DeploymentName
		} else {
			if node.StatefulSetName != "" {
				nodeName = node.StatefulSetName
			}
		}

		nodeStatus := &status.Nodes[nodeIndex]

		matchingLabels := map[string]string{
			"component":    "elasticsearch",
			"cluster-name": cluster.GetName(),
			"node-name":    nodeName,
		}

		nodePodList, err := GetPodList(cluster.GetNamespace(), matchingLabels, er.client)
		if err != nil {
			return err
		}

		for _, nodePod := range nodePodList.Items {

			isUnschedulable := false
			for _, podCondition := range nodePod.Status.Conditions {
				if podCondition.Type == v1.PodScheduled && podCondition.Status == v1.ConditionFalse {
					podCondition.Type = v1.PodReasonUnschedulable
					podCondition.Status = v1.ConditionTrue
					updatePodUnschedulableCondition(nodeStatus, podCondition)
					isUnschedulable = true
				}
			}

			if isUnschedulable {
				continue
			}
			updatePodUnschedulableCondition(nodeStatus, v1.PodCondition{
				Status: v1.ConditionFalse,
			})

			// if the pod can't be scheduled we shouldn't enter here
			for _, containerStatus := range nodePod.Status.ContainerStatuses {
				if containerStatus.Name == "elasticsearch" {
					if containerStatus.State.Waiting != nil {
						updatePodNotReadyCondition(
							nodeStatus,
							api.ESContainerWaiting,
							containerStatus.State.Waiting.Reason,
							containerStatus.State.Waiting.Message,
						)
					} else {
						updatePodNotReadyCondition(
							nodeStatus,
							api.ESContainerWaiting,
							"",
							"",
						)
					}
					if containerStatus.State.Terminated != nil {
						updatePodNotReadyCondition(
							nodeStatus,
							api.ESContainerTerminated,
							containerStatus.State.Terminated.Reason,
							containerStatus.State.Terminated.Message,
						)
					} else {
						updatePodNotReadyCondition(
							nodeStatus,
							api.ESContainerTerminated,
							"",
							"",
						)
					}
				}
				if containerStatus.Name == "proxy" {
					if containerStatus.State.Waiting != nil {
						updatePodNotReadyCondition(
							nodeStatus,
							api.ProxyContainerWaiting,
							containerStatus.State.Waiting.Reason,
							containerStatus.State.Waiting.Message,
						)
					} else {
						updatePodNotReadyCondition(
							nodeStatus,
							api.ProxyContainerWaiting,
							"",
							"",
						)
					}
					if containerStatus.State.Terminated != nil {
						updatePodNotReadyCondition(
							nodeStatus,
							api.ProxyContainerTerminated,
							containerStatus.State.Terminated.Reason,
							containerStatus.State.Terminated.Message,
						)
					} else {
						updatePodNotReadyCondition(
							nodeStatus,
							api.ProxyContainerTerminated,
							"",
							"",
						)
					}
				}
			}

			if !thresholdEnabled {
				// disk threshold is not enabled, continue to next node
				continue
			}

			usage, percent, err := er.esClient.GetNodeDiskUsage(nodeName)
			if err != nil {
				ll.Info("Unable to get disk usage", "error", err)
				continue
			}

			if exceedsLowWatermark(usage, percent) {
				if exceedsHighWatermark(usage, percent) {
					if exceedsFloodWatermark(usage, percent) {
						updatePodNodeStorageCondition(
							nodeStatus,
							"Disk Watermark Flood",
							fmt.Sprintf("Disk storage usage for node is %vb (%v%%). Every index having a shard allocated on this node is enforced a read-only block.", usage, percent),
						)
					} else {
						updatePodNodeStorageCondition(
							nodeStatus,
							"Disk Watermark High",
							fmt.Sprintf("Disk storage usage for node is %vb (%v%%). Shards will be relocated away from this node.", usage, percent),
						)
					}
				} else {
					updatePodNodeStorageCondition(
						nodeStatus,
						"Disk Watermark Low",
						fmt.Sprintf("Disk storage usage for node is %vb (%v%%). Shards will be not be allocated on this node.", usage, percent),
					)
				}
			} else {
				if percent > float64(0.0) {
					// if we were able to pull the usage but it isn't above the thresholds -- clear the status message
					updatePodNodeStorageCondition(nodeStatus, "", "")
				}
			}
		}
	}

	return nil
}

func (er *ElasticsearchRequest) refreshDiskWatermarkThresholds() {
	// quantity, err := resource.ParseQuantity(string)
	low, high, flood, err := er.esClient.GetDiskWatermarks()
	if err != nil {
		er.L().Info("Unable to refresh disk watermarks from cluster, using defaults", "error", err)
	}

	switch low.(type) {
	case float64:
		value := low.(float64)
		DiskWatermarkLowPct = &value
		DiskWatermarkLowAbs = nil
	case string:
		value, err := resource.ParseQuantity(strings.ToUpper(low.(string)))
		if err != nil {
			er.L().Info("Unable to parse quantity", "value", low.(string), "error", err)
		}
		DiskWatermarkLowAbs = &value
		DiskWatermarkLowPct = nil
	default:
		er.L().Error(err, "Unknown type for low", "type", fmt.Sprintf("%T", low))
	}

	switch high.(type) {
	case float64:
		value := high.(float64)
		DiskWatermarkHighPct = &value
		DiskWatermarkHighAbs = nil
	case string:
		value, err := resource.ParseQuantity(strings.ToUpper(high.(string)))
		if err != nil {
			er.L().Info("Unable to parse quantity", "value", high.(string), "error", err)
		}
		DiskWatermarkHighAbs = &value
		DiskWatermarkHighPct = nil
	default:
		// error
		er.L().Error(err, "Unknown type for high", "type", fmt.Sprintf("%T", high))
	}

	switch flood.(type) {
	case float64:
		value := flood.(float64)
		DiskWatermarkFloodPct = &value
		DiskWatermarkFloodAbs = nil
	case string:
		value, err := resource.ParseQuantity(strings.ToUpper(flood.(string)))
		if err != nil {
			er.L().Info("Unable to parse quantity", "value", flood.(string), "error", err)
		}
		DiskWatermarkFloodAbs = &value
		DiskWatermarkFloodPct = nil
	default:
		// error
		er.L().Error(err, "Unknown type for flood", "type", fmt.Sprintf("%T", flood))
	}
}

func exceedsLowWatermark(usage string, percent float64) bool {
	return exceedsWatermarks(usage, percent, DiskWatermarkLowAbs, DiskWatermarkLowPct)
}

func exceedsHighWatermark(usage string, percent float64) bool {
	return exceedsWatermarks(usage, percent, DiskWatermarkHighAbs, DiskWatermarkHighPct)
}

func exceedsFloodWatermark(usage string, percent float64) bool {
	return exceedsWatermarks(usage, percent, DiskWatermarkFloodAbs, DiskWatermarkFloodPct)
}

func exceedsWatermarks(usage string, percent float64, watermarkUsage *resource.Quantity, watermarkPercent *float64) bool {
	if usage == "" || percent < float64(0) {
		return false
	}

	quantity, err := resource.ParseQuantity(usage)
	if err != nil {
		log.Error(err, "Unable to parse quantity", "value", usage)
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

func updatePodCondition(node *api.ElasticsearchNodeStatus, condition *api.ClusterCondition) bool {
	if node.Conditions == nil {
		node.Conditions = make([]api.ClusterCondition, 0, 4)
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

func getPodCondition(node *api.ElasticsearchNodeStatus, conditionType api.ClusterConditionType) (int, *api.ClusterCondition) {
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

func updatePodUnschedulableCondition(node *api.ElasticsearchNodeStatus, podCondition v1.PodCondition) bool {
	return updatePodCondition(node, &api.ClusterCondition{
		Type:               api.Unschedulable,
		Status:             podCondition.Status,
		Reason:             podCondition.Reason,
		Message:            podCondition.Message,
		LastTransitionTime: podCondition.LastTransitionTime,
	})
}

func updatePodNotReadyCondition(node *api.ElasticsearchNodeStatus, conditionType api.ClusterConditionType, reason, message string) bool {
	var status v1.ConditionStatus
	if message == "" && reason == "" {
		status = v1.ConditionFalse
	} else {
		status = v1.ConditionTrue
	}

	return updatePodCondition(node, &api.ClusterCondition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
}

func updatePodNodeStorageCondition(node *api.ElasticsearchNodeStatus, reason, message string) bool {
	var status v1.ConditionStatus
	if message == "" && reason == "" {
		status = v1.ConditionFalse
	} else {
		status = v1.ConditionTrue
	}

	return updatePodCondition(node, &api.ClusterCondition{
		Type:               api.NodeStorage,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
}

func updateStatusConditions(status *api.ElasticsearchStatus) {
	if status.Conditions == nil {
		status.Conditions = make([]api.ClusterCondition, 0, 6)
	}
	if _, condition := getESNodeCondition(status.Conditions, api.UpdatingSettings); condition == nil {
		updateUpdatingSettingsCondition(status, v1.ConditionFalse)
	}
	if _, condition := getESNodeCondition(status.Conditions, api.ScalingUp); condition == nil {
		updateScalingUpCondition(status, v1.ConditionFalse)
	}
	if _, condition := getESNodeCondition(status.Conditions, api.ScalingDown); condition == nil {
		updateScalingDownCondition(status, v1.ConditionFalse)
	}
	if _, condition := getESNodeCondition(status.Conditions, api.Restarting); condition == nil {
		updateRestartingCondition(status, v1.ConditionFalse)
	}
	if _, condition := getESNodeCondition(status.Conditions, api.Recovering); condition == nil {
		updateRecoveringCondition(status, v1.ConditionFalse)
	}
	if _, condition := getESNodeCondition(status.Conditions, api.UpdatingESSettings); condition == nil {
		updateUpdatingESSettingsCondition(status, v1.ConditionFalse)
	}
}

func isPodUnschedulableConditionTrue(conditions []api.ClusterCondition) bool {
	_, condition := getESNodeCondition(conditions, api.Unschedulable)
	return condition != nil && condition.Status == v1.ConditionTrue
}

func isPodImagePullBackOff(conditions []api.ClusterCondition) bool {
	condition := getESNodeConditionWithReason(conditions, api.ESContainerWaiting, "ImagePullBackOff")
	return condition != nil && condition.Status == v1.ConditionTrue
}

func isPodCrashLoopBackOff(conditions []api.ClusterCondition) bool {
	condition := getESNodeConditionWithReason(conditions, api.ESContainerWaiting, "CrashLoopBackOff")
	return condition != nil && condition.Status == v1.ConditionTrue
}

func getESNodeCondition(conditions []api.ClusterCondition, conditionType api.ClusterConditionType) (int, *api.ClusterCondition) {
	if conditions == nil {
		return -1, nil
	}
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return i, &conditions[i]
		}
	}
	return -1, nil
}

func getESNodeConditionWithReason(conditions []api.ClusterCondition, conditionType api.ClusterConditionType, conditionReason string) *api.ClusterCondition {
	if conditions == nil {
		return nil
	}
	for i := range conditions {
		if conditions[i].Type == conditionType {
			if conditions[i].Reason == conditionReason {
				return &conditions[i]
			}
		}
	}
	return nil
}

func updateESNodeCondition(status *api.ElasticsearchStatus, condition *api.ClusterCondition) bool {
	condition.LastTransitionTime = metav1.Now()
	// Try to find this node condition.
	conditionIndex, oldCondition := getESNodeCondition(status.Conditions, condition.Type)

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

func updateConditionWithRetry(dpl *api.Elasticsearch, value v1.ConditionStatus,
	executeUpdateCondition func(*api.ElasticsearchStatus, v1.ConditionStatus) bool, client client.Client) error {
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := client.Get(context.TODO(), types.NamespacedName{Name: dpl.Name, Namespace: dpl.Namespace}, dpl); err != nil {
			log.Info("Could not get Elasticsearch", "cluster", dpl.Name, "error", err)
			return err
		}

		if changed := executeUpdateCondition(&dpl.Status, value); !changed {
			return nil
		}

		if err := client.Status().Update(context.TODO(), dpl); err != nil {
			log.Info("Failed to update Elasticsearch status", "cluster", dpl.Name, "error", err)
			return err
		}
		return nil
	})
	return kverrors.Wrap(retryErr, "failed to update elasticsearch status")
}

func updateInvalidMasterCountCondition(status *api.ElasticsearchStatus, value v1.ConditionStatus) bool {
	var message string
	var reason string
	if value == v1.ConditionTrue {
		message = fmt.Sprintf("Invalid master nodes count. Please ensure there are no more than %v total nodes with master roles", maxMasterCount)
		reason = "Invalid Settings"
	} else {
		message = ""
		reason = ""
	}
	return updateESNodeCondition(status, &api.ClusterCondition{
		Type:    api.InvalidMasters,
		Status:  value,
		Reason:  reason,
		Message: message,
	})
}

func updateInvalidDataCountCondition(status *api.ElasticsearchStatus, value v1.ConditionStatus) bool {
	var message string
	var reason string
	if value == v1.ConditionTrue {
		message = "No data nodes requested. Please ensure there is at least 1 node with data roles"
		reason = "Invalid Settings"
	} else {
		message = ""
		reason = ""
	}
	return updateESNodeCondition(status, &api.ClusterCondition{
		Type:    api.InvalidData,
		Status:  value,
		Reason:  reason,
		Message: message,
	})
}

func updateInvalidUUIDChangeCondition(cluster *api.Elasticsearch, value v1.ConditionStatus, message string, client client.Client) error {
	var reason string
	if value == v1.ConditionTrue {
		reason = "Invalid Spec"
	} else {
		reason = ""
	}

	return updateConditionWithRetry(
		cluster,
		value,
		func(status *api.ElasticsearchStatus, value v1.ConditionStatus) bool {
			return updateESNodeCondition(&cluster.Status, &api.ClusterCondition{
				Type:    api.InvalidUUID,
				Status:  value,
				Reason:  reason,
				Message: message,
			})
		},
		client,
	)
}

func updateInvalidReplicationCondition(status *api.ElasticsearchStatus, value v1.ConditionStatus) bool {
	var message string
	var reason string
	if value == v1.ConditionTrue {
		message = "Wrong RedundancyPolicy selected. Choose different RedundancyPolicy or add more nodes with data roles"
		reason = "Invalid Settings"
	} else {
		message = ""
		reason = ""
	}
	return updateESNodeCondition(status, &api.ClusterCondition{
		Type:    api.InvalidRedundancy,
		Status:  value,
		Reason:  reason,
		Message: message,
	})
}

func updateInvalidScaleDownCondition(status *api.ElasticsearchStatus, value v1.ConditionStatus) bool {
	var message string
	var reason string
	if value == v1.ConditionTrue {
		message = "Data node scale down rate is too high based on minimum number of replicas for all indices"
		reason = "Invalid Settings"
	}

	return updateESNodeCondition(status, &api.ClusterCondition{
		Type:    api.InvalidRedundancy,
		Status:  value,
		Reason:  reason,
		Message: message,
	})
}

func (er *ElasticsearchRequest) UpdateDegradedCondition(value bool, reason, message string) error {
	cluster := er.cluster

	statusValue := v1.ConditionFalse

	if value {
		statusValue = v1.ConditionTrue
	}

	return updateConditionWithRetry(
		cluster,
		statusValue,
		func(status *api.ElasticsearchStatus, statusValue v1.ConditionStatus) bool {
			return updateESNodeCondition(&cluster.Status, &api.ClusterCondition{
				Type:    api.DegradedState,
				Status:  statusValue,
				Reason:  reason,
				Message: message,
			})
		},
		er.client,
	)
}

func updateUpdatingSettingsCondition(status *api.ElasticsearchStatus, value v1.ConditionStatus) bool {
	return updateESNodeCondition(status, &api.ClusterCondition{
		Type:   api.UpdatingSettings,
		Status: value,
	})
}

func updateScalingUpCondition(status *api.ElasticsearchStatus, value v1.ConditionStatus) bool {
	return updateESNodeCondition(status, &api.ClusterCondition{
		Type:   api.ScalingUp,
		Status: value,
	})
}

func updateScalingDownCondition(status *api.ElasticsearchStatus, value v1.ConditionStatus) bool {
	return updateESNodeCondition(status, &api.ClusterCondition{
		Type:   api.ScalingDown,
		Status: value,
	})
}

func updateRestartingCondition(status *api.ElasticsearchStatus, value v1.ConditionStatus) bool {
	return updateESNodeCondition(status, &api.ClusterCondition{
		Type:   api.Restarting,
		Status: value,
	})
}

func updateRecoveringCondition(status *api.ElasticsearchStatus, value v1.ConditionStatus) bool {
	return updateESNodeCondition(status, &api.ClusterCondition{
		Type:   api.Recovering,
		Status: value,
	})
}

func updateUpdatingESSettingsCondition(status *api.ElasticsearchStatus, value v1.ConditionStatus) bool {
	return updateESNodeCondition(status, &api.ClusterCondition{
		Type:   api.UpdatingESSettings,
		Status: value,
	})
}
