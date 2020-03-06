#!/bin/bash

set -eou pipefail

echo -e "Dumping IMAGE env vars\n"
env | grep IMAGE
echo -e "\n\n"

IMAGE_ELASTICSEARCH_OPERATOR=${IMAGE_ELASTICSEARCH_OPERATOR:-quay.io/openshift/origin-elasticsearch-operator:latest}
if [ -n "${IMAGE_FORMAT:-}" ] ; then
  IMAGE_ELASTICSEARCH_OPERATOR=$(sed -e "s,\${component},elasticsearch-operator," <(echo $IMAGE_FORMAT))
fi

IMAGE_ELASTICSEARCH6=${IMAGE_ELASTICSEARCH6:-quay.io/openshift/origin-logging-elasticsearch6:latest}
if [ -n "${IMAGE_FORMAT:-}" ] ; then
  IMAGE_ELASTICSEARCH6=$(sed -e "s,\${component},logging-elasticsearch6," <(echo $IMAGE_FORMAT))
fi

IMAGE_OAUTH_PROXY=${IMAGE_OAUTH_PROXY:-quay.io/openshift/origin-oauth-proxy:latest}
if [ -n "${IMAGE_FORMAT:-}" ] ; then
  IMAGE_OAUTH_PROXY=$(sed -e "s,\${component},oauth-proxy," <(echo $IMAGE_FORMAT))
fi

IMAGE_LOGGING_KIBANA6=${IMAGE_OAUTH_PROXY:-quay.io/openshift/origin-logging-kibana6:latest}
if [ -n "${IMAGE_FORMAT:-}" ] ; then
  IMAGE_LOGGING_KIBANA6=$(sed -e "s,\${component},logging-kibana6," <(echo $IMAGE_FORMAT))
fi


# update the manifest with the image built by ci
sed -i "s,quay.io/openshift/origin-elasticsearch-operator:latest,${IMAGE_ELASTICSEARCH_OPERATOR}," /manifests/*/*clusterserviceversion.yaml
sed -i "s,quay.io/openshift/origin-logging-elasticsearch6:latest,${IMAGE_ELASTICSEARCH6}," /manifests/*/*clusterserviceversion.yaml
sed -i "s,quay.io/openshift/origin-oauth-proxy:latest,${IMAGE_OAUTH_PROXY}," /manifests/*/*clusterserviceversion.yaml
sed -i "s,quay.io/openshift/origin-logging-kibana6:latest,${IMAGE_LOGGING_KIBANA6}," /manifests/*/*clusterserviceversion.yaml

echo -e "substitution complete, dumping new csv\n\n"
cat /manifests/*/*clusterserviceversion.yaml

echo "generating sqlite database"

/usr/bin/initializer --manifests=/manifests --output=/bundle/bundles.db --permissive=true
