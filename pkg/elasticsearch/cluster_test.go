package elasticsearch_test

import (
	"reflect"
	"testing"

	"github.com/openshift/elasticsearch-operator/test/helpers"
)

func TestGetClusterNodeVersion(t *testing.T) {
	chatter := helpers.NewFakeElasticsearchChatter(map[string]helpers.FakeElasticsearchResponses{
		"_cluster/stats": {
			{
				StatusCode: 200,
				Body:       `{"nodes": {"versions": ["6.8.1"]}}`,
			},
			{
				StatusCode: 200,
				Body:       `{"nodes": {"versions": ["6.8.1", "5.6.16"]}}`,
			},
		},
	})
	esClient := helpers.NewFakeElasticsearchClient("elasticsearch", "test-namespace", fakeClient, chatter)

	tests := []struct {
		desc string
		want []string
	}{
		{
			desc: "single version",
			want: []string{"6.8.1"},
		},
		{
			desc: "multiple version",
			want: []string{"6.8.1", "5.6.16"},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
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
