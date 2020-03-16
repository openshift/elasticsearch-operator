#!/bin/sh
set -eou pipefail

retries=20
until [[ "$retries" -le "0" ]]; do
    output=$(oc get deployment -n ${ELASTICSEARCH_OPERATOR_NAMESPACE} elasticsearch-operator -o jsonpath='{.metadata.name}' 2>/dev/null || echo "waiting for olm to deploy the operator")

    if [ "${output}" = "elasticsearch-operator" ] ; then
        echo "${ELASTICSEARCH_OPERATOR_NAMESPACE}/elasticsearch-operator has been created" >&2
        exit 0
    fi

    retries=$((retries - 1))
    echo "${output} - remaining attempts: ${retries}" >&2

    sleep 3
done

exit 1