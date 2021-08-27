#!/bin/bash
set -eou pipefail

source $(dirname "${BASH_SOURCE[0]}")/env.sh

oc delete --wait --ignore-not-found ns ${ELASTICSEARCH_OPERATOR_NAMESPACE}

oc delete --wait --ignore-not-found crd kibanas.logging.openshift.io
oc delete --wait --ignore-not-found crd elasticsearches.logging.openshift.io
