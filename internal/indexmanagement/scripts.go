package indexmanagement

const indexManagementClient = `
#!/bin/python

import os, sys, ast
import json
from elasticsearch import Elasticsearch
from elasticsearch_dsl import Search, Q
from ssl import create_default_context

def getEsClient():
  esService = os.environ['ES_SERVICE']
  connectTimeout = os.getenv('CONNECT_TIMEOUT', 30)

  tokenFile = open('/var/run/secrets/kubernetes.io/serviceaccount/token', 'r')
  bearer_token = tokenFile.read()

  context = create_default_context(cafile='/etc/indexmanagement/keys/admin-ca')
  es_client = Elasticsearch([esService],
    timeout=connectTimeout,
    max_retries=5,
    retry_on_timeout=True,
    headers={"authorization": f"Bearer {bearer_token}"},
    ssl_context=context)
  return es_client

def getAlias(alias):
  try:
    es_client = getEsClient()
    return json.dumps(es_client.indices.get_alias(name=alias))
  except:
    return ""

def getIndicesAgeForAlias(alias):
  try:
    es_client = getEsClient()
    return json.dumps(es_client.indices.get_settings(index=alias, name="index.creation_date"))
  except:
    return ""

def deleteIndices(index):
  original_stdout = sys.stdout
  try:
    es_client = getEsClient()
    response = es_client.indices.delete(index=index)
    return True
  except Exception as e:
    sys.stdout = open('/tmp/response.txt', 'w')
    print(e)
    sys.stdout = original_stdout
    return False

def deleteByQuery(index, namespaceSpecs, defaultAge):
  original_stdout = sys.stdout
  try:
    es_client = getEsClient()
    s = Search(using=es_client)

    #Extract string values of namespace and minAge from namespaceSpecs to feed into dsl-query
    namespaceSpecs = json.loads(namespaceSpecs)
    for namespaceName in namespaceSpecs:
      s = s.query('prefix', kubernetes__namespace_name=namespaceName)
      minAge = namespaceSpecs[namespaceName]
      if minAge == "":
        s = s.filter('range', **{'@timestamp': {'lt': 'now-{}'.format(defaultAge)}})
      else:
        s = s.filter('range', **{'@timestamp': {'lt': 'now-{}'.format(minAge)}})
      response = es_client.delete_by_query(index=index, body=s.to_dict(), doc_type="_doc")
    return True
  except Exception as e:
    sys.stdout = open('/tmp/response.txt', 'w')
    print(e)
    sys.stdout = original_stdout
    return False

def removeAsWriteIndexForAlias(index, alias):
  es_client = getEsClient()
  response = es_client.indices.update_aliases({
    "actions": [
        {"add":{"index": f"{index}", "alias": f"{alias}", "is_write_index": False}}
    ]
  })
  return response['acknowledged']

def catWriteAliases(policy):
  try:
    es_client = getEsClient()
    alias_name = f"{policy}*-write"
    response = es_client.cat.aliases(name=alias_name, h="alias")
    response_list = list(response.split("\n"))
    return " ".join(sorted(set(response_list)))
  except:
    return ""

def rolloverForPolicy(alias, decoded):
  original_stdout = sys.stdout
  try:
    es_client = getEsClient()
    response = es_client.indices.rollover(alias=alias, body=decoded)
    sys.stdout = open('/tmp/response.txt', 'w')
    print(json.dumps(response))
    sys.stdout = original_stdout
    return True
  except:
    return False

def checkIndexExists(index):
  original_stdout = sys.stdout
  try:
    es_client = getEsClient()
    return es_client.indices.exists(index=index)
  except Exception as e:
    sys.stdout = open('/tmp/response.txt', 'w')
    print(e)
    sys.stdout = original_stdout
    return False

def updateWriteIndex(currentIndex, nextIndex, alias):
  original_stdout = sys.stdout
  try:
    es_client = getEsClient()
    response = es_client.indices.update_aliases({
      "actions": [
          {"add":{"index": f"{currentIndex}", "alias": f"{alias}", "is_write_index": False}},
          {"add":{"index": f"{nextIndex}", "alias": f"{alias}", "is_write_index": True}}
      ]
    })
    return response['acknowledged']
  except Exception as e:
    sys.stdout = open('/tmp/response.txt', 'w')
    print(e)
    sys.stdout = original_stdout
    return False
`

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
  responseRollover="$(rolloverForPolicy "${policy}-write" "$decoded")"

  echo "Checking results from _rollover call"

  if [ "$responseRollover" == False ] ; then
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
  indexExists="$(checkIndexExists "$nextIndex")"
  if [ "$indexExists" == False ] ; then
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
  responseUpdateWriteIndex="$(updateWriteIndex "$writeIndex" "$nextIndex" "${policy}-write")"

  if [ "$responseUpdateWriteIndex" == True ] ; then
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
  echo "indices = [$indices]"

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
      echo "No indices to delete"
      return 0
  else
      echo deleting indices: "${indices}"
  fi

  for sets in ${indices}; do
  
    response="$(deleteIndices "${sets}")"

    if [ "$response" == False ] ; then
      cat /tmp/response.txt
      return 1
    fi
  done

  echo "Done!"
}

