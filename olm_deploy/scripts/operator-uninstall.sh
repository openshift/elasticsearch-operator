#!/bin/bash
set -eou pipefail

ELASTICSEARCH_OPERATOR_NAMESPACE=${ELASTICSEARCH_OPERATOR_NAMESPACE:-openshift-operators-redhat}

oc delete --wait --ignore-not-found ns ${ELASTICSEARCH_OPERATOR_NAMESPACE}

oc delete --wait --ignore-not-found crd kibanas.logging.openshift.io
oc delete --wait --ignore-not-found crd elasticsearches.logging.openshift.io
