#!/bin/sh
set -eou pipefail

#source variables.env
OPM="${GOPATH}/src/github.com/operator-framework/operator-registry/bin/opm"

# this works if we use a public quay.io repo -- login to one here
podman login -u="ewolinet+write_bot" -p="0G159L19EN62UKOZHN6Q3NLR3LWIO09BCV5SIKO4EOS8SKHA43687LZYJEGDB6C2" quay.io

# TODO: image variable substitution prior to building bundle image
echo -e "Dumping IMAGE env vars\n"
env | grep IMAGE
echo -e "\n\n"

IMAGE_ELASTICSEARCH_OPERATOR=${IMAGE_ELASTICSEARCH_OPERATOR:-quay.io/openshift/origin-elasticsearch-operator:latest}
IMAGE_ELASTICSEARCH6=${IMAGE_ELASTICSEARCH6:-quay.io/openshift/origin-logging-elasticsearch6:latest}
IMAGE_ELASTICSEARCH_PROXY=${IMAGE_ELASTICSEARCH_PROXY:-quay.io/openshift/origin-elasticsearch-proxy:latest}
IMAGE_OAUTH_PROXY=${IMAGE_OAUTH_PROXY:-quay.io/openshift/origin-oauth-proxy:latest}
IMAGE_LOGGING_KIBANA6=${IMAGE_LOGGING_KIBANA6:-quay.io/openshift/origin-logging-kibana6:latest}

CRD_FILE="$(ls bundle/manifests/elasticsearch-operator*.yaml)"
temp_crd="$(mktemp olm_deploy/bundles/crd-XXXX.bkup)"
cat $CRD_FILE > "$temp_crd"

trap "[ -n $temp_crd ] && cat $temp_crd > $CRD_FILE && rm -f $temp_crd" SIGINT SIGTERM EXIT

# update the manifest with the image built by ci
sed -i "s,quay.io/openshift/origin-elasticsearch-operator:latest,${IMAGE_ELASTICSEARCH_OPERATOR}," $CRD_FILE
sed -i "s,quay.io/openshift/origin-logging-elasticsearch6:latest,${IMAGE_ELASTICSEARCH6}," $CRD_FILE
sed -i "s,quay.io/openshift/origin-elasticsearch-proxy:latest,${IMAGE_ELASTICSEARCH_PROXY}," $CRD_FILE
sed -i "s,quay.io/openshift/origin-oauth-proxy:latest,${IMAGE_OAUTH_PROXY}," $CRD_FILE
sed -i "s,quay.io/openshift/origin-logging-kibana6:latest,${IMAGE_LOGGING_KIBANA6}," $CRD_FILE

# update the manifest to pull always the operator image for non-CI environments
if [ -z "${IMAGE_FORMAT:-}" ] ; then
    echo -e "Set operator deployment's imagePullPolicy to 'Always'\n\n"
    sed -i 's,imagePullPolicy:\ IfNotPresent,imagePullPolicy:\ Always,' $CRD_FILE
fi

IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE="quay.io/ewolinet/elasticsearch-operator-bundle-test"
echo "Building operator bundle image ${IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE}"
podman build -f bundle.Dockerfile -t ${IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE} .

echo "Pushing image ${IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE}"
podman push --tls-verify=false ${IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE}

export IMAGE_ELASTICSEARCH_OPERATOR_INDEX="quay.io/ewolinet/elasticsearch-operator-index-test"

#$OPM index add --skip-tls --bundles ${REMOTE_BUNDLE_IMAGE}:latest --tag ${IMAGE_ELASTICSEARCH_OPERATOR_INDEX} --permissive
$OPM index add --skip-tls --bundles ${IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE}:latest --tag ${IMAGE_ELASTICSEARCH_OPERATOR_INDEX} -p podman

# push this to the image registry
podman push --tls-verify=false ${IMAGE_ELASTICSEARCH_OPERATOR_INDEX}

if oc get project ${ELASTICSEARCH_OPERATOR_NAMESPACE} > /dev/null 2>&1 ; then
  echo using existing project ${ELASTICSEARCH_OPERATOR_NAMESPACE} for operator installation
else
  oc create namespace ${ELASTICSEARCH_OPERATOR_NAMESPACE}
fi


envsubst < olm_deploy/bundles/catalog-source.yaml | oc create -n ${ELASTICSEARCH_OPERATOR_NAMESPACE} -f -
oc create -f olm_deploy/bundles/operator-group.yaml -n ${ELASTICSEARCH_OPERATOR_NAMESPACE}
oc create -f olm_deploy/bundles/subscription.yaml -n ${ELASTICSEARCH_OPERATOR_NAMESPACE}