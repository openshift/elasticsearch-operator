#!/bin/bash
# This test verifies only serviceaccounts with the desired rolebindings are 
# allowed to retrieve metrices from elasticsearch
set -euo pipefail

repo_dir="$(dirname $0)/../.."
source "${repo_dir}/hack/lib/init.sh"
source "${repo_dir}/hack/testing-olm/utils"

test_artifact_dir=$ARTIFACT_DIR/$(basename ${BASH_SOURCE[0]})
if [ ! -d $test_artifact_dir ] ; then
  mkdir -p $test_artifact_dir
fi

os::test::junit::declare_suite_start "[Elasticsearch] Verify Metrics Access"

suffix=$RANDOM
TEST_NAMESPACE="${TEST_NAMESPACE:-e2e-test-${suffix}}"
UNAUTHORIZED_SA="unauthorized-sa-${suffix}"
AUTHORIZED_SA="authorized-sa-${suffix}"
CLUSTERROLE="prometheus-k8s-${suffix}"

start_seconds=$(date +%s)
cleanup(){
  local return_code="$?"

  os::test::junit::declare_suite_end

  set +e
  os::log::info "Running cleanup"
  end_seconds=$(date +%s)
  runtime="$(($end_seconds - $start_seconds))s"
  
  if [ "${DO_CLEANUP:-true}" == "true" ] ; then
    if [ "$return_code" != "0" ] ; then
      gather_logging_resources ${TEST_NAMESPACE} $test_artifact_dir
    fi
    for item in "ns/${TEST_NAMESPACE}" "ns/openshift-operators-redhat"; do
      oc delete $item --wait=true --ignore-not-found --force --grace-period=0
    done
    oc delete clusterrole ${CLUSTERROLE} >> $test_artifact_dir/cleanup.log 2>&1 ||:
    oc delete clusterrolebinding ${CLUSTERROLE} >> $test_artifact_dir/cleanup.log 2>&1 ||:
    oc delete clusterrolebinding view-${CLUSTERROLE} >> $test_artifact_dir/cleanup.log 2>&1 ||:
    oc delete clusterrolebinding view-${CLUSTERROLE}-unauth >> $test_artifact_dir/cleanup.log 2>&1 ||:
  fi
  
  set -e
  exit ${return_code}
}
trap cleanup exit

if [ "${DO_SETUP:-true}" == "true" ] ; then
  for item in "${TEST_NAMESPACE}" "openshift-operators-redhat" ; do
    if oc get project ${item} > /dev/null 2>&1 ; then
      echo using existing project ${item}
    else
      oc create namespace ${item}
    fi
  done

  export ELASTICSEARCH_OPERATOR_NAMESPACE=${TEST_NAMESPACE}
  deploy_elasticsearch_operator

  os::cmd::expect_success "${repo_dir}/hack/cert_generation.sh /tmp/example-secrets ${TEST_NAMESPACE} elasticsearch"
  os::cmd::expect_success "${repo_dir}/hack/deploy-example-secrets.sh  ${TEST_NAMESPACE}"
  os::cmd::expect_success "oc -n ${TEST_NAMESPACE} create -f ${repo_dir}/hack/cr.yaml"

  os::log::info "---------------------------------------------------------------"
  os::log::info "Waiting for elasticsearch-operator to deploy the cluster..."
  os::log::info "---------------------------------------------------------------"
  os::cmd::try_until_success "oc -n ${TEST_NAMESPACE} get deployment -l component=elasticsearch -o jsonpath='{.items[0].metadata.name}'" $((2 * $minute))
  os::cmd::expect_success "oc wait -n ${TEST_NAMESPACE} --timeout=240s --for=condition=available deployment -l component=elasticsearch"

fi

