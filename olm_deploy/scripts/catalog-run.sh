#!/bin/bash

DB="${1:-/registry/index.db}"

/usr/bin/catalog-init.sh "$DB"
/usr/bin/opm registry serve --database "$DB" --debug --skip-tls
