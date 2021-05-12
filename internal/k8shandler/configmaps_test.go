package k8shandler

import (
	"bytes"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openshift/elasticsearch-operator/test/helpers"
)

var _ = Describe("configmaps.go", func() {
	defer GinkgoRecover()
	var ()

	Describe("#renderLog4j2Properties", func() {
		It("should create a well-formed file without error", func() {
			out := bytes.NewBufferString("")
			logConfig := LogConfig{"debug", "trace", "mylogger"}
			if err := renderLog4j2Properties(out, logConfig); err != nil {
				Fail(fmt.Sprintf("unable to render Log4J properties. %s\r\n", err.Error()))
			}
			Expect(out.String()).To(Equal(`
status = error

# log action execution errors for easier debugging
logger.action.name = org.elasticsearch.action
logger.action.level = debug

logger.security.name = com.amazon.opendistroforelasticsearch.security
logger.security.level = debug

appender.console.type = Console
appender.console.name = console
appender.console.layout.type = PatternLayout
appender.console.layout.pattern = [%d{ISO8601}][%-5p][%-25c{1.}] %marker%m%n

appender.rolling.type = RollingFile
appender.rolling.name = rolling
appender.rolling.fileName = ${sys:es.logs.base_path}${sys:file.separator}${sys:es.logs.cluster_name}.log
appender.rolling.layout.type = PatternLayout
appender.rolling.layout.pattern = [%d{ISO8601}][%-5p][%-25c{1.}] %marker%.-10000m%n
appender.rolling.filePattern = ${sys:es.logs.base_path}${sys:file.separator}${sys:es.logs.cluster_name}-%d{yyyy-MM-dd}.log
appender.rolling.policies.type = Policies
appender.rolling.policies.time.type = TimeBasedTriggeringPolicy
appender.rolling.policies.time.interval = 1
appender.rolling.policies.time.modulate = true
appender.rolling.policies.size.type=SizeBasedTriggeringPolicy
appender.rolling.policies.size.size=100MB
appender.rolling.strategy.type=DefaultRolloverStrategy
appender.rolling.strategy.max=5

rootLogger.level = trace
rootLogger.appenderRef.mylogger.ref = mylogger

appender.deprecation_rolling.type = RollingFile
appender.deprecation_rolling.name = deprecation_rolling
appender.deprecation_rolling.fileName = ${sys:es.logs.base_path}${sys:file.separator}${sys:es.logs.cluster_name}_deprecation.log
appender.deprecation_rolling.layout.type = PatternLayout
appender.deprecation_rolling.layout.pattern = [%d{ISO8601}][%-5p][%-25c{1.}] %marker%.-10000m%n
appender.deprecation_rolling.filePattern = ${sys:es.logs.base_path}${sys:file.separator}${sys:es.logs.cluster_name}_deprecation-%i.log.gz
appender.deprecation_rolling.policies.type = Policies
appender.deprecation_rolling.policies.size.type = SizeBasedTriggeringPolicy
appender.deprecation_rolling.policies.size.size = 1GB
appender.deprecation_rolling.strategy.type = DefaultRolloverStrategy
appender.deprecation_rolling.strategy.max = 4

logger.deprecation.name = org.elasticsearch.deprecation
logger.deprecation.level = warn
logger.deprecation.appenderRef.deprecation_rolling.ref = deprecation_rolling
logger.deprecation.additivity = false

appender.index_search_slowlog_rolling.type = RollingFile
appender.index_search_slowlog_rolling.name = index_search_slowlog_rolling
appender.index_search_slowlog_rolling.fileName = ${sys:es.logs.base_path}${sys:file.separator}${sys:es.logs.cluster_name}_index_search_slowlog.log
appender.index_search_slowlog_rolling.layout.type = PatternLayout
appender.index_search_slowlog_rolling.layout.pattern = [%d{ISO8601}][%-5p][%-25c] %marker%.-10000m%n
appender.index_search_slowlog_rolling.filePattern = ${sys:es.logs.base_path}${sys:file.separator}${sys:es.logs.cluster_name}_index_search_slowlog-%d{yyyy-MM-dd}.log
appender.index_search_slowlog_rolling.policies.type = Policies
appender.index_search_slowlog_rolling.policies.time.type = TimeBasedTriggeringPolicy
appender.index_search_slowlog_rolling.policies.time.interval = 1
appender.index_search_slowlog_rolling.policies.time.modulate = true

logger.index_search_slowlog_rolling.name = index.search.slowlog
logger.index_search_slowlog_rolling.level = trace
logger.index_search_slowlog_rolling.appenderRef.index_search_slowlog_rolling.ref = index_search_slowlog_rolling
logger.index_search_slowlog_rolling.additivity = false

appender.index_indexing_slowlog_rolling.type = RollingFile
appender.index_indexing_slowlog_rolling.name = index_indexing_slowlog_rolling
appender.index_indexing_slowlog_rolling.fileName = ${sys:es.logs.base_path}${sys:file.separator}${sys:es.logs.cluster_name}_index_indexing_slowlog.log
appender.index_indexing_slowlog_rolling.layout.type = PatternLayout
appender.index_indexing_slowlog_rolling.layout.pattern = [%d{ISO8601}][%-5p][%-25c] %marker%.-10000m%n
appender.index_indexing_slowlog_rolling.filePattern = ${sys:es.logs.base_path}${sys:file.separator}${sys:es.logs.cluster_name}_index_indexing_slowlog-%d{yyyy-MM-dd}.log
appender.index_indexing_slowlog_rolling.policies.type = Policies
appender.index_indexing_slowlog_rolling.policies.time.type = TimeBasedTriggeringPolicy
appender.index_indexing_slowlog_rolling.policies.time.interval = 1
appender.index_indexing_slowlog_rolling.policies.time.modulate = true

logger.index_indexing_slowlog.name = index.indexing.slowlog.index
logger.index_indexing_slowlog.level = trace
logger.index_indexing_slowlog.appenderRef.index_indexing_slowlog_rolling.ref = index_indexing_slowlog_rolling
logger.index_indexing_slowlog.additivity = false`))
		})
	})
})

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

prometheus:
  indices: false

# increase the max header size above 8kb default
http.max_header_size: 128kb

opendistro_security:
  authcz.admin_dn:
  - CN=system.admin,OU=OpenShift,O=Logging
  - CN=system.admin,OU=Logging,O=OpenShift
  config_index_name: ".security"
  restapi:
    roles_enabled: ["kibana_server"]
  ssl:
    transport:
      enabled: true
      enforce_hostname_verification: false
      keystore_type: PKCS12
      keystore_filepath: /etc/elasticsearch/secret/searchguard-key.p12
      keystore_password: kspass
      truststore_type: PKCS12
      truststore_filepath: /etc/elasticsearch/secret/searchguard-truststore.p12
      truststore_password: tspass
    http:
      enabled: true
      keystore_type: PKCS12
      keystore_filepath: /etc/elasticsearch/secret/key.p12
      keystore_password: kspass
      clientauth_mode: OPTIONAL
      truststore_type: PKCS12
      truststore_filepath: /etc/elasticsearch/secret/truststore.p12
      truststore_password: tspass`)
		})
	})
})
