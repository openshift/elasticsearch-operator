package k8shandler

import (
	"errors"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"
	api "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/internal/elasticsearch"
	"github.com/openshift/elasticsearch-operator/internal/utils"
	v1 "k8s.io/api/core/v1"
)

// ErrFlushShardsFailed indicates a failure when trying to flush shards
var ErrFlushShardsFailed = kverrors.New("flush shards failed")

type ClusterRestart struct {
	client           elasticsearch.Client
	clusterName      string
	clusterNamespace string
	scheduledNodes   []NodeTypeInterface
}

type Restarter struct {
	scheduledNodes   []NodeTypeInterface
	clusterName      string
	clusterNamespace string
	clusterStatus    *api.ElasticsearchStatus
	nodeStatus       *api.ElasticsearchNodeStatus

	precheck func() error
	prep     func() error
	main     func() error
	post     func() error
	recovery func() error

	precheckCondition func() bool
	prepCondition     func() bool
	mainCondition     func() bool
	postCondition     func() bool
	recoveryCondition func() bool

	precheckSignaler func()
	prepSignaler     func()
	mainSignaler     func()
	postSignaler     func()
	recoverySignaler func()
}

func (er *ElasticsearchRequest) PerformFullClusterUpdate(nodes []NodeTypeInterface) error {
	r := ClusterRestart{
		client:           er.esClient,
		clusterName:      er.cluster.Name,
		clusterNamespace: er.cluster.Namespace,
		scheduledNodes:   nodes,
	}

	restarter := Restarter{
		scheduledNodes:   nodes,
		clusterName:      er.cluster.Name,
		clusterNamespace: er.cluster.Namespace,
		precheck:         r.ensureClusterHealthValid,
		prep:             r.requiredSetPrimariesShardsAndFlush,
		main:             r.pushNodeUpdates,
		post:             r.waitAllNodesRejoinAndSetAllShards,
		recovery:         r.ensureClusterHealthValid,
	}

	updateStatus := func() {
		for _, node := range r.scheduledNodes {
			nodeStatus := er.getNodeState(node)
			nodeStatus.UpgradeStatus.ScheduledForCertRedeploy = v1.ConditionFalse

			if err := er.setNodeStatus(node, nodeStatus, &er.cluster.Status); err != nil {
				log.Error(err, "unable to update node status", "namespace", er.cluster.Namespace, "name", er.cluster.Name)
			}
		}
	}

	restarter.setClusterConditions(updateStatus)
	restarter.clusterStatus = &er.cluster.Status
	return restarter.restartCluster()
}

func (er *ElasticsearchRequest) PerformFullClusterCertRestart(nodes []NodeTypeInterface) error {
	r := ClusterRestart{
		client:           er.esClient,
		clusterName:      er.cluster.Name,
		clusterNamespace: er.cluster.Namespace,
		scheduledNodes:   nodes,
	}

	restarter := Restarter{
		scheduledNodes:   nodes,
		clusterName:      er.cluster.Name,
		clusterNamespace: er.cluster.Namespace,
		precheck:         r.restartNoop,
		prep:             r.restartNoop,
		main:             er.scaleDownThenUpFunc(r),
		post:             r.waitAllNodesRejoinAndSetAllShards,
		recovery:         r.ensureClusterHealthValid,
	}

	updateStatus := func() {
		for _, node := range r.scheduledNodes {
			nodeStatus := er.getNodeState(node)
			nodeStatus.UpgradeStatus.ScheduledForCertRedeploy = v1.ConditionFalse

			if err := er.setNodeStatus(node, nodeStatus, &er.cluster.Status); err != nil {
				log.Error(err, "unable to update node status", "namespace", er.cluster.Namespace, "name", er.cluster.Name)
			}
		}
	}

	restarter.setClusterConditions(updateStatus)
	restarter.clusterStatus = &er.cluster.Status
	return restarter.restartCluster()
}

func (er *ElasticsearchRequest) PerformFullClusterRestart(nodes []NodeTypeInterface) error {
	r := ClusterRestart{
		client:           er.esClient,
		clusterName:      er.cluster.Name,
		clusterNamespace: er.cluster.Namespace,
		scheduledNodes:   nodes,
	}

	restarter := Restarter{
		scheduledNodes:   nodes,
		clusterName:      er.cluster.Name,
		clusterNamespace: er.cluster.Namespace,
		precheck:         r.ensureClusterHealthValid,
		prep:             r.optionalSetPrimariesShardsAndFlush,
		main:             er.scaleDownThenUpFunc(r),
		post:             r.waitAllNodesRejoinAndSetAllShards,
		recovery:         r.ensureClusterHealthValid,
	}

	updateStatus := func() {
		for _, node := range r.scheduledNodes {
			nodeStatus := er.getNodeState(node)
			nodeStatus.UpgradeStatus.ScheduledForCertRedeploy = v1.ConditionFalse

			if err := er.setNodeStatus(node, nodeStatus, &er.cluster.Status); err != nil {
				log.Error(err, "unable to update node status", "namespace", er.cluster.Namespace, "name", er.cluster.Name)
			}
		}
	}

	restarter.setClusterConditions(updateStatus)
	restarter.clusterStatus = &er.cluster.Status
	return restarter.restartCluster()
}

