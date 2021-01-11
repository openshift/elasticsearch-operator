# elasticsearch-operator

Elasticsearch operator to run Elasticsearch cluster on top of Openshift and Kubernetes.
Operator uses [Operator Framework SDK](https://github.com/operator-framework/operator-sdk).

## Why Use An Operator?

Operator is designed to provide self-service for the Elasticsearch cluster operations, see [Operator Capability Levels](https://sdk.operatorframework.io/docs/advanced-topics/operator-capabilities/operator-capabilities/).

- Elasticsearch operator ensures proper layout of the pods
- Elasticsearch operator enables proper rolling cluster restarts
- Elasticsearch operator provides kubectl interface to manage your Elasticsearch cluster
- Elasticsearch operator provides kubectl interface to monitor your Elasticsearch cluster

To experiment or contribute to the development of elasticsearch-operator, see [HACKING.md](./docs/HACKING.md) and [REVIEW.md](./docs/REVIEW.md)
