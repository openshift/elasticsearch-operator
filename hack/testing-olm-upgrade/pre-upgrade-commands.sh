#!/bin/bash

repo_dir="$( cd "$(dirname "$0")/../.." ; pwd -P )"
source "$repo_dir/hack/testing-olm-upgrade/upgrade-common"

# deploy elasticsearch CR
log::info "Deploying ES CR..."
oc -n "openshift-operators-redhat" create -f ${repo_dir}/hack/testing-olm-upgrade/resources/cr.yaml

check_for_es_pods

# get a list of the aliases and make sure that we have them based on the expected aliases
try_func_until_result_is_not_empty get_es_aliases_names ${ES_POD_TIMEOUT}
get_es_aliases_names > "${E2E_ARTIFACT_DIR}/old-aliases"

try_func_until_result_is_not_empty get_es_indices_names ${ES_POD_TIMEOUT}
get_es_indices_names > "${E2E_ARTIFACT_DIR}/old-indices"

get_current_pvcs > "${E2E_ARTIFACT_DIR}/old-pvcs"
