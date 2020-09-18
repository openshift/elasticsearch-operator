#!/bin/bash
# Given an OLM manifest, verify a green field deployment
# of cluster logging by asserting CLO creates the resources
# that begets the operands that make up logging.

set -euo pipefail

repo_dir="$( cd "$(dirname "$0")/../.." ; pwd -P )"
source "$repo_dir/hack/testing-olm/utils"
source "$repo_dir/hack/testing-olm/assertions"

start_seconds=$(date +%s)

ARTIFACT_DIR=${ARTIFACT_DIR:-"$repo_dir/_output/"}
ARTIFACT_DIR="${ARTIFACT_DIR}/$(basename ${BASH_SOURCE[0]})"
if [ ! -d $ARTIFACT_DIR ] ; then
  mkdir -p $ARTIFACT_DIR
fi

KUBECONFIG=${KUBECONFIG:-$HOME/.kube/config}
TIMEOUT_MIN=$((2 * $minute))
ES_POD_TIMEOUT=$((5 * $minute))
export CLUSTER_LOGGING_OPERATOR_NAMESPACE="openshift-logging"
manifest_dir=${repo_dir}/manifests
version="4.5"
previous_version="4.4"

cleanup(){
  local return_code="$?"
  set +e
  log::info "Running cleanup"
  end_seconds=$(date +%s)
  runtime="$(($end_seconds - $start_seconds))s"
  oc -n openshift-operators-redhat -o yaml get subscription elasticsearch-operator > $ARTIFACT_DIR/subscription-eo.yml 2>&1 ||:
  oc -n openshift-operators-redhat -o yaml get deployment/elasticsearch-operator > $ARTIFACT_DIR/elasticsearch-operator-deployment.yml 2>&1 ||:
  oc -n openshift-operators-redhat -o yaml get pods > $ARTIFACT_DIR/openshift-operators-redhat-pods.yml 2>&1 ||:
  oc -n openshift-operators-redhat -o yaml get configmaps > $ARTIFACT_DIR/openshift-operators-redhat-configmaps.yml 2>&1 ||:
  oc -n openshift-operators-redhat describe deployment/elasticsearch-operator > $ARTIFACT_DIR/elasticsearch-operator.describe 2>&1 ||:

  oc logs -n "openshift-operators-redhat" deployment/elasticsearch-operator > $ARTIFACT_DIR/elasticsearch-operator.log 2>&1 ||:
  oc  -n openshift-operator-lifecycle-manager logs --since=$runtime deployment/catalog-operator > $ARTIFACT_DIR/catalog-operator.logs 2>&1 ||:
  oc  -n openshift-operator-lifecycle-manager logs --since=$runtime deployment/olm-operator > $ARTIFACT_DIR/olm-operator.logs 2>&1 ||:

  ${repo_dir}/olm_deploy/scripts/operator-uninstall.sh
  ${repo_dir}/olm_deploy/scripts/catalog-uninstall.sh

  set -e
  exit ${return_code}
}
trap cleanup exit

get_es_pods_count() {
  	oc -n openshift-operators-redhat get pods -l component=elasticsearch --no-headers=true --ignore-not-found | wc -l
}

get_es_cluster_status() {
  oc -n openshift-operators-redhat get pods -l component=elasticsearch --no-headers=true --ignore-not-found \
   | awk 'NR==1{print $1}' \
   | xargs -I '{}' oc -n openshift-operators-redhat exec '{}' -c elasticsearch -- es_util --query=_cluster/health \
   | jq .status
}

get_es_indices() {
  oc -n openshift-operators-redhat get pods -l component=elasticsearch --no-headers=true --ignore-not-found \
    | awk 'NR==1{print $1}' \
    | xargs -I '{}' oc -n openshift-operators-redhat exec '{}' -c elasticsearch -- es_util --query=_cat/indices
}

get_es_indices_names() {
  oc -n openshift-operators-redhat get pods -l component=elasticsearch --no-headers=true --ignore-not-found \
    | awk 'NR==1{print $1}' \
    | xargs -I '{}' oc -n openshift-operators-redhat exec '{}' -c elasticsearch -- es_util --query=_cat/indices \
    | awk '{print $3}'
}

read_es_indices() {
	local -n map=$1
	while IFS= read -r line; do
		id="$(echo "$line" | awk '{print $3}')"
		echo "Index: '$id' stored"
		map[$id]=$line
	done <<< "$(get_es_indices)"
}

compare_indices_names(){
	local failed=false
	local -n old_m=$1
	local -n new_m=$2
	for k in "${!old_m[@]}"; do
		echo "Checking index $k..."
		if [ ! ${new_m[$k]+_} ]; then
			failed=true
			printf "\t'%s' is missing in the set of indices\n" "$k"
		else
			printf "\t'%s' found\n" "$k"
		fi
	done
	if [ "$failed" = true ]; then
		echo "Compare indices names failed!!!"
		return 1
	fi
	echo "Indices names are fully matched"
}

es_cluster_ready() {
	oc -n openshift-operators-redhat get pods -l component=elasticsearch \
		--no-headers=true --ignore-not-found \
		| awk '{print $2}' | grep '2/2'
}

# deploy elasticsearch-operator
log::info "Deploying elasticsearch-operator ${previous_version} from marketplace..."
deploy_marketplace_operator "openshift-operators-redhat" "elasticsearch-operator" "$previous_version" "elasticsearch-operator"

