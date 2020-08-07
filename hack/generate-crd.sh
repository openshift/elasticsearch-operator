#!/usr/bin/bash

source .bingo/variables.env

set -euo pipefail

MANIFESTS_DIR=${1:-"manifests/${OCP_VERSION}"}
ES_CRD_FILE="logging.openshift.io_elasticsearches_crd.yaml"
KB_CRD_FILE="logging.openshift.io_kibanas_crd.yaml"

echo "--------------------------------------------------------------"
echo "Generate k8s golang code"
echo "--------------------------------------------------------------"
$OPERATOR_SDK generate k8s

echo "--------------------------------------------------------------"
echo "Generate CRDs for apiVersion v1beta1"
echo "--------------------------------------------------------------"
$OPERATOR_SDK generate crds --crd-version v1beta1
mv deploy/crds/*.yaml "${MANIFESTS_DIR}"

echo "---------------------------------------------------------------"
echo "Kustomize: Patch CRDs for backward-compatibility"
echo "---------------------------------------------------------------"
oc kustomize "${MANIFESTS_DIR}"  | \
    awk -v es="${MANIFESTS_DIR}/${ES_CRD_FILE}" \
        -v kb="${MANIFESTS_DIR}/${KB_CRD_FILE}"\
        'BEGIN{filename = es} /---/ {getline; filename = kb}{print $0> filename}'

echo "---------------------------------------------------------------"
echo "Cleanup operator-sdk generation folder"
echo "---------------------------------------------------------------"
rm -rf deploy
