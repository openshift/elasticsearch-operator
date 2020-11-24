package elasticsearchop

import (
	"io/ioutil"
	"log"
)

func (ec *esClient) GetClusterNodeVersions() ([]string, error) {
	es := ec.eoclient
	res, err := es.Info()
	res, err = es.Cluster.Stats(es.Cluster.Stats.WithPretty())
	if err != nil {
		log.Fatalf("Error getting the cluster response: %s\n", err)
	}
	defer res.Body.Close()
	var nodeVersions []string

	if res.IsError() {
		log.Printf("ERROR: %s: %s", res.Status(), res)
	} else {
		body, _ := ioutil.ReadAll(res.Body)
		str := string(body)
		outputmap, err := getMapFromBody(str)

		if err != nil {
			log.Fatalf("Error getting the cluster response: %s\n", err)
		}

		if versions := walkInterfaceMap("nodes.versions", outputmap); versions != nil {
			for _, value := range versions.([]interface{}) {
				version := value.(string)
				nodeVersions = append(nodeVersions, version)
			}
		}
	}
	return nodeVersions, nil
}
