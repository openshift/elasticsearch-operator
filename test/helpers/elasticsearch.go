package helpers

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo"

	"github.com/openshift/elasticsearch-operator/pkg/elasticsearch"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewFakeElasticsearchChatter(responses map[string]FakeElasticsearchResponse) *FakeElasticsearchChatter {
	return &FakeElasticsearchChatter{
		Requests:  map[string]string{},
		Responses: responses,
	}
}

type FakeElasticsearchChatter struct {
	Requests  map[string]string
	Responses map[string]FakeElasticsearchResponse
}

type FakeElasticsearchResponse struct {
	Error      error
	StatusCode int
	Body       string
}

func (chat *FakeElasticsearchChatter) GetRequest(key string) (string, bool) {
	request, found := chat.Requests[key]
	return request, found
}

func (chat *FakeElasticsearchChatter) GetResponse(key string) (FakeElasticsearchResponse, bool) {
	response, found := chat.Responses[key]
	return response, found
}

func (response *FakeElasticsearchResponse) BodyAsResponseBody() map[string]interface{} {
	body := &map[string]interface{}{}
	if err := json.Unmarshal([]byte(response.Body), body); err != nil {
		Fail(fmt.Sprintf("Unable to convert to response body %q: %v", response.Body, err))
	}
	return *body
}

func NewFakeElasticsearchClient(cluster, namespace string, k8sClient client.Client, chatter *FakeElasticsearchChatter) elasticsearch.Client {
	sendFakeRequest := NewFakeSendRequestFn(chatter)
	c := elasticsearch.NewClient(cluster, namespace, k8sClient)
	c.SetSendRequestFn(sendFakeRequest)
	return c
}

func NewFakeSendRequestFn(chatter *FakeElasticsearchChatter) elasticsearch.FnEsSendRequest {
	return func(cluster, namespace string, payload *elasticsearch.EsRequest, client client.Client) {
		chatter.Requests[payload.URI] = payload.RequestBody
		if val, found := chatter.GetResponse(payload.URI); found {
			payload.Error = val.Error
			payload.StatusCode = val.StatusCode
			payload.ResponseBody = val.BodyAsResponseBody()
		} else {
			payload.Error = fmt.Errorf("No fake response found for uri %q: %v", payload.URI, payload)
		}
	}
}
