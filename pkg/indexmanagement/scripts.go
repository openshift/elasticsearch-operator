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
  indices = sys.argv[2]
except IndexError:
  raise SystemExit(f"Usage: {sys.argv[0]} <write_alias> <write_indices>")

try:
  data = json.loads(indices)
except ValueError:
  raise SystemExit(f"Invalid JSON: {indices}")

try:
  writeIndex = [index for index in data if data[index]['aliases'][alias].get('is_write_index')]
  if len(writeIndex) > 0:
    print(writeIndex[0])
except:
  e = sys.exc_info()[0]
  raise SystemExit(f"Error trying to determine the 'write' index from {data}: {e}")
`

const rolloverScript = `
set -euo pipefail
decoded=$(echo $PAYLOAD | base64 -d)

# get current write index
# find out the current write index for ${POLICY_MAPPING}-write and check if there is the next generation of it
writeIndices=$(curl -s $ES_SERVICE/${POLICY_MAPPING}-*/_alias/${POLICY_MAPPING}-write \
  --cacert /etc/indexmanagement/keys/admin-ca \
  --connect-timeout ${CONNECT_TIMEOUT} \
  -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  -HContent-Type:application/json)

if echo "$writeIndices" | grep "\"error\"" ; then
  echo "Error while attemping to determine the active write alias: $writeIndices"
  exit 1
fi

if ! writeIndex="$(python /tmp/scripts/getWriteIndex.py "${POLICY_MAPPING}-write" "$writeIndices")" ; then
  echo $writeIndex
  exit 1
fi

echo "Current write index for ${POLICY_MAPPING}-write: $writeIndex"

# try to rollover
code=$(curl -s "$ES_SERVICE/${POLICY_MAPPING}-write/_rollover?pretty" \
  -w "%{response_code}" \
  --cacert /etc/indexmanagement/keys/admin-ca \
  -HContent-Type:application/json \
  -XPOST \
  -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  -o /tmp/response.txt \
  -d $decoded)

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
  -o /dev/null)
if [ "$code" == "404" ] ; then
  cat /tmp/response.txt
  exit 1
fi

echo "Checking if $nextIndex is the write index for ${POLICY_MAPPING}-write"

# if true, ensure write-alias points to next index
writeIndices=$(curl -s $ES_SERVICE/${POLICY_MAPPING}-*/_alias/${POLICY_MAPPING}-write \
  --cacert /etc/indexmanagement/keys/admin-ca \
  --connect-timeout ${CONNECT_TIMEOUT} \
  -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  -HContent-Type:application/json)

if echo "$writeIndices" | grep "\"error\"" ; then
  echo "Error while attemping to determine the active write alias: $writeIndices"
  exit 1
fi

if ! writeIndex="$(python /tmp/scripts/getWriteIndex.py "${POLICY_MAPPING}-write" "$writeIndices")" ; then
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
  -d '{"actions":[{"add":{"index": "'$writeIndex'", "alias": "'${POLICY_MAPPING}-write'", "is_write_index": false}},{"add":{"index": "'$nextIndex'", "alias": "'${POLICY_MAPPING}-write'", "is_write_index": true}}]}')

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
echo "" > $ERRORS

writeIndices=$(curl -s $ES_SERVICE/${POLICY_MAPPING}-*/_alias/${POLICY_MAPPING}-write \
  --cacert /etc/indexmanagement/keys/admin-ca \
  -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  -HContent-Type:application/json)

if echo "$writeIndices" | grep "\"error\"" ; then
  echo "Error while attemping to determine the active write alias: $writeIndices"
  exit 1
fi

CMD=$(cat <<END
from __future__ import print_function
import json,sys
r=json.load(sys.stdin)
alias="${POLICY_MAPPING}-write"
try:
  indices = [index for index in r if r[index]['aliases'][alias].get('is_write_index')]
  if len(indices) > 0:
    print(indices[0])
except:
  e = sys.exc_info()[0]
  sys.stderr.write("Error trying to determine the 'write' index from '%r': %r" % (r,e))
  sys.exit(1)
END
)
writeIndex=$(echo "${writeIndices}" | python -c "$CMD" 2>>$ERRORS)
if [ "$?" != "0" ] ; then
  cat $ERRORS
  exit 1
fi


indices=$(curl -s $ES_SERVICE/${POLICY_MAPPING}/_settings/index.creation_date \
  --cacert /etc/indexmanagement/keys/admin-ca \
  -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  -HContent-Type:application/json)

# Delete in batches of 25 for cases where there are a large number of indices to remove
nowInMillis=$(date +%s%3N)
minAgeFromEpoc=$(($nowInMillis - $MIN_AGE))
CMD=$(cat <<END
from __future__ import print_function
import json,sys
r=json.load(sys.stdin)
indices = []
for index in r:
  try:
    if 'settings' in r[index]:
      settings = r[index]['settings']
      if 'index' in settings:
        meta = settings['index']
        if 'creation_date' in meta:
          creation_date = meta['creation_date']
          if int(creation_date) < $minAgeFromEpoc:
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
if "$writeIndex" in indices:
  indices.remove("$writeIndex")
for i in range(0, len(indices), 25):
  print(','.join(indices[i:i+25]))
END
)
indices=$(echo "${indices}"  | python -c "$CMD" 2>>$ERRORS)
if [ "$?" != "0" ] ; then
  cat $ERRORS
  exit 1
fi
# Dump any findings to stdout but don't error
cat $ERRORS
  
if [ "${indices}" == "" ] ; then
    echo No indices to delete
    exit 0
else
    echo deleting indices: "${indices}"
fi

for sets in ${indices}; do
code=$(curl -s $ES_SERVICE/${sets}?pretty \
  -w "%{response_code}" \
  --cacert /etc/indexmanagement/keys/admin-ca \
  -HContent-Type:application/json \
  -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  -o /tmp/response.txt \
  -XDELETE )

if [ $code -ne 200 ] ; then
  cat /tmp/response.txt
  exit 1
fi
done
`

var scriptMap = map[string]string{
	"delete":           deleteScript,
	"rollover":         rolloverScript,
	"getWriteIndex.py": getWriteIndex,
	"checkRollover.py": checkRollover,
}
