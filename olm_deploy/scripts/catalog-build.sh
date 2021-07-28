#!/bin/bash

set -eou pipefail

source .bingo/variables.env

echo -e "Dumping IMAGE env vars\n"
env | grep IMAGE
echo -e "\n\n"

IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE=${LOCAL_IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE:-$IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE}
IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY=${LOCAL_IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY:-$IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY}

if [ -n ${LOCAL_IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY} ] ; then
    coproc oc -n openshift-image-registry port-forward service/image-registry 5000:5000
    trap "kill -15 $COPROC_PID" EXIT
    read PORT_FORWARD_STDOUT <&"${COPROC[0]}"
    if [[ "$PORT_FORWARD_STDOUT" =~ ^Forwarding.*5000$ ]] ; then
        user=$(oc whoami | sed s/://)
        podman login --tls-verify=false -u ${user} -p $(oc whoami -t) 127.0.0.1:5000
    else
        echo "Unexpected message from oc port-forward: $PORT_FORWARD_STDOUT"
    fi
fi

echo "Generating operator registry db for ${IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY}"
#
# TODO: Remove --generate and subsequent `podman build` call when upstream fix available
#       https://github.com/operator-framework/operator-registry/issues/619
#
$OPM index add \
    -c podman \
    --skip-tls \
    --bundles "${IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE}" \
    --tag "${IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY}" \
    --generate

echo "Building image ${IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY}"
podman build -t "${IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY}" -f olm_deploy/operatorregistry/Dockerfile .

echo "Pushing image ${IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY}"
podman push --tls-verify=false "${IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY}"
