#!/bin/bash

source .bingo/variables.env

set -euo pipefail

MANIFESTS_DIR=${1:-"manifests/${OCP_VERSION}"}

echo "--------------------------------------------------------------"
echo "Generate k8s golang code"
echo "--------------------------------------------------------------"
$CONTROLLER_GEN object paths="./apis/..."

echo "--------------------------------------------------------------"
echo "Generate CRDs for apiVersion v1"
echo "--------------------------------------------------------------"
# $OPERATOR_SDK generate crds --crd-version v1
# mv deploy/crds/*.yaml "${MANIFESTS_DIR}"
# CRD_OPTIONS?="crd:crdVersions=v1"
$CONTROLLER_GEN crd:crdVersions=v1 rbac:roleName=elasticsearch-operator paths="./apis/..." output:crd:artifacts:config=config/crd/bases
# TODO: copy generated CRD, RBAC to MANIFESTS_DIR

echo "---------------------------------------------------------------"
echo "Cleanup operator-sdk generation folder"
echo "---------------------------------------------------------------"
# rm -rf deploy
