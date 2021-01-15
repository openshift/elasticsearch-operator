package elasticsearch

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"testing"

	elasticsearch6 "github.com/elastic/go-elasticsearch/v6"
	"github.com/elastic/go-elasticsearch/v6/esapi"
	estypes "github.com/openshift/elasticsearch-operator/internal/types/elasticsearch"
	"github.com/tidwall/gjson"
)

type mockTransp struct {
	response *http.Response
	err      error
}

func (t *mockTransp) Perform(req *http.Request) (*http.Response, error) {
	return t.response, t.err
}

func getFakeESClient(clusterName, namespace string, res *http.Response, err error) Client {

	mocktransport := &mockTransp{res, err}
	elasticsearchClient := elasticsearch6.Client{Transport: mocktransport, API: esapi.New(mocktransport)}
	esClient := NewClient(clusterName, namespace, elasticsearchClient)

	return esClient
}

func TestGetClusterNodeVersion_actual(t *testing.T) {
	esAddr := "http://localhost:9200"

	elasticsearchClient, err := getESClient(esAddr)

	/*body := `[{"health" : "yellow","status" : "open","index" : "test_create","uuid" : "4aCsuyn6SbGTDWwd7Ypwrw","pri" : "5","rep" : "1","docs.count" : "1","docs.deleted" : "0","store.size" : "4.4kb","pri.store.size" : "4.4kb"}]`
	index := &estypes.Index{}
	err = json.Unmarshal([]byte(body), index)*/
	esClient := NewClient("default", "default", *elasticsearchClient)
	//esClient := NewClient("default", "default", k
	log.Printf("testing")
	//err = esClient.ReIndex("test", "test_create", "", "")
	got, err := esClient.ListIndicesForAlias("*")
	log.Println(got)
	log.Println(err)
}

func TestParse(t *testing.T) {
	jsonStr := `{"test" : {"aliases" : {"2016" : {"filter" : {"term" : {"year" : 2016 }}}}},"test_create" : {"aliases" : {"2017" : {"filter" : {"term" : {"year" : 2017 }}}}}`

	log.Printf(gjson.Get(jsonStr, "test").Raw)
	log.Printf(gjson.Get(jsonStr, "*").Raw)

	m, ok := gjson.Parse(jsonStr).Value().(map[string]interface{})

	if !ok {
		// not a map
		fmt.Println("not map")

	}

	for key, element := range m {
		//output := element[key]
		data, _ := json.Marshal(element)
		fmt.Printf("%s", data)
		output := fmt.Sprintf("%v   %v", key, element)
		fmt.Println(output)
	}
	fmt.Println(m)

}

func TestUpdateAlias_actual(t *testing.T) {
	esAddr := "http://localhost:9200"

	elasticsearchClient, err := getESClient(esAddr)

	esClient := NewClient("default", "default", *elasticsearchClient)
	body := `{"actions" : [{ "add" : { "index" : "test", "alias" : "alias1" } }]}`
	actions := &estypes.AliasActions{}
	err = json.Unmarshal([]byte(body), actions)
	err = esClient.UpdateAlias(*actions)
	if err != nil {
		t.Errorf("got err: %s", err)
	}
	log.Println(err)
}

func TestGetIndexSettings_actual(t *testing.T) {
	esAddr := "http://localhost:9200"

	elasticsearchClient, err := getESClient(esAddr)

	esClient := NewClient("default", "default", *elasticsearchClient)

	res, err := esClient.GetIndexSettings("my-index-000001")
	if err != nil {
		t.Errorf("got err: %s", err)
	}

	log.Println(res)
}

func TestGetIndex_actual(t *testing.T) {
	esAddr := "http://localhost:9200"

	elasticsearchClient, err := getESClient(esAddr)

	esClient := NewClient("default", "default", *elasticsearchClient)

	res, err := esClient.GetIndex("my-index-000001")
	if err != nil {
		t.Errorf("got err: %s", err)
	}

	log.Println(res)
}

func TestNodeDiskUsage_actual(t *testing.T) {
	esAddr := "http://localhost:9200"

	elasticsearchClient, err := getESClient(esAddr)

	esClient := NewClient("default", "default", *elasticsearchClient)

	usage, percentage, err := esClient.GetNodeDiskUsage("l-1tztk")
	if err != nil {
		t.Errorf("got err: %s", err)
	}

	log.Println(usage)
	log.Println(percentage)
}

func TestUpdateReplicaCount_actual(t *testing.T) {
	esAddr := "http://localhost:9200"

	elasticsearchClient, err := getESClient(esAddr)

	esClient := NewClient("default", "default", *elasticsearchClient)

	err = esClient.UpdateReplicaCount(8)
	if err != nil {
		t.Errorf("got err: %s", err)
	}

}

func TestGetIndexTemplates_actual(t *testing.T) {
	esAddr := "http://localhost:9200"

	elasticsearchClient, err := getESClient(esAddr)

	esClient := NewClient("default", "default", *elasticsearchClient)

	err = esClient.UpdateTemplatePrimaryShards(5)
	if err != nil {
		t.Errorf("got err: %s", err)
	}
	//log.Println(res)

}
