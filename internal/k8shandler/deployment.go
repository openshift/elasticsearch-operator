package k8shandler

import (
	"context"
	"time"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/openshift/elasticsearch-operator/internal/elasticsearch"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ViaQ/logerr/log"
	api "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type deploymentNode struct {
	self apps.Deployment
	// prior hash for configmap content
	configmapHash string
	// prior hash for secret content
	secretHash string

	clusterName string

	replicas int32

	client client.Client

	esClient elasticsearch.Client
}

func (node *deploymentNode) populateReference(nodeName string, n api.ElasticsearchNode, cluster *api.Elasticsearch, roleMap map[api.ElasticsearchNodeRole]bool, replicas int32, client client.Client, esClient elasticsearch.Client) {
	labels := newLabels(cluster.Name, nodeName, roleMap)

	deployment := apps.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: apps.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeName,
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
	}

	node.replicas = replicas

	progressDeadlineSeconds := int32(1800)
	logConfig := getLogConfig(cluster.GetAnnotations())
	deployment.Spec = apps.DeploymentSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: newLabelSelector(cluster.Name, nodeName, roleMap),
		},
		Strategy: apps.DeploymentStrategy{
			Type: "Recreate",
		},
		ProgressDeadlineSeconds: &progressDeadlineSeconds,
		Paused:                  false,
		Template:                newPodTemplateSpec(nodeName, cluster.Name, cluster.Namespace, n, cluster.Spec.Spec, labels, roleMap, client, logConfig),
	}

	cluster.AddOwnerRefTo(&deployment)

	node.self = deployment
	node.clusterName = cluster.Name

	node.client = client
	node.esClient = esClient
}

func (node *deploymentNode) updateReference(n NodeTypeInterface) {
	node.self = n.(*deploymentNode).self
}

func (node *deploymentNode) scaleDown() error {
	return node.setReplicaCount(0)
}

func (node *deploymentNode) scaleUp() error {
	return node.setReplicaCount(node.replicas)
}

func (node *deploymentNode) name() string {
	return node.self.Name
}

func (node *deploymentNode) getSecretHash() string {
	return node.secretHash
}

func (node *deploymentNode) state() api.ElasticsearchNodeStatus {
	// var rolloutForReload v1.ConditionStatus
	var rolloutForUpdate v1.ConditionStatus
	var rolloutForCertReload v1.ConditionStatus

	// see if we need to update the deployment object
	if node.isChanged() {
		rolloutForUpdate = v1.ConditionTrue
	}

	// check for a case where our hash is missing -- operator restarted?
	newSecretHash := getSecretDataHash(node.clusterName, node.self.Namespace, node.client)
	if node.secretHash == "" {
		// if we were already scheduled to restart, don't worry? -- just grab
		// the current hash -- we should have already had our upgradeStatus set if
		// we required a restart...
		node.secretHash = newSecretHash
	} else {
		// check if the secretHash changed
		if newSecretHash != node.secretHash {
			rolloutForCertReload = v1.ConditionTrue
		}
	}

	return api.ElasticsearchNodeStatus{
		DeploymentName: node.self.Name,
		UpgradeStatus: api.ElasticsearchNodeUpgradeStatus{
			ScheduledForUpgrade:      rolloutForUpdate,
			ScheduledForCertRedeploy: rolloutForCertReload,
		},
	}
}

func (node *deploymentNode) delete() error {
	return node.client.Delete(context.TODO(), &node.self)
}

func (node *deploymentNode) create() error {
	if node.self.ObjectMeta.ResourceVersion == "" {
		err := node.client.Create(context.TODO(), &node.self)
		if err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return kverrors.Wrap(err, "could not create node resource")
			} else {
				return node.pause()
			}
		}

		// created unpaused, pause after deployment...
		// wait until we have a revision annotation...
		if err = node.waitForInitialRollout(); err != nil {
			return err
		}

		// update the hashmaps
		node.configmapHash = getConfigmapDataHash(node.clusterName, node.self.Namespace, node.client)
		node.secretHash = getSecretDataHash(node.clusterName, node.self.Namespace, node.client)
	}

	return node.pause()
}

