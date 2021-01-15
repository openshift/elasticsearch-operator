# Troubleshooting

## Kibana

### Why am I unable to see infrastructure logs
Infrastructure logs are visible from the `Global` tenant and require `administrator` permissions. See the [access control](access-control.md) documentation for additional information about how a user is determined to have the `administrator` role.
### kube:admin is unable to see infrastructure logs
`kube:admin` by default does not have the correct permissions to be given the admin role.   See the [access control](access-control.md) documentation for additional information.  You may grant the permissions by:
```
oc adm policy add-cluster-role-to-user cluster-admin kube:admin
```

## Amount of logs per project

The new [data model](https://github.com/openshift/enhancements/blob/master/enhancements/cluster-logging/cluster-logging-es-rollover-data-design.md#data-model) was introduced in OCP 4.5.
Since then, the logs from individual namespaces no longer end up in dedicated indices by default, but they share a common index.

To learn which projects are generating most of the logs you can use Elasticsearch query language to calculate
aggregated statistics.

The following is an example of hourly date histogram aggregation (for last three hours) with nested number of logs grouped by top namespaces:


```json
GET /app,infra,audit/_search
{
  "size": 0,
  "query": {
    "range": {
      "@timestamp": {
        "gte": "now-3h",
        "lt": "now"
      }
    }
  },
  "aggs": {
    "Histogram": {
      "date_histogram": {
        "field": "@timestamp",
        "interval": "hour"
      },
      "aggs": {
        "top_namespaces": {
          "terms": {
            "size": 10,
            "order" : { "_count" : "desc"},
            "field": "kubernetes.namespace_name"
          }
        }
      }
    }
  }
}
```
You can leave out any members from `[app,infra,audit]` list in the beginning to make the query more focused.

The query is in the format that can be directly used in Kibana [Dev Tools Console](https://www.elastic.co/guide/en/kibana/6.8/console-kibana.html) window. To use this query in CLI save it into a file called `query.json` (remember to leave out the first line starting with "GET") and execute:

```shell
QUERY=`cat query.json`; \
oc exec <es_pod> -c elasticsearch -- \
  es_util --query="app,infra,audit/_search?pretty" \
  -d "$QUERY"
```
