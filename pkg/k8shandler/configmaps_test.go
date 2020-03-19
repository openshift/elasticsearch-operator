package k8shandler

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bytes"

	"github.com/openshift/elasticsearch-operator/test/helpers"
)

var _ = Describe("configmaps", func() {
	defer GinkgoRecover()

	Describe("#renderEsYml", func() {
		It("should produce an elasticsearch.yml for our managed elasticsearch instance", func() {
			result := &bytes.Buffer{}
			Expect(renderEsYml(result, "unique", "my.unicast.host", "7", "4")).To(BeNil(), "Exp. no errors when rendering the configuration")
			helpers.ExpectYaml(result.String()).ToEqual(`
cluster:
  name: ${CLUSTER_NAME}

script:
  inline: true
  stored: true

node:
  name: ${DC_NAME}
  master: ${IS_MASTER}
  data: ${HAS_DATA}
  max_local_storage_nodes: 1

network:
  publish_host: ${POD_IP}
  bind_host: ["${POD_IP}",_local_]

discovery.zen:
  ping.unicast.hosts: my.unicast.host
  minimum_master_nodes: 7

gateway:
  recover_after_nodes: 7
  expected_nodes: 4
  recover_after_time: ${RECOVER_AFTER_TIME}

io.fabric8.elasticsearch.kibana.mapping.app: /usr/share/elasticsearch/index_patterns/com.redhat.viaq-openshift.index-pattern.json
io.fabric8.elasticsearch.kibana.mapping.ops: /usr/share/elasticsearch/index_patterns/com.redhat.viaq-openshift.index-pattern.json
io.fabric8.elasticsearch.kibana.mapping.empty: /usr/share/elasticsearch/index_patterns/com.redhat.viaq-openshift.index-pattern.json

openshift.config:
  use_common_data_model: true
  project_index_prefix: "project"
  time_field_name: "@timestamp"

openshift.searchguard:
  keystore.path: /etc/elasticsearch/secret/admin.jks
  truststore.path: /etc/elasticsearch/secret/searchguard.truststore

openshift.kibana.index.mode: unique

path:
  data: /elasticsearch/persistent/${CLUSTER_NAME}/data
  logs: /elasticsearch/persistent/${CLUSTER_NAME}/logs

searchguard:
  authcz.admin_dn:
  - CN=system.admin,OU=OpenShift,O=Logging
  config_index_name: ".searchguard"
  ssl:
    transport:
      enabled: true
      enforce_hostname_verification: false
      keystore_type: JKS
      keystore_filepath: /etc/elasticsearch/secret/searchguard.key
      keystore_password: kspass
      truststore_type: JKS
      truststore_filepath: /etc/elasticsearch/secret/searchguard.truststore
      truststore_password: tspass
    http:
      enabled: true
      keystore_type: JKS
      keystore_filepath: /etc/elasticsearch/secret/key
      keystore_password: kspass
      clientauth_mode: OPTIONAL
      truststore_type: JKS
      truststore_filepath: /etc/elasticsearch/secret/truststore
      truststore_password: tspass`)
		})
	})

})
