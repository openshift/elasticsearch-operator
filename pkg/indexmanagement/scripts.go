package indexmanagement

const rolloverScript = `
set -euo pipefail
decoded=$(echo $PAYLOAD | base64 -d)
code=$(curl -s "$ES_SERVICE/${POLICY_MAPPING}-write/_rollover?pretty" \
  -w "%{response_code}" \
  --cacert /etc/indexmanagement/keys/admin-ca \
  -HContent-Type:application/json \
  -XPOST \
  -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  -o /tmp/response.txt \
  -d $decoded)
if [ "$code" == "200" ] ; then
  exit 0
fi
cat /tmp/response.txt
exit 1
`
const deleteScript = `
set -euo pipefail

writeIndices=$(curl -s $ES_SERVICE/${POLICY_MAPPING}-*/_alias/${POLICY_MAPPING}-write \
  --cacert /etc/indexmanagement/keys/admin-ca \
  -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  -HContent-Type:application/json)

CMD=$(cat <<END
import json,sys
r=json.load(sys.stdin)
alias="${POLICY_MAPPING}-write"
indices = [index for index in r if r[index]['aliases'][alias]['is_write_index']]
if len(indices) > 0:
  print indices[0]
END
)
writeIndex=$(echo "${writeIndices}" | python -c "$CMD")


indices=$(curl -s $ES_SERVICE/${POLICY_MAPPING}/_settings/index.creation_date \
  --cacert /etc/indexmanagement/keys/admin-ca \
  -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  -HContent-Type:application/json)

nowInMillis=$(date +%s%3N)
minAgeFromEpoc=$(($nowInMillis - $MIN_AGE))
CMD=$(cat <<END
import json,sys
r=json.load(sys.stdin)
indices = [index for index in r if int(r[index]['settings']['index']['creation_date']) < $minAgeFromEpoc ]
if "$writeIndex" in indices:
  indices.remove("$writeIndex")
indices.sort()
print ','.join(indices)
END
)
indices=$(echo "${indices}"  | python -c "$CMD")

if [ "${indices}" == "" ] ; then
    echo No indices to delete
    exit 0
else
    echo deleting indices: "${indices}"
fi

code=$(curl -s $ES_SERVICE/${indices}?pretty \
  -w "%{response_code}" \
  --cacert /etc/indexmanagement/keys/admin-ca \
  -HContent-Type:application/json \
  -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  -o /tmp/response.txt \
  -XDELETE )

if [ "$code" == "200" ] ; then
  exit 0
fi
cat /tmp/response.txt
exit 1
`