func (node *deploymentNode) waitForInitialRollout() error {
	err := wait.Poll(time.Second*1, time.Second*30, func() (done bool, err error) {
		if err := node.client.Get(context.TODO(), types.NamespacedName{Name: node.self.Name, Namespace: node.self.Namespace}, &node.self); err != nil {
			return false, err
		}

		_, ok := node.self.ObjectMeta.Annotations["deployment.kubernetes.io/revision"]
		if ok {
			return true, nil
		}

		return false, nil
	})
	return err
}

func (node *deploymentNode) nodeRevision() string {
	val, ok := node.self.ObjectMeta.Annotations["deployment.kubernetes.io/revision"]

	if ok {
		return val
	}

	return ""
}

func (node *deploymentNode) waitForNodeRollout() error {
	podLabels := map[string]string{
		"node-name": node.name(),
	}

	err := wait.Poll(time.Second*1, time.Second*30, func() (done bool, err error) {
		return node.checkPodSpecMatches(podLabels), nil
	})
	return err
}

func (node *deploymentNode) podSpecMatches() bool {
	podLabels := map[string]string{
		"node-name": node.name(),
	}

	return node.checkPodSpecMatches(podLabels)
}

func (node *deploymentNode) checkPodSpecMatches(labels map[string]string) bool {
	podList, err := GetPodList(node.self.Namespace, labels, node.client)
	if err != nil {
		log.Error(err, "Could not get node pods", "node", node.name())
		return false
	}

	for _, pod := range podList.Items {
		if !ArePodSpecDifferent(pod.Spec, node.self.Spec.Template.Spec, false) {
			return true
		}
	}

	return false
}

func (node *deploymentNode) pause() error {
	return node.setPaused(true)
}

func (node *deploymentNode) unpause() error {
	return node.setPaused(false)
}

func (node *deploymentNode) setPaused(paused bool) error {
	// we use pauseNode so that we don't revert any new changes that should be made and
	// noticed in state()
	pauseNode := node.self.DeepCopy()
	ll := log.WithValues("node", pauseNode.Name)

	nretries := -1
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		nretries++
		if err := node.client.Get(context.TODO(), types.NamespacedName{Name: pauseNode.Name, Namespace: pauseNode.Namespace}, pauseNode); err != nil {
			ll.Info("Could not get Elasticsearch node resource",
				"error", err)
			return err
		}

		if pauseNode.Spec.Paused == paused {
			return nil
		}

		pauseNode.Spec.Paused = paused

		if err := node.client.Update(context.TODO(), pauseNode); err != nil {
			ll.Info("failed to update node resource",
				"error", err)
			return err
		}
		return nil
	})
	if retryErr != nil {
		return kverrors.Wrap(retryErr, "could not update Elasticsearch node after retries",
			"node", node.self.Name,
			"retries", nretries,
		)
	}

	node.self.Spec.Paused = pauseNode.Spec.Paused

	return nil
}

func (node *deploymentNode) setReplicaCount(replicas int32) error {
	nodeCopy := &apps.Deployment{}
	nretries := -1
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		nretries++
		if err := node.client.Get(context.TODO(), types.NamespacedName{Name: node.self.Name, Namespace: node.self.Namespace}, nodeCopy); err != nil {
			log.Info("Could not get Elasticsearch node resource, Retrying...", "error", err)
			return err
		}

		if *nodeCopy.Spec.Replicas == replicas {
			return nil
		}

		nodeCopy.Spec.Replicas = &replicas

		if err := node.client.Update(context.TODO(), nodeCopy); err != nil {
			log.Info("failed to update node resource", "node", node.self.Name, "error", err)
			return err
		}

		node.self.Spec.Replicas = &replicas

		return nil
	})
	if retryErr != nil {
		return kverrors.Wrap(retryErr, "could not update Elasticsearch node",
			"node", node.self.Name,
			"retries", nretries,
		)
	}

	return nil
}

