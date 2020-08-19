package k8shandler

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	logging "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/pkg/indexmanagement"
	"github.com/openshift/elasticsearch-operator/pkg/log"
	esapi "github.com/openshift/elasticsearch-operator/pkg/types/elasticsearch"
)

const (
	//ocpTemplatePrefix is the prefix all operator generated templates
	ocpTemplatePrefix = "ocp-gen"
)

func (er *ElasticsearchRequest) CreateOrUpdateIndexManagement() error {

	cluster := er.cluster
	if cluster.Spec.IndexManagement == nil {
		return nil
	}
	spec := indexmanagement.VerifyAndNormalize(cluster)
	policies := spec.PolicyMap()
	if er.AnyNodeReady() {
		er.cullIndexManagement(spec.Mappings, policies)

		for _, mapping := range spec.Mappings {
			ll := log.WithValues("mapping", mapping.Name)
			ll.Info("reconciling index management")
			// create or update template
			if err := er.createOrUpdateIndexTemplate(mapping); err != nil {
				ll.Error(err, "failed to create index template")
				return err
			}
			// TODO: Can we have partial success?
			if err := er.initializeIndexIfNeeded(mapping); err != nil {
				ll.Error(err, "Failed to initialize index")
				return err
			}
		}
	}

	if err := indexmanagement.ReconcileCurationConfigmap(er.client, er.cluster); err != nil {
		return err
	}
	primaryShards := getDataCount(er.cluster)
	for _, mapping := range spec.Mappings {
		policy := policies[mapping.PolicyRef]
		ll := log.WithValues("mapping", mapping.Name, "policy", policy.Name)
		if err := indexmanagement.ReconcileRolloverCronjob(er.client, er.cluster, policy, mapping, primaryShards); err != nil {
			ll.Error(err, "could not reconcile rollover cronjob")
			return err
		}
		if err := indexmanagement.ReconcileCurationCronjob(er.client, er.cluster, policy, mapping, primaryShards); err != nil {
			ll.Error(err, "could not reconcile curation cronjob")
			return err
		}
	}

	return nil
}

func (er *ElasticsearchRequest) cullIndexManagement(mappings []logging.IndexManagementPolicyMappingSpec, policies logging.PolicyMap) {
	cluster := er.cluster
	client := er.client
	esClient := er.esClient

	if err := indexmanagement.RemoveCronJobsForMappings(client, cluster, mappings, policies); err != nil {
		log.Error(err, "Unable to cull cronjobs")
	}
	mappingNames := sets.NewString()
	for _, mapping := range mappings {
		mappingNames.Insert(formatTemplateName(mapping.Name))
	}

	existing, err := esClient.ListTemplates()
	if err != nil {
		log.Error(err, "Unable to list existing templates in order to reconcile stale ones")
		return
	}
	difference := existing.Difference(mappingNames)

	for _, template := range difference.List() {
		if strings.HasPrefix(template, ocpTemplatePrefix) {
			if err := esClient.DeleteIndexTemplate(template); err != nil {
				log.Error(err, "Unable to delete stale template in order to reconcile", "template", template)
			}
		}
	}
}
func (er *ElasticsearchRequest) initializeIndexIfNeeded(mapping logging.IndexManagementPolicyMappingSpec) error {
	cluster := er.cluster
	esClient := er.esClient

	pattern := formatWriteAlias(mapping)
	indices, err := esClient.ListIndicesForAlias(pattern)
	if err != nil {
		return err
	}
	if len(indices) < 1 {
		indexName := fmt.Sprintf("%s-000001", mapping.Name)
		primaryShards := getDataCount(cluster)
		replicas := int32(calculateReplicaCount(cluster))
		index := esapi.NewIndex(indexName, primaryShards, replicas)
		index.AddAlias(mapping.Name, false)
		index.AddAlias(pattern, true)
		for _, alias := range mapping.Aliases {
			index.AddAlias(alias, false)
		}
		return esClient.CreateIndex(indexName, index)
	}
	return nil
}

func formatTemplateName(name string) string {
	return fmt.Sprintf("%s-%s", ocpTemplatePrefix, name)
}

func formatWriteAlias(mapping logging.IndexManagementPolicyMappingSpec) string {
	return fmt.Sprintf("%s-write", mapping.Name)
}

func (er *ElasticsearchRequest) createOrUpdateIndexTemplate(mapping logging.IndexManagementPolicyMappingSpec) error {
	cluster := er.cluster
	esClient := er.esClient

	name := formatTemplateName(mapping.Name)
	pattern := fmt.Sprintf("%s*", mapping.Name)
	primaryShards := getDataCount(cluster)
	replicas := int32(calculateReplicaCount(cluster))
	aliases := append(mapping.Aliases, mapping.Name)
	template := esapi.NewIndexTemplate(pattern, aliases, primaryShards, replicas)

	return esClient.CreateIndexTemplate(name, template)
}
