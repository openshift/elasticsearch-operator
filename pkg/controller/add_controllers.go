package controller

import (
	"github.com/openshift/elasticsearch-operator/pkg/controller/elasticsearch"
	"github.com/openshift/elasticsearch-operator/pkg/controller/kibana"
	"github.com/openshift/elasticsearch-operator/pkg/controller/secret"
)

func init() {
	AddToManagerFuncs = append(AddToManagerFuncs,
		kibana.Add,
		elasticsearch.Add,
		secret.Add)
}
