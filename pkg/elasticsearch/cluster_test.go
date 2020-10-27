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

func TestGetLowestClusterVersion(t *testing.T) {
	chatter := helpers.NewFakeElasticsearchChatter(map[string]helpers.FakeElasticsearchResponses{
		"_cluster/stats/nodes/_all": {
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
		want string
	}{
		{
			desc: "single version",
			want: "6.8.1",
		},
		{
			desc: "split versions",
			want: "5.6.16",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
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

func TestIsNodeInCluster(t *testing.T) {
	chatter := helpers.NewFakeElasticsearchChatter(map[string]helpers.FakeElasticsearchResponses{
		"_cluster/state/nodes": {
			{
				StatusCode: 200,
				Body:       `{"nodes": {"nodeuuid1": {"name": "node1"}}}`,
			},
			{
				StatusCode: 200,
				Body:       `{"nodes": {"nodeuuid1": {"name": "node1"}, "nodeuuid2": {"name": "node2"}}}`,
			},
		},
	})
	esClient := helpers.NewFakeElasticsearchClient("elasticsearch", "test-namespace", fakeClient, chatter)

	tests := []struct {
		desc string
		want bool
	}{
		{
			desc: "node not in cluster",
			want: false,
		},
		{
			desc: "node in cluster",
			want: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			got, err := esClient.IsNodeInCluster("node2")
			if err != nil {
				t.Errorf("got err: %s", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %#v, want %#v", got, test.want)
			}
		})
	}
}
