#!/bin/bash
# This test verifies only serviceaccounts with the desired rolebindings are 
# allowed to retrieve metrices from elasticsearch
set -euo pipefail

KUBECONFIG=${KUBECONFIG:-$HOME/.kube/config}

repo_dir="$(dirname $0)/../.."
source "${repo_dir}/hack/lib/init.sh"
source "${repo_dir}/hack/testing-olm/utils"

test_name="test-200-verify-es-metrics-access"

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
fi

CLUSTERROLE=${CLUSTERROLE} AUTHORIZED_SA=${AUTHORIZED_SA} UNAUTHORIZED_SA=${UNAUTHORIZED_SA} \
  TEST_OPERATOR_NAMESPACE=${TEST_NAMESPACE} \
  TEST_WATCH_NAMESPACE=${TEST_NAMESPACE} \
  go test ./test/e2e-olm/... -kubeconfig=${KUBECONFIG} -parallel=1 -timeout 1500s -run TestElasticsearchOperatorMetrics | \
  $GO_JUNIT_REPORT | awk '/<properties>/,/<\/properties>/ {next} {print}' > "$JUNIT_REPORT_OUTPUT_DIR/$test_name.xml"
