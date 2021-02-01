package elasticsearch

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/inhies/go-bytesize"
	"github.com/tidwall/gjson"
)

func (ec *esClient) GetNodeDiskUsage(nodeName string) (string, float64, error) {
	es := ec.client
	res, err := es.Nodes.Stats(es.Nodes.Stats.WithPretty())
	usage := ""
	percentUsage := float64(-1)

	if err != nil {
		return usage, percentUsage, err
	}

	defer res.Body.Close()

	if res.IsError() {
		return usage, percentUsage, fmt.Errorf("ERROR: %s: %s", res.Status(), res)
	}

	body, _ := ioutil.ReadAll(res.Body)
	jsonStr := string(body)
	result := gjson.Get(jsonStr, "nodes").Value()

	if payload, ok := result.(map[string]interface{}); ok {
		for _, stats := range payload {
			// ignore the key name here, it is the node UUID
			if parseString("name", stats.(map[string]interface{})) == nodeName {
				total := parseFloat64("fs.total.total_in_bytes", stats.(map[string]interface{}))
				available := parseFloat64("fs.total.available_in_bytes", stats.(map[string]interface{}))

				percentUsage = (total - available) / total * 100.00
				usage = strings.TrimSuffix(fmt.Sprintf("%s", bytesize.New(total)-bytesize.New(available)), "B")

				break
			}
		}
	}

	return usage, percentUsage, nil
}
