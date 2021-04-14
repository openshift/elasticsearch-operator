package indexmanagement

const checkRollover = `
#!/bin/python

import json,sys

try:
  fileToRead = sys.argv[1]
  currentIndex = sys.argv[2]
except IndexError:
  raise SystemExit("Usage: % <file_to_read> <current_index>" % {sys.argv[0]})

try:
  with open(fileToRead) as f:
    data = json.load(f)
except ValueError:
  raise SystemExit("Invalid JSON: %" % {f})

try:
  if not data['acknowledged']:
    if not data['rolled_over']:
      for condition in data['conditions']:
        if data['conditions'][condition]:
          print("Index was not rolled over despite meeting conditions to do so: %" % {data['conditions']})
          sys.exit(1)

      print(data['old_index'])
      sys.exit(0)

  if data['old_index'] != currentIndex:
    print("old index % does not match expected index %" % ({data['old_index']}, {currentIndex}))
    sys.exit(1)

  print(data['new_index'])
except KeyError as e:
  raise SystemExit("Unable to check rollover for %: missing key %" % ({data}, {e}))
`

const getWriteIndex = `
#!/bin/python

import json,sys

try:
  alias = sys.argv[1]
  output = sys.argv[2]
except IndexError:
  raise SystemExit("Usage: % <write_alias> <json_response>" % {sys.argv[0]})

try:
  response = json.loads(output)
except ValueError:
  raise SystemExit("Invalid JSON: %" % {output})

lastIndex = len(response) - 1

if 'error' in response[lastIndex]:
  print("Error while attemping to determine the active write alias: %" % {response[lastIndex]})
  sys.exit(1)

data = response[lastIndex]

try:
  writeIndex = [index for index in data if data[index]['aliases'][alias].get('is_write_index')]
  if len(writeIndex) > 0:
    writeIndex.sort(reverse=True)
    print(" ".join(writeIndex))
except:
  e = sys.exc_info()[0]
  raise SystemExit("Error trying to determine the 'write' index from %: %" % ({data}, {e}))
`

const getNext25Indices = `
#!/bin/python

from __future__ import print_function
import json,sys

try:
  minAgeFromEpoc = sys.argv[1]
  writeIndex = sys.argv[2]
  output = sys.argv[3]
except IndexError:
  raise SystemExit("Usage: % <minAgeFromEpoc> <writeIndex> <json_response>" % {sys.argv[0]})

try:
  response = json.loads(output)
except ValueError:
  raise SystemExit("Invalid JSON: %" % {output})

lastIndex = len(response) - 1

if 'error' in response[lastIndex]:
  print("Error while attemping to determine index creation dates: %" % {response[lastIndex]})
  sys.exit(1)

r = response[lastIndex]

indices = []
for index in r:
  try:
    if 'settings' in r[index]:
      settings = r[index]['settings']
      if 'index' in settings:
        meta = settings['index']
        if 'creation_date' in meta:
          creation_date = meta['creation_date']
          if int(creation_date) < int(minAgeFromEpoc):
            indices.append(index)
        else:
          sys.stderr.write("'creation_date' missing from index settings: %r" % (meta))
      else:
        sys.stderr.write("'index' missing from setting: %r" % (settings))
    else:
      sys.stderr.write("'settings' missing for %r" % (index))
  except:
    e = sys.exc_info()[0]
    sys.stderr.write("Error trying to evaluate index from '%r': %r" % (r,e))
if writeIndex in indices:
  indices.remove(writeIndex)
for i in range(0, len(indices), 25):
  print(','.join(indices[i:i+25]))
`

const indexManagement = `

CONNECT_TIMEOUT=${CONNECT_TIMEOUT:-30}

function getWriteIndex() {

  local policy="$1"

  # find out the current write index for ${POLICY_MAPPING}-write and check if there is the next generation of it
  aliasResponse=$(curl -s $ES_SERVICE/_alias/${policy} \
    --cacert /etc/indexmanagement/keys/admin-ca \
    --connect-timeout ${CONNECT_TIMEOUT} \
    -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
    -HContent-Type:application/json \
    --retry 5 \
    --retry-delay 5)

  if [ -z "$aliasResponse" ]; then
    echo "Received an empty response from elasticsearch -- server may not be ready"
    exit 1
  fi

  jsonResponse="$(echo [$aliasResponse] | sed 's/}{/},{/g')"

  echo "calling getWriteIndex.py: ${policy} $jsonResponse"

  if ! writeIndices="$(python /tmp/scripts/getWriteIndex.py "${policy}" "$jsonResponse")" ; then
    echo $writeIndices
    exit 1
  fi

  writeIndex="$(ensureOneWriteIndex "$policy" "$writeIndices")"

  echo $writeIndex
}

function ensureOneWriteIndex() {

  local policy="$1"
  local writeIndices="$2"

  # first index received is the latest one
  writeIndex=""
  for index in $writeIndices; do
    if [ -z "$writeIndex" ]; then
      writeIndex="$index"
    else
      # extra write index -- mark it as not a write index
      curl -s "$ES_SERVICE/_aliases" \
        --connect-timeout ${CONNECT_TIMEOUT} \
        --cacert /etc/indexmanagement/keys/admin-ca \
        -HContent-Type:application/json \
        -XPOST \
        -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
        -o /dev/null \
        -d '{"actions":[{"add":{"index": "'$index'", "alias": "'${POLICY_MAPPING}-write'", "is_write_index": false}}]}' \
        --retry 5 \
        --retry-delay 5
    fi
  done

  echo $writeIndex
}
`

