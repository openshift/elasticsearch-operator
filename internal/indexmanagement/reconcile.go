package indexmanagement

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/go-logr/logr"
	batch "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ViaQ/logerr/log"
	apis "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/internal/constants"
	"github.com/openshift/elasticsearch-operator/internal/elasticsearch"
	"github.com/openshift/elasticsearch-operator/internal/elasticsearch/esclient"
	"github.com/openshift/elasticsearch-operator/internal/manifests/configmap"
	"github.com/openshift/elasticsearch-operator/internal/manifests/cronjob"
	"github.com/openshift/elasticsearch-operator/internal/manifests/pod"
	"github.com/openshift/elasticsearch-operator/internal/manifests/rbac"
	esapi "github.com/openshift/elasticsearch-operator/internal/types/elasticsearch"
	"github.com/openshift/elasticsearch-operator/internal/utils/comparators"
)

const (
	indexManagementConfigmap = "indexmanagement-scripts"
	defaultShardSize         = int32(40)
	workingDir               = "/tmp/scripts"

	jobHistoryLimitFailed  int32 = 1
	jobHistoryLimitSuccess int32 = 1
)

var (
	defaultCPURequest    = resource.MustParse("100m")
	defaultMemoryRequest = resource.MustParse("32Mi")

	millisPerSecond = uint64(1000)
	millisPerMinute = uint64(60 * millisPerSecond)
	millisPerHour   = uint64(millisPerMinute * 60)
	millisPerDay    = uint64(millisPerHour * 24)
	millisPerWeek   = uint64(millisPerDay * 7)

	// fullExecMode octal 0777
	fullExecMode int32 = 0o777

	imLabels = map[string]string{
		"provider":      "openshift",
		"component":     "indexManagement",
		"logging-infra": "indexManagement",
	}
)

type rolloverConditions struct {
	MaxAge  string `json:"max_age,omitempty"`
	MaxDocs int32  `json:"max_docs,omitempty"`
	MaxSize string `json:"max_size,omitempty"`
}

type IndexManagementRequest struct {
	client   client.Client
	cluster  *apis.Elasticsearch
	esClient esclient.Client
	ll       logr.Logger
}

func Reconcile(req *apis.Elasticsearch, reqClient client.Client) error {
	esClient := esclient.NewClient(req.Name, req.Namespace, reqClient)

	imr := IndexManagementRequest{
		client:   reqClient,
		esClient: esClient,
		cluster:  req,
		ll:       log.WithValues("cluster", req.Name, "namespace", req.Namespace, "handler", "indexmanagement"),
	}

	return imr.createOrUpdateIndexManagement()
}

func (imr *IndexManagementRequest) createOrUpdateIndexManagement() error {
	if imr.cluster.Spec.IndexManagement == nil {
		return nil
	}
	spec := verifyAndNormalize(imr.cluster)
	policies := spec.PolicyMap()

	imr.cullIndexManagement(spec.Mappings, policies)
	for _, mapping := range spec.Mappings {
		ll := log.WithValues("mapping", mapping.Name)
		// create or update template
		if err := imr.createOrUpdateIndexTemplate(mapping); err != nil {
			ll.Error(err, "failed to create index template")
			return err
		}
		// TODO: Can we have partial success?
		if err := imr.initializeIndexIfNeeded(mapping); err != nil {
			ll.Error(err, "Failed to initialize index")
			return err
		}
	}

	if err := createOrUpdateCurationConfigmap(imr.client, imr.cluster); err != nil {
		return err
	}

	if err := imr.reconcileIndexManagmentRbac(); err != nil {
		return err
	}

	primaryShards := elasticsearch.GetDataCount(imr.cluster)
	for _, mapping := range spec.Mappings {
		policy := policies[mapping.PolicyRef]
		ll := log.WithValues("mapping", mapping.Name, "policy", policy.Name)
		if err := imr.reconcileIndexManagementCronjob(policy, mapping, primaryShards); err != nil {
			ll.Error(err, "could not reconcile indexmanagement cronjob")
			return err
		}
	}

	return nil
}

