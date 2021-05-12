#!/bin/bash
set -euo pipefail

KUBECONFIG=${KUBECONFIG:-$HOME/.kube/config}

repo_dir="$(dirname $0)/../.."
source "${repo_dir}/hack/lib/init.sh"
source "${repo_dir}/hack/testing-olm/utils"

test_name="test-001-operator-sdk"

test_artifact_dir=$ARTIFACT_DIR/$test_name
if [ ! -d $test_artifact_dir ] ; then
  mkdir -p $test_artifact_dir
fi

TEST_NAMESPACE="${TEST_NAMESPACE:-e2e-test-${RANDOM}}"

start_seconds=$(date +%s)
cleanup(){
  local return_code="$?"

  set +e
  os::log::info "Running cleanup"
  end_seconds=$(date +%s)
  runtime="$(($end_seconds - $start_seconds))s"
  
  if [ "${SKIP_CLEANUP:-false}" == "false" ] ; then

    if [ "$return_code" != "0" ] ; then
      gather_logging_resources ${TEST_NAMESPACE} $test_artifact_dir
    fi

    ${repo_dir}/olm_deploy/scripts/catalog-uninstall.sh
    ${repo_dir}/olm_deploy/scripts/operator-uninstall.sh
    oc delete ns/${TEST_NAMESPACE} --wait=true --ignore-not-found --force --grace-period=0
  fi

  set -e
  exit ${return_code}
}
trap cleanup exit

if oc get namespace ${TEST_NAMESPACE} > /dev/null 2>&1 ; then
  echo using existing project ${TEST_NAMESPACE}
else
  oc create namespace ${TEST_NAMESPACE}
fi

# install the catalog containing the elasticsearch operator csv
export ELASTICSEARCH_OPERATOR_NAMESPACE=${TEST_NAMESPACE}
deploy_elasticsearch_operator

TEST_OPERATOR_NAMESPACE=${TEST_NAMESPACE} \
TEST_WATCH_NAMESPACE=${TEST_NAMESPACE} \
  go test ./test/e2e-olm/... -kubeconfig=${KUBECONFIG} -parallel=1 -timeout 1500s 2>&1 -run "TestKibana|TestElasticsearchCluster" | \
  $GO_JUNIT_REPORT | awk '/<properties>/,/<\/properties>/ {next} {print}' > "$JUNIT_REPORT_OUTPUT_DIR/$test_name.xml"