func (er *ElasticsearchRequest) PerformNodeRestart(node NodeTypeInterface) error {
	scheduledNode := []NodeTypeInterface{node}

	r := ClusterRestart{
		client:           er.esClient,
		clusterName:      er.cluster.Name,
		clusterNamespace: er.cluster.Namespace,
		scheduledNodes:   scheduledNode,
	}

	restarter := Restarter{
		scheduledNodes:   scheduledNode,
		clusterName:      er.cluster.Name,
		clusterNamespace: er.cluster.Namespace,
		precheck:         r.ensureClusterHealthValid,
		prep:             r.optionalSetPrimariesShardsAndFlush,
		main:             r.scaleDownThenUpNodes,
		post:             r.waitAllNodesRejoinAndSetAllShards,
		recovery:         r.ensureClusterHealthValid,
	}

	updateStatus := func() {
		if err := er.setNodeStatus(node, restarter.nodeStatus, &er.cluster.Status); err != nil {
			log.Error(err, "unable to update node status", "namespace", er.cluster.Namespace, "name", er.cluster.Name)
		}
	}

	restarter.setNodeConditions(updateStatus)

	restarter.nodeStatus = er.getNodeState(node)
	return restarter.restartCluster()
}

func (er *ElasticsearchRequest) PerformNodeUpdate(node NodeTypeInterface) error {
	scheduledNode := []NodeTypeInterface{node}

	r := ClusterRestart{
		client:           er.esClient,
		clusterName:      er.cluster.Name,
		clusterNamespace: er.cluster.Namespace,
		scheduledNodes:   scheduledNode,
	}

	restarter := Restarter{
		scheduledNodes:   scheduledNode,
		clusterName:      er.cluster.Name,
		clusterNamespace: er.cluster.Namespace,
		precheck:         r.ensureClusterHealthValid,
		prep:             r.requiredSetPrimariesShardsAndFlush,
		main:             r.pushNodeUpdates,
		post:             r.waitAllNodesRejoinAndSetAllShards,
		recovery:         r.ensureClusterHealthValid,
	}

	updateStatus := func() {
		if err := er.setNodeStatus(node, restarter.nodeStatus, &er.cluster.Status); err != nil {
			log.Error(err, "unable to update node status", "namespace", er.cluster.Namespace, "name", er.cluster.Name)
		}
	}

	restarter.setNodeConditions(updateStatus)

	restarter.nodeStatus = er.getNodeState(node)
	return restarter.restartCluster()
}

func (er *ElasticsearchRequest) PerformRollingUpdate(nodes []NodeTypeInterface) error {
	for _, node := range nodes {
		if err := er.PerformNodeUpdate(node); err != nil {
			return err
		}
	}

	return nil
}

func (er *ElasticsearchRequest) PerformRollingRestart(nodes []NodeTypeInterface) error {
	for _, node := range nodes {
		if err := er.PerformNodeRestart(node); err != nil {
			return err
		}
	}

	return nil
}

// scaleDownThenUpFunc returns a func() error that uses the ElasticsearchRequest function AnyNodeReady
// to determine if the cluster has any nodes running. If we use the NodeInterface function waitForNodeLeaveCluster
// we may get stuck because we have no cluster nodes to query from.
func (er *ElasticsearchRequest) scaleDownThenUpFunc(clusterRestart ClusterRestart) func() error {
	return func() error {
		if err := clusterRestart.scaleDownNodes(); err != nil {
			return err
		}

		if er.AnyNodeReady() {
			return kverrors.New("waiting for all nodes to leave the cluster")
		}

		if err := clusterRestart.scaleUpNodes(); err != nil {
			return err
		}

		return nil
	}
}

// used for when we have no operations to perform during a restart phase
func (cr ClusterRestart) restartNoop() error {
	return nil
}

func (cr ClusterRestart) ensureClusterHealthValid() error {
	if status, _ := cr.client.GetClusterHealthStatus(); !utils.Contains(desiredClusterStates, status) {
		return kverrors.New("Waiting for cluster to be recovered",
			"namespace", cr.clusterNamespace,
			"cluster", cr.clusterName,
			"status", status,
			"desired_status", desiredClusterStates)
	}

	return nil
}

