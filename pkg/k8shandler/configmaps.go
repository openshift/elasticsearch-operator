package k8shandler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"html/template"
	"io"
	"runtime"
	"strconv"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	esConfig            = "elasticsearch.yml"
	log4jConfig         = "log4j2.properties"
	indexSettingsConfig = "index_settings"
)

// esYmlStruct is used to render esYmlTmpl to a proper elasticsearch.yml format
type esYmlStruct struct {
	KibanaIndexMode      string
	EsUnicastHost        string
	NodeQuorum           string
	RecoverExpectedNodes string
	SystemCallFilter     string
}

type log4j2PropertiesStruct struct {
	RootLogger       string
	LogLevel         string
	SecurityLogLevel string
}

type indexSettingsStruct struct {
	PrimaryShards string
	ReplicaShards string
}

// CreateOrUpdateConfigMap reconciles a configmap
func (er *ElasticsearchRequest) CreateOrUpdateConfigMap(cm *v1.ConfigMap) error {
	err := er.client.Create(context.TODO(), cm)
	if err == nil {
		return nil
	}
	if !apierrors.IsAlreadyExists(kverrors.Root(err)) {
		return kverrors.Wrap(err, "failed to construct configmap",
			"name", cm.Name,
			"namespace", cm.Namespace)
	}

	// Get existing configMap to check if it is same as what we want
	current := cm.DeepCopy()
	err = er.client.Get(context.TODO(), types.NamespacedName{Name: current.Name, Namespace: current.Namespace}, current)
	if err != nil {
		return kverrors.Wrap(err, "failed to update configmap",
			"name", cm.Name,
			"namespace", cm.Namespace)
	}

	if configMapContentChanged(current, cm) {
		// Cluster settings has changed, make sure it doesnt go unnoticed
		if err := updateConditionWithRetry(er.cluster, v1.ConditionTrue, updateUpdatingSettingsCondition, er.client); err != nil {
			return err
		}

		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := er.client.Get(context.TODO(), types.NamespacedName{Name: current.Name, Namespace: current.Namespace}, current); err != nil {
				log.Info("Could not get configmap, retrying...", "configmap", cm.Name, "error", err)
				return err
			}

			current.Data = cm.Data
			if err := er.client.Update(context.TODO(), current); err != nil {
				log.Error(err, "Failed to update configmap, retrying...", "configmap", cm.Name)
				return err
			}
			return nil
		})
		return kverrors.Wrap(err, "failed to update configmap",
			"name", cm.Name,
			"namespace", cm.Namespace)
	} else {
		if err := updateConditionWithRetry(er.cluster, v1.ConditionFalse, updateUpdatingSettingsCondition, er.client); err != nil {
			return err
		}
	}

	return nil
}

// CreateOrUpdateConfigMaps ensures the existence of ConfigMaps with Elasticsearch configuration
func (er *ElasticsearchRequest) CreateOrUpdateConfigMaps() (err error) {
	dpl := er.cluster

	kibanaIndexMode, err := kibanaIndexMode("")
	if err != nil {
		return err
	}
	dataNodeCount := int(getDataCount(dpl))
	masterNodeCount := int(getMasterCount(dpl))

	logConfig := getLogConfig(dpl.GetAnnotations())

	configmap := newConfigMap(
		dpl.Name,
		dpl.Namespace,
		dpl.Labels,
		kibanaIndexMode,
		esUnicastHost(dpl.Name, dpl.Namespace),
		strconv.Itoa(masterNodeCount/2+1),
		strconv.Itoa(dataNodeCount),
		strconv.Itoa(calculatePrimaryCount(dpl)),
		strconv.Itoa(calculateReplicaCount(dpl)),
		strconv.FormatBool(runtime.GOARCH == "amd64"),
		logConfig,
	)

	dpl.AddOwnerRefTo(configmap)

	err = er.client.Create(context.TODO(), configmap)
	if err == nil {
		return nil
	}
	if !apierrors.IsAlreadyExists(kverrors.Root(err)) {
		return kverrors.Wrap(err, "failed to construct elasticsearch configmap",
			"name", configmap.Name,
			"namespace", configmap.Namespace,
			"cluster", configmap.ClusterName)
	}

	// Get existing configMap to check if it is same as what we want
	current := configmap.DeepCopy()
	err = er.client.Get(context.TODO(), types.NamespacedName{Name: current.Name, Namespace: current.Namespace}, current)
	if err != nil {
		return kverrors.Wrap(err, "failed to get Elasticsearch cluster configMap",
			"name", current.Name,
			"namespace", current.Namespace,
			"cluster", current.ClusterName)
	}

	if configMapContentChanged(current, configmap) {
		// Cluster settings has changed, make sure it doesnt go unnoticed
		if err := updateConditionWithRetry(dpl, v1.ConditionTrue, updateUpdatingSettingsCondition, er.client); err != nil {
			return err
		}

		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := er.client.Get(context.TODO(), types.NamespacedName{Name: current.Name, Namespace: current.Namespace}, current); err != nil {
				log.Error(err, "Could not get Elasticsearch configmap", configmap.Name)
				return err
			}

			current.Data = configmap.Data
			if err := er.client.Update(context.TODO(), current); err != nil {
				log.Error(err, "Failed to update Elasticsearch configmap", configmap.Name)
				return err
			}
			return nil
		})
		return kverrors.Wrap(err, "failed to update configmap",
			"name", configmap.Name,
			"namespace", configmap.Namespace,
			"cluster", configmap.ClusterName)
	} else {
		if err := updateConditionWithRetry(dpl, v1.ConditionFalse, updateUpdatingSettingsCondition, er.client); err != nil {
			return err
		}
	}

	return nil
}

