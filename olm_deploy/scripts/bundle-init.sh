#!/bin/bash

set -eou pipefail

echo -e "Dumping IMAGE env vars\n"
env | grep IMAGE
echo -e "\n\n"

IMAGE_ELASTICSEARCH_OPERATOR=${IMAGE_ELASTICSEARCH_OPERATOR:-quay.io/openshift-logging/elasticsearch-operator:latest}
IMAGE_KUBE_RBAC_PROXY=${IMAGE_KUBE_RBAC_PROXY:-quay.io/openshift/origin-kube-rbac-proxy:latest}
IMAGE_ELASTICSEARCH6=${IMAGE_ELASTICSEARCH6:-quay.io/openshift-logging/elasticsearch6:latest}
IMAGE_ELASTICSEARCH_PROXY=${IMAGE_ELASTICSEARCH_PROXY:-quay.io/openshift-logging/elasticsearch-proxy:latest}
IMAGE_OAUTH_PROXY=${IMAGE_OAUTH_PROXY:-quay.io/openshift/origin-oauth-proxy:latest}
IMAGE_KIBANA6=${IMAGE_KIBANA6:-quay.io/openshift-logging/kibana6:latest}
IMAGE_CURATOR5=${IMAGE_CURATOR5:-quay.io/openshift-logging/curator5:latest}

# update the manifest with the image built by ci
sed -i "s,quay.io/openshift-logging/elasticsearch-operator:latest,${IMAGE_ELASTICSEARCH_OPERATOR}," /manifests/*clusterserviceversion.yaml
sed -i "s,quay.io/openshift/origin-kube-rbac-proxy:latest,${IMAGE_KUBE_RBAC_PROXY}," /manifests/*clusterserviceversion.yaml
sed -i "s,quay.io/openshift-logging/elasticsearch6:latest,${IMAGE_ELASTICSEARCH6}," /manifests/*clusterserviceversion.yaml
sed -i "s,quay.io/openshift-logging/elasticsearch-proxy:latest,${IMAGE_ELASTICSEARCH_PROXY}," /manifests/*clusterserviceversion.yaml
sed -i "s,quay.io/openshift/origin-oauth-proxy:latest,${IMAGE_OAUTH_PROXY}," /manifests/*clusterserviceversion.yaml
sed -i "s,quay.io/openshift-logging/kibana6:latest,${IMAGE_KIBANA6}," /manifests/*clusterserviceversion.yaml
sed -i "s,quay.io/openshift-logging/curator5:latest,${IMAGE_CURATOR5}," /manifests/*clusterserviceversion.yaml

# update the manifest to pull always the operator image for non-CI environments
if [ "${OPENSHIFT_CI:-false}" == "false" ] ; then
    echo -e "Set operator deployment's imagePullPolicy to 'Always'\n\n"
    sed -i 's,imagePullPolicy:\ IfNotPresent,imagePullPolicy:\ Always,' /manifests/*clusterserviceversion.yaml
fi
