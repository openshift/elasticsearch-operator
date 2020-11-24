package elasticsearch

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"

	api "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
)

func TestGetClusterHealth(t *testing.T) {

	clusterHealth := api.ClusterHealth{}
	clusterHealth.Status = "yellow"
	clusterHealth.NumNodes = 1
	clusterHealth.NumDataNodes = 1
	clusterHealth.ActivePrimaryShards = 5
	clusterHealth.ActiveShards = 5
	clusterHealth.RelocatingShards = 0
	clusterHealth.InitializingShards = 0
	clusterHealth.UnassignedShards = 5
	clusterHealth.PendingTasks = 0

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         api.ClusterHealth
	}{
		{
			desc:        "Cluster Health",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{  "cluster_name" : "docker-cluster", "status" : "yellow",  "timed_out" : false,  "number_of_nodes" : 1,  "number_of_data_nodes" : 1,  "active_primary_shards" : 5,  "active_shards" : 5,  "relocating_shards" : 0,  "initializing_shards" : 0,  "unassigned_shards" : 5,  "delayed_unassigned_shards" : 0,  "number_of_pending_tasks" : 0,  "number_of_in_flight_fetch" : 0,  "task_max_waiting_in_queue_millis" : 0,  "active_shards_percent_as_number" : 50.0`)),
			},
			want: clusterHealth,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got, err := esClient.GetClusterHealth()
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestGetClusterHealthStatus(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         string
	}{
		{
			desc:        "Cluster Health Status",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{  "cluster_name" : "docker-cluster", "status" : "yellow"`)),
			},
			want: "yellow",
		},
		{
			desc:        "Cluster Health",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{  "cluster_name" : "docker-cluster"`)),
			},
			want: "",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got, err := esClient.GetClusterHealthStatus()
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestGetClusterHealthNodes(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         int32
	}{
		{
			desc:        "Cluster Health Nodes",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{  "cluster_name" : "docker-cluster", "status" : "yellow",  "timed_out" : false,  "number_of_nodes" : 5}`)),
			},
			want: 5,
		},
		{
			desc:        "Cluster Health Nodes",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{  "cluster_name" : "docker-cluster", "status" : "yellow",  "timed_out" : false}`)),
			},
			want: 0,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got, err := esClient.GetClusterNodeCount()
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}