func renderData(kibanaIndexMode, esUnicastHost, nodeQuorum, recoverExpectedNodes, primaryShardsCount, replicaShardsCount, systemCallFilter string, logConfig LogConfig) (map[string]string, error) {
	data := map[string]string{}
	buf := &bytes.Buffer{}
	if err := renderEsYml(buf, kibanaIndexMode, esUnicastHost, nodeQuorum, recoverExpectedNodes, systemCallFilter); err != nil {
		return data, err
	}
	data[esConfig] = buf.String()

	buf = &bytes.Buffer{}
	if err := renderLog4j2Properties(buf, logConfig); err != nil {
		return data, err
	}
	data[log4jConfig] = buf.String()

	buf = &bytes.Buffer{}
	if err := renderIndexSettings(buf, primaryShardsCount, replicaShardsCount); err != nil {
		return data, err
	}
	data[indexSettingsConfig] = buf.String()

	return data, nil
}

// newConfigMap returns a v1.ConfigMap object
func newConfigMap(configMapName, namespace string, labels map[string]string,
	kibanaIndexMode, esUnicastHost, nodeQuorum, recoverExpectedNodes, primaryShardsCount, replicaShardsCount, systemCallFilter string, logConfig LogConfig) *v1.ConfigMap {
	data, err := renderData(kibanaIndexMode, esUnicastHost, nodeQuorum, recoverExpectedNodes, primaryShardsCount, replicaShardsCount, systemCallFilter, logConfig)
	if err != nil {
		return nil
	}

	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: v1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: data,
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

func renderEsYml(w io.Writer, kibanaIndexMode, esUnicastHost, nodeQuorum, recoverExpectedNodes, systemCallFilter string) error {
	t := template.New("elasticsearch.yml")
	config := esYmlTmpl
	t, err := t.Parse(config)
	if err != nil {
		return err
	}
	esy := esYmlStruct{
		KibanaIndexMode:      kibanaIndexMode,
		EsUnicastHost:        esUnicastHost,
		NodeQuorum:           nodeQuorum,
		RecoverExpectedNodes: recoverExpectedNodes,
		SystemCallFilter:     systemCallFilter,
	}

	return t.Execute(w, esy)
}

func renderLog4j2Properties(w io.Writer, logConfig LogConfig) error {
	t := template.New("log4j2.properties")
	t, err := t.Parse(log4j2PropertiesTmpl)
	if err != nil {
		return err
	}

	log4jProp := log4j2PropertiesStruct{
		RootLogger:       logConfig.ServerAppender,
		LogLevel:         logConfig.ServerLoglevel,
		SecurityLogLevel: logConfig.LogLevel,
	}

	return t.Execute(w, log4jProp)
}

func renderIndexSettings(w io.Writer, primaryShardsCount, replicaShardsCount string) error {
	t := template.New("index_settings")
	t, err := t.Parse(indexSettingsTmpl)
	if err != nil {
		return err
	}

	indexSettings := indexSettingsStruct{
		PrimaryShards: primaryShardsCount,
		ReplicaShards: replicaShardsCount,
	}

	return t.Execute(w, indexSettings)
}

func getConfigmap(configmapName, namespace string, client client.Client) *v1.ConfigMap {
	configMap := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: v1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmapName,
			Namespace: namespace,
		},
	}

	err := client.Get(context.TODO(), types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, &configMap)
	if err != nil {
		// check if doesn't exist
	}

	return &configMap
}

func getConfigmapDataHash(configmapName, namespace string, client client.Client) string {
	hash := ""

	configMap := getConfigmap(configmapName, namespace, client)

	dataHashes := make(map[string][32]byte)

	for key, data := range configMap.Data {
		if key != "index_settings" {
			dataHashes[key] = sha256.Sum256([]byte(data))
		}
	}

	sortedKeys := sortDataHashKeys(dataHashes)

	for _, key := range sortedKeys {
		hash = fmt.Sprintf("%s%s", hash, dataHashes[key])
	}

	return hash
}
