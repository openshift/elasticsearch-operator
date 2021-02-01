package k8shandler

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/ViaQ/logerr/log"
	logging "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/internal/constants"
	"github.com/openshift/elasticsearch-operator/internal/indexmanagement"
	esapi "github.com/openshift/elasticsearch-operator/internal/types/elasticsearch"
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
		if err := indexmanagement.ReconcileIndexManagementCronjob(er.client, er.cluster, policy, mapping, primaryShards); err != nil {
			ll.Error(err, "could not reconcile indexmanagement cronjob")
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
		if strings.HasPrefix(template, constants.OcpTemplatePrefix) {
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
		primaryShards := int32(calculatePrimaryCount(cluster))
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
	return fmt.Sprintf("%s-%s", constants.OcpTemplatePrefix, name)
}

func formatWriteAlias(mapping logging.IndexManagementPolicyMappingSpec) string {
	return fmt.Sprintf("%s-write", mapping.Name)
}

func (er *ElasticsearchRequest) createOrUpdateIndexTemplate(mapping logging.IndexManagementPolicyMappingSpec) error {
	cluster := er.cluster
	esClient := er.esClient

	name := formatTemplateName(mapping.Name)
	pattern := fmt.Sprintf("%s*", mapping.Name)
	primaryShards := int32(calculatePrimaryCount(cluster))
	replicas := int32(calculateReplicaCount(cluster))
	aliases := append(mapping.Aliases, mapping.Name)
	template := esapi.NewIndexTemplate(pattern, aliases, primaryShards, replicas)

	// check to compare the current index templates vs what we just generated
	templates, err := esClient.GetIndexTemplates()
	if err != nil {
		return err
	}

	for templateName := range templates {
		if templateName == name {
			return nil
		}
	}

	return esClient.CreateIndexTemplate(name, template)
}
