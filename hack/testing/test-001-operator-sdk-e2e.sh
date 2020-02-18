#!/bin/bash
set -euo pipefail

if [ -n "${DEBUG:-}" ]; then
    set -x
fi

KUBECONFIG=${KUBECONFIG:-$HOME/.kube/config}

repo_dir="$(dirname $0)/../.."
source "${repo_dir}/hack/lib/log/output.sh"
source "${repo_dir}/hack/testing/utils"
ARTIFACT_DIR=${ARTIFACT_DIR:-"$repo_dir/_output/$(basename ${BASH_SOURCE[0]})"}
test_artifact_dir=$ARTIFACT_DIR/test-001-operator-sdk
if [ ! -d $test_artifact_dir ] ; then
  mkdir -p $test_artifact_dir
fi

IMAGE_ELASTICSEARCH_OPERATOR=${IMAGE_ELASTICSEARCH_OPERATOR:-$(format_image elasticsearch-operator)}
IMAGE_ELASTICSEARCH_PROXY=${IMAGE_ELASTICSEARCH_PROXY:-$(format_image elasticsearch-proxy)}
ELASTICSEARCH_IMAGE=${ELASTICSEARCH_IMAGE:-$(format_image logging-elasticsearch6)}

manifest=$(mktemp)
files="01-service-account.yaml 02-role.yaml 03-role-bindings.yaml 05-deployment.yaml"
pushd manifests;
  for f in ${files}; do
     cat ${f} >> ${manifest};
  done;
popd
# update the manifest with the image built by ci
sed -i "s,quay.io/openshift/origin-elasticsearch-operator:latest,${IMAGE_ELASTICSEARCH_OPERATOR}," ${manifest}
sed -i "s,quay.io/openshift/origin-logging-elasticsearch6:latest,${ELASTICSEARCH_IMAGE}," ${manifest}
sed -i "s,quay.io/openshift/origin-elasticsearch-proxy:latest,${IMAGE_ELASTICSEARCH_PROXY}," ${manifest}

if [ "${REMOTE_CLUSTER:-false}" = false ] ; then
  sudo sysctl -w vm.max_map_count=262144 ||:
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
    gather_logging_resources ${TEST_NAMESPACE} $test_artifact_dir
    for item in "ns/${TEST_NAMESPACE}" "clusterrole/elasticsearch-operator" "clusterrolebinding/elasticsearch-operator-rolebinding"; do
      oc delete $item --wait=true --ignore-not-found --force --grace-period=0
    done
  fi
  
  set -e
  exit ${return_code}
}
trap cleanup exit

if [ "${DO_SETUP:-true}" == "true" ] ; then
  for item in "${TEST_NAMESPACE}" "openshift-operators-redhat"; do
    if oc get project ${item} > /dev/null 2>&1 ; then
      echo using existing project ${item}
    else
      oc create namespace ${item}
    fi
  done

  deploy_elasticsearch_operator
fi

TEST_NAMESPACE=${TEST_NAMESPACE} go test ./test/e2e/. -root=$(pwd) -timeout 1200s -parallel=1 -v

