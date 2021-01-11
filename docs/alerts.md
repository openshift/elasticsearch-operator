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

Cluster nodes could be failed or the Elasticsearch process crashes due to heavy load.

## Elasticsearch Cluster Healthy is Yellow

Replica shards for at least one primary shard aren't allocated to nodes.

### Troubleshooting

Check the disk space of the elasticsearch node. Increase the node count or the disk space of existing nodes.

## Elasticsearch Write Requests Rejection Jumps

### Troubleshooting

## Elasticsearch Node Disk Low Watermark Reached

Elasticsearch will not allocate shards to nodes that [reach the low watermark](https://www.elastic.co/guide/en/elasticsearch/reference/6.8/disk-allocator.html).

### Troubleshooting

Check the disk space of the node.

## Elasticsearch Node Disk High Watermark Reached

Elasticsearch will attempt to relocate shards away from a node for [reaching the high watermark](https://www.elastic.co/guide/en/elasticsearch/reference/6.8/disk-allocator.html).

### Troubleshooting

Check the disk space of the node.

## Elasticsearch Node Disk Flood Watermark Reached

 Elasticsearch enforces a read-only index block on every index that has one or more shards allocated on the node, and that has at least one disk exceeding the [flood stage](https://www.elastic.co/guide/en/elasticsearch/reference/6.8/disk-allocator.html).

### Troubleshooting

Check the disk space of the node.

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

Check the disk space of the Elasticsearch node.

## Elasticsearch FileDescriptor Usage is high

The projected number of file descriptors on the node is [insufficient](https://www.elastic.co/guide/en/elasticsearch/reference/current/file-descriptors.html).

### Troubleshooting

Check the *max_file_descriptors* configured for each node.