package k8shandler

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	v1alpha1 "github.com/t0ffel/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type elasticsearchNode struct {
	ClusterName         string
	Namespace           string
	DeployName          string
	Roles               []v1alpha1.ElasticsearchNodeRole
	ESNodeSpec          v1alpha1.ElasticsearchNode
	ElasticsearchSecure v1alpha1.ElasticsearchSecure
	NodeNum             int32
	ReplicaNum          int32
	ServiceAccountName  string
	ConfigMapName       string
}

func constructNodeSpec(dpl *v1alpha1.Elasticsearch, esNode v1alpha1.ElasticsearchNode, configMapName, serviceAccountName string, nodeNum int32, replicaNum int32) (elasticsearchNode, error) {
	nodeCfg := elasticsearchNode{
		ClusterName:         dpl.Name,
		Namespace:           dpl.Namespace,
		Roles:               esNode.Roles,
		ESNodeSpec:          esNode,
		ElasticsearchSecure: dpl.Spec.Secure,
		NodeNum:             nodeNum,
		ReplicaNum:          replicaNum,
		ServiceAccountName:  serviceAccountName,
		ConfigMapName:       configMapName,
	}
	deployName, err := constructDeployName(dpl.Name, esNode.Roles, nodeNum, replicaNum)
	if err != nil {
		return nodeCfg, err
	}
	nodeCfg.DeployName = deployName

	nodeCfg.ESNodeSpec.Spec = reconcileNodeSpec(dpl.Spec.Spec, esNode.Spec)
	return nodeCfg, nil
}

func constructDeployName(name string, roles []v1alpha1.ElasticsearchNodeRole, nodeNum int32, replicaNum int32) (string, error) {
	if len(roles) == 0 {
		return "", fmt.Errorf("No node roles specified for a node in cluster %s", name)
	}
	var nodeType []string
	for _, role := range roles {
		if role != "client" && role != "data" && role != "master" {
			return "", fmt.Errorf("Unknown node's role: %s", role)
		}
		nodeType = append(nodeType, string(role))
	}

	sort.Strings(nodeType)

	return fmt.Sprintf("%s-%s-%d-%d", name, strings.Join(nodeType, ""), nodeNum, replicaNum), nil
}

func reconcileNodeSpec(commonSpec, nodeSpec v1alpha1.ElasticsearchNodeSpec) v1alpha1.ElasticsearchNodeSpec {
	var image string
	if nodeSpec.Image == "" {
		image = commonSpec.Image
	} else {
		image = nodeSpec.Image
	}
	nodeSpec = v1alpha1.ElasticsearchNodeSpec{
		Image:     image,
		Resources: getResourceRequirements(commonSpec.Resources, nodeSpec.Resources),
	}
	return nodeSpec
}

// getReplicas returns the desired number of replicas in the deployment/statefulset
// if this is a data deployment, we always want to create separate deployment per replica
// so we'll return 1. if this is not a data node, we can simply scale existing replica.
func (cfg *elasticsearchNode) getReplicas() int32 {
	if cfg.isNodeData() {
		return 1
	}
	return cfg.ESNodeSpec.Replicas
}

func (cfg *elasticsearchNode) isNodeMaster() bool {
	for _, role := range cfg.Roles {
		if role == "master" {
			return true
		}
	}
	return false
}

func (cfg *elasticsearchNode) isNodeData() bool {
	for _, role := range cfg.Roles {
		if role == "data" {
			return true
		}
	}
	return false
}

func (cfg *elasticsearchNode) isNodeClient() bool {
	for _, role := range cfg.Roles {
		if role == "client" {
			return true
		}
	}
	return false
}

func (cfg *elasticsearchNode) getLabels() map[string]string {
	return map[string]string{
		"component": fmt.Sprintf("elasticsearch-%s", cfg.ClusterName),
		//"es-node-role":   cfg.NodeType,
		"es-node-client": strconv.FormatBool(cfg.isNodeClient()),
		"es-node-data":   strconv.FormatBool(cfg.isNodeData()),
		"es-node-master": strconv.FormatBool(cfg.isNodeMaster()),
		"cluster":        cfg.ClusterName,
	}
}

func (cfg *elasticsearchNode) getNode() NodeTypeInterface {
	if cfg.isNodeData() {
		return NewDeploymentNode(cfg.DeployName, cfg.Namespace)
	}
	return NewStatefulSetNode(cfg.DeployName, cfg.Namespace)
}

func (cfg *elasticsearchNode) CreateOrUpdateNode(owner metav1.OwnerReference) error {
	node := cfg.getNode()
	err := node.query()
	if err != nil {
		// Node's resource doesn't exist, we can construct one
		logrus.Infof("Constructing new resource %v", cfg.DeployName)
		dep, err := node.constructNodeResource(cfg, owner)
		if err != nil {
			return fmt.Errorf("Could not construct node resource: %v", err)
		}
		err = sdk.Create(dep)
		if err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("Could not create node resource: %v", err)
		}
		return nil
	}

	// TODO: what is allowed to be changed in the StatefulSet ?
	// Validate Elasticsearch cluster parameters
	diff, err := node.isDifferent(cfg)
	if err != nil {
		return fmt.Errorf("Failed to see if the node resource is different from what's needed: %v", err)
	}

	if diff {
		dep, err := node.constructNodeResource(cfg, metav1.OwnerReference{})
		if err != nil {
			return fmt.Errorf("Could not construct node resource for update: %v", err)
		}
		logrus.Infof("Updating node resource %v", cfg.DeployName)
		err = sdk.Update(dep)
		if err != nil {
			return fmt.Errorf("Failed to update node resource: %v", err)
		}
	}
	return nil
}

func (cfg *elasticsearchNode) IsUpdateNeeded() bool {
	// FIXME: to be refactored. query() must not exist here, since
	// we already have information in clusterState
	node := cfg.getNode()
	err := node.query()
	if err != nil {
		// resource doesn't exist, so the update is needed
		return true
	}

	diff, err := node.isDifferent(cfg)
	if err != nil {
		logrus.Errorf("Failed to obtain if there is a significant difference in resources: %v", err)
		return false
	}

	if diff {
		return true
	}
	return false
}
