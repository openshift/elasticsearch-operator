package elasticsearch

import (
	"log"
	"testing"
)

func TestHeaderGenEmptyToken(t *testing.T) {
	tokenFile := "../../test/files/emptyToken"

	_, ok := readSAToken(tokenFile)

	if ok {
		t.Errorf("Expected to be unable to read file [%s] due to empty file but could", tokenFile)
	}
}

func TestHeaderGenWithToken(t *testing.T) {
	tokenFile := "../../test/files/testToken"

	expected := "test\n"

	actual, ok := readSAToken(tokenFile)

	if !ok {
		t.Errorf("Expected to be able to read file [%s] but couldn't", tokenFile)
	} else {
		if actual != expected {
			t.Errorf("Expected %q but got %q", expected, actual)
		}
	}
}

func TestHeaderGenWithNoToken(t *testing.T) {
	tokenFile := "../../test/files/errorToken"

	_, ok := readSAToken(tokenFile)

	if ok {
		t.Errorf("Expected to be unable to read file [%s]", tokenFile)
	}
}

func NTestGetIndexSettings_actual(t *testing.T) {
	esAddr := "http://localhost:9200"

	elasticsearchClient, err := getESClient(esAddr)

	esClient := NewClient("default", "default", elasticsearchClient)

	res, err := esClient.GetIndexSettings("my-index-000001")
	if err != nil {
		t.Errorf("got err: %s", err)
	}

	log.Println(res)
}
