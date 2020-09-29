package controller

import (
	"github.com/openshift/elasticsearch-operator/pkg/controller/elasticsearch"
	"github.com/openshift/elasticsearch-operator/pkg/controller/kibana"
)

func init() {
	AddToManagerFuncs = append(AddToManagerFuncs,
		kibana.Add,
		elasticsearch.Add)
}
