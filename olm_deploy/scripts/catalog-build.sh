#!/bin/sh
set -eou pipefail

echo "Building operator registry image ${IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY}"
docker build -f olm_deploy/operatorregistry/Dockerfile -t ${IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY} .

echo "Pushing operator registry image ${IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY}"
docker push ${IMAGE_ELASTICSEARCH_OPERATOR_REGISTRY}
