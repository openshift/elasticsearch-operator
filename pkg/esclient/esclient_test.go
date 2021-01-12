package elasticsearch

import (
	"log"
	"testing"
)

func Test_actual(t *testing.T) {
	esAddr := "http://localhost:9200"

	elasticsearchClient, err := getESClient(esAddr)

	esClient := NewClient("default", "default", *elasticsearchClient)
	log.Printf("testing")
	got, err := esClient.GetThresholdEnabled()
	log.Println(got)
	log.Println(err)

	ans1, err1 := esClient.GetClusterNodeVersions()
	log.Println(ans1)
	log.Println(err1)

}