function getAlias() {
  local alias="$1"

  python -c 'import indexManagementClient; print(indexManagementClient.getAlias("'$alias'"))'
}

function getIndicesAgeForAlias() {
  local alias="$1"

  python -c 'import indexManagementClient; print(indexManagementClient.getIndicesAgeForAlias("'$alias'"))'
}

function deleteIndices() {
  local index="$1"

  python -c 'import indexManagementClient; print(indexManagementClient.deleteIndices("'$index'"))'
}

function pruneNamespaces() {
  local indexMappings="$1"
  local namespacespec="$2"
  local defaultAge="$3"

  echo "========================"
  echo "Index management prune process starting for $indexMappings"
  echo "namespace_spec = $namespacespec"

  python -c 'import indexManagementClient; print(indexManagementClient.deleteByQuery("'$indexMappings'","'$namespacespec'","'$defaultAge'"))'
}

function removeAsWriteIndexForAlias() {
  local index="$1"
  local alias="$2"

  python -c 'import indexManagementClient; print(indexManagementClient.removeAsWriteIndexForAlias("'$index'","'$alias'"))' > /tmp/response.txt
}

function catWriteAliases() {
  local policy="$1"

  python -c 'import indexManagementClient; print(indexManagementClient.catWriteAliases("'$policy'"))'
}

function rolloverForPolicy() {
  local policy="$1"
  local decoded="$2"

  python -c 'import indexManagementClient; print(indexManagementClient.rolloverForPolicy("'$policy'",'$decoded'))'
}

function checkIndexExists() {
  local index="$1"

  python -c 'import indexManagementClient; print(indexManagementClient.checkIndexExists("'$index'"))'
}

function updateWriteIndex() {
  currentIndex="$1"
  nextIndex="$2"
  alias="$3"

  python -c 'import indexManagementClient; print(indexManagementClient.updateWriteIndex("'$currentIndex'","'$nextIndex'","'$alias'"))'
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

const deleteThenRolloverScript = `
set -uo pipefail

/tmp/scripts/delete
delete_rc=$?

/tmp/scripts/rollover
rollover_rc=$?

if [ $delete_rc -ne 0 ] || [ $rollover_rc -ne 0 ]; then
    exit 1
fi

exit 0
`

const pruneNamespacesScript = `
set -uo pipefail

source /tmp/scripts/indexManagement

DEFAULT_AGE="7d"

if  [ -z "$NAMESPACE_SPECS" ] ;  then
  echo "No namespaces to prune"
  exit 1
fi

namespaceSpec="$(echo $NAMESPACE_SPECS | sed 's/\"/\\\"/g')"

# Prune namespaces runs on all current index patterns
indexMappings="$(echo $POLICY_MAPPING*)"

if ! pruneNamespaces "$indexMappings" "$namespaceSpec" "$DEFAULT_AGE" ; then
    exit 1
fi

exit 0
`

var scriptMap = map[string]string{
	"delete":                   deleteScript,
	"rollover":                 rolloverScript,
	"delete-then-rollover":     deleteThenRolloverScript,
	"prune-namespaces":         pruneNamespacesScript,
	"indexManagement":          indexManagement,
	"getWriteIndex.py":         getWriteIndex,
	"checkRollover.py":         checkRollover,
	"getNext25Indices.py":      getNext25Indices,
	"indexManagementClient.py": indexManagementClient,
}
