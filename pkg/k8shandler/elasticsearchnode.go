package k8shandler

import (
	"fmt"

	"github.com/operator-framework/operator-sdk/pkg/sdk/action"
	"github.com/sirupsen/logrus"
	v1alpha1 "github.com/t0ffel/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type elasticsearchNode struct {
	ClusterName         string
	Namespace           string
	DeployName          string
	NodeType            string
	ESNodeSpec          v1alpha1.ElasticsearchNode
	ElasticsearchSecure v1alpha1.ElasticsearchSecure
	NodeNum             int32
	ReplicaNum          int32
}

func constructNodeConfig(dpl *v1alpha1.Elasticsearch, esNode v1alpha1.ElasticsearchNode, nodeNum int32, replicaNum int32) (elasticsearchNode, error) {
	nodeCfg := elasticsearchNode{}
	nodeCfg.NodeNum = nodeNum
	nodeCfg.ReplicaNum = replicaNum
	nodeCfg.DeployName = fmt.Sprintf("%s-%s-%d-%d", dpl.Name, esNode.NodeRole, nodeNum, replicaNum)
	nodeCfg.ClusterName = dpl.Name
	nodeCfg.NodeType = esNode.NodeRole
	nodeCfg.ESNodeSpec = esNode
	nodeCfg.ElasticsearchSecure = dpl.Spec.Secure
	nodeCfg.ESNodeSpec.Config = dpl.Spec.Config
    nodeCfg.Namespace = dpl.Namespace
	return nodeCfg, nil
}

// getReplicas returns the desired number of replicas in the deployment/statefulset
// if this is a data deployment, we always want to create separate deployment per replica
// so we'll return 1. if this is not a data node, we can simply scale existing replica.
func (cfg *elasticsearchNode) getReplicas() int32 {
	if cfg.isNodeData() == "true" {
		return 1
	}
	return cfg.ESNodeSpec.Replicas
}

func (cfg *elasticsearchNode) isNodeMaster() string {
	if cfg.NodeType == "clientdatamaster" || cfg.NodeType == "master" {
		return "true"
	}
	return "false"
}

func (cfg *elasticsearchNode) isNodeData() string {
	if cfg.NodeType == "clientdatamaster" || cfg.NodeType == "clientdata" || cfg.NodeType == "data" {
		return "true"
	}
	return "false"
}

func (cfg *elasticsearchNode) isNodeClient() string {
	if cfg.NodeType == "clientdatamaster" || cfg.NodeType == "clientdata" || cfg.NodeType == "client" {
		return "true"
	}
	return "false"
}

func (cfg *elasticsearchNode) getLabels() map[string]string {
	return map[string]string{
		"component":      fmt.Sprintf("elasticsearch-%s", cfg.ClusterName),
		"es-node-role":   cfg.NodeType,
		"es-node-client": cfg.isNodeClient(),
		"es-node-data":   cfg.isNodeData(),
		"es-node-master": cfg.isNodeMaster(),
		"cluster":        cfg.ClusterName,
	}
}

func (cfg *elasticsearchNode) getNode() NodeTypeInterface {
	if cfg.isNodeData() == "true" {
		return NewDeploymentNode(cfg.DeployName, cfg.Namespace)
	}
	return NewStatefulSetNode(cfg.DeployName, cfg.Namespace)
}

func (cfg *elasticsearchNode) CreateOrUpdateNode(dpl *v1alpha1.Elasticsearch) error {
	node := cfg.getNode()
	err := node.query()
	if err != nil {
		// Node's resource doesn't exist, we can construct one
		logrus.Infof("Constructing new resource %v", cfg.DeployName)
		dep, err := node.constructNodeResource(cfg, asOwner(dpl))
		if err != nil {
			return fmt.Errorf("Could not construct node resource: %v", err)
		}
		err = action.Create(dep)
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
		err = action.Update(dep)
		if err != nil {
			return fmt.Errorf("Failed to update node resource: %v", err)
		}
	}
	return nil
}

func (cfg *elasticsearchNode) IsUpdateNeeded(dpl *v1alpha1.Elasticsearch) bool {
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
