package migrations

import (
	"encoding/json"
	"fmt"

	"github.com/openshift/elasticsearch-operator/pkg/log"
	estypes "github.com/openshift/elasticsearch-operator/pkg/types/elasticsearch"
)

const (
	kibanaIndex         = ".kibana"
	kibana6Index        = ".kibana-6"
	kibana5to6EsVersion = "6"
)

func (mr *migrationRequest) reIndexKibana5to6() error {
	ok, err := mr.matchRequiredMajorVersion(kibana5to6EsVersion)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("skipping migration not all nodes match min required versions %q", kibana5to6EsVersion)
	}

	if mr.migrationCompleted() {
		return nil
	}

	if err := mr.setKibanaIndexReadOnly(); err != nil {
		return err
	}

	if err := mr.createNewKibana6Index(); err != nil {
		return err
	}

	if err := mr.reIndexIntoKibana6(); err != nil {
		return err
	}

	if err := mr.aliasKibana(); err != nil {
		return err
	}

	return nil
}

func (mr *migrationRequest) migrationCompleted() bool {
	indices, err := mr.esClient.ListIndicesForAlias(kibanaIndex)
	if err != nil {
		log.Error(err, "failed to list indices for alias", "alias", kibanaIndex)
		return false
	}

	return len(indices) != 0
}

func (mr *migrationRequest) setKibanaIndexReadOnly() error {
	curSett, err := mr.esClient.GetIndexSettings(kibanaIndex)
	if err != nil {
		return fmt.Errorf("failed to get index settings for %q: %s", kibanaIndex, err)
	}

	if curSett != nil {
		if curSett.Index != nil {
			if curSett.Index.Blocks.Write {
				log.Info("skipping setting index to read-only because already completed", "index", kibanaIndex)
				return nil
			}
		}
	}

	settings := &estypes.IndexSettings{
		Index: &estypes.IndexingSettings{
			Blocks: &estypes.IndexBlocksSettings{
				Write: true,
			},
		},
	}

	if err := mr.esClient.UpdateIndexSettings(kibanaIndex, settings); err != nil {
		return fmt.Errorf("failed to set index %q to read only: %s", kibanaIndex, err)
	}
	return nil
}

func (mr *migrationRequest) createNewKibana6Index() error {
	curIndex, err := mr.esClient.GetIndex(kibana6Index)
	if err != nil {
		return fmt.Errorf("failed to get index for %q: %s", kibana6Index, err)
	}

	if curIndex != nil {
		if curIndex.Name == kibana6Index {
			log.Info("skipping creating index anew because already existing", "index", kibana6Index)
			return nil
		}
	}

	mappings := make(map[string]interface{})
	err = json.Unmarshal([]byte(kibana6IndexMappings), &mappings)
	if err != nil {
		return fmt.Errorf("failed to read kibana 6 mappings: %s", err)
	}

	index := &estypes.Index{
		Name: kibana6Index,
		Settings: estypes.IndexSettings{
			NumberOfShards: 1,
			Index: &estypes.IndexingSettings{
				Format: 6,
				Mapper: &estypes.IndexMapperSettings{
					Dynamic: false,
				},
			},
		},
		Mappings: mappings,
	}

	if err := mr.esClient.CreateIndex(kibana6Index, index); err != nil {
		return fmt.Errorf("failed to create new index %q: %s", kibana6Index, err)
	}
	return nil
}

func (mr *migrationRequest) reIndexIntoKibana6() error {
	indices, err := mr.esClient.GetAllIndices(kibana6Index)
	if err != nil {
		return fmt.Errorf("failed fetching doc count for %q before re-indexing: %s", kibana6Index, err)
	}

	var index *estypes.CatIndicesResponse
	for _, idx := range indices {
		if idx.Index == kibana6Index {
			index = &idx
			break
		}
	}

	if index == nil {
		return fmt.Errorf("failed fetching doc count for index %q: index not found", kibana6Index)
	}

	if index.DocsCount != "0" {
		log.Info("skipping re-indexing because already completed", "from", kibanaIndex, "to", kibana6Index)
		return nil
	}

	err = mr.esClient.ReIndex(kibanaIndex, kibana6Index, kibanReIndexScript, "painless")
	if err != nil {
		return fmt.Errorf("failed to reindex kibana6: %s", err)
	}
	return nil
}

func (mr *migrationRequest) aliasKibana() error {
	if mr.migrationCompleted() {
		log.Info("skipping aliasing index because alias existing", "alias", kibanaIndex, "index", kibana6Index)
		return nil
	}

	actions := estypes.AliasActions{
		Actions: []estypes.AliasAction{
			{
				Add: &estypes.AddAliasAction{
					Index: kibana6Index,
					Alias: kibanaIndex,
				},
			},
			{
				RemoveIndex: &estypes.RemoveAliasAction{
					Index: kibanaIndex,
				},
			},
		},
	}

	if err := mr.esClient.UpdateAlias(actions); err != nil {
		return fmt.Errorf("failed to change alias %s to %s: %s", kibanaIndex, kibana6Index, err)
	}
	return nil
}