func (cr ClusterRestart) requiredSetPrimariesShardsAndFlush() error {
	// set shard allocation as primaries
	if ok, err := cr.client.SetShardAllocation(api.ShardAllocationPrimaries); !ok {
		return kverrors.Wrap(err, "unable to set shard allocation to primaries",
			"namespace", cr.clusterNamespace,
			"cluster", cr.clusterName)
	}

	// flush nodes
	if ok, err := cr.client.DoSynchronizedFlush(); !ok {
		log.Error(err, "failed to flush nodes",
			"namespace", cr.clusterNamespace,
			"cluster", cr.clusterName,
		)
		return ErrFlushShardsFailed
	}

	return nil
}

func (cr ClusterRestart) optionalSetPrimariesShardsAndFlush() error {
	err := cr.requiredSetPrimariesShardsAndFlush()
	if err != nil {
		log.Error(err, "failed to set primaries shards and flush")
	}

	return nil
}

func (cr ClusterRestart) waitAllNodesRejoinAndSetAllShards() error {
	// reenable shard allocation
	if err := cr.waitAllNodesRejoin(); err != nil {
		return err
	}

	if err := cr.setAllShards(); err != nil {
		return err
	}

	return nil
}

func (cr ClusterRestart) waitAllNodesRejoin() error {
	for _, node := range cr.scheduledNodes {
		if _, err := node.waitForNodeRejoinCluster(); err != nil {
			return err
		}
	}

	return nil
}

func (cr ClusterRestart) setAllShards() error {
	// reenable shard allocation
	if ok, err := cr.client.SetShardAllocation(api.ShardAllocationAll); !ok {
		return kverrors.Wrap(err, "failed to enable shard allocation")
	}

	return nil
}

func (cr ClusterRestart) scaleDownThenUpNodes() error {
	if err := cr.scaleDownNodes(); err != nil {
		return err
	}

	for _, node := range cr.scheduledNodes {
		if _, err := node.waitForNodeLeaveCluster(); err != nil {
			return err
		}
	}

	if err := cr.scaleUpNodes(); err != nil {
		return err
	}

	if err := cr.waitAllNodesRejoin(); err != nil {
		return err
	}

	return nil
}

func (cr ClusterRestart) scaleDownNodes() error {
	// scale down all nodes
	for _, node := range cr.scheduledNodes {
		if err := node.scaleDown(); err != nil {
			return err
		}
	}

	return nil
}

func (cr ClusterRestart) scaleUpNodes() error {
	// scale all nodes back up
	for _, node := range cr.scheduledNodes {
		if err := node.scaleUp(); err != nil {
			return err
		}

		node.refreshHashes()
	}

	return nil
}

func (cr ClusterRestart) pushNodeUpdates() error {
	for _, node := range cr.scheduledNodes {
		if err := node.progressNodeChanges(); err != nil {
			return err
		}
	}

	return nil
}

func (r *Restarter) setClusterConditions(updateStatus func()) {
	// cluster conditions
	r.precheckCondition = func() bool {
		return containsClusterCondition(api.Restarting, v1.ConditionFalse, r.clusterStatus) &&
			containsClusterCondition(api.UpdatingESSettings, v1.ConditionFalse, r.clusterStatus) &&
			containsClusterCondition(api.Recovering, v1.ConditionFalse, r.clusterStatus)
	}

	r.prepCondition = func() bool {
		return containsClusterCondition(api.Restarting, v1.ConditionFalse, r.clusterStatus) &&
			containsClusterCondition(api.UpdatingESSettings, v1.ConditionTrue, r.clusterStatus) &&
			containsClusterCondition(api.Recovering, v1.ConditionFalse, r.clusterStatus)
	}

	r.mainCondition = func() bool {
		return containsClusterCondition(api.Restarting, v1.ConditionTrue, r.clusterStatus) &&
			containsClusterCondition(api.UpdatingESSettings, v1.ConditionFalse, r.clusterStatus) &&
			containsClusterCondition(api.Recovering, v1.ConditionFalse, r.clusterStatus)
	}

	r.postCondition = func() bool {
		return containsClusterCondition(api.Restarting, v1.ConditionTrue, r.clusterStatus) &&
			containsClusterCondition(api.UpdatingESSettings, v1.ConditionTrue, r.clusterStatus) &&
			containsClusterCondition(api.Recovering, v1.ConditionFalse, r.clusterStatus)
	}

	r.recoveryCondition = func() bool {
		return containsClusterCondition(api.Restarting, v1.ConditionFalse, r.clusterStatus) &&
			containsClusterCondition(api.UpdatingESSettings, v1.ConditionFalse, r.clusterStatus) &&
			containsClusterCondition(api.Recovering, v1.ConditionTrue, r.clusterStatus)
	}

	// cluster signalers
	r.precheckSignaler = func() {
		log.Info("Beginning restart cluster", "cluster", r.clusterName, "namespace", r.clusterNamespace)
		updateUpdatingESSettingsCondition(r.clusterStatus, v1.ConditionTrue)
	}

	r.prepSignaler = func() {
		updateRestartingCondition(r.clusterStatus, v1.ConditionTrue)
		updateUpdatingESSettingsCondition(r.clusterStatus, v1.ConditionFalse)
	}

	r.mainSignaler = func() {
		updateUpdatingESSettingsCondition(r.clusterStatus, v1.ConditionTrue)
	}

	r.postSignaler = func() {
		// since we restarted we are no longer needing to be scheduled for a certRedeploy
		updateStatus()

		updateUpdatingESSettingsCondition(r.clusterStatus, v1.ConditionFalse)
		updateRecoveringCondition(r.clusterStatus, v1.ConditionTrue)
		updateRestartingCondition(r.clusterStatus, v1.ConditionFalse)
	}

	r.recoverySignaler = func() {
		log.Info("Completed restart of cluster", "cluster", r.clusterName, "namespace", r.clusterNamespace)
		updateRestartingCondition(r.clusterStatus, v1.ConditionFalse)
		updateRecoveringCondition(r.clusterStatus, v1.ConditionFalse)
	}
}