const rolloverScript = `
set -euo pipefail
source /tmp/scripts/indexManagement

decoded=$(echo $PAYLOAD | base64 -d)

echo "Index management rollover process starting"

# get current write index
if ! writeIndex="$(getWriteIndex "${POLICY_MAPPING}-write")" ; then
  echo $writeIndex
  exit 1
fi

echo "Current write index for ${POLICY_MAPPING}-write: $writeIndex"

# try to rollover
code=$(curl -s "$ES_SERVICE/${POLICY_MAPPING}-write/_rollover?pretty" \
  -w "%{response_code}" \
  --connect-timeout ${CONNECT_TIMEOUT} \
  --cacert /etc/indexmanagement/keys/admin-ca \
  -HContent-Type:application/json \
  -XPOST \
  -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  -o /tmp/response.txt \
  -d $decoded \
  --retry 5 \
  --retry-delay 5)

echo "Checking results from _rollover call"

if [ "$code" != "200" ] ; then
  # already in bad state
  echo "Calculating next write index based on current write index..."

  indexGeneration="$(echo $writeIndex | cut -d'-' -f2)"
  writeBase="$(echo $writeIndex | cut -d'-' -f1)"

  # if we don't strip off the leading 0s it does math wrong...
  generation=$(echo $indexGeneration | sed 's/^0*//')
  # pad the index name again with 0s
  nextGeneration="$(printf '%06g' $(($generation + 1)))"
  nextIndex="$writeBase-$nextGeneration"
else
  # check response to see if it did roll over (field in response)
  if ! nextIndex="$(python /tmp/scripts/checkRollover.py "/tmp/response.txt" "$writeIndex")" ; then
    echo $nextIndex
    exit 1
  fi
fi

echo "Next write index for ${POLICY_MAPPING}-write: $nextIndex"
echo "Checking if $nextIndex exists"

# if true, ensure next index was created
code=$(curl -s "$ES_SERVICE/$nextIndex/" \
  -w "%{response_code}" \
  --connect-timeout ${CONNECT_TIMEOUT} \
  --cacert /etc/indexmanagement/keys/admin-ca \
  -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  -o /dev/null \
  --retry 5 \
  --retry-delay 5)
if [ "$code" == "404" ] ; then
  cat /tmp/response.txt
  exit 1
fi

echo "Checking if $nextIndex is the write index for ${POLICY_MAPPING}-write"

## if true, ensure write-alias points to next index
if ! writeIndex="$(getWriteIndex "${POLICY_MAPPING}-write")" ; then
  echo $writeIndex
  exit 1
fi

if [ "$nextIndex" == "$writeIndex" ] ; then
  echo "Done!"
  exit 0
fi

echo "Updating alias for ${POLICY_MAPPING}-write"

# else - try to update alias to be correct
code=$(curl -s "$ES_SERVICE/_aliases" \
  -w "%{response_code}" \
  --connect-timeout ${CONNECT_TIMEOUT} \
  --cacert /etc/indexmanagement/keys/admin-ca \
  -HContent-Type:application/json \
  -XPOST \
  -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  -o /tmp/response.txt \
  -d '{"actions":[{"add":{"index": "'$writeIndex'", "alias": "'${POLICY_MAPPING}-write'", "is_write_index": false}},{"add":{"index": "'$nextIndex'", "alias": "'${POLICY_MAPPING}-write'", "is_write_index": true}}]}' \
  --retry 5 \
  --retry-delay 5)

if [ "$code" == 200 ] ; then
  echo "Done!"
  exit 0
fi
cat /tmp/response.txt
exit 1
`
const deleteScript = `
set -uo pipefail
ERRORS=/tmp/errors.txt

source /tmp/scripts/indexManagement

echo "" > $ERRORS

echo "Index management delete process starting"

if ! writeIndex="$(getWriteIndex "${POLICY_MAPPING}-write")" ; then
  echo $writeIndex
  exit 1
fi

indices=$(curl -s $ES_SERVICE/${POLICY_MAPPING}/_settings/index.creation_date \
  --cacert /etc/indexmanagement/keys/admin-ca \
  --connect-timeout ${CONNECT_TIMEOUT} \
  -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  -HContent-Type:application/json \
  --retry 5 \
  --retry-delay 5)

if [ -z "$indices" ]; then
  echo "Received an empty response from elasticsearch -- server may not be ready"
  exit 1
fi

jsonResponse="$(echo [$indices] | sed 's/}{/},{/g')"

# Delete in batches of 25 for cases where there are a large number of indices to remove
nowInMillis=$(date +%s%3N)
minAgeFromEpoc=$(($nowInMillis - $MIN_AGE))

if ! indices=$(python /tmp/scripts/getNext25Indices.py "$minAgeFromEpoc" "$writeIndex" "$jsonResponse" 2>>$ERRORS) ; then
  cat $ERRORS
  exit 1
fi
# Dump any findings to stdout but don't error
if [ -s $ERRORS ]; then
  cat $ERRORS
fi

if [ "${indices}" == "" ] ; then
    echo No indices to delete
    exit 0
else
    echo deleting indices: "${indices}"
fi

for sets in ${indices}; do
  code=$(curl -s $ES_SERVICE/${sets}?pretty \
    -w "%{response_code}" \
    --connect-timeout ${CONNECT_TIMEOUT} \
    --cacert /etc/indexmanagement/keys/admin-ca \
    -HContent-Type:application/json \
    -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
    -o /tmp/response.txt \
    -XDELETE \
    --retry 5 \
    --retry-delay 5)

  if [ "$code" != 200 ] ; then
    cat /tmp/response.txt
    exit 1
  fi
done

echo "Done!"
`

var scriptMap = map[string]string{
	"delete":              deleteScript,
	"rollover":            rolloverScript,
	"indexManagement":     indexManagement,
	"getWriteIndex.py":    getWriteIndex,
	"checkRollover.py":    checkRollover,
	"getNext25Indices.py": getNext25Indices,
}
