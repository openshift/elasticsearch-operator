set -euo pipefail

if [ -n "${DEBUG:-}" ]; then
    set -x
fi

KUBECONFIG=${KUBECONFIG:-$HOME/.kube/config}

repo_dir="$(dirname $0)/../.."
source "${repo_dir}/hack/testing-olm/utils"
ARTIFACT_DIR=${ARTIFACT_DIR:-"$(pwd)/_output"}
test_artifact_dir=$ARTIFACT_DIR/$(basename ${BASH_SOURCE[0]})
if [ ! -d $test_artifact_dir ] ; then
  mkdir -p $test_artifact_dir
fi

TEST_NAMESPACE="${TEST_NAMESPACE:-e2e-test-${RANDOM}}"

start_seconds=$(date +%s)
cleanup(){
  local return_code="$?"
  set +e
  log::info "Running cleanup"
  end_seconds=$(date +%s)
  runtime="$(($end_seconds - $start_seconds))s"
  
  if [ "${DO_CLEANUP:-true}" == "true" ] ; then
    gather_logging_resources ${TEST_NAMESPACE} $test_artifact_dir
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
  #deploy elasticsearch cluster
  expect_success "${repo_dir}/hack/deploy-example-secrets.sh  ${TEST_NAMESPACE}"
  expect_success "oc -n ${TEST_NAMESPACE} create -f ${repo_dir}/hack/cr.yaml"

  log::info "Waiting for elasticsearch-operator to deploy the cluster..."
  try_until_success "oc -n ${TEST_NAMESPACE} get deployment -l component=elasticsearch -o jsonpath={.items[0].metadata.name})" $((2 * $minute))

fi
#wait for pod
log::info "Waiting for elasticsearch deployment to be ready..."
wait_for_deployment_to_be_ready ${TEST_NAMESPACE} $(oc -n ${TEST_NAMESPACE} get deployment -l component=elasticsearch -o jsonpath={.items[0].metadata.name}) $((2 * $minute))

pod=$(oc -n $TEST_NAMESPACE get pod -l component=elasticsearch -o jsonpath={.items[0].metadata.name})
log::info Attempt to autocreate an index without a '-write' suffix...
expect_success_and_text "oc -n $TEST_NAMESPACE exec $pod -c elasticsearch -- es_util --query=foo/_doc/1 -d {\"key\":\"value\"} -XPUT -w %{http_code}" ".*201"

log::info Attempt to autocreate an index with a '-write' suffix...
expect_success_and_text "oc -n $TEST_NAMESPACE exec $pod -c elasticsearch -- es_util --query=foo-write/_doc/1 -d {\"key\":\"value\"} -XPUT -w %{http_code}" ".*404"

log::info Explicitly creating an index with a '-write' suffix...
expect_success_and_text "oc -n $TEST_NAMESPACE exec $pod -c elasticsearch -- es_util --query=foo-write -XPUT -w %{http_code}" ".*200"
log::info Verifying can write to index with a '-write' suffix...
expect_success_and_text "oc -n $TEST_NAMESPACE exec $pod -c elasticsearch -- es_util --query=foo-write/_doc/1 -d {\"key\":\"value\"} -XPUT  -w %{http_code}" ".*201"
