package k8shandler

import (
	"fmt"

	v1alpha1 "github.com/t0ffel/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	//		appsv1 "k8s.io/api/apps/v1"
	"github.com/sirupsen/logrus"

	//		"k8s.io/api/core/v1"
	"github.com/operator-framework/operator-sdk/pkg/k8sclient"
	"github.com/operator-framework/operator-sdk/pkg/sdk/action"
	"github.com/operator-framework/operator-sdk/pkg/sdk/query"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// newesDeploy demonstrates how to create a Elasticsearch deployment
func createOrUpdateElasticsearchCluster(dpl *v1alpha1.Elasticsearch) error {
	for _, node := range dpl.Spec.Nodes {
		role := node.NodeRole
		statefulSetName := fmt.Sprintf("%s-%s", dpl.Name, role)

		nodeCfg, err := constructNodeConfig(dpl, node)
		if err != nil {
			return fmt.Errorf("Unable to construct ES node config %v", err)
		}

		existingSSet := statefulSet(statefulSetName, dpl.Namespace)
		err = query.Get(existingSSet)
		if err != nil {
			// StatefulSet doesn't exist, we can construct one
			logrus.Infof("Constructing new StatefulSet %v", statefulSetName)
			dep, err := nodeCfg.constructNodeStatefulSet(dpl.Namespace)
			if err != nil {
				return fmt.Errorf("Could not construct StatefulSet: %v", err)
			}
			addOwnerRefToObject(dep, asOwner(dpl))
			err = action.Create(dep)
			if err != nil && !errors.IsAlreadyExists(err) {
				return fmt.Errorf("Could not create StatefulSet: %v", err)
			}
			return nil
		}

		// TODO: what is allowed to be changed in the StatefulSet ?
		// Validate Elasticsearch cluster parameters
		diff, err := nodeCfg.isDifferent(existingSSet)
		if err != nil {
			return fmt.Errorf("Failed to see if StatefulSet is different from what's needed: %v", err)
		}

		if diff {
			dep, err := nodeCfg.constructNodeStatefulSet(dpl.Namespace)
			if err != nil {
				return fmt.Errorf("Could not construct StatefulSet for update: %v", err)
			}
			addOwnerRefToObject(dep, asOwner(dpl))
			logrus.Infof("Updating StatefulSet %v", statefulSetName)
			err = action.Update(dep)
			if err != nil {
				return fmt.Errorf("Failed to update StatefulSet: %v", err)
			}
		}
	}

	// list existing StatefulSets and delete those unneeded
	sSetList, err := listStatefulSets(dpl)
	if err != nil {
		return fmt.Errorf("Unable to list Elasticsearch's StatefulSets: %v", err)
	}
	for _, sSet := range sSetList.Items {
		//logrus.Infof("found statefulset: %v", sSet.ObjectMeta.Name)
		if !isNodeConfigured(sSet, dpl) {
			resClient, _, err := k8sclient.GetResourceClient("apps/v1", "StatefulSet", dpl.Namespace)
			if err != nil {
				return fmt.Errorf("failed to get resource client: %v", err)
			}
			err = resClient.Delete(sSet.ObjectMeta.Name, &metav1.DeleteOptions{})
			if err != nil {
				return fmt.Errorf("Unable to delete StatefulSet %v: ", err)
			}
		}
	}
	return nil
}
