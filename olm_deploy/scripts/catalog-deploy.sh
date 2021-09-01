#!/bin/bash
set -eou pipefail
OPENSHIFT_VERSION=${OPENSHIFT_VERSION:-4.7}
LOGGING_VERSION=${LOGGING_VERSION:-5.0}
LOGGING_ES_VERSION=${LOGGING_ES_VERSION:-5.y}
LOGGING_ES_PROXY_VERSION=${LOGGING_ES_PROXY_VERSION:-5.y}
LOGGING_KIBANA_VERSION=${LOGGING_KIBANA_VERSION:-5.y}
LOGGING_IS=${LOGGING_IS:-logging}
export IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY=${IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY:-registry.ci.openshift.org/${LOGGING_IS}/${LOGGING_VERSION}:elasticsearch-operator-registry}
export IMAGE_ELASTICSEARCH_OPERATOR=${IMAGE_ELASTICSEARCH_OPERATOR:-registry.ci.openshift.org/${LOGGING_IS}/${LOGGING_VERSION}:elasticsearch-operator}
export IMAGE_ELASTICSEARCH6=${IMAGE_ELASTICSEARCH6:-registry.ci.openshift.org/${LOGGING_IS}/${LOGGING_ES_VERSION}:logging-elasticsearch6}
export IMAGE_ELASTICSEARCH_PROXY=${IMAGE_ELASTICSEARCH_PROXY:-registry.ci.openshift.org/${LOGGING_IS}/${LOGGING_ES_PROXY_VERSION}:elasticsearch-proxy}
export IMAGE_LOGGING_KIBANA6=${IMAGE_LOGGING_KIBANA6:-registry.ci.openshift.org/${LOGGING_IS}/${LOGGING_KIBANA_VERSION}:logging-kibana6}
export IMAGE_OAUTH_PROXY=${IMAGE_OAUTH_PROXY:-registry.ci.openshift.org/ocp/${OPENSHIFT_VERSION}:oauth-proxy}

ELASTICSEARCH_OPERATOR_NAMESPACE=${ELASTICSEARCH_OPERATOR_NAMESPACE:-openshift-operators-redhat}

echo "Using images: "
echo "elastic operator registry: ${IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY}"
echo "elastic operator: ${IMAGE_ELASTICSEARCH_OPERATOR}"
echo "elastic6: ${IMAGE_ELASTICSEARCH6}"
echo "elasticsearch proxy: ${IMAGE_ELASTICSEARCH_PROXY}"
echo "kibana: ${IMAGE_LOGGING_KIBANA6}"
echo "oauth proxy: ${IMAGE_OAUTH_PROXY}"

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
