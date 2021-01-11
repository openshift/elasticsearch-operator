#!/bin/bash
# Given an OLM manifest, verify a green field deployment
# of cluster logging by asserting CLO creates the resources
# that begets the operands that make up logging.

repo_dir="$( cd "$(dirname "$0")/../.." ; pwd -P )"
source "$repo_dir/hack/testing-olm-upgrade/upgrade-common"

version="4.5"
previous_version="4.4"

deploy_previous_version $previous_version
deploy_es_secret

# deploy elasticsearch CR
log::info "Deploying ES CR..."
oc -n "openshift-operators-redhat" create -f ${repo_dir}/hack/testing-olm-upgrade/resources/cr.yaml

log::info "Getting the previous statate of elasticsearch-operator deployment"
# get the previous status of the elasticsearch-operator
oc describe -n "openshift-operators-redhat" deployment/elasticsearch-operator > "$ARTIFACT_DIR"/elasticsearch-operator.describe.before_update 2>&1

check_for_es_pods

# read OLD 4.4 indices into and map them by their names
log::info "Reading old ES indices"
try_func_until_result_is_not_empty get_es_indices ${ES_POD_TIMEOUT}
old_indices=$(get_es_indices_names)

old_pvcs="$(get_current_pvcs)"

#### INSTALLING 4.5
log::info "Deploying the ES operator from the catalog..."
# deploy cluster logging catalog from local code
"${repo_dir}"/olm_deploy/scripts/catalog-deploy.sh

patch_subscription
patch_minkube_version

#verify deployment is rolled out
check_deployment_rolled_out

check_for_es_pods

expected_aliases="$(get_expected_aliases)"

# read new 4.5 indices and map them by their names
log::info "Reading new ES indices"
try_func_until_result_is_not_empty get_es_indices_names ${ES_POD_TIMEOUT}
new_indices=$(get_es_indices_names)

log::info "Validating indices match"
check_list_contained_in "$old_indices" "$new_indices"

# check to make sure new_aliases is contained in expected_aliases
log::info "Validating aliases match"
check_list_contained_in "$expected_aliases" "$new_aliases"

current_pvcs="$(get_current_pvcs)"

log::info "Validating PVCs haven't changed"
# check to make sure the current list of pvcs is contained in (same as) old pvcs
check_list_contained_in "$current_pvcs" "$old_pvcs"

log::info "Test passed"
