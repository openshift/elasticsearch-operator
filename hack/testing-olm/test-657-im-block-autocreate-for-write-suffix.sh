#!/bin/bash

set -euo pipefail

KUBECONFIG=${KUBECONFIG:-$HOME/.kube/config}

repo_dir="$(dirname $0)/../.."
source "${repo_dir}/hack/lib/init.sh"
source "${repo_dir}/hack/testing-olm/utils"

test_name="test-657-im-block-autocreate-for-write-suffix"

test_artifact_dir=$ARTIFACT_DIR/$(basename ${BASH_SOURCE[0]})
if [ ! -d $test_artifact_dir ] ; then
  mkdir -p $test_artifact_dir
fi

os::test::junit::declare_suite_start "[Elasticsearch] Index Management Block Auto-Create For Write Suffix"

TEST_NAMESPACE="${TEST_NAMESPACE:-e2e-test-${RANDOM}}"

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

TEST_WATCH_NAMESPACE=${TEST_NAMESPACE} TEST_OPERATOR_NAMESPACE=${TEST_NAMESPACE} \
  go test ./test/e2e-olm/... -kubeconfig=${KUBECONFIG} -parallel=1 -timeout 1500s -run TestElasticsearchWrite  | \
  $GO_JUNIT_REPORT | awk '/<properties>/,/<\/properties>/ {next} {print}' > "$JUNIT_REPORT_OUTPUT_DIR/$test_name.xml"