# TODO: to be deleted
move_to_go(){
  os::log::info "------------------------------------------"
  os::log::info "Creating serviceaccounts to verify metrics"
  os::log::info "------------------------------------------"
  oc -n ${TEST_NAMESPACE} create serviceaccount ${UNAUTHORIZED_SA}
  oc -n ${TEST_NAMESPACE} create serviceaccount ${AUTHORIZED_SA}
  os::log::info "-------------------------------------------------------------"
  os::log::info "Creating RBAC for authorised serviceaccount to verify metrics"
  os::log::info "-------------------------------------------------------------"
  result=$(oc get clusterrole ${CLUSTERROLE} --ignore-not-found ||:)
  if [ "$result" == "" ] ; then
    echo "{\"apiVersion\":\"rbac.authorization.k8s.io/v1\", \"kind\":\"ClusterRole\",\"metadata\":{\"name\":\"${CLUSTERROLE}\"},\"rules\":[{\"nonResourceURLs\":[\"/metrics\"],\"verbs\":[\"get\"]}]}" | oc create -f -
  fi
  result=$(oc get clusterrolebinding ${CLUSTERROLE} --ignore-not-found ||:)
  if [ "$result" == "" ] ; then
    os::log::info Binding ${AUTHORIZED_SA} to be cable of reading metrics
    oc create clusterrolebinding --clusterrole=${CLUSTERROLE} ${CLUSTERROLE} --serviceaccount=${TEST_NAMESPACE}:${AUTHORIZED_SA}
  fi
  result=$(oc get clusterrolebinding view-${CLUSTERROLE} --ignore-not-found ||:)
  if [ "$result" == "" ] ; then
    os::log::info Binding ${AUTHORIZED_SA} to be cable of getting namespaces
    oc create clusterrolebinding --clusterrole=basic-user view-${CLUSTERROLE} --serviceaccount=${TEST_NAMESPACE}:${AUTHORIZED_SA}
  fi
  os::log::info "---------------------------------------------------------------"
  os::log::info "Creating RBAC for unauthorised serviceaccount to verify metrics"
  os::log::info "---------------------------------------------------------------"
  result=$(oc get clusterrolebinding view-${CLUSTERROLE}-unauth --ignore-not-found ||:)
  if [ "$result" == "" ] ; then
    os::log::info Binding ${UNAUTHORIZED_SA} to be cable of getting namespaces
    oc create clusterrolebinding --clusterrole=basic-user view-${CLUSTERROLE}-unauth --serviceaccount=${TEST_NAMESPACE}:${UNAUTHORIZED_SA}
  fi
  es_pod=$(oc -n ${TEST_NAMESPACE} get pod -l component=elasticsearch -o jsonpath={.items[0].metadata.name})
  os::log::info "---------------------------------------------------------------"
  os::log::info "Waiting until elasticsearch cluster initialization is completed"
  os::log::info "---------------------------------------------------------------"
  os::cmd::expect_success "oc -n ${TEST_NAMESPACE} wait --for=condition=Ready pod/${es_pod} --timeout=120s"
}

push_test_script_to_es(){
  es_pod=$1
  token=$2
  service_ip=elasticsearch-metrics.${TEST_NAMESPACE}.svc
  echo "curl -ks -o /tmp/metrics.txt https://${service_ip}:60001/_prometheus/metrics -H Authorization:'Bearer ${token}' -w '%{response_code}\n'" > /tmp/test
  os::cmd::expect_success "oc -n ${TEST_NAMESPACE} cp /tmp/test ${es_pod}:/tmp/test -c elasticsearch"
  os::cmd::expect_success "oc -n ${TEST_NAMESPACE} exec ${es_pod} -c elasticsearch -- chmod 777 /tmp/test"
}

os::log::info "---------------------------------------------------------------------------"
os::log::info "Checking ${AUTHORIZED_SA} ability to read metrics through metrics service"
os::log::info "---------------------------------------------------------------------------"
token=$(oc -n ${TEST_NAMESPACE} serviceaccounts get-token $AUTHORIZED_SA)
push_test_script_to_es $es_pod $token
os::cmd::expect_success_and_text "oc -n ${TEST_NAMESPACE} exec ${es_pod} -c elasticsearch -- bash -c /tmp/test" '200'

os::log::info "---------------------------------------------------------------------------"
os::log::info "Checking ${UNAUTHORIZED_SA} ability to read metrics through metrics service"
os::log::info "---------------------------------------------------------------------------"
token=$(oc -n ${TEST_NAMESPACE} serviceaccounts get-token $UNAUTHORIZED_SA)
push_test_script_to_es $es_pod $token
os::cmd::expect_success_and_text "oc -n ${TEST_NAMESPACE} exec ${es_pod} -c elasticsearch -- bash -c /tmp/test" '403'
