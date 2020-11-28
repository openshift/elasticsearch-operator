package migrations

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/openshift/elasticsearch-operator/internal/elasticsearch"
	"github.com/openshift/elasticsearch-operator/test/helpers"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Running", func() {
	defer GinkgoRecover()

	var (
		chatter   *helpers.FakeElasticsearchChatter
		client    elasticsearch.Client
		k8sClient = fake.NewFakeClient()
	)

	const (
		esCluster   = "elasticsearch"
		esNamespace = "openshift-logging"
	)

	Describe("Kibana migrations", func() {
		It("should skip if `.kibana` index health is not green", func() {
			chatter = helpers.NewFakeElasticsearchChatter(map[string]helpers.FakeElasticsearchResponses{
				".kibana": {
					{
						StatusCode: 200,
						Body:       `{}`,
					},
				},
				"_cat/indices/.kibana?format=json": {
					{
						StatusCode: 200,
						Body:       `[{"health":"red","status":"open","index":".kibana","uuid":"KNegGDiRSs6dxWzdxWqkaQ","pri":"1","rep":"1","docs.count":"1","docs.deleted":"0","store.size":"6.4kb","pri.store.size":"3.2kb"}]`,
					},
				},
			})
			client = helpers.NewFakeElasticsearchClient(esCluster, esNamespace, k8sClient, chatter)

			kr := migrationRequest{
				client:   k8sClient,
				esClient: client,
			}

			Expect(kr.RunKibanaMigrations()).ShouldNot(Succeed())
		})

		It("should skip if `.kibana` index not existing", func() {
			chatter = helpers.NewFakeElasticsearchChatter(map[string]helpers.FakeElasticsearchResponses{
				".kibana": {
					{
						StatusCode: 404,
						Body:       `{}`,
					},
				},
			})
			client = helpers.NewFakeElasticsearchClient(esCluster, esNamespace, k8sClient, chatter)

			kr := migrationRequest{
				client:   k8sClient,
				esClient: client,
			}

			Expect(kr.RunKibanaMigrations()).Should(Succeed())
		})

		It("should skip if `.kibana` index health is not available", func() {
			chatter = helpers.NewFakeElasticsearchChatter(map[string]helpers.FakeElasticsearchResponses{
				".kibana": {
					{
						StatusCode: 200,
						Body:       `{}`,
					},
				},
				"_cat/indices/.kibana?format=json": {
					{
						StatusCode: 404,
						Body:       `{}`,
					},
				},
			})
			client = helpers.NewFakeElasticsearchClient(esCluster, esNamespace, k8sClient, chatter)

			kr := migrationRequest{
				client:   k8sClient,
				esClient: client,
			}

			Expect(kr.RunKibanaMigrations()).ShouldNot(Succeed())
		})
	})
})
