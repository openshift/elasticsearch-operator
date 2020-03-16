#!/bin/sh
set -eou pipefail

ELASTICSEARCH_OPERATOR_NAMESPACE=${ELASTICSEARCH_OPERATOR_NAMESPACE:-openshift-operators-redhat}

oc delete --ignore-not-found -n ${ELASTICSEARCH_OPERATOR_NAMESPACE} subscription/elasticsearch 

set +e
oc wait -n ${ELASTICSEARCH_OPERATOR_NAMESPACE} --timeout=180s --for=delete deployment/elasticsearch-operator
set -e

oc delete --wait --ignore-not-found ns ${ELASTICSEARCH_OPERATOR_NAMESPACE}
