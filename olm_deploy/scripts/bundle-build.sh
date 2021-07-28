#!/bin/bash
set -eou pipefail

IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE=${LOCAL_IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE:-$IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE}
IMAGE_ELASTICSEARCH_OPERATOR=${IMAGE_ELASTICSEARCH_OPERATOR:-quay.io/openshift-logging/elasticsearch-operator:latest}
IMAGE_KUBE_RBAC_PROXY=${IMAGE_KUBE_RBAC_PROXY:-quay.io/openshift/origin-kube-rbac-proxy:latest}
IMAGE_ELASTICSEARCH6=${IMAGE_ELASTICSEARCH6:-quay.io/openshift-logging/elasticsearch6:latest}
IMAGE_ELASTICSEARCH_PROXY=${IMAGE_ELASTICSEARCH_PROXY:-quay.io/openshift-logging/elasticsearch-proxy:latest}
IMAGE_OAUTH_PROXY=${IMAGE_OAUTH_PROXY:-quay.io/openshift/origin-oauth-proxy:latest}
IMAGE_KIBANA6=${IMAGE_KIBANA6:-quay.io/openshift-logging/kibana6:latest}
IMAGE_CURATOR5=${IMAGE_CURATOR5:-quay.io/openshift-logging/curator5:latest}

echo "Building operator bundle image ${IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE}"
podman build \
       -f olm_deploy/bundle/Dockerfile \
       -t ${IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE} \
       --build-arg IMAGE_ELASTICSEARCH_OPERATOR="${IMAGE_ELASTICSEARCH_OPERATOR}" \
       --build-arg IMAGE_KUBE_RBAC_PROXY="${IMAGE_KUBE_RBAC_PROXY}" \
       --build-arg IMAGE_ELASTICSEARCH6="${IMAGE_ELASTICSEARCH6}" \
       --build-arg IMAGE_ELASTICSEARCH_PROXY="${IMAGE_ELASTICSEARCH_PROXY}" \
       --build-arg IMAGE_OAUTH_PROXY="${IMAGE_OAUTH_PROXY}" \
       --build-arg IMAGE_KIBANA6="${IMAGE_KIBANA6}" \
       --build-arg IMAGE_CURATOR5="${IMAGE_CURATOR5}" \
       --build-arg OPENSHIFT_CI="true" \
       .

if [ -n ${LOCAL_IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE} ] ; then
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
echo "Pushing image ${IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE}"
podman push --tls-verify=false ${IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE}
