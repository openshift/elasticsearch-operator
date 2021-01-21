package elasticsearch

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"

	"github.com/ViaQ/logerr/kverrors"
	estypes "github.com/openshift/elasticsearch-operator/internal/types/elasticsearch"
	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	indexTemplate = estypes.NewIndexTemplate("abc-**", []string{"foo"}, 1, 0)
)

func TestCreateIndexTemplate(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         error
	}{
		{
			desc:        "Create Index Template",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{}`)),
			},
			want: nil,
		},
		{
			desc:        "Create Index Template",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 500,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{}`)),
			},
			want: kverrors.NewContext(
				"namespace", "namespace", "cluster", "testcluster").New("Failed to create Index Template", "response_status", 500, "response_body", "{}", "response_error", "[500 Internal Server Error] "),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got := esClient.CreateIndexTemplate("foo", indexTemplate)

			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestDeleteIndexTemplate(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         error
	}{
		{
			desc:        "Delete index template",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{}`)),
			},
			want: nil,
		},
		{
			desc:        "Delete index template",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 500,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{}`)),
			},
			want: kverrors.NewContext(
				"namespace", "namespace", "cluster", "testcluster").New("Failed to Delete Index Template", "response_status", 500, "response_body", "{}", "response_error", "[500 Internal Server Error] "),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got := esClient.DeleteIndexTemplate("foo")

			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}
func TestListTemplates(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         sets.String
	}{

		{
			desc:        "List templates",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{".ml-notifications":{"order":0,"version":6081299,"index_patterns":[".ml-notifications"],"settings":{},"mappings":{},"aliases":{}},".ml-anomalies":{"order":0,"version":6081299,"index_patterns":[".ml-notifications"],"settings":{},"mappings":{},"aliases":{}}}`)),
			},
			want: sets.NewString(".ml-notifications", ".ml-anomalies"),
		},
		{
			desc:        "List templates error",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 500,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{}`)),
			},
			want: sets.NewString(),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got, _ := esClient.ListTemplates()
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestGetIndexTemplates(t *testing.T) {
	emptyTemplate := make(map[string]estypes.GetIndexTemplate)
	template := make(map[string]estypes.GetIndexTemplate)
	var indexTemplate estypes.GetIndexTemplate
	indexTemplate.Order = 0
	indexTemplate.IndexPatterns = append(indexTemplate.IndexPatterns, ".ml-notifications")
	template[".ml-notifications"] = indexTemplate
	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         map[string]estypes.GetIndexTemplate
	}{
		{
			desc:        "Get Index Templates",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{".ml-notifications":{"order":0,"version":6081299,"index_patterns":[".ml-notifications"],"settings":{"index":{"number_of_shards":"","unassigned":{"node_left":{"delayed_timeout":""}}}}}}`)),
			},
			want: template,
		},

		{
			desc:        "Get Index Template 500 error",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 500,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{}`)),
			},
			want: emptyTemplate,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got, _ := esClient.GetIndexTemplates()
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestUpdateTemplatePrimaryShards(t *testing.T) {

	tests := []struct {
		desc         string
		clusterName  string
		namespace    string
		fakeResponse *http.Response
		fakeError    error
		want         error
	}{
		{
			desc:        "Update Template PrimaryShards test",
			clusterName: "testcluster",
			namespace:   "namespace",
			fakeResponse: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{".ml-notifications":{"order":0,"version":6081299,"index_patterns":[".ml-notifications"],"settings":{"index":{"number_of_shards":"1","auto_expand_replicas":"0-1","unassigned":{"node_left":{"delayed_timeout":"1m"}}}},"mappings":{},"aliases":{}}}`)),
			},
			want: nil,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			esClient := getFakeESClient(test.clusterName, test.namespace, test.fakeResponse, test.fakeError)
			got := esClient.UpdateTemplatePrimaryShards(5)

			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}