func (imr *IndexManagementRequest) cullIndexManagement(mappings []apis.IndexManagementPolicyMappingSpec, policies apis.PolicyMap) {
	if err := imr.removeCronJobsForMappings(mappings, policies); err != nil {
		log.Error(err, "Unable to cull cronjobs")
	}
	mappingNames := sets.NewString()
	for _, mapping := range mappings {
		mappingNames.Insert(formatTemplateName(mapping.Name))
	}

	existing, err := imr.esClient.ListTemplates()
	if err != nil {
		log.Error(err, "Unable to list existing templates in order to reconcile stale ones")
		return
	}
	difference := existing.Difference(mappingNames)

	for _, template := range difference.List() {
		if strings.HasPrefix(template, constants.OcpTemplatePrefix) {
			if err := imr.esClient.DeleteIndexTemplate(template); err != nil {
				log.Error(err, "Unable to delete stale template in order to reconcile", "template", template)
			}
		}
	}
}

func (imr *IndexManagementRequest) initializeIndexIfNeeded(mapping apis.IndexManagementPolicyMappingSpec) error {
	pattern := formatWriteAlias(mapping)
	indices, err := imr.esClient.ListIndicesForAlias(pattern)
	if err != nil {
		return err
	}
	if len(indices) < 1 {
		indexName := fmt.Sprintf("%s-000001", mapping.Name)
		primaryShards := int32(elasticsearch.CalculatePrimaryCount(imr.cluster))
		replicas := int32(elasticsearch.CalculateReplicaCount(imr.cluster))
		index := esapi.NewIndex(indexName, primaryShards, replicas)
		index.AddAlias(mapping.Name, false)
		index.AddAlias(pattern, true)
		for _, alias := range mapping.Aliases {
			index.AddAlias(alias, false)
		}
		return imr.esClient.CreateIndex(indexName, index)
	}
	return nil
}

func formatTemplateName(name string) string {
	return fmt.Sprintf("%s-%s", constants.OcpTemplatePrefix, name)
}

func formatWriteAlias(mapping apis.IndexManagementPolicyMappingSpec) string {
	return fmt.Sprintf("%s-write", mapping.Name)
}

func (imr *IndexManagementRequest) createOrUpdateIndexTemplate(mapping apis.IndexManagementPolicyMappingSpec) error {
	name := formatTemplateName(mapping.Name)
	pattern := fmt.Sprintf("%s*", mapping.Name)
	primaryShards := int32(elasticsearch.CalculatePrimaryCount(imr.cluster))
	replicas := int32(elasticsearch.CalculateReplicaCount(imr.cluster))
	aliases := append(mapping.Aliases, mapping.Name)
	template := esapi.NewIndexTemplate(pattern, aliases, primaryShards, replicas)

	// check to compare the current index templates vs what we just generated
	templates, err := imr.esClient.GetIndexTemplates()
	if err != nil {
		return err
	}

	for templateName := range templates {
		if templateName == name {
			return nil
		}
	}

	return imr.esClient.CreateIndexTemplate(name, template)
}

func (imr *IndexManagementRequest) removeCronJobsForMappings(mappings []apis.IndexManagementPolicyMappingSpec, policies apis.PolicyMap) error {
	expected := sets.NewString()
	for _, mapping := range mappings {
		expected.Insert(fmt.Sprintf("%s-im-%s", imr.cluster.Name, mapping.Name))
	}

	cronList, err := cronjob.List(context.TODO(), imr.client, imr.cluster.Namespace, imLabels)
	if err != nil {
		return kverrors.Wrap(err, "failed to list cron jobs",
			"namespace", imr.cluster.Namespace,
			"labels", imLabels,
		)
	}

	existing := sets.NewString()
	for _, cron := range cronList {
		existing.Insert(cron.Name)
	}

	difference := existing.Difference(expected)
	for _, name := range difference.List() {
		key := client.ObjectKey{Name: name, Namespace: imr.cluster.Namespace}
		err := cronjob.Delete(context.TODO(), imr.client, key)
		if err != nil && !apierrors.IsNotFound(err) {
			log.Error(err, "failed to remove cronjob", "namespace", imr.cluster.Namespace, "name", name)
		}
	}
	return nil
}

func createOrUpdateCurationConfigmap(apiclient client.Client, cluster *apis.Elasticsearch) error {
	data := scriptMap
	desired := configmap.New(indexManagementConfigmap, cluster.Namespace, imLabels, data)
	cluster.AddOwnerRefTo(desired)

	_, err := configmap.CreateOrUpdate(context.TODO(), apiclient, desired, configmap.DataEqual, configmap.MutateDataOnly)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update index management configmap",
			"cluster", cluster.Name,
			"namespace", cluster.Namespace,
		)
	}

	return nil
}