const (
	kibanReIndexScript = `
ctx._source = [ ctx._type : ctx._source ]; ctx._source.type = ctx._type; ctx._id = ctx._type + ":" + ctx._id; ctx._type = "doc";
`

	kibana6IndexMappings = `
{
  "doc": {
    "properties": {
      "type": {
        "type": "keyword"
      },
      "updated_at": {
        "type": "date"
      },
      "config": {
        "properties": {
          "buildNum": {
            "type": "keyword"
          }
        }
      },
      "index-pattern": {
        "properties": {
          "fieldFormatMap": {
            "type": "text"
          },
          "fields": {
            "type": "text"
          },
          "intervalName": {
            "type": "keyword"
          },
          "notExpandable": {
            "type": "boolean"
          },
          "sourceFilters": {
            "type": "text"
          },
          "timeFieldName": {
            "type": "keyword"
          },
          "title": {
            "type": "text"
          }
        }
      },
      "visualization": {
        "properties": {
          "description": {
            "type": "text"
          },
          "kibanaSavedObjectMeta": {
            "properties": {
              "searchSourceJSON": {
                "type": "text"
              }
            }
          },
          "savedSearchId": {
            "type": "keyword"
          },
          "title": {
            "type": "text"
          },
          "uiStateJSON": {
            "type": "text"
          },
          "version": {
            "type": "integer"
          },
          "visState": {
            "type": "text"
          }
        }
      },
      "search": {
        "properties": {
          "columns": {
            "type": "keyword"
          },
          "description": {
            "type": "text"
          },
          "hits": {
            "type": "integer"
          },
          "kibanaSavedObjectMeta": {
            "properties": {
              "searchSourceJSON": {
                "type": "text"
              }
            }
          },
          "sort": {
            "type": "keyword"
          },
          "title": {
            "type": "text"
          },
          "version": {
            "type": "integer"
          }
        }
      },
      "dashboard": {
        "properties": {
          "description": {
            "type": "text"
          },
          "hits": {
            "type": "integer"
          },
          "kibanaSavedObjectMeta": {
            "properties": {
              "searchSourceJSON": {
                "type": "text"
              }
            }
          },
          "optionsJSON": {
            "type": "text"
          },
          "panelsJSON": {
            "type": "text"
          },
          "refreshInterval": {
            "properties": {
              "display": {
                "type": "keyword"
              },
              "pause": {
                "type": "boolean"
              },
              "section": {
                "type": "integer"
              },
              "value": {
                "type": "integer"
              }
            }
          },
          "timeFrom": {
            "type": "keyword"
          },
          "timeRestore": {
            "type": "boolean"
          },
          "timeTo": {
            "type": "keyword"
          },
          "title": {
            "type": "text"
          },
          "uiStateJSON": {
            "type": "text"
          },
          "version": {
            "type": "integer"
          }
        }
      },
      "url": {
        "properties": {
          "accessCount": {
            "type": "long"
          },
          "accessDate": {
            "type": "date"
          },
          "createDate": {
            "type": "date"
          },
          "url": {
            "type": "text",
            "fields": {
              "keyword": {
                "type": "keyword",
                "ignore_above": 2048
              }
            }
          }
        }
      },
      "server": {
        "properties": {
          "uuid": {
            "type": "keyword"
          }
        }
      },
      "timelion-sheet": {
        "properties": {
          "description": {
            "type": "text"
          },
          "hits": {
            "type": "integer"
          },
          "kibanaSavedObjectMeta": {
            "properties": {
              "searchSourceJSON": {
                "type": "text"
              }
            }
          },
          "timelion_chart_height": {
            "type": "integer"
          },
          "timelion_columns": {
            "type": "integer"
          },
          "timelion_interval": {
            "type": "keyword"
          },
          "timelion_other_interval": {
            "type": "keyword"
          },
          "timelion_rows": {
            "type": "integer"
          },
          "timelion_sheet": {
            "type": "text"
          },
          "title": {
            "type": "text"
          },
          "version": {
            "type": "integer"
          }
        }
      },
      "graph-workspace": {
        "properties": {
          "description": {
            "type": "text"
          },
          "kibanaSavedObjectMeta": {
            "properties": {
              "searchSourceJSON": {
                "type": "text"
              }
            }
          },
          "numLinks": {
            "type": "integer"
          },
          "numVertices": {
            "type": "integer"
          },
          "title": {
            "type": "text"
          },
          "version": {
            "type": "integer"
          },
          "wsState": {
            "type": "text"
          }
        }
      }
    }
  }
}
`
)
