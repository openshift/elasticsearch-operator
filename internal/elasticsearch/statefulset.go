package elasticsearch

import (
	"context"
	"time"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/go-logr/logr"
	"github.com/openshift/elasticsearch-operator/internal/elasticsearch/esclient"
	"github.com/openshift/elasticsearch-operator/internal/manifests/configmap"
	"github.com/openshift/elasticsearch-operator/internal/manifests/pod"
	"github.com/openshift/elasticsearch-operator/internal/manifests/secret"
	"github.com/openshift/elasticsearch-operator/internal/manifests/statefulset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/ViaQ/logerr/log"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	apps "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type statefulSetNode struct {
	self apps.StatefulSet
	// prior hash for configmap content
	configmapHash string
	// prior hash for secret content
	secretHash string

	clusterName string

	replicas int32

	client client.Client

	esClient esclient.Client

	l logr.Logger
}

// L is the logger relative to this node
//
// TODO remove this construct when context.Context is passed and it should contain any relevant contextual values
func (n *statefulSetNode) L() logr.Logger {
	if n.l == nil {
		n.l = log.WithValues("node", n.name())
	}
	return n.l
}

func (n *statefulSetNode) populateReference(nodeName string, node api.ElasticsearchNode, cluster *api.Elasticsearch, roleMap map[api.ElasticsearchNodeRole]bool, replicas int32, client client.Client, esClient esclient.Client) {
	labels := newLabels(cluster.Name, nodeName, roleMap)
	partition := int32(0)
	logConfig := getLogConfig(cluster.GetAnnotations())

	template := newPodTemplateSpec(
		nodeName, cluster.Name, cluster.Namespace, node,
		cluster.Spec.Spec, labels, roleMap, client, logConfig,
	)

	sts := statefulset.New(nodeName, cluster.Namespace, labels, replicas).
		WithSelector(metav1.LabelSelector{
			MatchLabels: newLabelSelector(cluster.Name, nodeName, roleMap),
		}).
		WithTemplate(template).
		WithUpdateStrategy(apps.StatefulSetUpdateStrategy{
			Type: apps.RollingUpdateStatefulSetStrategyType,
			RollingUpdate: &apps.RollingUpdateStatefulSetStrategy{
				Partition: &partition,
			},
		}).
		Build()

	sts.Spec.Template.Spec.Containers[0].ReadinessProbe = nil

	cluster.AddOwnerRefTo(sts)

	n.self = *sts
	n.clusterName = cluster.Name
	n.replicas = replicas

	n.client = client
	n.esClient = esClient
}

func (n *statefulSetNode) updateReference(desired NodeTypeInterface) {
	n.self = desired.(*statefulSetNode).self
}

func (n *statefulSetNode) scaleDown() error {
	return n.setReplicaCount(0)
}

func (n *statefulSetNode) scaleUp() error {
	return n.setReplicaCount(n.replicas)
}

func (n *statefulSetNode) getSecretHash() string {
	return n.secretHash
}

func (n *statefulSetNode) state() api.ElasticsearchNodeStatus {
	var rolloutForUpdate v1.ConditionStatus
	var rolloutForCertReload v1.ConditionStatus

	// see if we need to update the deployment object
	if n.isChanged() {
		rolloutForUpdate = v1.ConditionTrue
	}

	// check for a case where our hash is missing -- operator restarted?
	key := client.ObjectKey{Name: n.clusterName, Namespace: n.self.Namespace}
	newSecretHash := secret.GetDataSHA256(context.TODO(), n.client, key)
	if n.secretHash == "" {
		// if we were already scheduled to restart, don't worry? -- just grab
		// the current hash -- we should have already had our upgradeStatus set if
		// we required a restart...
		n.secretHash = newSecretHash
	} else {
		// check if the secretHash changed
		if newSecretHash != n.secretHash {
			rolloutForCertReload = v1.ConditionTrue
		}
	}

	return api.ElasticsearchNodeStatus{
		StatefulSetName: n.self.Name,
		UpgradeStatus: api.ElasticsearchNodeUpgradeStatus{
			ScheduledForUpgrade:      rolloutForUpdate,
			ScheduledForCertRedeploy: rolloutForCertReload,
		},
	}
}

