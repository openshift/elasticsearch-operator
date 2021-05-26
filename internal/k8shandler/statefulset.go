package k8shandler

import (
	"context"
	"time"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/go-logr/logr"
	"github.com/openshift/elasticsearch-operator/internal/elasticsearch"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/ViaQ/logerr/log"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
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

	esClient elasticsearch.Client

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

func (n *statefulSetNode) populateReference(nodeName string, node api.ElasticsearchNode, cluster *api.Elasticsearch, roleMap map[api.ElasticsearchNodeRole]bool, replicas int32, client client.Client, esClient elasticsearch.Client) {
	labels := newLabels(cluster.Name, nodeName, roleMap)

	statefulSet := apps.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: apps.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeName,
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
	}

	n.replicas = replicas

	partition := int32(0)
	logConfig := getLogConfig(cluster.GetAnnotations())
	statefulSet.Spec = apps.StatefulSetSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: newLabelSelector(cluster.Name, nodeName, roleMap),
		},
		Template: newPodTemplateSpec(nodeName, cluster.Name, cluster.Namespace, node, cluster.Spec.Spec, labels, roleMap, client, logConfig),
		UpdateStrategy: apps.StatefulSetUpdateStrategy{
			Type: apps.RollingUpdateStatefulSetStrategyType,
			RollingUpdate: &apps.RollingUpdateStatefulSetStrategy{
				Partition: &partition,
			},
		},
	}
	statefulSet.Spec.Template.Spec.Containers[0].ReadinessProbe = nil

	cluster.AddOwnerRefTo(&statefulSet)

	n.self = statefulSet
	n.clusterName = cluster.Name

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
	newSecretHash := getSecretDataHash(n.clusterName, n.self.Namespace, n.client)
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
	nodeCopy := n.self.DeepCopy()

	nretries := -1
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		nretries++
		if err := n.client.Get(context.TODO(), types.NamespacedName{Name: n.self.Name, Namespace: n.self.Namespace}, nodeCopy); err != nil {
			n.L().Info("Could not get Elasticsearch node resource", "error", err)
			return err
		}

		if *nodeCopy.Spec.UpdateStrategy.RollingUpdate.Partition == partitions {
			return nil
		}

		nodeCopy.Spec.UpdateStrategy.RollingUpdate.Partition = &partitions

		if err := n.client.Update(context.TODO(), nodeCopy); err != nil {
			n.L().Info("Failed to update node resource. Retrying...", "error", err)
			return err
		}

		n.self.Spec.UpdateStrategy.RollingUpdate.Partition = &partitions

		return nil
	})
	if err != nil {
		return kverrors.Wrap(err, "could not update Elasticsearch node",
			"node", n.self.Name,
			"retries", nretries,
		)
	}

	n.L().Info("successfully updated Elasticsearch node")
	return nil
}

func (n *statefulSetNode) partition() (int32, error) {
	desired := &apps.StatefulSet{}

	if err := n.client.Get(context.TODO(), types.NamespacedName{Name: n.self.Name, Namespace: n.self.Namespace}, desired); err != nil {
		n.L().Info("Could not get Elasticsearch node resource", "error", err)
		return -1, err
	}

	return *desired.Spec.UpdateStrategy.RollingUpdate.Partition, nil
}

func (n *statefulSetNode) setReplicaCount(replicas int32) error {
	nodeCopy := n.self.DeepCopy()

	nretries := -1
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		nretries++
		if err := n.client.Get(context.TODO(), types.NamespacedName{Name: n.self.Name, Namespace: n.self.Namespace}, nodeCopy); err != nil {
			n.L().Error(err, "Could not get Elasticsearch node resource")
			return err
		}

		if *nodeCopy.Spec.Replicas == replicas {
			return nil
		}

		nodeCopy.Spec.Replicas = &replicas

		if err := n.client.Update(context.TODO(), &n.self); err != nil {
			n.L().Error(err, "Failed to update node resource")
			return err
		}

		n.self.Spec.Replicas = &replicas

		return nil
	})
	if retryErr != nil {
		return kverrors.Wrap(retryErr, "could not update Elasticsearch node",
			"node", n.self.Name,
			"retries", nretries,
		)
	}

	return nil
}

