package stub

import (
	"github.com/t0ffel/elasticsearch-operator/pkg/apis/elasticsearch/v1alpha1"
	"github.com/t0ffel/elasticsearch-operator/pkg/k8shandler"

	"github.com/operator-framework/operator-sdk/pkg/sdk/handler"
	"github.com/operator-framework/operator-sdk/pkg/sdk/types"
)

func NewHandler() handler.Handler {
	return &Handler{}
}

type Handler struct {
	// Fill me
}

func (h *Handler) Handle(ctx types.Context, event types.Event) error {
	if event.Deleted {
		return nil
	}

	switch o := event.Object.(type) {
	case *v1alpha1.Elasticsearch:
		return k8shandler.Reconcile(o)
	}
	return nil
}

