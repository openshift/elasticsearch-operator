#!/bin/bash
set -eou pipefail

export ELASTICSEARCH_OPERATOR_NAMESPACE=${ELASTICSEARCH_OPERATOR_NAMESPACE:-openshift-operators-redhat}


if oc get project ${ELASTICSEARCH_OPERATOR_NAMESPACE} > /dev/null 2>&1 ; then
  echo using existing project ${ELASTICSEARCH_OPERATOR_NAMESPACE} for operator installation
else
  oc create namespace ${ELASTICSEARCH_OPERATOR_NAMESPACE}
fi

set +e
oc label ns/${ELASTICSEARCH_OPERATOR_NAMESPACE} openshift.io/cluster-monitoring=true --overwrite
set -e

echo "##################"
echo "oc version"
oc version
echo "##################"

# create the operatorgroup
oc create -n ${ELASTICSEARCH_OPERATOR_NAMESPACE} -f olm_deploy/subscription/operator-group.yaml

# create the subscription
export OPERATOR_PACKAGE_CHANNEL=\"$(grep name manifests/elasticsearch-operator.package.yaml | grep  -oh "[0-9]\+\.[0-9]\+")\"
subscription=$(envsubst < olm_deploy/subscription/subscription.yaml)
echo "Creating:"
echo "$subscription"
echo "$subscription" | oc create -n ${ELASTICSEARCH_OPERATOR_NAMESPACE} -f -

olm_deploy/scripts/wait_for_deployment.sh ${ELASTICSEARCH_OPERATOR_NAMESPACE} elasticsearch-operator
oc wait -n ${ELASTICSEARCH_OPERATOR_NAMESPACE} --timeout=180s --for=condition=available deployment/elasticsearch-operator