func (n *statefulSetNode) replicaCount() (int32, error) {
	desired := &apps.StatefulSet{}

	if err := n.client.Get(context.TODO(), types.NamespacedName{Name: n.self.Name, Namespace: n.self.Namespace}, desired); err != nil {
		return -1, err
	}

	return desired.Status.Replicas, nil
}

func (n *statefulSetNode) isMissing() bool {
	obj := &apps.StatefulSet{}
	key := types.NamespacedName{Name: n.name(), Namespace: n.self.Namespace}

	if err := n.client.Get(context.TODO(), key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return true
		}
	}

	return false
}

func (n *statefulSetNode) delete() error {
	return n.client.Delete(context.TODO(), &n.self)
}

func (n *statefulSetNode) create() error {
	if n.self.ObjectMeta.ResourceVersion == "" {
		err := n.client.Create(context.TODO(), &n.self)
		if err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return kverrors.Wrap(err, "could not create node resource")
			} else {
				n.scale()
				return nil
			}
		}

		// update the hashmaps
		n.configmapHash = getConfigmapDataHash(n.clusterName, n.self.Namespace, n.client)
		n.secretHash = getSecretDataHash(n.clusterName, n.self.Namespace, n.client)
	} else {
		n.scale()
	}

	return nil
}

func (n *statefulSetNode) executeUpdate() error {
	// see if we need to update the deployment object and verify we have latest to update
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentStatefulSet := apps.StatefulSet{}

		err := n.client.Get(context.TODO(), types.NamespacedName{Name: n.self.Name, Namespace: n.self.Namespace}, &currentStatefulSet)
		// error check that it exists, etc
		if err != nil {
			n.L().Error(err, "Failed to get node")
			return err
		}

		if ArePodTemplateSpecDifferent(currentStatefulSet.Spec.Template, n.self.Spec.Template) {
			currentStatefulSet.Spec.Template = CreateUpdatablePodTemplateSpec(currentStatefulSet.Spec.Template, n.self.Spec.Template)

			if updateErr := n.client.Update(context.TODO(), &currentStatefulSet); updateErr != nil {
				n.L().Error(err, "Failed to update node resource")
				return updateErr
			}
		}
		return nil
	})
}

func (n *statefulSetNode) refreshHashes() {
	newConfigmapHash := getConfigmapDataHash(n.clusterName, n.self.Namespace, n.client)
	if newConfigmapHash != n.configmapHash {
		n.configmapHash = newConfigmapHash
	}

	newSecretHash := getSecretDataHash(n.clusterName, n.self.Namespace, n.client)
	if newSecretHash != n.secretHash {
		n.secretHash = newSecretHash
	}
}

func (n *statefulSetNode) scale() {
	desired := n.self.DeepCopy()
	err := n.client.Get(context.TODO(), types.NamespacedName{Name: n.self.Name, Namespace: n.self.Namespace}, &n.self)
	// error check that it exists, etc
	if err != nil {
		// if it doesn't exist, return true
		return
	}

	if *desired.Spec.Replicas != *n.self.Spec.Replicas {
		n.self.Spec.Replicas = desired.Spec.Replicas
		n.L().Info("Resource has different container replicas than desired")

		if err := n.setReplicaCount(*n.self.Spec.Replicas); err != nil {
			n.L().Error(err, "unable to set replicate count")
		}
	}
}

func (n *statefulSetNode) isChanged() bool {
	desiredTemplate := n.self.Spec.Template
	currentStatefulSet := apps.StatefulSet{}

	err := n.client.Get(context.TODO(), types.NamespacedName{Name: n.self.Name, Namespace: n.self.Namespace}, &currentStatefulSet)
	// error check that it exists, etc
	if err != nil {
		// if it doesn't exist, return true
		return false
	}

	return ArePodTemplateSpecDifferent(currentStatefulSet.Spec.Template, desiredTemplate)
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
