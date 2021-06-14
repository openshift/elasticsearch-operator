package elasticsearch

import (
	"context"
	"time"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/openshift/elasticsearch-operator/internal/elasticsearch/esclient"
	"github.com/openshift/elasticsearch-operator/internal/manifests/configmap"
	"github.com/openshift/elasticsearch-operator/internal/manifests/deployment"
	"github.com/openshift/elasticsearch-operator/internal/manifests/pod"
	"github.com/openshift/elasticsearch-operator/internal/manifests/secret"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
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

	esClient esclient.Client
}

func (node *deploymentNode) populateReference(nodeName string, n api.ElasticsearchNode, cluster *api.Elasticsearch, roleMap map[api.ElasticsearchNodeRole]bool, replicas int32, client client.Client, esClient esclient.Client) {
	labels := newLabels(cluster.Name, nodeName, roleMap)

	progressDeadlineSeconds := int32(1800)
	logConfig := getLogConfig(cluster.GetAnnotations())
	template := newPodTemplateSpec(nodeName, cluster.Name, cluster.Namespace, n, cluster.Spec.Spec, labels, roleMap, client, logConfig)

	dpl := deployment.New(nodeName, cluster.Namespace, labels, replicas).
		WithSelector(metav1.LabelSelector{
			MatchLabels: newLabelSelector(cluster.Name, nodeName, roleMap),
		}).
		WithStrategy(apps.RecreateDeploymentStrategyType).
		WithProgressDeadlineSeconds(progressDeadlineSeconds).
		WithTemplate(template).
		WithPaused(false).
		Build()

	cluster.AddOwnerRefTo(dpl)

	node.self = *dpl
	node.clusterName = cluster.Name
	node.replicas = replicas

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
	key := client.ObjectKey{Name: node.clusterName, Namespace: node.self.Namespace}
	newSecretHash := secret.GetDataSHA256(context.TODO(), node.client, key)
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
	key := client.ObjectKey{Name: node.self.Name, Namespace: node.self.Namespace}
	return deployment.Delete(context.TODO(), node.client, key)
}

func (node *deploymentNode) create() error {
	if node.self.ObjectMeta.ResourceVersion == "" {

		err := deployment.Create(context.TODO(), node.client, &node.self)
		if err != nil {
			if !apierrors.IsAlreadyExists(kverrors.Root(err)) {
				return kverrors.Wrap(err, "failed to create or update elasticsearch node deployment",

					"cluster", node.clusterName,
					"namespace", node.self.Namespace,
				)
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
		node.refreshHashes()
	}

	return node.pause()
}

func (node *deploymentNode) waitForInitialRollout() error {
	err := wait.Poll(time.Second*1, time.Second*30, func() (done bool, err error) {
		key := client.ObjectKey{Name: node.self.Name, Namespace: node.self.Namespace}
		dpl, err := deployment.Get(context.TODO(), node.client, key)
		if err != nil {
			return false, err
		}

		node.self = *dpl
		_, ok := dpl.Annotations["deployment.kubernetes.io/revision"]
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
	err := wait.Poll(time.Second*1, time.Second*30, func() (done bool, err error) {
		return node.podSpecMatches(), nil
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
	podList, err := pod.List(context.TODO(), node.client, node.self.Namespace, labels)
	if err != nil {
		log.Error(err, "Could not get node pods", "node", node.name())
		return false
	}

	for _, p := range podList {
		if !pod.ArePodSpecEqual(p.Spec, node.self.Spec.Template.Spec, false) {
			return false
		}
	}

	return true
}

func (node *deploymentNode) pause() error {
	return node.setPaused(true)
}

func (node *deploymentNode) unpause() error {
	return node.setPaused(false)
}

func (node *deploymentNode) setPaused(paused bool) error {
	equalFunc := func(current, _ *apps.Deployment) bool {
		return current.Spec.Paused == paused
	}
	mutateFunc := func(current, _ *apps.Deployment) {
		current.Spec.Paused = paused
	}
	// we use pauseNode so that we don't revert any new changes that should be made and
	// noticed in state()
	pausedNode := node.self.DeepCopy()
	err := deployment.Update(context.TODO(), node.client, pausedNode, equalFunc, mutateFunc)
	if err != nil {
		return kverrors.Wrap(err, "failed to update elasticsearch node deployment",
			"cluster", node.clusterName,
			"namespace", node.self.Namespace,
		)
	}

	node.self.Spec.Paused = paused

	return nil
}

func (node *deploymentNode) setReplicaCount(replicas int32) error {
	equalFunc := func(current, _ *apps.Deployment) bool {
		if current.Spec.Replicas == nil {
			return false
		}
		return *current.Spec.Replicas == replicas
	}
	mutateFunc := func(current, _ *apps.Deployment) {
		current.Spec.Replicas = &replicas
	}

	err := deployment.Update(context.TODO(), node.client, &node.self, equalFunc, mutateFunc)
	if err != nil {
		return kverrors.Wrap(err, "failed to update elasticsearch node deployment",
			"cluster", node.clusterName,
			"namespace", node.self.Namespace,
		)
	}

	node.self.Spec.Replicas = &replicas

	return nil
}

func (node *deploymentNode) replicaCount() (int32, error) {
	key := client.ObjectKey{Name: node.self.Name, Namespace: node.self.Namespace}
	dpl, err := deployment.Get(context.TODO(), node.client, key)
	if err != nil {
		log.Error(err, "Could not get Elasticsearch node resource")
		return -1, err
	}

	return dpl.Status.Replicas, nil
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
	key := client.ObjectKey{Name: node.name(), Namespace: node.self.Namespace}
	_, err := deployment.Get(context.TODO(), node.client, key)
	if err != nil {
		if apierrors.IsNotFound(kverrors.Root(err)) {
			return true
		}
	}

	return false
}

func (node *deploymentNode) executeUpdate() error {
	equalFunc := func(current, desired *apps.Deployment) bool {
		return pod.ArePodTemplateSpecEqual(current.Spec.Template, desired.Spec.Template)
	}

	mutateFunc := func(current, desired *apps.Deployment) {
		current.Spec.Template = createUpdatablePodTemplateSpec(current.Spec.Template, desired.Spec.Template)
	}

	err := deployment.Update(context.TODO(), node.client, &node.self, equalFunc, mutateFunc)
	if err != nil {
		return kverrors.Wrap(err, "failed to update elasticsearch node deployment",
			"cluster", node.clusterName,
			"namespace", node.self.Namespace,
		)
	}

	return nil
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
	key := client.ObjectKey{Name: node.clusterName, Namespace: node.self.Namespace}

	newConfigmapHash := configmap.GetDataSHA256(context.TODO(), node.client, key, excludeConfigMapKeys)
	if newConfigmapHash != "" && newConfigmapHash != node.configmapHash {
		node.configmapHash = newConfigmapHash
	}

	newSecretHash := secret.GetDataSHA256(context.TODO(), node.client, key)
	if newSecretHash != "" && newSecretHash != node.secretHash {
		node.secretHash = newSecretHash
	}
}

func (node *deploymentNode) isChanged() bool {
	key := client.ObjectKey{Name: node.self.Name, Namespace: node.self.Namespace}
	current, err := deployment.Get(context.TODO(), node.client, key)
	if err != nil {
		// if it doesn't exist, return true
		return false
	}

	return !pod.ArePodTemplateSpecEqual(current.Spec.Template, node.self.Spec.Template)
}
