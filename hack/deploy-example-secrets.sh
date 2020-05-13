#!/bin/bash

set -euo pipefail

namespace=$1

oc -n $namespace delete secret elasticsearch ||:
oc -n $namespace create secret generic elasticsearch  \
	--from-file=admin-key=/tmp/example-secrets/system.admin.key \
	--from-file=admin-cert=/tmp/example-secrets/system.admin.crt \
	--from-file=admin-ca=/tmp/example-secrets/ca.crt \
	--from-file=/tmp/example-secrets/elasticsearch.key \
	--from-file=/tmp/example-secrets/elasticsearch.crt \
	--from-file=/tmp/example-secrets/logging-es.key \
	--from-file=/tmp/example-secrets/logging-es.crt

