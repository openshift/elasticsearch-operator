package indexmanagement

const checkRollover = `
#!/bin/python

import json,sys

try:
  fileToRead = sys.argv[1]
  currentIndex = sys.argv[2]
except IndexError:
  raise SystemExit(f"Usage: {sys.argv[0]} <file_to_read> <current_index>")

try:
  with open(fileToRead) as f:
    data = json.load(f)
except ValueError:
  raise SystemExit(f"Invalid JSON: {f}")

try:
  if not data['acknowledged']:
    if not data['rolled_over']:
      for condition in data['conditions']:
        if data['conditions'][condition]:
          print(f"Index was not rolled over despite meeting conditions to do so: {data['conditions']}")
          sys.exit(1)

      print(data['old_index'])
      sys.exit(0)

  if data['old_index'] != currentIndex:
    print(f"old index {data['old_index']} does not match expected index {currentIndex}")
    sys.exit(1)

  print(data['new_index'])
except KeyError as e:
  raise SystemExit(f"Unable to check rollover for {data}: missing key {e}")
`

const getWriteIndex = `
#!/bin/python

import json,sys

try:
  alias = sys.argv[1]
  output = sys.argv[2]
except IndexError:
  raise SystemExit(f"Usage: {sys.argv[0]} <write_alias> <json_response>")

try:
  response = json.loads(output)
except ValueError:
  raise SystemExit(f"Invalid JSON: {output}")

lastIndex = len(response) - 1

if 'error' in response[lastIndex]:
  print(f"Error while attemping to determine the active write alias: {response[lastIndex]}")
  sys.exit(1)

data = response[lastIndex]

try:
  writeIndex = [index for index in data if data[index]['aliases'][alias].get('is_write_index')]
  if len(writeIndex) > 0:
    writeIndex.sort(reverse=True)
    print(" ".join(writeIndex))
except:
  e = sys.exc_info()[0]
  raise SystemExit(f"Error trying to determine the 'write' index from {data}: {e}")
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
  raise SystemExit(f"Usage: {sys.argv[0]} <minAgeFromEpoc> <writeIndex> <json_response>")

try:
  response = json.loads(output)
except ValueError:
  raise SystemExit(f"Invalid JSON: {output}")

lastIndex = len(response) - 1

if 'error' in response[lastIndex]:
  print(f"Error while attemping to determine index creation dates: {response[lastIndex]}")
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
  aliasResponse="$(getAlias "${policy}")"

  if [ -z "$aliasResponse" ]; then
    echo "Received an empty response from elasticsearch -- server may not be ready"
    return 1
  fi

  jsonResponse="$(echo [$aliasResponse] | sed 's/}{/},{/g')"

  if ! writeIndices="$(python /tmp/scripts/getWriteIndex.py "${policy}" "$jsonResponse")" ; then
    echo $writeIndices
    return 1
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
      removeAsWriteIndexForAlias "$index" "$policy"
    fi
  done

  echo $writeIndex
}

function getWriteAliases() {
  local policy="$1"

  # "_cat/aliases/${policy}*-write?h=alias" | uniq
  aliasResponse="$(catWriteAliases "$policy")"

  if [ -z "$aliasResponse" ]; then
    echo "Received an empty response from elasticsearch -- server may not be ready"
    return 1
  fi

  echo $aliasResponse
}

function rollover() {

  local policy="$1"
  local decoded="$2"

  echo "========================"
  echo "Index management rollover process starting for $policy"
  echo ""

  # get current write index
  if ! writeIndex="$(getWriteIndex "${policy}-write")" ; then
    echo $writeIndex
    return 1
  fi

  echo "Current write index for ${policy}-write: $writeIndex"

  # try to rollover
  code="$(rolloverForPolicy "${policy}-write" "$decoded")"

  echo "Checking results from _rollover call"

  if [ "$code" != "200" ] ; then
    # already in bad state
    echo "Calculating next write index based on current write index..."

    indexGeneration="$(echo $writeIndex | cut -d'-' -f2)"
    writeBase="$(echo $writeIndex | cut -d'-' -f1)"

    # if we don't strip off the leading 0s it does math wrong...
    generation="$(echo $indexGeneration | sed 's/^0*//')"
    # pad the index name again with 0s
    nextGeneration="$(printf '%06g' $(($generation + 1)))"
    nextIndex="$writeBase-$nextGeneration"
  else
    # check response to see if it did roll over (field in response)
    if ! nextIndex="$(python /tmp/scripts/checkRollover.py "/tmp/response.txt" "$writeIndex")" ; then
      echo $nextIndex
      return 1
    fi
  fi

  echo "Next write index for ${policy}-write: $nextIndex"
  echo "Checking if $nextIndex exists"

  # if true, ensure next index was created and
  # cluster permits operations on it, e.g. not in read-only
  # state because of low disk space.
  code="$(checkIndexExists "$nextIndex")"
  if [ "$code" == "404" ] || [ "$code" == "403" ] ; then
    cat /tmp/response.txt
    return 1
  fi

  echo "Checking if $nextIndex is the write index for ${policy}-write"

  ## if true, ensure write-alias points to next index
  if ! writeIndex="$(getWriteIndex "${policy}-write")" ; then
    echo $writeIndex
    return 1
  fi

  if [ "$nextIndex" == "$writeIndex" ] ; then
    echo "Done!"
    return 0
  fi

  echo "Updating alias for ${policy}-write"

  # else - try to update alias to be correct
  code="$(updateWriteIndex "$writeIndex" "$nextIndex" "${policy}-write")"

  if [ "$code" == 200 ] ; then
    echo "Done!"
    return 0
  fi
  cat /tmp/response.txt
  return 1
}

function delete() {

  local policy="$1"
  ERRORS="$(mktemp /tmp/delete-XXXXXX)"

  echo "========================"
  echo "Index management delete process starting for $policy"
  echo ""

  if ! writeIndex="$(getWriteIndex "${policy}-write")" ; then
    echo $writeIndex
    return 1
  fi

  indices="$(getIndicesAgeForAlias "${policy}-write")"

  if [ -z "$indices" ]; then
    echo "Received an empty response from elasticsearch -- server may not be ready"
    return 1
  fi

  jsonResponse="$(echo [$indices] | sed 's/}{/},{/g')"

  # Delete in batches of 25 for cases where there are a large number of indices to remove
  nowInMillis=$(date +%s%3N)
  minAgeFromEpoc=$(($nowInMillis - $MIN_AGE))
  if ! indices=$(python /tmp/scripts/getNext25Indices.py "$minAgeFromEpoc" "$writeIndex" "$jsonResponse" 2>>$ERRORS) ; then
    cat $ERRORS
    rm $ERRORS
    return 1
  fi
  # Dump any findings to stdout but don't error
  if [ -s $ERRORS ]; then
    cat $ERRORS
    rm $ERRORS
  fi

  if [ "${indices}" == "" ] ; then
      echo No indices to delete
      return 0
  else
      echo deleting indices: "${indices}"
  fi

  for sets in ${indices}; do
    code="$(deleteIndices "${sets}")"

    if [ "$code" != 200 ] ; then
      cat /tmp/response.txt
      return 1
    fi
  done

  echo "Done!"
}

function curlES() {
  curl -s \
  --connect-timeout "${CONNECT_TIMEOUT}" \
  --cacert /etc/indexmanagement/keys/admin-ca \
  -HContent-Type:application/json \
  -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  --retry 5 \
  --retry-delay 5 \
  "$@"
}

function getAlias() {
  local alias="$1"

  curlES "$ES_SERVICE/_alias/${alias}"
}

function getIndicesAgeForAlias() {
  local alias="$1"

  curlES "$ES_SERVICE/${alias}/_settings/index.creation_date"
}

function deleteIndices() {
  local index="$1"

  curlES "$ES_SERVICE/${index}?pretty" -w "%{response_code}" -o /tmp/response.txt -XDELETE
}

function removeAsWriteIndexForAlias() {
  local index="$1"
  local alias="$2"

  curlES "$ES_SERVICE/_aliases" -o /dev/null -d '{"actions":[{"add":{"index": "'$index'", "alias": "'$alias'", "is_write_index": false}}]}'
}

function catWriteAliases() {
  local policy="$1"

  curlES "$ES_SERVICE/_cat/aliases/${policy}*-write?h=alias" | sort | uniq
}

function rolloverForPolicy() {
  local policy="$1"
  local decoded="$2"

  curlES "$ES_SERVICE/${policy}/_rollover?pretty" -w "%{response_code}" -XPOST -o /tmp/response.txt -d $decoded
}

function checkIndexExists() {
  local index="$1"

  curlES "$ES_SERVICE/${index}" -w "%{response_code}" -o /dev/null
}

function updateWriteIndex() {
  currentIndex="$1"
  nextIndex="2"
  alias="$3"

  curlES "$ES_SERVICE/_aliases" -w "%{response_code}" -XPOST -o /tmp/response.txt -d '{"actions":[{"add":{"index": "'$currentIndex'", "alias": "'$alias'", "is_write_index": false}},{"add":{"index": "'$nextIndex'", "alias": "'$alias'", "is_write_index": true}}]}'
}
`

const rolloverScript = `
set -euo pipefail
source /tmp/scripts/indexManagement

decoded=$(echo $PAYLOAD | base64 -d)

# need to get a list of all mappings under ${POLICY_MAPPING}, drop suffix '-write' iterate over
writeAliases="$(getWriteAliases "$POLICY_MAPPING")"

for aliasBase in $writeAliases; do

  alias="$(echo $aliasBase | sed 's/-write$//g')"
  if ! rollover "$alias" "$decoded" ; then
    exit 1
  fi
done
`

const deleteScript = `
set -uo pipefail

source /tmp/scripts/indexManagement

# need to get a list of all mappings under ${POLICY_MAPPING}, drop suffix '-write' iterate over
writeAliases="$(getWriteAliases "$POLICY_MAPPING")"

for aliasBase in $writeAliases; do

  alias="$(echo $aliasBase | sed 's/-write$//g')"
  if ! delete "$alias" ; then
    exit 1
  fi
done
`

var scriptMap = map[string]string{
	"delete":              deleteScript,
	"rollover":            rolloverScript,
	"indexManagement":     indexManagement,
	"getWriteIndex.py":    getWriteIndex,
	"checkRollover.py":    checkRollover,
	"getNext25Indices.py": getNext25Indices,
}