func (n *statefulSetNode) name() string {
	return n.self.Name
}

func (n *statefulSetNode) waitForNodeRejoinCluster() (bool, error) {
	err := wait.Poll(time.Second*1, time.Second*60, func() (done bool, err error) {
		clusterSize, err := n.esClient.GetClusterNodeCount()
		if err != nil {
			n.L().Error(err, "Unable to get cluster size waiting to rejoin cluster")
			return false, err
		}

		return n.replicas <= clusterSize, nil
	})

	return err == nil, err
}

func (n *statefulSetNode) waitForNodeLeaveCluster() (bool, error) {
	err := wait.Poll(time.Second*1, time.Second*60, func() (done bool, err error) {
		clusterSize, err := n.esClient.GetClusterNodeCount()
		if err != nil {
			n.L().Error(err, "Unable to get cluster size waiting to leave cluster")
			return false, err
		}

		return n.replicas > clusterSize, nil
	})

	return err == nil, err
}

func (n *statefulSetNode) setPartition(partitions int32) error {
	equalFunc := func(current, _ *apps.StatefulSet) bool {
		if current.Spec.UpdateStrategy.RollingUpdate.Partition == nil {
			return false
		}
		return *current.Spec.UpdateStrategy.RollingUpdate.Partition == partitions
	}
	mutateFunc := func(current, _ *apps.StatefulSet) {
		current.Spec.UpdateStrategy.RollingUpdate.Partition = &partitions
	}

	err := statefulset.Update(context.TODO(), n.client, &n.self, equalFunc, mutateFunc)
	if err != nil {
		return kverrors.Wrap(err, "failed to update elasticsearch node statefulset",
			"node_statefulset_name", n.self.Name,
		)
	}

	n.self.Spec.UpdateStrategy.RollingUpdate.Partition = &partitions

	return nil
}

func (n *statefulSetNode) partition() (int32, error) {
	key := client.ObjectKey{Name: n.name(), Namespace: n.self.Namespace}
	sts, err := statefulset.Get(context.TODO(), n.client, key)
	if err != nil {
		n.L().Info("Could not get Elasticsearch node resource", "error", err)
		return -1, err
	}

	return *sts.Spec.UpdateStrategy.RollingUpdate.Partition, nil
}

func (n *statefulSetNode) setReplicaCount(replicas int32) error {
	equalFunc := func(current, _ *apps.StatefulSet) bool {
		if current.Spec.Replicas == nil {
			return false
		}
		return *current.Spec.Replicas == replicas
	}
	mutateFunc := func(current, _ *apps.StatefulSet) {
		current.Spec.Replicas = &replicas
	}

	err := statefulset.Update(context.TODO(), n.client, &n.self, equalFunc, mutateFunc)
	if err != nil {
		return kverrors.Wrap(err, "failed to update elasticsearch node statefulset",
			"node_statefulset_name", n.self.Name,
		)
	}

	n.self.Spec.Replicas = &replicas

	return nil
}

func (n *statefulSetNode) replicaCount() (int32, error) {
	key := client.ObjectKey{Name: n.name(), Namespace: n.self.Namespace}
	sts, err := statefulset.Get(context.TODO(), n.client, key)
	if err != nil {
		return -1, err
	}

	return sts.Status.Replicas, nil
}

func (n *statefulSetNode) isMissing() bool {
	key := client.ObjectKey{Name: n.name(), Namespace: n.self.Namespace}
	_, err := statefulset.Get(context.TODO(), n.client, key)
	if err != nil {
		if apierrors.IsNotFound(kverrors.Root(err)) {
			return true
		}
	}

	return false
}

func (n *statefulSetNode) delete() error {
	key := client.ObjectKey{Name: n.self.Name, Namespace: n.self.Namespace}
	return statefulset.Delete(context.TODO(), n.client, key)
}

func (n *statefulSetNode) create() error {
	if n.self.ObjectMeta.ResourceVersion == "" {
		err := statefulset.Create(context.TODO(), n.client, &n.self)
		if err != nil {
			if !apierrors.IsAlreadyExists(kverrors.Root(err)) {
				return kverrors.Wrap(err, "failed to create or update elasticsearch node statefulset",
					"node_statefulset_name", n.self.Name,
				)
			} else {
				n.scale()
				return nil
			}
		}

		// update the hashmaps
		n.refreshHashes()
	} else {
		n.scale()
	}

	return nil
}

