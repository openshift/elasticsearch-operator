#!/bin/bash
set -eou pipefail
OCP_VERSION=${OCP_VERSION:-4.7}
LOGGING_VERSION=${LOGGING_VERSION:-5.2}
LOGGING_IS=${LOGGING_IS:-logging}
export IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY=${IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY:-registry.ci.openshift.org/${LOGGING_IS}/${LOGGING_VERSION}:elasticsearch-operator-registry}
export IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE=${IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE:-registry.ci.openshift.org/${LOGGING_IS}/${LOGGING_VERSION}:elasticsearch-operator-bundle}
ELASTICSEARCH_OPERATOR_NAMESPACE=${ELASTICSEARCH_OPERATOR_NAMESPACE:-openshift-operators-redhat}

echo "Using images: "
echo "elastic operator registry: ${IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY}"

echo "In namespace: ${ELASTICSEARCH_OPERATOR_NAMESPACE}"

if oc get project ${ELASTICSEARCH_OPERATOR_NAMESPACE} > /dev/null 2>&1 ; then
  echo using existing project ${ELASTICSEARCH_OPERATOR_NAMESPACE} for operator catalog deployment
else
  oc create namespace ${ELASTICSEARCH_OPERATOR_NAMESPACE}
fi

# substitute image names into the catalog deployment yaml and deploy it
envsubst < olm_deploy/operatorregistry/registry-deployment.yaml | oc create -n ${ELASTICSEARCH_OPERATOR_NAMESPACE} -f -
olm_deploy/scripts/wait_for_deployment.sh ${ELASTICSEARCH_OPERATOR_NAMESPACE} elasticsearch-operator-registry
oc wait -n ${ELASTICSEARCH_OPERATOR_NAMESPACE} --timeout=120s --for=condition=available deployment/elasticsearch-operator-registry

# create the catalog service
oc create -n ${ELASTICSEARCH_OPERATOR_NAMESPACE} -f olm_deploy/operatorregistry/service.yaml

# find the catalog service ip, substitute it into the catalogsource and create the catalog source
export CLUSTER_IP=$(oc get -n ${ELASTICSEARCH_OPERATOR_NAMESPACE} service elasticsearch-operator-registry -o jsonpath='{.spec.clusterIP}')
envsubst < olm_deploy/operatorregistry/catalog-source.yaml | oc create -n ${ELASTICSEARCH_OPERATOR_NAMESPACE} -f -
