package elasticsearch

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
)

func TestClearTransientShardAllocation(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         bool
	}{
		{
			desc:        "Clear Transient Shard Allocation",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`true`)),
			},
			want: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got, err := esClient.ClearTransientShardAllocation()
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestSetShardAllocation(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         bool
	}{
		{
			desc:        "Set Shard Allocation",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`true`)),
			},
			want: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got, err := esClient.SetShardAllocation("all")
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}
func TestGetShardAllocation(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         string
	}{
		{
			desc:        "Get Shard Allocation",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"defaults":  {"cluster" : {"routing" : {"allocation" : {"enable": "all"}}}}}`)),
			},
			want: "all",
		},
		{
			desc:        "persistent none",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"persistent":  {"cluster" : {"routing" : {"allocation" : {"enable": "none"}}}}}`)),
			},
			want: "none",
		},
		{
			desc:        "transient empty",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{}`)),
			},
			want: "",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got, err := esClient.GetShardAllocation()
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}
