package elasticsearch

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"

	estypes "github.com/openshift/elasticsearch-operator/internal/types/elasticsearch"
)

//TODO fix it
func TestGetIndex(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         string
	}{
		{
			desc:        "Get Details of Index",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"test":{"aliases":{},"mappings":{"_doc":{"properties":{"title":{"type":"text","fields":{"keyword":{"type":"keyword","ignore_above":256}}}}}}},"index":{"number_of_shards":"5","number_of_replicas":"1"}}`)),
			},
			want: "test",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got, err := esClient.GetIndex("test")
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(got.Name, test.want) {
				t.Errorf("got %#v, want %#v", got.Name, test.want)
			}
		})
	}
}

func TestCreateIndex(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         error
	}{
		{
			desc:        "Create Index",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
			},
			want: nil,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			err := esClient.CreateIndex("test", &estypes.Index{})
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(err, test.want) {
				t.Errorf("got %#v, want %#v", err, test.want)
			}
		})
	}
}

//TODO: Test with actual client
func TestReIndex(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         error
	}{
		{
			desc:        "Re-Index",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
			},
			want: nil,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			err := esClient.ReIndex("test1", "testt2", "", "")
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(err, test.want) {
				t.Errorf("got %#v, want %#v", err, test.want)
			}
		})
	}
}

func TestGetAllIndices(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         int
	}{
		{
			desc:        "Get Details of All Indices",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body: ioutil.NopCloser(bytes.NewBufferString(`[{"health" : "yellow","status" : "open","index" : "test","uuid" : "4aCsuyn6SbGTDWwd7Ypwrw","pri" : "5","rep" : "1","docs.count" : "1","docs.deleted" : "0","store.size" : "4.4kb","pri.store.size" : "4.4kb"}]
				`)),
			},
			want: 1,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got, err := esClient.GetAllIndices("test")
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(len(got), test.want) {
				t.Errorf("got %#v, want %#v", len(got), test.want)
			}
		})
	}
}

func TestListAllAliases(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         string
	}{
		{
			desc:        "List All Aliases",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body: ioutil.NopCloser(bytes.NewBufferString(`{"test" : {"aliases" : {"2016" : {"filter" : {"term" : {"year" : "2016" }}}}},"test_create" : {"aliases" : {"2017" : {"filter" : {"term" : {"year" : "2017" }}}}}	}`)),
			},
			want: `{"aliases":{"2017":{"filter":{"term":{"year":"2017"}}}}}`,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got, err := esClient.ListIndicesForAlias("*")
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(got[1], test.want) {
				t.Errorf("got %#v, want %#v", got[1], test.want)
			}
		})
	}
}

func TestUpdateAliases(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         error
	}{
		{
			desc:        "Update Alias",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"actions" : [{ "add" : { "index" : "test", "alias" : "alias1" } }]}`)),
			},
			want: nil,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			body := `{"actions" : [{ "add" : { "index" : "test", "alias" : "alias1" } }]}`
			actions := &estypes.AliasActions{}
			err := json.Unmarshal([]byte(body), actions)
			got := esClient.UpdateAlias(*actions)
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}

//TODO fix this
func TestGetIndexSettings(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         int32
	}{
		{
			desc:        "Get Index Setting",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"aliases":{},"mappings":{},"settings":{"index":{"number_of_shards":"3","number_of_replicas":"2"}}}`)),
			},
			want: 3,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got, err := esClient.GetIndexSettings("test")
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestUpdateIndexSettings(t *testing.T) {

	var setting estypes.IndexSettings
	setting.NumberOfReplicas = 10
	setting.NumberOfShards = 5

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         error
	}{
		{
			desc:        "Update Index Setting",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{}`)),
			},
			want: nil,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got := esClient.UpdateIndexSettings("test", &setting)

			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestAddAliasForOldIndices(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         bool
	}{
		{
			desc:        "Add Alias for old Indices",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"aliases":{},"mappings":{},"settings":{"index":{"number_of_shards":"3","number_of_replicas":"2"}}}`)),
			},
			want: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got := esClient.AddAliasForOldIndices()

			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}
