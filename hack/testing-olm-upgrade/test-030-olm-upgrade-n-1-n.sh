#!/bin/bash
# Given an OLM manifest, verify a green field deployment
# of elasticsearch by asserting EO creates and upgrades
# the resources that beget the operands that make up logging.

repo_dir="$( cd "$(dirname "$0")/../.." ; pwd -P )"
source "$repo_dir/hack/testing-olm-upgrade/upgrade-common"

discover_versions

deploy_previous_version $previous_version
deploy_es_secret

# deploy elasticsearch CR
log::info "Deploying ES CR..."
oc -n "openshift-operators-redhat" create -f ${repo_dir}/hack/testing-olm-upgrade/resources/cr.yaml

check_for_es_pods

log::info "Getting the previous state of elasticsearch-operator deployment"
# get the previous status of the elasticsearch-operator
oc describe -n "openshift-operators-redhat" deployment/elasticsearch-operator > "$ARTIFACT_DIR"/elasticsearch-operator.describe.before_update 2>&1

# get a list of the aliases and make sure that we have them based on the expected aliases
try_func_until_result_is_not_empty get_es_aliases_names ${ES_POD_TIMEOUT}
old_aliases="$(get_es_aliases_names)"

try_func_until_result_is_not_empty get_es_indices_names ${ES_POD_TIMEOUT}
old_indices=$(get_es_indices_names)

old_pvcs="$(get_current_pvcs)"

#### INSTALLING current version
log::info "Deploying the ES operator from the catalog..."
# deploy cluster logging catalog from local code
"${repo_dir}"/olm_deploy/scripts/catalog-deploy.sh

patch_subscription
patch_minkube_version

#verify deployment is rolled out
check_deployment_rolled_out

check_for_es_pods

# wait here until we get indices expected based on the index management spec
expected_aliases="$(get_expected_aliases)"

# get a list of the aliases and make sure that we have them based on the expected aliases
try_func_until_result_is_not_empty get_es_aliases_names ${ES_POD_TIMEOUT}
new_aliases="$(get_es_aliases_names)"

# read new 4.5 indices and map them by their names
log::info "Reading new ES indices"
try_func_until_result_is_not_empty get_es_indices_names ${ES_POD_TIMEOUT}
new_indices=$(get_es_indices_names)

log::info "Validating indices match"
check_list_contained_in "$old_indices" "$new_indices"

log::info "Validating expected aliases exist"
check_list_contained_in "$expected_aliases" "$new_aliases"

# check to make sure new_aliases is contained in expected_aliases
log::info "Validating aliases match"
check_list_contained_in "$old_aliases" "$new_aliases"

current_pvcs="$(get_current_pvcs)"

log::info "Validating PVCs haven't changed"
# check to make sure the current list of pvcs is contained in (same as) old pvcs
check_list_contained_in "$current_pvcs" "$old_pvcs"

log::info "Test passed"
