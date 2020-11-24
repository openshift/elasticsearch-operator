package elasticsearch

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/inhies/go-bytesize"
)

func (ec *esClient) GetNodeDiskUsage(nodeName string) (string, float64, error) {
	es := ec.client
	res, err := es.Nodes.Stats(es.Nodes.Stats.WithPretty())
	usage := ""
	percentUsage := float64(-1)

	if err != nil {
		return usage, percentUsage, err
	}

	ec.fnSendEsRequest(ec.cluster, ec.namespace, payload, ec.k8sClient)

	usage := ""
	percentUsage := float64(-1)

	if payload, ok := payload.ResponseBody["nodes"].(map[string]interface{}); ok {
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

	return usage, percentUsage, payload.Error
}
