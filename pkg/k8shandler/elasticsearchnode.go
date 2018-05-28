package k8shandler

import (
	"fmt"

	"github.com/operator-framework/operator-sdk/pkg/sdk/action"
	"github.com/sirupsen/logrus"
	v1alpha1 "github.com/t0ffel/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
)

type elasticsearchNode struct {
	ClusterName string
	DeployName  string
	NodeType    string
	// StorageType		string
	ESNodeSpec v1alpha1.ElasticsearchNode
}

func constructNodeConfig(dpl *v1alpha1.Elasticsearch, esNode v1alpha1.ElasticsearchNode) (elasticsearchNode, error) {
	nodeCfg := elasticsearchNode{}
	nodeCfg.DeployName = fmt.Sprintf("%s-%s", dpl.Name, esNode.NodeRole)
	nodeCfg.ClusterName = dpl.Name
	nodeCfg.NodeType = esNode.NodeRole
	nodeCfg.ESNodeSpec = esNode

	return nodeCfg, nil
}

func (cfg *elasticsearchNode) getReplicas() int32 {
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

func (cfg *elasticsearchNode) CreateOrUpdateNode(dpl *v1alpha1.Elasticsearch) error {
	var node NodeTypeInterface
	if cfg.isNodeData() == "true" {
		node = NewDeploymentNode(cfg.DeployName, dpl.Namespace)
	} else {
		node = NewStatefulSetNode(cfg.DeployName, dpl.Namespace)
	}
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
		dep, err := node.constructNodeResource(cfg, asOwner(dpl))
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
