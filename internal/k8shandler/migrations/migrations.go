package migrations

import (
	"github.com/ViaQ/logerr/kverrors"
	"github.com/openshift/elasticsearch-operator/internal/elasticsearch"
	estypes "github.com/openshift/elasticsearch-operator/internal/types/elasticsearch"
	"github.com/openshift/elasticsearch-operator/internal/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MigrationRequest interface {
	RunKibanaMigrations() error
	RunElasticsearchMigrations() error
}

func NewMigrationRequest(client client.Client, esClient elasticsearch.Client) MigrationRequest {
	return &migrationRequest{
		client:   client,
		esClient: esClient,
	}
}

type migrationRequest struct {
	client   client.Client
	esClient elasticsearch.Client
}

func (mr *migrationRequest) RunKibanaMigrations() error {
	if index, _ := mr.esClient.GetIndex(kibanaIndex); index == nil {
		return nil
	}

	indices, err := mr.esClient.GetAllIndices(kibanaIndex)
	if err != nil {
		return kverrors.Wrap(err, "failed to get indices before running migrations",
			"alias", kibanaIndex,
		)
	}

	health, err := getIndexHealth(indices, kibanaIndex)
	if err != nil {
		return kverrors.Wrap(err, "failed to get index health before running migrations",
			"index", kibanaIndex)
	}

	if health != "green" && health != "yellow" {
		return kverrors.New("waiting for index recovery before running migrations",
			"current_status", health,
			"desired_status", "green/yellow",
			"index", kibanaIndex)
	}

	if err := mr.reIndexKibana5to6(); err != nil {
		return kverrors.Wrap(err, "failed to reindex",
			"from", kibanaIndex,
			"to", kibana6Index)
	}
	return nil
}

func (mr *migrationRequest) RunElasticsearchMigrations() error {
	return nil
}

func (mr *migrationRequest) matchRequiredMajorVersion(version string) (bool, error) {
	versions, err := mr.esClient.GetClusterNodeVersions()
	if err != nil {
		return false, err
	}

	if versions == nil {
		return false, nil
	}

	if len(versions) > 1 {
		return false, nil
	}

	return utils.GetMajorVersion(versions[0]) == version, nil
}

func getIndexHealth(indices estypes.CatIndicesResponses, name string) (string, error) {
	if len(indices) == 0 {
		return "unknown", kverrors.New("failed to get index health",
			"index", name)
	}

	return indices[0].Health, nil
}