// percentScript is supposed to delete until a percentage given by MAX_PERCENT
// the script will sort all of the indices based on time and delete the oldest ones
// calculations of current size are done by the '/_nodes/stats/os' API
//
// to Run this you should do th following:
// TODO(rogreen) So far I was able to run the following without full success:
// 1. copy the script to the elasticsearch master node
// 2. try to run in with `CACERT=/etc/elasticsearch/secret/admin-ca OTHER_CERTS='--cert /etc/elasticsearch/secret/admin-cert --key /etc/elasticsearch/secret/admin-key' ES_SERVICE=https://localhost:9200 POLICY_MAPPING=app MAX_PERCENT=90 bash -x dest_script.sh`
// 3. see it fails
const percentScript = `
set -euo pipefail

CACERT="${CACERT:-/etc/indexmanagement/keys/admin-ca}"
writeIndices=$(curl -s $ES_SERVICE/${POLICY_MAPPING}-*/_alias/${POLICY_MAPPING}-write \
   --cacert "${CACERT}" ${OTHER_CERTS} \
  -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  -HContent-Type:application/json)

CMD=$(cat <<END
import json,sys
r=json.load(sys.stdin)
alias="${POLICY_MAPPING}-write"
indices = [index for index in r if r[index]['aliases'][alias]['is_write_index']]
if len(indices) > 0:
  print indices[0]
END
)
writeIndex=$(echo "${writeIndices}" | python -c "$CMD")

nodeResources=$(curl -s $ES_SERVICE/_nodes/stats/os \
   --cacert "${CACERT}" ${OTHER_CERTS} \
  -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  -HContent-Type:application/json)

CMD=$(cat <<END
import json,sys
r=json.load(sys.stdin)
node_sizes = [(node['name'], node['os']['mem']['free_percent'], node['os']['mem']['free_in_bytes']) for node in r['nodes'].values()]
highest_percent=0
highest_bytes=0
max_percent=float("${MAX_PERCENT}")
found = False
for node_size in node_sizes:
  node_name, node_percent, node_bytes = node_size
  node_percent = float(node_percent)
  if node_percent > max_percent and node_percent > highest_percent:
    found = True
    highest_percent = node_percent
    highest_bytes = node_bytes

if not found:
  sys.exit(0)
max_bytes = (highest_bytes / ( highest_percent / 100.0 ) ) * (max_percent / 100.0)
bytes_to_delete = highest_bytes - max_bytes
print(bytes_to_delete)

END
)

bytesToDelete=$(echo "${nodeResources}" | python -c "$CMD")
if [[ -z "${bytesToDelete}" ]]; then
  echo "There are no nodes with usage higher than the threashold of '${MAX_PERCENT}', no action required"
  exit 0
fi

storeSizes=$(curl -s $ES_SERVICE/_stats/store \
   --cacert "${CACERT}" ${OTHER_CERTS} \
  -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  -HContent-Type:application/json)

CMD=$(cat <<END
import json,sys
r=json.load(sys.stdin)
print(json.dumps(r['indices']))
END
)
sizeOfIndices=$(echo "${storeSizes}" | python -c "$CMD")

indices=$(curl -s $ES_SERVICE/${POLICY_MAPPING}/_settings/index.creation_date \
  --cacert "${CACERT}" ${OTHER_CERTS} \
  -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  -HContent-Type:application/json)

CMD=$(cat <<END
import json,sys
r = json.load(sys.stdin)

# Join sizeOfIndices and indices dictionaries based on index name
sizesDict=json.loads("${sizeOfIndices}")

# joinDicts takes two dictionaries and merges the keys (backwards compatible with python2)
joinDicts = lambda dict1,dict2: return {k: v for d in (dict1, dict2) for k, v in d.items()}

rb={}
for i, j in joinDicts(r,sizesDict).items():
    # if the key exists in both dicts
    if i in r and i in sizesDict:
        # make a merged dict with both keys
        rb[i]= joinDicts(r[i], sizesDict[i])
    else:
        rb[i] = j

# Convert to list to allow maintaining sort order
rl = [ {i:j} for i, j in rb.items() ]

# getCreationDate takes an index and extracts the creation date
getCreationDate = lambda x: return int(x['settings']['index']['creation_date'])

rl.sort(key=getCreationDate)

# getSizeInBytes takes an index and extracts it's size in bytes
getSizeInBytes = lambda x: return x['indices']['total']['store']['size_in_bytes']

totalSize = 0
lastPos = -1
for pos, index in enumerate(rl):
  currentSize = getSizeInBytes(index)
  # sum up all the sizes to see if they pass the threshold, bytesToDelete.
  if totalSize + currentSize < "${bytesToDelete}":
    totalSize += currentSize
    lastPos = pos
  else
    break

if "$writeIndex" in rl:
  rl.remove("$writeIndex")

# take only the indices up to threshold and print with comma seperator
print(','.join(rl[:lastPos]))
END
)
indices=$(echo "${indices}"  | python -c "$CMD")

CMD=$(cat <<END
import json,sys
r=json.load(sys.stdin)
END
)
indices=$(echo "${indices}"  | python -c "$CMD")

code=$(curl -s $ES_SERVICE/${indices}?pretty \
  -w "%{response_code}" \
  --cacert "${CACERT}" ${OTHER_CERTS} \
  -HContent-Type:application/json \
  -H"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  -o /tmp/response.txt \
  -XDELETE )

if [ "$code" == "200" ] ; then
  exit 0
fi
cat /tmp/response.txt
exit 1
`

var scriptMap = map[string]string{
	"delete":   deleteScript,
	"percent":  percentScript,
	"rollover": rolloverScript,
}