func (node *deploymentNode) replicaCount() (int32, error) {
	nodeCopy := &apps.Deployment{}

	if err := node.client.Get(context.TODO(), types.NamespacedName{Name: node.self.Name, Namespace: node.self.Namespace}, nodeCopy); err != nil {
		log.Error(err, "Could not get Elasticsearch node resource")
		return -1, err
	}

	return nodeCopy.Status.Replicas, nil
}

func (node *deploymentNode) waitForNodeRejoinCluster() (bool, error) {
	err := wait.Poll(time.Second*1, time.Second*60, func() (done bool, err error) {
		return node.esClient.IsNodeInCluster(node.name())
	})

	return err == nil, err
}

func (node *deploymentNode) waitForNodeLeaveCluster() (bool, error) {
	err := wait.Poll(time.Second*1, time.Second*60, func() (done bool, err error) {
		inCluster, checkErr := node.esClient.IsNodeInCluster(node.name())

		return !inCluster, checkErr
	})

	return err == nil, err
}

func (node *deploymentNode) isMissing() bool {
	obj := &apps.Deployment{}
	key := types.NamespacedName{Name: node.name(), Namespace: node.self.Namespace}

	if err := node.client.Get(context.TODO(), key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return true
		}
	}

	return false
}

func (node *deploymentNode) executeUpdate() error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// isChanged() will get the latest revision from the apiserver
		// and return false if there is nothing to change and will update the node object if required

		currentDeployment := apps.Deployment{}
		err := node.client.Get(context.TODO(), types.NamespacedName{Name: node.self.Name, Namespace: node.self.Namespace}, &currentDeployment)
		if err != nil {
			return err
		}

		if ArePodTemplateSpecDifferent(currentDeployment.Spec.Template, node.self.Spec.Template) {
			currentDeployment.Spec.Template = CreateUpdatablePodTemplateSpec(currentDeployment.Spec.Template, node.self.Spec.Template)

			if err := node.client.Update(context.TODO(), &currentDeployment); err != nil {
				log.Info("Failed to update node resource", "error", err)
				return err
			}
		}
		return nil
	})
}

func (node *deploymentNode) progressNodeChanges() error {
	if !node.isChanged() && node.podSpecMatches() {
		return nil
	}

	if err := node.executeUpdate(); err != nil {
		return err
	}

	if err := node.unpause(); err != nil {
		return kverrors.Wrap(err, "unable to unpause node",
			"node", node.name(),
		)
	}

	if err := node.waitForNodeRollout(); err != nil {
		return kverrors.New("timed out waiting for node to rollout",
			"node", node.name(),
		)
	}

	if err := node.pause(); err != nil {
		return kverrors.Wrap(err, "unable to pause node",
			"node", node.name(),
		)
	}

	node.refreshHashes()
	return nil
}

func (node *deploymentNode) refreshHashes() {
	newConfigmapHash := getConfigmapDataHash(node.clusterName, node.self.Namespace, node.client)
	if newConfigmapHash != node.configmapHash {
		node.configmapHash = newConfigmapHash
	}

	newSecretHash := getSecretDataHash(node.clusterName, node.self.Namespace, node.client)
	if newSecretHash != node.secretHash {
		node.secretHash = newSecretHash
	}
}

func (node *deploymentNode) isChanged() bool {
	desiredTemplate := node.self.Spec.Template
	currentDeployment := apps.Deployment{}

	err := node.client.Get(context.TODO(), types.NamespacedName{Name: node.self.Name, Namespace: node.self.Namespace}, &currentDeployment)
	// error check that it exists, etc
	if err != nil {
		// if it doesn't exist, return true
		return false
	}

	return ArePodTemplateSpecDifferent(currentDeployment.Spec.Template, desiredTemplate)
}