func (imr *IndexManagementRequest) reconcileIndexManagmentRbac() error {
	cluster := imr.cluster
	client := imr.client

	role := rbac.NewRole(
		"elasticsearch-index-management",
		cluster.Namespace,
		rbac.NewPolicyRules(
			rbac.NewPolicyRule(
				[]string{"elasticsearch.openshift.io"},
				[]string{"indices"},
				[]string{},
				[]string{"*"},
				[]string{},
			),
		),
	)

	cluster.AddOwnerRefTo(role)

	err := rbac.CreateOrUpdateRole(context.TODO(), client, role)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update index management role",
			"cluster", cluster.Name,
			"namespace", cluster.Namespace,
		)
	}

	subject := rbac.NewSubject(
		"ServiceAccount",
		cluster.Name,
		cluster.Namespace,
	)
	subject.APIGroup = ""
	roleBinding := rbac.NewRoleBinding(
		role.Name,
		role.Namespace,
		role.Name,
		rbac.NewSubjects(subject),
	)
	cluster.AddOwnerRefTo(roleBinding)

	err = rbac.CreateOrUpdateRoleBinding(context.TODO(), client, roleBinding)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update index management rolebinding",
			"cluster", cluster.Name,
			"namespace", cluster.Namespace,
		)
	}

	return nil
}

func (imr *IndexManagementRequest) reconcileIndexManagementCronjob(policy apis.IndexManagementPolicySpec, mapping apis.IndexManagementPolicyMappingSpec, primaryShards int32) error {
	if policy.Phases.Delete == nil && policy.Phases.Hot == nil {
		log.V(1).Info("Skipping indexmanagement cronjob for policymapping; no phases are defined", "policymapping", mapping.Name)
		return nil
	}

	envvars := []corev1.EnvVar{
		{Name: "POLICY_MAPPING", Value: mapping.Name},
	}

	if policy.Phases.Delete != nil {
		minAgeMillis, err := calculateMillisForTimeUnit(policy.Phases.Delete.MinAge)
		if err != nil {
			return err
		}
		envvars = append(envvars,
			corev1.EnvVar{Name: "MIN_AGE", Value: strconv.FormatUint(minAgeMillis, 10)},
		)
	} else {
		log.V(1).Info("Skipping curation management for policymapping; delete phase not defined", "policymapping", mapping.Name)
	}

	if policy.Phases.Hot != nil {
		conditions := calculateConditions(policy, primaryShards)
		payload, err := json.Marshal(map[string]rolloverConditions{"conditions": conditions})
		if err != nil {
			return kverrors.Wrap(err, "failed to serialize the rollover conditions to JSON")
		}
		envvars = append(envvars,
			corev1.EnvVar{Name: "PAYLOAD", Value: base64.StdEncoding.EncodeToString(payload)},
		)

	} else {
		log.V(1).Info("Skipping rollover management for policymapping; hot phase not defined", "policymapping", mapping.Name)
	}

	schedule, err := crontabScheduleFor(policy.PollInterval)
	if err != nil {
		return kverrors.Wrap(err, "failed to reconcile rollover cronjob", "policymapping", mapping.Name)
	}

	name := fmt.Sprintf("%s-im-%s", imr.cluster.Name, mapping.Name)
	script := formatCmd(policy)
	desired := newCronJob(imr.cluster.Name, imr.cluster.Namespace, name, schedule, script, imr.cluster.Spec.Spec.NodeSelector, imr.cluster.Spec.Spec.Tolerations, envvars)

	imr.cluster.AddOwnerRefTo(desired)

	err = cronjob.CreateOrUpdate(context.TODO(), imr.client, desired, areCronJobsSame, cronjob.Mutate)
	if err != nil {
		return kverrors.Wrap(err, "failed to create or update cronjob",
			"cluster", desired.Name,
			"namespace", desired.Namespace,
		)
	}

	return nil
}

func formatCmd(policy apis.IndexManagementPolicySpec) string {
	cmd := []string{}
	result := []string{}
	if policy.Phases.Delete != nil {
		cmd = append(cmd, "./delete", "delete_rc=$?")
		result = append(result, "exit $delete_rc")
	}
	if policy.Phases.Hot != nil {
		cmd = append(cmd, "./rollover", "rollover_rc=$?")
		result = append(result, "exit $rollover_rc")
	}
	if len(cmd) == 0 {
		return ""
	}
	cmd = append(cmd, fmt.Sprintf("$(%s)", strings.Join(result, "&&")))
	script := strings.Join(cmd, ";")
	return script
}

