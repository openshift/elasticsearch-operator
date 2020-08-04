#!/bin/sh
set -exou pipefail

#source variables.env
OPM="${GOPATH}/src/github.com/operator-framework/operator-registry/bin/opm"

# TODO: image variable substitution prior to building bundle image

IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE=${IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE:-$LOCAL_IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE}
echo "Building operator bundle image ${IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE}"
podman build -f bundle.Dockerfile -t ${IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE} .

if [ -n ${IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE} ] ; then
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

# this works if we use a public quay.io repo -- login to one here

# push this to a local docker registry
# also needs to be pushed to image registry
podman push --tls-verify=false ${IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE}

#REMOTE_BUNDLE_IMAGE="$(oc get is -n openshift elasticsearch-operator-bundle -o jsonpath='{.status.dockerImageRepository}')"
export IMAGE_ELASTICSEARCH_OPERATOR_INDEX=${IMAGE_ELASTICSEARCH_OPERATOR_INDEX:-$LOCAL_IMAGE_ELASTICSEARCH_OPERATOR_INDEX}

#$OPM index add --skip-tls --bundles ${REMOTE_BUNDLE_IMAGE}:latest --tag ${IMAGE_ELASTICSEARCH_OPERATOR_INDEX} --permissive
$OPM index add --skip-tls --bundles ${IMAGE_ELASTICSEARCH_OPERATOR_BUNDLE}:latest --tag ${IMAGE_ELASTICSEARCH_OPERATOR_INDEX} -p podman

# this works if we use a public quay.io repo -- login to one here

# push this to the image registry
podman push --tls-verify=false ${IMAGE_ELASTICSEARCH_OPERATOR_INDEX}

if oc get project ${ELASTICSEARCH_OPERATOR_NAMESPACE} > /dev/null 2>&1 ; then
  echo using existing project ${ELASTICSEARCH_OPERATOR_NAMESPACE} for operator installation
else
  oc create namespace ${ELASTICSEARCH_OPERATOR_NAMESPACE}
fi

#export REMOTE_INDEX_IMAGE="$(oc get is -n openshift elasticsearch-operator-index -o jsonpath='{.status.dockerImageRepository}')"
envsubst < olm_deploy/bundles/catalog-source.yaml | oc create -n ${ELASTICSEARCH_OPERATOR_NAMESPACE} -f -

oc create -f olm_deploy/bundles/operator-group.yaml -n ${ELASTICSEARCH_OPERATOR_NAMESPACE}

oc create -f olm_deploy/bundles/subscription.yaml -n ${ELASTICSEARCH_OPERATOR_NAMESPACE}