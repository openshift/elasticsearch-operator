#!/bin/bash
set -eou pipefail

ELASTICSEARCH_OPERATOR_NAMESPACE=${ELASTICSEARCH_OPERATOR_NAMESPACE:-openshift-operators-redhat}

oc delete --wait --ignore-not-found project ${ELASTICSEARCH_OPERATOR_NAMESPACE}