func areCronJobsSame(lhs, rhs *batch.CronJob) bool {
	if len(lhs.Spec.JobTemplate.Spec.Template.Spec.Containers) != len(lhs.Spec.JobTemplate.Spec.Template.Spec.Containers) {
		return false
	}
	if !comparators.AreStringMapsSame(lhs.Spec.JobTemplate.Spec.Template.Spec.NodeSelector, rhs.Spec.JobTemplate.Spec.Template.Spec.NodeSelector) {
		return false
	}

	if !comparators.AreTolerationsSame(lhs.Spec.JobTemplate.Spec.Template.Spec.Tolerations, rhs.Spec.JobTemplate.Spec.Template.Spec.Tolerations) {
		return false
	}
	if lhs.Spec.Schedule != rhs.Spec.Schedule {
		lhs.Spec.Schedule = rhs.Spec.Schedule
		return false
	}
	if lhs.Spec.Suspend != nil && rhs.Spec.Suspend != nil && *lhs.Spec.Suspend != *rhs.Spec.Suspend {
		return false
	}
	for i, container := range lhs.Spec.JobTemplate.Spec.Template.Spec.Containers {
		other := rhs.Spec.JobTemplate.Spec.Template.Spec.Containers[i]
		if !areContainersSame(container, other) {
			return false
		}
	}
	return true
}

func areContainersSame(container, other corev1.Container) bool {
	if container.Name != other.Name {
		return false
	}
	if container.Image != other.Image {
		return false
	}

	if !reflect.DeepEqual(container.Command, other.Command) {
		return false
	}
	if !reflect.DeepEqual(container.Args, other.Args) {
		return false
	}

	if !comparators.AreResourceRequementsSame(container.Resources, other.Resources) {
		return false
	}

	if !comparators.EnvValueEqual(container.Env, other.Env) {
		return false
	}
	return true
}

func newContainer(clusterName, name, image, scriptPath string, envvars []corev1.EnvVar) corev1.Container {
	envvars = append(envvars, corev1.EnvVar{Name: "ES_SERVICE", Value: fmt.Sprintf("https://%s:9200", clusterName)})
	container := corev1.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: defaultMemoryRequest,
				corev1.ResourceCPU:    defaultCPURequest,
			},
		},
		WorkingDir: workingDir,
		Env:        envvars,
		Command:    []string{"bash"},
		Args: []string{
			"-c",
			scriptPath,
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "certs", ReadOnly: true, MountPath: "/etc/indexmanagement/keys"},
			{Name: "scripts", ReadOnly: false, MountPath: workingDir},
		},
	}

	return container
}

func newCronJob(clusterName, namespace, name, schedule, script string, nodeSelector map[string]string, tolerations []corev1.Toleration, envvars []corev1.EnvVar) *batch.CronJob {
	containerName := "indexmanagement"
	containers := []corev1.Container{
		newContainer(clusterName, containerName, constants.PackagedElasticsearchImage(), script, envvars),
	}
	volumes := []corev1.Volume{
		{
			Name: "certs",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: clusterName,
				},
			},
		},
		{
			Name: "scripts",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: indexManagementConfigmap,
					},
					DefaultMode: &fullExecMode,
				},
			},
		},
	}

	podSpec := pod.NewSpec(clusterName, containers, volumes).
		WithNodeSelectors(nodeSelector).
		WithTolerations(tolerations...).
		WithRestartPolicy(corev1.RestartPolicyNever).
		WithRestartPolicy(corev1.RestartPolicyNever).
		WithTerminationGracePeriodSeconds(300 * time.Second).
		Build()

	return cronjob.New(name, namespace, imLabels).
		WithConcurrencyPolicy(batch.ForbidConcurrent).
		WithSuccessfulJobsHistoryLimit(jobHistoryLimitSuccess).
		WithFailedJobsHistoryLimit(jobHistoryLimitFailed).
		WithSchedule(schedule).
		WithBackoffLimit(0).
		WithParallelism(1).
		WithPodSpec(containerName, podSpec).
		Build()
}
