# Prior PVC data recovery

## Why

In the case where you had a cluster that was running, but somehow ended up with a new cluster backed by a new PVC (typically caused by the elasticsearch CR being deleted and then recreated). You may want to recover that data from your past cluster.

This guide seeks to walk users through the steps necessary to reindex data from their past cluster(s) to a currently running one.

## How

This process involves using the `_reindex` api made available by ElasticSearch, specifically the [reindex from a remote cluster](https://www.elastic.co/guide/en/elasticsearch/reference/6.8/reindex-upgrade-remote.html) procedure.

To begin, this guide assumes you have a currently running cluster in the `openshift-logging` namespace with the cluster name `elasticsearch` (this is also the name of your elasticsearch CR).

1. Update your `elasticsearch` CR to be `Unmanaged`
    - We will be updating it's configmap, and we don't want the Elasticsearch Operator reverting those changes until we're done recovering.

1. Create a local elasticsearch CR for your recovery cluster
    - There is an example in the [appendix](#appendix)
    - You may need to decrease the resources for this new cluster so that you can ensure the pods will be deployed.
    - Ensure that the following match your existing cluster:
      - nodeCount and roles
      - redundancyPolicy
    - IMPORTANT: Do not specify a storage spec, this will cause another PVC to be created and we will just end up replacing this value later on.
    - IMPORTANT: Do not add an Index Management section, this is intentionally left off as we do not want any rollover to occur while reindexing.

1. `oc create -f` your recovery elasticsearch CR

1. Observe that the pods are able to start up and be 'Ready'
    - `oc get pods -n openshift-logging -w -l cluster-name=elasticsearch-recovery`
    - If they are not ready, you may need to adjust the resources for your nodes in your CR. `oc describe` one of your pods to check the events to see if there are resource issues with the scheduling a pod.

1. Once all the pods are ready, update your `elasticsearch-recovery` CR to be `Unmanaged`

1. To prevent metrics being scraped for the recovery cluster, delete its `servicemonitor` object
    - `oc delete servicemonitor -n openshift-logging monitor-elasticsearch-recovery-cluster`

1. Scale down all your recovery nodes
    - IMPORTANT: Make note of the number of replicas before scaling down your statefulsets (if applicable).
    - `oc scale deployment --replicas=0 -l cluster-name=elasticsearch-recovery`
    - `oc scale statefulset --replicas=0 -l cluster-name=elasticsearch-recovery`

1. For each of your deployments|statefulsets in your recovery cluster, `oc edit` and update the following:
    - the `CLUSTER_NAME` env var must match your prior cluster name, for the sake of these examples it would be `elasticsearch`. If this does not match the prior cluster you will not see your old data when the recovery cluster spins up.
    - remove the `paused: true` line.
    - update the `secret` volume definition in `Volumes` to no longer be `elasticsearch-recovery` but instead be `elasticsearch`. If you do not update this the cluster will not be placed because it is waiting for the secret to be available.

1. Get a list of your prior PVCs you wish to recover. We will want to pay attention to the UUIDs for them. Do not include the UUIDs that are being used by the current cluster.
    - With a PVC name like: `elasticsearch-elasticsearch-cdm-xm9nl5cb-1` the UUID is `xm9nl5cb`.
    - With a Pod name like: `elasticsearch-cdm-xm9nl5cb-1-6755bb948f-g4plq` the UUID is `xm9nl5cb`.

1. Mount all the previously used PVC for a given UUID to the recovery cluster. for each node:
    - `oc set volume deployment/<recovery_node_name> --add -t persistentVolumeClaim --claim-name=<name of the old pvc> --name=elasticsearch-storage --overwrite`
      - Note: while not necessary, it is a good idea to match the PVC numbering to the node numbering for ease of remembering, e.g. `oc set volume deployment/elasticsearch-cdm-ap5obh1j-1 --add -t persistentVolumeClaim --claim-name=elasticsearch-elasticsearch-cdm-xm9nl5cb-1 --name=elasticsearch-storage --overwrite`

1. Scale up all your recovery nodes
    - `oc scale deployment --replicas=1 -l cluster-name=elasticsearch-recovery`
    - `oc scale statefulset --replicas=<value before scaledown> -l cluster-name=elasticsearch-recovery`

1. Wait for the recovery cluster to come up and ensure the prior indices exist.
    - `oc exec -c elasticsearch <es_pod_name> -- indices`
    - if you do not see your data, verify that you have correctly updated the `CLUSTER_NAME` env var name for each of your nodes.

1. Update the recovery cluster to allow the ES service account to read its indices.
    - Run this once on the recovery cluster (not for each node)
    ```
    oc exec -c elasticsearch <recovery_pod_name> -- sed -i sgconfig/roles_mapping.yml -e '/system.admin/a\' -e "    - 'CN=logging-es,OU=OpenShift,O=Logging'"`
    - `oc exec -c elasticsearch <recovery_pod_name> -- es_seed_acl
    ```

1. Update the Elasticsearch configmap to know how to reindex from remote
    - `oc edit configmap elasticsearch`
    - insert the following in `elasticsearch.yml` be sure to pay attention to indentation, `reindex` should be at the same indentation amount as `path`
    ```
    reindex:
      remote.whitelist: "elasticsearch-recovery.openshift-logging.svc:9200"
      ssl:
        verification_mode: certificate
        truststore:
          path: /etc/elasticsearch/secret/truststore.p12
          password: tspass
          type: p12
        keystore:
          path: /etc/elasticsearch/secret/key.p12
          type: p12
          password: kspass
    ```
    - e.g.
    ```
    gateway:
      recover_after_nodes: 2
      expected_nodes: 3
      recover_after_time: ${RECOVER_AFTER_TIME}

    path:
      data: /elasticsearch/persistent/${CLUSTER_NAME}/data
      logs: /elasticsearch/persistent/${CLUSTER_NAME}/logs

    reindex:
      remote.whitelist: "elasticsearch-recovery.openshift-logging.svc:9200"
      ssl:
        verification_mode: certificate
        truststore:
          path: /etc/elasticsearch/secret/truststore.p12
          password: tspass
          type: p12
        keystore:
          path: /etc/elasticsearch/secret/key.p12
          type: p12
          password: kspass
    ```

1. Restart the cluster to pick up the configmap changes:
    - `oc delete pod -l cluster-name=elasticsearch`

1. Wait for the cluster to come back up

1. Run the recovery script [found in the appendix](#appendix) from your local machine where you are running your other `oc` commands.
    - Assuming the script is saved as "reindex.sh"
    ```
    sh reindex.sh elasticsearch-recovery-cdm-1kb3lm48-1-a7f2ee0icq-pd87v elasticsearch-cdm-xm9nl5cb-1-6755bb948f-g4plq
    ```

1. Double-check your existing cluster's indices to ensure old data had been recovered.
    - Indices will have "recovered" in the name
      - `oc exec -c elasticsearch <es_pod_name> -- indices`

1. If required to recover other volumes, scale down the recovery cluster and repeat steps 9 - 12 using the other PVC and rerun the recovery script (step 16).

1. As part of clean up, remove the old unused PVC that data has been recovered from
    - `oc delete pvc <pvc_name>`

1. Remove the elasticsearch recovery CR
    - `oc delete elasticsearch elasticsearch-recovery`

1. Update the elasticsearch CR to be `Managed` again.

# Appendix

## Example Recovery CR

```
apiVersion: logging.openshift.io/v1
kind: Elasticsearch
metadata:
  name: elasticsearch-recovery
  namespace: openshift-logging
spec:
  managementState: Managed
  nodeSpec:
    proxyResources:
      limits:
        memory: 64Mi
      requests:
        cpu: 100m
        memory: 64Mi
    resources:
      limits:
        memory: 1Gi
      requests:
        cpu: 150m
        memory: 1Gi
  nodes:
  - nodeCount: 3
    proxyResources: {}
    resources: {}
    roles:
    - client
    - data
    - master
    storage: {}
  redundancyPolicy: SingleRedundancy
  ```


## Recovery script

```
#!/bin/bash

recovery_node=$1
destination_node=$2

function seed_recovery_templates() {

  kibana=$(oc exec -c elasticsearch $destination_node -- es_util --query="_template/common.settings.kibana.template.json?pretty")

  primary="$(echo $kibana | jq -r '.["common.settings.kibana.template.json"].settings.index.number_of_shards')"
  replicas="$(echo $kibana | jq -r '.["common.settings.kibana.template.json"].settings.index.number_of_replicas')"

operations=$(cat <<EOF
{
  "order": 0,
  "index_patterns": [
    ".operations.*"
  ],
  "settings": {
    "index": {
      "number_of_shards": $primary,
      "number_of_replicas": $replicas
    }
  }
}
EOF
)

  oc exec -c elasticsearch $destination_node -- es_util --query="_template/recovery.operations.template.json" -XPUT -d "${operations}"

projects=$(cat <<EOF
{
  "order": 0,
  "index_patterns": [
    "project.*"
  ],
  "settings": {
    "index": {
      "number_of_shards": $primary,
      "number_of_replicas": $replicas
    }
  }
}
EOF
)

  oc exec -c elasticsearch $destination_node -- es_util --query="_template/recovery.projects.template.json" -XPUT -d "${projects}"

}

function remove_recovery_templates() {
  oc exec -c elasticsearch $destination_node -- es_util --query="_template/recovery.operations.template.json" -XDELETE

  oc exec -c elasticsearch $destination_node -- es_util --query="_template/recovery.projects.template.json" -XDELETE
}

function reindex() {
  source=$1
  destination=$2

reindex=$(cat <<EOF
{
  "source": {
    "remote": {
      "host": "https://elasticsearch-recovery.openshift-logging.svc:9200"
    },
    "index": "$source"
  },
  "dest": {
    "index": "$destination"
  }
}
EOF
)

  oc exec -c elasticsearch $destination_node -- es_util --query="_reindex" -XPOST -d "${reindex}"
}

seed_recovery_templates

indices=$(oc exec -c elasticsearch $recovery_node -- es_util --query="_cat/indices?h=index")

for index in $indices; do

  if [[ $index =~ project\..* ]]; then
    dest_name=$(echo $index | awk -F'.' '{print $1"."$2"."$3".recovered."$4"."$5"."$6}')

    reindex $index $dest_name
  elif [[ $index =~ \.operations\..* ]]; then
    dest_name=$(echo $index | awk -F'.' '{print "."$2".recovered."$3"."$4"."$5}')

    reindex $index $dest_name
  
  elif [[ $index =~ (app|infra|audit)\-.* ]]; then
    dest_name=$(echo $index | awk -F'-' '{print $1"-recovered-"$2}')

    reindex $index $dest_name
  fi

done

remove_recovery_templates
```
