package migrations

import (
	"encoding/json"

	"github.com/ViaQ/logerr/kverrors"
	"github.com/ViaQ/logerr/log"
	estypes "github.com/openshift/elasticsearch-operator/internal/types/elasticsearch"
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
		return kverrors.New("skipping migration not all nodes match min required versions",
			"min_required_version", kibana5to6EsVersion)
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
		return kverrors.Wrap(err, "failed to get index settings",
			"index", kibanaIndex)
	}

	if curSett != nil {
		if curSett.Settings != nil {
			if curSett.Settings.Index != nil {
				if curSett.Settings.Index.Blocks.Write {
					log.Info("skipping setting index to read-only because already completed", "index", kibanaIndex)
					return nil
				}
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
		return kverrors.Wrap(err, "failed to set index to read only",
			"index", kibanaIndex)
	}
	return nil
}

func (mr *migrationRequest) createNewKibana6Index() error {
	curIndex, err := mr.esClient.GetIndex(kibana6Index)
	if err != nil {
		return kverrors.Wrap(err, "failed to get index",
			"index", kibanaIndex)
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
		return kverrors.Wrap(err, "failed to parse kibana 6 mappings")
	}

	index := &estypes.Index{
		Name: kibana6Index,
		Settings: &estypes.IndexSettings{
			Index: &estypes.IndexingSettings{
				NumberOfShards: 1,
				Format:         6,
				Mapper: &estypes.IndexMapperSettings{
					Dynamic: false,
				},
			},
		},
		Mappings: mappings,
	}

	if err := mr.esClient.CreateIndex(kibana6Index, index); err != nil {
		return kverrors.Wrap(err, "failed to create new index",
			"index", kibana6Index)
	}
	return nil
}

func (mr *migrationRequest) reIndexIntoKibana6() error {
	indices, err := mr.esClient.GetAllIndices(kibana6Index)
	if err != nil {
		return kverrors.Wrap(err, "failed to fetch doc count before re-indexing",
			"index", kibana6Index)
	}

	var index *estypes.CatIndicesResponse
	for i, idx := range indices {
		if idx.Index == kibana6Index {
			index = &indices[i]
			break
		}
	}

	if index == nil {
		return kverrors.New("failed to fetch doc count before re-indexing",
			"index", kibana6Index,
			"reason", "index not found")
	}

	if index.DocsCount != "0" {
		log.Info("skipping re-indexing because already completed", "from", kibanaIndex, "to", kibana6Index)
		return nil
	}

	err = mr.esClient.ReIndex(kibanaIndex, kibana6Index, kibanReIndexScript, "painless")
	if err != nil {
		return kverrors.Wrap(err, "failed to reindex")
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
		return kverrors.Wrap(err, "failed to update alias")
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
