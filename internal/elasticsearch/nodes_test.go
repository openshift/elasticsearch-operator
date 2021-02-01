package elasticsearch

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
)

func TestGetNodeDiskUsage(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         float64
	}{
		{
			desc:        "Get Node Disk Usuage",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"_nodes":{"total":1,"successful":1,"failed":0},"cluster_name":"docker-cluster","nodes":{"l-1tztkKTv6aplYhvV48Ew":{"timestamp":1609998456153,"name":"l-1tztk","transport_address":"172.17.0.2:9300","host":"172.17.0.2","ip":"172.17.0.2:9300","fs":{"timestamp":1609998456165,"total":{"total_in_bytes":62725623808,"free_in_bytes":33110786048,"available_in_bytes":29894070272}},}}}`)),
			},
			want: 52.34153371913166,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			_, got, err := esClient.GetNodeDiskUsage("l-1tztk")
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}