func (n *statefulSetNode) executeUpdate() error {
	equalFunc := func(current, desired *apps.StatefulSet) bool {
		return pod.ArePodTemplateSpecEqual(current.Spec.Template, desired.Spec.Template)
	}

	mutateFunc := func(current, desired *apps.StatefulSet) {
		current.Spec.Template = createUpdatablePodTemplateSpec(current.Spec.Template, desired.Spec.Template)
	}

	err := statefulset.Update(context.TODO(), n.client, &n.self, equalFunc, mutateFunc)
	if err != nil {
		return kverrors.Wrap(err, "failed to update elasticsearch node statefulset",
			"node_statefulset_name", n.self.Name,
		)
	}

	return nil
}

func (n *statefulSetNode) refreshHashes() {
	key := client.ObjectKey{Name: n.clusterName, Namespace: n.self.Namespace}

	newConfigmapHash := configmap.GetDataSHA256(context.TODO(), n.client, key, excludeConfigMapKeys)
	if newConfigmapHash != "" && newConfigmapHash != n.configmapHash {
		n.configmapHash = newConfigmapHash
	}

	newSecretHash := secret.GetDataSHA256(context.TODO(), n.client, key)
	if newSecretHash != "" && newSecretHash != n.secretHash {
		n.secretHash = newSecretHash
	}
}

func (n *statefulSetNode) scale() {
	key := client.ObjectKey{Name: n.name(), Namespace: n.self.Namespace}
	sts, err := statefulset.Get(context.TODO(), n.client, key)
	if err != nil {
		return
	}

	if *sts.Spec.Replicas != *n.self.Spec.Replicas {
		n.self.Spec.Replicas = sts.Spec.Replicas
		n.L().Info("Resource has different container replicas than desired")

		if err := n.setReplicaCount(*n.self.Spec.Replicas); err != nil {
			n.L().Error(err, "unable to set replicate count")
		}
	}
}

func (n *statefulSetNode) isChanged() bool {
	key := client.ObjectKey{Name: n.name(), Namespace: n.self.Namespace}
	sts, err := statefulset.Get(context.TODO(), n.client, key)
	if err != nil {
		return false
	}

	return !pod.ArePodTemplateSpecEqual(sts.Spec.Template, n.self.Spec.Template)
}

func (n *statefulSetNode) progressNodeChanges() error {
	if !n.isChanged() {
		return nil
	}
	replicas, err := n.replicaCount()
	if err != nil {
		return kverrors.Wrap(err, "Unable to get number of replicas prior to restart for node",
			"node", n.name(),
		)
	}

	if err := n.setPartition(replicas); err != nil {
		n.L().Error(err, "unable to set partition")
	}

	if err := n.executeUpdate(); err != nil {
		return err
	}

	ordinal, err := n.partition()
	if err != nil {
		return kverrors.Wrap(err, "unable to get node ordinal value")
	}

	// start partition at replicas and incrementally update it to 0
	// making sure nodes rejoin between each one
	for index := ordinal; index > 0; index-- {

		// make sure we have all nodes in the cluster first -- always
		if _, err := n.waitForNodeRejoinCluster(); err != nil {
			return kverrors.Wrap(err, "timed out waiting for node to rejoin cluster",
				"node", n.name(),
			)
		}

		// update partition to cause next pod to be updated
		if err := n.setPartition(index - 1); err != nil {
			n.L().Info("unable to set partition", "error", err)
		}

		// wait for the node to leave the cluster
		if _, err := n.waitForNodeLeaveCluster(); err != nil {
			return kverrors.Wrap(err, "timed out waiting for node to leave the cluster",
				"node", n.name(),
			)
		}
	}

	// this is here again because we need to make sure all nodes have rejoined
	// before we move on and say we're done
	if _, err := n.waitForNodeRejoinCluster(); err != nil {
		return kverrors.Wrap(err, "timed out waiting for node to rejoin cluster",
			"node", n.name(),
		)
	}

	n.refreshHashes()
	return nil
}
