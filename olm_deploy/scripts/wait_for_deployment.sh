#!/bin/bash
# $1 - namespace
# $2 - deployment name
set -eou pipefail

retries=20
until [[ "$retries" -le "0" ]]; do
    output=$(oc get deployment -n ${1} ${2} -o jsonpath='{.metadata.name}' 2>/dev/null || echo "waiting for deployment ${1}/${2}")

    if [ "${output}" = "${2}" ] ; then
        echo "deployment ${1}/${2} has been created" >&2
        exit 0
    fi

    retries=$((retries - 1))
    echo "${output} - remaining attempts: ${retries}" >&2

    sleep 3
done

exit 1