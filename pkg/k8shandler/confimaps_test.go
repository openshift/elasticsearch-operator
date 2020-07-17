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
			Expect(renderEsYml(result, "", "my.unicast.host", "7", "4", "false")).To(BeNil(), "Exp. no errors when rendering the configuration")
			helpers.ExpectYaml(result.String()).ToEqual(`
cluster:
  name: ${CLUSTER_NAME}

bootstrap:
  system_call_filter: false

node:
  name: ${DC_NAME}
  master: ${IS_MASTER}
  data: ${HAS_DATA}
  max_local_storage_nodes: 1

action.auto_create_index: "-*-write,+*"

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

path:
  data: /elasticsearch/persistent/${CLUSTER_NAME}/data
  logs: /elasticsearch/persistent/${CLUSTER_NAME}/logs

opendistro_security:
  authcz.admin_dn:
  - CN=system.admin,OU=OpenShift,O=Logging
  config_index_name: ".security"
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