func (r *Restarter) setNodeConditions(updateStatus func()) {
	// node conditions
	r.precheckCondition = func() bool {
		return r.nodeStatus.UpgradeStatus.UnderUpgrade != v1.ConditionTrue
	}

	r.prepCondition = func() bool {
		return r.nodeStatus.UpgradeStatus.UpgradePhase == "" ||
			r.nodeStatus.UpgradeStatus.UpgradePhase == api.ControllerUpdated
	}

	r.mainCondition = func() bool {
		return r.nodeStatus.UpgradeStatus.UpgradePhase == api.PreparationComplete
	}

	r.postCondition = func() bool {
		return r.nodeStatus.UpgradeStatus.UpgradePhase == api.NodeRestarting
	}

	r.recoveryCondition = func() bool {
		return r.nodeStatus.UpgradeStatus.UpgradePhase == api.RecoveringData
	}

	// node signalers
	r.precheckSignaler = func() {
		r.nodeStatus.UpgradeStatus.UnderUpgrade = v1.ConditionTrue

		// for node restarts there should be only a single node
		log.Info("Beginning restart of node",
			"node", r.scheduledNodes[0].name(),
			"cluster", r.clusterName,
			"namespace", r.clusterNamespace)
		updateStatus()
	}

	r.prepSignaler = func() {
		r.nodeStatus.UpgradeStatus.UpgradePhase = api.PreparationComplete

		updateStatus()
	}

	r.mainSignaler = func() {
		r.nodeStatus.UpgradeStatus.UpgradePhase = api.NodeRestarting

		updateStatus()
	}

	r.postSignaler = func() {
		r.nodeStatus.UpgradeStatus.UpgradePhase = api.RecoveringData

		updateStatus()
	}

	r.recoverySignaler = func() {
		// for node restarts there should be only a single node
		log.Info("Completed restart of node",
			"node", r.scheduledNodes[0].name(),
			"cluster", r.clusterName,
			"namespace", r.clusterNamespace)

		r.nodeStatus.UpgradeStatus.UpgradePhase = api.ControllerUpdated
		r.nodeStatus.UpgradeStatus.UnderUpgrade = ""

		r.nodeStatus.UpgradeStatus.ScheduledForUpgrade = ""

		updateStatus()
	}
}

// template function used for all restarts
func (r Restarter) restartCluster() error {
	if r.precheckCondition() {
		if err := r.precheck(); err != nil {
			return err
		}

		// set conditions here for next check
		r.precheckSignaler()
	}

	if r.prepCondition() {
		if err := r.prep(); err != nil {
			// ignore flush failures
			if !errors.Is(err, ErrFlushShardsFailed) {
				return err
			}
		}

		r.prepSignaler()
	}

	if r.mainCondition() {

		if err := r.main(); err != nil {
			return err
		}

		r.mainSignaler()
	}

	if r.postCondition() {

		if err := r.post(); err != nil {
			return err
		}

		r.postSignaler()
	}

	if r.recoveryCondition() {

		if err := r.recovery(); err != nil {
			return err
		}

		r.recoverySignaler()
	}

	return nil
}
