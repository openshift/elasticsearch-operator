#!/bin/bash
set -eou pipefail
source $(dirname "${BASH_SOURCE[0]}")/env.sh

echo "Building operator registry image ${IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY}"
podman build -f olm_deploy/operatorregistry/Dockerfile -t ${IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY} .

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
echo "Pushing image ${IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY}"
podman push  --tls-verify=false ${IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY}
