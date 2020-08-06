package k8shandler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/openshift/elasticsearch-operator/pkg/elasticsearch"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/pkg/logger"
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

	currentRevision string

	replicas int32

	client client.Client

	esClient elasticsearch.Client
}

func (deploymentNode *deploymentNode) populateReference(nodeName string, node api.ElasticsearchNode, cluster *api.Elasticsearch, roleMap map[api.ElasticsearchNodeRole]bool, replicas int32, client client.Client, esClient elasticsearch.Client) {

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

	deploymentNode.replicas = replicas

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
		Template:                newPodTemplateSpec(nodeName, cluster.Name, cluster.Namespace, node, cluster.Spec.Spec, labels, roleMap, client, logConfig),
	}

	addOwnerRefToObject(&deployment, getOwnerRef(cluster))

	deploymentNode.self = deployment
	deploymentNode.clusterName = cluster.Name

	deploymentNode.client = client
	deploymentNode.esClient = esClient
}

func (current *deploymentNode) updateReference(node NodeTypeInterface) {
	current.self = node.(*deploymentNode).self
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

func (node *deploymentNode) state() api.ElasticsearchNodeStatus {

	//var rolloutForReload v1.ConditionStatus
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
			if !errors.IsAlreadyExists(err) {
				return fmt.Errorf("Could not create node resource: %v", err)
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
		if getErr := node.client.Get(context.TODO(), types.NamespacedName{Name: node.self.Name, Namespace: node.self.Namespace}, &node.self); getErr != nil {
			logrus.Debugf("Could not get Elasticsearch node resource %v: %v", node.self.Name, getErr)
			return false, getErr
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
		logrus.Warnf("Could not get node %q pods: %v", node.name(), err)
		return false
	}

	matches := false

	for _, pod := range podList.Items {
		// follow pattern used in other places of "current, desired"
		if !ArePodSpecDifferent(pod.Spec, node.self.Spec.Template.Spec, false) {
			matches = true
		}
	}

	return matches
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

	nretries := -1
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		nretries++
		if getErr := node.client.Get(context.TODO(), types.NamespacedName{Name: pauseNode.Name, Namespace: pauseNode.Namespace}, pauseNode); getErr != nil {
			logrus.Debugf("Could not get Elasticsearch node resource %v: %v", pauseNode.Name, getErr)
			return getErr
		}

		if pauseNode.Spec.Paused == paused {
			return nil
		}

		pauseNode.Spec.Paused = paused

		if updateErr := node.client.Update(context.TODO(), pauseNode); updateErr != nil {
			logrus.Debugf("Failed to update node resource %v: %v", pauseNode.Name, updateErr)
			return updateErr
		}
		return nil
	})
	if retryErr != nil {
		return fmt.Errorf("Error: could not update Elasticsearch node %v after %v retries: %v", node.self.Name, nretries, retryErr)
	}

	node.self.Spec.Paused = pauseNode.Spec.Paused

	return nil
}

func (node *deploymentNode) setReplicaCount(replicas int32) error {

	nodeCopy := &apps.Deployment{}

	nretries := -1
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		nretries++
		if getErr := node.client.Get(context.TODO(), types.NamespacedName{Name: node.self.Name, Namespace: node.self.Namespace}, nodeCopy); getErr != nil {
			logrus.Debugf("Could not get Elasticsearch node resource %v: %v", node.self.Name, getErr)
			return getErr
		}

		if *nodeCopy.Spec.Replicas == replicas {
			return nil
		}

		nodeCopy.Spec.Replicas = &replicas

		if updateErr := node.client.Update(context.TODO(), nodeCopy); updateErr != nil {
			logrus.Debugf("Failed to update node resource %v: %v", node.self.Name, updateErr)
			return updateErr
		}

		node.self.Spec.Replicas = &replicas

		return nil
	})
	if retryErr != nil {
		return fmt.Errorf("Error: could not update Elasticsearch node %v after %v retries: %v", node.self.Name, nretries, retryErr)
	}

	return nil
}

func (node *deploymentNode) replicaCount() (error, int32) {
	nodeCopy := &apps.Deployment{}

	if err := node.client.Get(context.TODO(), types.NamespacedName{Name: node.self.Name, Namespace: node.self.Namespace}, nodeCopy); err != nil {
		logrus.Debugf("Could not get Elasticsearch node resource %v: %v", node.self.Name, err)
		return err, -1
	}

	return nil, nodeCopy.Status.Replicas
}

func (node *deploymentNode) waitForNodeRejoinCluster() (error, bool) {
	err := wait.Poll(time.Second*1, time.Second*60, func() (done bool, err error) {
		return node.esClient.IsNodeInCluster(node.name())
	})

	return err, (err == nil)
}

func (node *deploymentNode) waitForNodeLeaveCluster() (error, bool) {
	err := wait.Poll(time.Second*1, time.Second*60, func() (done bool, err error) {
		inCluster, checkErr := node.esClient.IsNodeInCluster(node.name())

		return !inCluster, checkErr
	})

	return err, (err == nil)
}

func (node *deploymentNode) isMissing() bool {
	getNode := &apps.Deployment{}
	if getErr := node.client.Get(context.TODO(), types.NamespacedName{Name: node.name(), Namespace: node.self.Namespace}, getNode); getErr != nil {
		if errors.IsNotFound(getErr) {
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
			currentDeployment.Spec.Template = node.self.Spec.Template
			if logger.IsDebugEnabled() {
				pretty, err := json.MarshalIndent(node.self, "", "  ")
				if err != nil {
					logger.Debugf("Error marshaling node for debug log: %v", err)
				}
				logger.Debugf("Attempting to update node deployment: %+v", string(pretty))
			}
			if updateErr := node.client.Update(context.TODO(), &currentDeployment); updateErr != nil {
				logger.Debugf("Failed to update node resource %v: %v", node.self.Name, updateErr)
				return updateErr
			}
		}
		return nil
	})
}

func (node *deploymentNode) progressNodeChanges() error {
	if node.isChanged() || !node.podSpecMatches() {
		if err := node.executeUpdate(); err != nil {
			return err
		}

		if err := node.unpause(); err != nil {
			return fmt.Errorf("unable to unpause node %q: %v", node.name(), err)
		}

		logrus.Debugf("Waiting for node '%s' to rollout...", node.name())

		if err := node.waitForNodeRollout(); err != nil {
			return fmt.Errorf("Timed out waiting for node %v to rollout", node.name())
		}

		if err := node.pause(); err != nil {
			return fmt.Errorf("unable to pause node %q: %v", node.name(), err)
		}

		node.refreshHashes()
	}
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
