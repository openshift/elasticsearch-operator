#!/bin/bash

DB="${1:-/registry/index.db}"
REGISTRY="$(sqlite3 $DB 'SELECT bundlepath FROM operatorbundle')"
PRUNED_REGISTRY="${REGISTRY/127.0.0.1/image-registry.openshift-image-registry.svc}"

if [[ $REGISTRY != *"127.0.0.1"* ]]; then
    echo "Skipping mutate db as no 127.0.0.1:5000 refs found"
    exit 0
fi

echo "Update api_provider"
sqlite3 -echo "$DB" "UPDATE api_provider SET operatorbundle_path='$PRUNED_REGISTRY' WHERE operatorbundle_path='$REGISTRY'"

echo "Update operatorbundle"
sqlite3 -echo "$DB" "UPDATE operatorbundle SET bundlepath='$PRUNED_REGISTRY' WHERE bundlepath='$REGISTRY'"

echo "Update properties"
sqlite3 -echo "$DB" "UPDATE properties SET operatorbundle_path='$PRUNED_REGISTRY' WHERE operatorbundle_path='$REGISTRY'"

echo "Update related image refs"
sqlite3 -echo "$DB" "UPDATE related_image SET image='$PRUNED_REGISTRY' WHERE image='$REGISTRY'"