# check if the operator is running
log::info "Verifying if elasticsearch-operator deployment is ready..."
try_until_text "oc -n openshift-operators-redhat get deployment elasticsearch-operator -o jsonpath={.status.updatedReplicas} --ignore-not-found" "1" ${TIMEOUT_MIN}

log::info "Deploying ES secrets..."
rm -rf /tmp/example-secrets ||: \
	mkdir /tmp/example-secrets && \
	"$repo_dir"/hack/cert_generation.sh /tmp/example-secrets "openshift-operators-redhat" elasticsearch
"$repo_dir"/hack/deploy-example-secrets.sh "openshift-operators-redhat"

# deploy elasticsearch CR
log::info "Deploying ES CR..."
oc -n "openshift-operators-redhat" create -f ${repo_dir}/hack/testing-olm-upgrade/resources/cr.yaml

# check if there are elasticsearch pod
log::info "Checking if ES pod is ready in the namespace openshift-operators-redhat..."
try_until_text "oc -n openshift-operators-redhat get pods -l component=elasticsearch -o jsonpath={.items[0].status.phase} --ignore-not-found" "Running" ${ES_POD_TIMEOUT}

log::info "Getting the previous statate of elasticsearch-operator deployment"
# get the previous status of the elasticsearch-operator
oc describe -n "openshift-operators-redhat" deployment/elasticsearch-operator > "$ARTIFACT_DIR"/elasticsearch-operator.describe.before_update 2>&1

# check if ES cluster has 3 running pods
log::info "Checking if the ES has 3 nodes"
try_func_until_text get_es_pods_count "3" ${ES_POD_TIMEOUT}

log::info "Check if at least 1 node is ready (2/2 containers running)"
try_until_success es_cluster_ready ${ES_POD_TIMEOUT}

# check if ES cluster is in green state
log::info "Checking if the ES cluster is all yellow/green"
try_func_until_text_alt get_es_cluster_status "\"green\"" "\"yellow\"" ${ES_POD_TIMEOUT}

# read OLD 4.4 indices into and map them by their names
log::info "Reading old ES indices"
try_func_until_result_is_not_empty get_es_indices ${ES_POD_TIMEOUT}
old_indices=$(get_es_indices_names)

#### INSTALLING 4.5
log::info "Deploying the ES operator from the catalog..."
# deploy cluster logging catalog from local code
"${repo_dir}"/olm_deploy/scripts/catalog-deploy.sh

log::info "Patching subscription..."
# patch subscription
payload="{\"op\":\"replace\",\"path\":\"/spec/source\",\"value\":\"elasticsearch-catalog\"}"
payload="$payload,{\"op\":\"replace\",\"path\":\"/spec/sourceNamespace\",\"value\":\"openshift-operators-redhat\"}"
payload="$payload,{\"op\":\"replace\",\"path\":\"/spec/channel\",\"value\":\"$version\"}"
oc -n "openshift-operators-redhat" patch subscription elasticsearch-operator --type json -p "[$payload]"

# patch the minikube version
log::info "Patching minKubeVersion to 1.16.1..."
pl="{\"op\":\"replace\",\"path\":\"/spec/minKubeVersion\",\"value\":\"1.16.1\"}"
try_until_success "oc -n openshift-operators-redhat patch clusterserviceversion elasticsearch-operator.v${version}.0 --type json -p [$pl]" ${TIMEOUT_MIN}

#verify deployment is rolled out
log::info "Checking if deployment successfully updated..."
OPENSHIFT_VERSION=${OPENSHIFT_VERSION:-4.5}
IMAGE_ELASTICSEARCH_OPERATOR=${IMAGE_ELASTICSEARCH_OPERATOR:-registry.svc.ci.openshift.org/ocp/${OPENSHIFT_VERSION}:elasticsearch-operator}
if [ -n "${IMAGE_FORMAT:-}" ] ; then
  IMAGE_ELASTICSEARCH_OPERATOR=$(echo $IMAGE_FORMAT | sed -e "s,\${component},elasticsearch-operator,")
fi
log::info "Checking if deployment version is ${IMAGE_ELASTICSEARCH_OPERATOR}..."
try_until_text "oc -n openshift-operators-redhat get deployment elasticsearch-operator -o jsonpath={.spec.template.spec.containers[0].image}" "${IMAGE_ELASTICSEARCH_OPERATOR}" ${TIMEOUT_MIN}

log::info "Checking if the ES pod is running..."
try_until_text "oc -n openshift-operators-redhat get deployment -l component=elasticsearch -o jsonpath={.items[0].status.updatedReplicas} --ignore-not-found" "1" ${ES_POD_TIMEOUT}

# check if ES cluster has 3 running pods
log::info "Checking if the ES cluster still has 3 nodes..."
try_func_until_text get_es_pods_count "3" ${ES_POD_TIMEOUT}

log::info "Check if at least 1 node is ready (2/2 containers running)"
try_until_success es_cluster_ready ${ES_POD_TIMEOUT}

# check if ES cluster is in green state
log::info "Checking if the ES cluster is all yellow/green"
try_func_until_text_alt get_es_cluster_status "\"green\"" "\"yellow\"" ${ES_POD_TIMEOUT}

# read new 4.5 indices and map them by their names
log::info "Reading new ES indices"
try_func_until_result_is_not_empty get_es_indices_names ${ES_POD_TIMEOUT}
new_indices=$(get_es_indices_names)

if [ "$old_indices" != "$new_indices" ]; then
  log:info "Test failed"
  exit 1
fi

log:info "Test passed"
