package k8shandler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1alpha1"
	"github.com/openshift/elasticsearch-operator/pkg/utils"
	"github.com/sirupsen/logrus"
)

const (
	esConfig            = "elasticsearch.yml"
	log4jConfig         = "log4j2.properties"
	indexSettingsConfig = "index_settings"
)

// CreateOrUpdateConfigMaps ensures the existence of ConfigMaps with Elasticsearch configuration
func CreateOrUpdateConfigMaps(client client.Client, dpl *v1alpha1.Elasticsearch) (string, error) {
	owner := asOwner(dpl)
	configMapName := v1alpha1.ConfigMapName

	kibanaIndexMode, err := kibanaIndexMode("")
	if err != nil {
		return "", err
	}
	dataNodeCount := int((getDataCount(dpl)))
	masterNodeCount := int((getMasterCount(dpl)))

	primaryShardsCount := strconv.Itoa(dataNodeCount)
	replicaShardsCount := strconv.Itoa(calculateReplicaCount(dpl))
	recoverExpectedShards := strconv.Itoa(dataNodeCount)
	nodeQuorum := strconv.Itoa(masterNodeCount/2 + 1)

	esUnicastHost := esUnicastHost(dpl.Name)
	rootLogger := rootLogger()

	err = createOrUpdateConfigMap(client, dpl, configMapName, dpl.Namespace, dpl.Name, kibanaIndexMode, esUnicastHost, rootLogger, nodeQuorum, recoverExpectedShards, primaryShardsCount, replicaShardsCount, owner, dpl.Labels)
	if err != nil {
		return configMapName, fmt.Errorf("Failure creating ConfigMap %v", err)
	}
	return configMapName, nil
}

func createOrUpdateConfigMap(client client.Client, dpl *v1alpha1.Elasticsearch, configMapName, namespace, clusterName, kibanaIndexMode, esUnicastHost, rootLogger, nodeQuorum, recoverExpectedShards, primaryShardsCount, replicaShardsCount string,
	owner metav1.OwnerReference, labels map[string]string) error {
	elasticsearchCM, err := createConfigMap(configMapName, namespace, clusterName, kibanaIndexMode, esUnicastHost, rootLogger, nodeQuorum, recoverExpectedShards, primaryShardsCount, replicaShardsCount, labels)
	if err != nil {
		return err
	}
	addOwnerRefToObject(elasticsearchCM, owner)
	err = client.Create(context.TODO(), elasticsearchCM)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("Failure constructing Elasticsearch ConfigMap: %v", err)
	} else if errors.IsAlreadyExists(err) {
		// Get existing configMap to check if it is same as what we want
		existingCM := &v1.ConfigMap{}
		err = client.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: namespace}, existingCM)
		if err != nil {
			return fmt.Errorf("Unable to get Elasticsearch cluster configMap: %v", err)
		}

		if configMapContentChanged(existingCM, elasticsearchCM) {
			// Cluster settings has changed, make sure it doesnt go unnoticed
			if err := utils.UpdateConditionWithRetry(client, dpl, v1alpha1.ConditionTrue, utils.UpdateUpdatingSettingsCondition); err != nil {
				return fmt.Errorf("Unable to update Elasticsearch cluster status: %v", err)
			}

			return retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if getErr := client.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: namespace}, existingCM); getErr != nil {
					logrus.Debugf("Could not get Elasticsearch %v: %v", dpl.Name, getErr)
					return getErr
				}

				existingCM.Data[esConfig] = elasticsearchCM.Data[esConfig]
				existingCM.Data[log4jConfig] = elasticsearchCM.Data[log4jConfig]
				existingCM.Data[indexSettingsConfig] = elasticsearchCM.Data[indexSettingsConfig]

				if updateErr := client.Update(context.TODO(), existingCM); updateErr != nil {
					logrus.Debugf("Failed to update Elasticsearch %v status: %v", dpl.Name, updateErr)
					return updateErr
				}
				return nil
			})
		}
	}
	return nil
}

func createConfigMap(configMapName, namespace, clusterName, kibanaIndexMode, esUnicastHost, rootLogger, nodeQuorum, recoverExpectedShards, primaryShardsCount, replicaShardsCount string,
	labels map[string]string) (*v1.ConfigMap, error) {
	cm := configMap(configMapName, namespace, labels)
	cm.Data = map[string]string{}
	buf := &bytes.Buffer{}
	if err := renderEsYml(buf, kibanaIndexMode, esUnicastHost, nodeQuorum, recoverExpectedShards); err != nil {
		return cm, err
	}
	cm.Data[esConfig] = buf.String()

	buf = &bytes.Buffer{}
	if err := renderLog4j2Properties(buf, rootLogger); err != nil {
		return cm, err
	}
	cm.Data[log4jConfig] = buf.String()

	buf = &bytes.Buffer{}
	if err := renderIndexSettings(buf, primaryShardsCount, replicaShardsCount); err != nil {
		return cm, err
	}
	cm.Data[indexSettingsConfig] = buf.String()

	return cm, nil
}

// configMap returns a v1.ConfigMap object
func configMap(configMapName string, namespace string, labels map[string]string) *v1.ConfigMap {
	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
			Labels:    labels,
		},
	}
}

func configMapContentChanged(old, new *v1.ConfigMap) bool {
	oldEsConfigSum := sha256.Sum256([]byte(old.Data[esConfig]))
	newEsConfigSum := sha256.Sum256([]byte(new.Data[esConfig]))

	if oldEsConfigSum != newEsConfigSum {
		return true
	}

	oldLog4jConfig := sha256.Sum256([]byte(old.Data[log4jConfig]))
	newLog4jConfig := sha256.Sum256([]byte(new.Data[log4jConfig]))

	if oldLog4jConfig != newLog4jConfig {
		return true
	}

	oldIndexSettingsConfig := sha256.Sum256([]byte(old.Data[indexSettingsConfig]))
	newIndexSettingsConfig := sha256.Sum256([]byte(new.Data[indexSettingsConfig]))

	if oldIndexSettingsConfig != newIndexSettingsConfig {
		return true
	}

	return false
}
