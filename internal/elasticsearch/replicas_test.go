package elasticsearch

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
)

func TestGetIndexReplicaCounts(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         string
	}{
		{
			desc:        "Get Index Replicas Count",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"my-index-000001":{"settings":{"index":{"number_of_replicas":"5"}}}}`)),
			},
			want: `{"my-index-000001":{"settings":{"index":{"number_of_replicas":"5"}}}}`,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got, err := esClient.GetIndexReplicaCounts()

			m := make(map[string]interface{})
			err = json.Unmarshal([]byte(test.want), &m)
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(got, m) {
				t.Errorf("got %#v, want %#v", got, m)
			}
		})
	}
}

func TestUpdateReplicaCount(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         error
	}{
		{
			desc:        "Update Replicas Count",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"my-index-000001":{"settings":{"index":{"number_of_replicas":"5"}}}}`)),
			},
			want: nil,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got := esClient.UpdateReplicaCount(15)

			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}
