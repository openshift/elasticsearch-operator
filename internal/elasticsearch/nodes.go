package elasticsearch

func (ec *esClient) GetNodeDiskUsage(nodeName string) (string, float64, error) {

	usage := ""
	percentUsage := float64(-1)

	return usage, percentUsage, nil
}
