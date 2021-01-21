package elasticsearch

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
)

func TestGetClusterNodeVersion(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         []string
	}{
		{
			desc:        "single version",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"nodes": {"versions": ["6.8.1"]}}`)),
			},
			want: []string{"6.8.1"},
		},
		{
			desc:        "multiple version",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"nodes": {"versions": ["6.8.1", "5.6.16"]}}`)),
			},
			want: []string{"6.8.1", "5.6.16"},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got, err := esClient.GetClusterNodeVersions()
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestGetClusterLowestVersion(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         string
	}{
		{
			desc:        "Lowest version",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"nodes": {"versions": [ "7.1.0",  "6.6.1", "6.8.1"]}}`)),
			},
			want: "6.6.1",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got, err := esClient.GetLowestClusterVersion()
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestGetThresholdEnabled(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         bool
	}{
		{
			desc:        "default threshold true",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"defaults":  {"cluster" : {"routing" : {"allocation" : {"disk": {"threshold_enabled" : "true"}}}}}}`)),
			},
			want: true,
		},
		{
			desc:        "default threshold false",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"defaults":  {"cluster" : {"routing" : {"allocation" : {"disk": {"threshold_enabled" : "false"}}}}}}`)),
			},
			want: false,
		},
		{
			desc:        "persistent threshold true",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"persistent":  {"cluster" : {"routing" : {"allocation" : {"disk": {"threshold_enabled" : "true"}}}}}}`)),
			},
			want: true,
		},
		{
			desc:        "transient threshold empty",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{}`)),
			},
			want: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got, err := esClient.GetThresholdEnabled()
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestGetDiskWatermarks(t *testing.T) {
	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		wantLow      interface{}
		wantHigh     interface{}
	}{
		{
			desc:        "default watermark low",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"defaults":  {"cluster" : {"routing" : {"allocation" : {"disk": {"watermark": {"low": "35.5b"}}}}}}}`)),
			},
			wantLow: "35.5",
		},
		{
			desc:        "default watermark high",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"defaults":  {"cluster" : {"routing" : {"allocation" : {"disk": {"watermark": {"high": "85.5b"}}}}}}}`)),
			},
			wantHigh: "85.5",
		},
		{
			desc:        "persistent watermark low",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"persistent":  {"cluster" : {"routing" : {"allocation" : {"disk": {"watermark": {"low": "15.6b"}}}}}}}`)),
			},
			wantLow: "15.6",
		},
		{
			desc:        "persistent watermark high",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"persistent":  {"cluster" : {"routing" : {"allocation" : {"disk": {"watermark": {"high": "95.6b"}}}}}}}`)),
			},
			wantHigh: "95.6",
		},
		{
			desc:        "transient watermark low",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"transient":  {"cluster" : {"routing" : {"allocation" : {"disk": {"watermark": {"low": "5.6b"}}}}}}}`)),
			},
			wantLow: "5.6",
		},
		{
			desc:        "transient watermark high",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"transient":  {"cluster" : {"routing" : {"allocation" : {"disk": {"watermark": {"high": "90.6"}}}}}}}`)),
			},
			wantHigh: "90.6",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			low, high, err := esClient.GetDiskWatermarks()
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(low, test.wantLow) {
				t.Errorf("got %#v, want %#v", low, test.wantLow)
			}
			if !reflect.DeepEqual(high, test.wantHigh) {
				t.Errorf("got %#v, want %#v", high, test.wantHigh)
			}
		})
	}
}

func TestGetMinMasterNodes(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         int32
	}{

		{
			desc:        "Get min master nodes",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"persistent":  {"discovery" :{"zen" : {"minimum_master_nodes" : 5}}}}`)),
			},
			want: int32(5),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got, err := esClient.GetMinMasterNodes()
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestSetMinMasterNodes(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         bool
	}{

		{
			desc:        "Set min master nodes true",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"acknowledged": true}`)),
			},
			want: true,
		},
		{
			desc:        "Set min master nodes false",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"acknowledged": false}`)),
			},
			want: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got, err := esClient.SetMinMasterNodes(2)
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestIsNodeInCluster(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         bool
		nodeName     string
	}{
		{
			desc:        "Node is present",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"cluster_name" : "my-cluster", "nodes": {"xyz1": {"name": "mycluster1"}}}`)),
			},
			nodeName: "mycluster1",
			want:     true,
		},
		{
			desc:        "Node is present",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"cluster_name" : "my-cluster", "nodes": {"xyz1": {"name": "mycluster1"}}}`)),
			},
			nodeName: "mycluster3",
			want:     false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got, err := esClient.IsNodeInCluster(test.nodeName)
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestDoSynchronizedFlush(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         bool
		nodeName     string
	}{
		{
			desc:        "Flush Sync",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{  "_shards" : {    "total" : 10,    "successful" : 5,    "failed" : 0  }}`)),
			},
			want: true,
		},
		{
			desc:        "Flush Sync",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{  "_shards" : {    "total" : 10,    "successful" : 5,    "failed" : 2  }}`)),
			},
			want: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got, err := esClient.DoSynchronizedFlush()
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}
