# Alerts

<!-- TOC depthTo:2 -->

- Elasticsearch cluster
    - [Elasticsearch Cluster Health is Red](#Elasticsearch-Cluster-Health-is-Red)
    - [Elasticsearch Cluster Healthy is Yellow](#Elasticsearch-Cluster-Healthy-is-Yellow)
    - [Elasticsearch Write Requests Rejection Jumps](#Elasticsearch-Write-Requests-Rejection-Jumps)
    - [Elasticsearch Node Disk Low Watermark Reached](#Elasticsearch-Node-Disk-Low-Watermark-Reached)
    - [Elasticsearch Node Disk High Watermark Reached](#Elasticsearch-Node-Disk-High-Watermark-Reached)
    - [Elasticsearch Node Disk Flood Watermark Reached](#Elasticsearch-Node-Disk-Flood-Watermark-Reached)
    - [Elasticsearch JVM Heap Use is High](#Elasticsearch-JVM-Heap-Use-is-High)
    - [Aggregated Logging System CPU is High](#Aggregated-Logging-System-CPU-is-High)
    - [Elasticsearch Process CPU is High](#Elasticsearch-Process-CPU-is-High)
    - [Elasticsearch Disk Space is Running Low](#Elasticsearch-Disk-Space-is-Running-Low)
    - [Elasticsearch FileDescriptor Usage is high](#Elasticsearch-FileDescriptor-Usage-is-high)

<!-- /TOC -->

## Elasticsearch Cluster Health is Red

At least one primary shard and its replicas are not allocated to a node.

### Troubleshooting

1. Confirm if the elasticsearch cluster’s health is RED:
   ```
   oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_cluster_health
   ```
   Check for the “status” field in the output of the above command.

2. Check the nodes that have joined the cluster:
   ```
   oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_util --query=_cat/nodes?v
   ```
   Also, check the elasticsearch pods:
   ```
   oc -n openshift-logging get pods -l component=elasticsearch
   ```

3. If not all nodes have joined the cluster:
    - Check for the elected master:
      ```
      oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_util --query=_cat/master?v
      ```
    - Check master pod logs:
      ```
      oc logs <elasticsearch_master_pod_name> -c elasticsearch -n openshift-logging
      ```
    - Check logs of nodes that haven’t joined the cluster:
      ```
      oc logs <elasticsearch_node_name> -c elasticsearch -n openshift-logging
      ```

4. If all nodes have joined the cluster
    - Check if cluster is in the process of recovering
      ```
      oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_util --query=_cat/recovery?active_only=true
      ```
      If the above command returns nothing then no process of recovery is ongoing.
      Check if there is any pending tasks:
      ```
      oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_cluster_health |grep  number_of_pending_tasks
      ```
    - If it looks like it is still recovering
       - Just wait, this can take time depending on size of cluster, etc.
    - If it looks like recovery has stalled
       - Check if “cluster.routing.allocation.enable” is set to “none”
         ```
         oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_util --query=_cluster/settings?pretty
         ```
       - If it is set to “none” then set it to “all”
         ```
         oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_util --query=_cluster/settings?pretty -X PUT -d '{"persistent": {"cluster.routing.allocation.enable":"all"}}'
         ```
       - Check which indices are still red
         ```
         oc exec -n openshift-logging -c <elasticsearch_pod_name> -- es_util --query=_cat/indices?v
         ```
       - If there are any red indices
         - Try clearing the cache:
           ```
           oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_util --query=<elasticsearch_index_name>/_cache/clear?pretty
           ```
         - Increase max allocation retries:
           ```
           oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_util --query=<elasticsearch_index_name>/_settings?pretty -X PUT -d '{"index.allocation.max_retries":10}'
           ```
         - Delete all the scroll:
           ```
           oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_util --query=_search/scroll/_all -X DELETE
           ```
         - Increase the timeout:
           ```
           oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_util --query=<elasticsearch_index_name>/_settings?pretty -X PUT -d '{"index.unassigned.node_left.delayed_timeout":"10m"}'
           ```
         - If nothing is working, deleting the red index is the final solution:
           - Identify the red index name:
             ```
             oc exec -n openshift-logging -c <elasticsearch_pod_name> -- es_util --query=_cat/indices?v
             ```
           - Delete the red index:
             ```
             oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_util --query=<elasticsearch_red_index_name> -X DELETE
             ```
       - If no red indices exist, check if red cluster status is due to a continuous heavy processing load on a data node. This can be due to:
          - Elasticsearch JVM Heap usage is high.
            ```
            oc exec -n openshift-logging -c <elasticsearch_pod_name> -- es_util --query=_nodes/stats?pretty
            ```
            Check under “node_name.jvm.mem.heap_used_percent” field to determine the JVM Heap usage.
          - High CPU Utilization

## Elasticsearch Cluster Healthy is Yellow

Replica shards for at least one primary shard aren't allocated to nodes.

### Troubleshooting

Check the disk space of the elasticsearch node. Increase the node count or the disk space of existing nodes.

## Elasticsearch Write Requests Rejection Jumps

### Troubleshooting

## Elasticsearch Node Disk Low Watermark Reached

Elasticsearch will not allocate shards to nodes that [reach the low watermark](https://www.elastic.co/guide/en/elasticsearch/reference/6.8/disk-allocator.html).

### Troubleshooting

1. Identify the node on which elasticsearch is deployed:
   ```
   oc -n openshift-logging get po -o wide
   ```

2. Check if there are “unassigned shards” present:
   ```
   oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_util --query=_cluster/health?pretty | grep unassigned_shards
   ```

3. If unassigned shards > 0 then:
    - Check the disk space on each node:
      ```
      oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_util --query=_nodes/stats/fs?pretty
      ```
      Check the “nodes.node_name.fs” field to determine the free disk space on that node.
    - If used disk percentage is above 85% then:
        - It signifies that you have crossed low watermark already and shards cannot be allocated to this particular node anymore.
        - Increase disk space on all nodes.
        - If increasing disk space isn’t possible, then add a new data node to the cluster.
        - If adding a new data node is problematic, then decrease the total cluster redundancy policy.
            - Check current redundancyPolicy:
              ```
              oc edit es elasticsearch -n openshift-logging
              ```
              Note: If using Cluster Logging Custom Resource then:
              ```
              oc edit cl instance -n openshift-logging
              ```
            - If cluster redundancy policy is higher than SingleRedundancy then
                - Set it to SingleRedundancy and save it.
        - If nothing above helps, then last solution is to delete old indices
            - Check the status of all indices on elasticsearch:
              ```
              oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- indices
              ```
            - Identify the index that can be deleted.
            - Delete the index:
              ```
              oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_util --query=<elasticsearch_index_name> -X DELETE
              ```

## Elasticsearch Node Disk High Watermark Reached

Elasticsearch will attempt to relocate shards away from a node for [reaching the high watermark](https://www.elastic.co/guide/en/elasticsearch/reference/6.8/disk-allocator.html).

### Troubleshooting

1. Identify the node on which elasticsearch is deployed:
    ```
   oc -n openshift-logging get po -o wide
   ```

2. Check the disk space on each node:
   ```
   oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_util --query=_nodes/stats/fs?pretty
   ```

3. Check if the cluster is rebalancing:
   ```
   oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_util --query=_cluster/health?pretty | grep relocating_shards
   ```

4. If there are any relocating shards present then:
    - The High Watermark which is 90% by default has been exceeded.
    - The shards will start relocating to a different node which has low disk usage and hasn’t crossed any watermark threshold limits.
    - To allocate shards to this particular node, free up some spaces:
        - Increase disk space on all nodes.
        - If increasing disk space isn’t possible, then add a new data node to the cluster.
        - If adding a new data node is problematic, then decrease the total cluster redundancy policy
            - Check current redundancyPolicy
              ```
              oc edit es elasticsearch -n openshift-logging
              ```
              Note: If using Cluster Logging Custom Resource then:
              ```
              oc edit cl instance -n openshift-logging
              ```
            - If cluster redundancy policy is higher than SingleRedundancy then
                - Set it to SingleRedundancy and save it.
        - If nothing above helps, then last solution is to delete old indices
            - Check the status of all indices on elasticsearch:
              ```
              oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- indices
              ```
            - Identify the index that can be deleted.
            - Delete the index:
              ```
              oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_util --query=<elasticsearch_index_name> -X DELETE
              ```
    
## Elasticsearch Node Disk Flood Watermark Reached

 Elasticsearch enforces a read-only index block on every index that has one or more shards allocated on the node, and that has at least one disk exceeding the [flood stage](https://www.elastic.co/guide/en/elasticsearch/reference/6.8/disk-allocator.html).

### Troubleshooting

1. Check the disk space of the Elasticsearch node:
   ```
   oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_util --query=_nodes/stats/fs?pretty
   ```
   Check the “nodes.node_name.fs” field to determine the free disk space on that node.

2. If used disk percentage is above 95% then:
    - It signifies that you have crossed flood watermark already and writing will be blocked for shards allocated on this particular node.
    - Increase disk space on all nodes.
    - If increasing disk space isn’t possible, then add a new data node to the cluster.
    - If adding a new data node is problematic, then decrease the total cluster redundancy policy
        - Check current redundancyPolicy
          ```
          oc edit es elasticsearch -n openshift-logging
          ```
          Note: If using Cluster Logging Custom Resource then:
          ```
          oc edit cl instance -n openshift-logging
          ```
        - If cluster redundancy policy is higher than SingleRedundancy then
            - Set it to SingleRedundancy and save it.
    - If nothing above helps, then last solution is to delete old indices
        - Check the status of all indices on elasticsearch:
          ```
          oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- indices
          ```
        - Identify the index that can be deleted.
        - Delete the index:
          ```
          oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_util --query=<elasticsearch_index_name> -X DELETE
          ```
    - After freeing up the disk space, when used disk percentage drops below 90% then:
        - Unblock write to this particular node.
          ```
          oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_util --query=_all/_settings?pretty -X PUT -d '{"index.blocks.read_only_allow_delete": null}'
          ```

## Elasticsearch JVM Heap Use is High

The elasticsearch node JVM Heap memory used is above 75%.

### Troubleshooting

Consider [increasing the heap size](https://www.elastic.co/guide/en/elasticsearch/reference/current/important-settings.html#heap-size-settings).

## Aggregated Logging System CPU is High

System CPU usage on the node is high.

### Troubleshooting

Check the CPU of the cluster node. Consider allocating more CPU resources to the node.

## Elasticsearch Process CPU is High

Elasticsearch process CPU usage on the node is high. 

### Troubleshooting

Check the CPU of the cluster node. Consider allocating more CPU resources to the node.

## Elasticsearch Disk Space is Running Low

The elasticsearch Cluster is predicted to be out of disk space within the next 6h.

### Troubleshooting

1. Check the disk space of the Elasticsearch node:
   ```
   oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_util --query=_nodes/stats/fs?pretty
   ```
   Check the “nodes.node_name.fs” field to determine the free disk space on that node.

2. The prediction is done based on the current trend in occupying the disk space on that node.
    
3. Since the disk space will get over soon within next 6h:
    - Increase disk space on all nodes.
    - If increasing disk space isn’t possible, then add a new data node to the cluster.
    - If adding a new data node is problematic,then decrease the total cluster redundancy policy
        - Check current redundancyPolicy
          ```
          oc edit es elasticsearch -n openshift-logging
          ```
          Note: If using Cluster Logging Custom Resource then:
          ```
          oc edit cl instance -n openshift-logging
          ```
        - If cluster redundancy policy is higher than SingleRedundancy then
            - Set it to SingleRedundancy and save it.
    - If nothing above helps, then last solution is to delete old indices
        - Check the status of all indices on elasticsearch:
          ```
          oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- indices
          ```
        - Identify the index that can be deleted.
        - Delete the index:
          ```
          oc exec -n openshift-logging -c elasticsearch <elasticsearch_pod_name> -- es_util --query=<elasticsearch_index_name> -X DELETE
          ```

## Elasticsearch FileDescriptor Usage is high

The projected number of file descriptors on the node is [insufficient](https://www.elastic.co/guide/en/elasticsearch/reference/current/file-descriptors.html).

### Troubleshooting

Check the *max_file_descriptors* configured for each node.