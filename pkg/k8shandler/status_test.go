package k8shandler

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/google/go-cmp/cmp"

	loggingv1 "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
)

func TestPruneMissingNodes(t *testing.T) {
	nodes = map[string][]NodeTypeInterface{}

	tests := []struct {
		desc        string
		cluster     *loggingv1.Elasticsearch
		deployments []runtime.Object
		pods        []runtime.Object
		status      *loggingv1.ElasticsearchStatus
		missingPods []string
		missingDpl  []string
		wantStatus  *loggingv1.ElasticsearchStatus
		wantErr     error
	}{
		{
			desc: "no prunning",
			cluster: &loggingv1.Elasticsearch{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "elasticsearch",
					Namespace: "openshift-logging",
				},
			},
			deployments: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "elasticsearch-cdm-1-deadbeef",
						Namespace: "openshift-logging",
					},
				},
			},
			pods: []runtime.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "elasticsearch-cdm-1-deadbeef-abcdef",
						Namespace: "openshift-logging",
						Labels: map[string]string{
							"component":    "elasticsearch",
							"cluster-name": "elasticsearch",
							"node-name":    "elasticsearch-cdm-1-deadbeef",
						},
					},
				},
			},
			status: &loggingv1.ElasticsearchStatus{
				Nodes: []loggingv1.ElasticsearchNodeStatus{
					{DeploymentName: "elasticsearch-cdm-1-deadbeef"},
				},
				Pods: map[loggingv1.ElasticsearchNodeRole]loggingv1.PodStateMap{
					loggingv1.ElasticsearchRoleClient: {"ready": {"elasticsearch-cdm-1-deadbeef-abcdef"}},
					loggingv1.ElasticsearchRoleData:   {"ready": {"elasticsearch-cdm-1-deadbeef-abcdef"}},
					loggingv1.ElasticsearchRoleMaster: {"ready": {"elasticsearch-cdm-1-deadbeef-abcdef"}},
				},
			},
			wantStatus: &loggingv1.ElasticsearchStatus{
				Nodes: []loggingv1.ElasticsearchNodeStatus{
					{DeploymentName: "elasticsearch-cdm-1-deadbeef"},
				},
				Pods: map[loggingv1.ElasticsearchNodeRole]loggingv1.PodStateMap{
					loggingv1.ElasticsearchRoleClient: {"ready": {"elasticsearch-cdm-1-deadbeef-abcdef"}},
					loggingv1.ElasticsearchRoleData:   {"ready": {"elasticsearch-cdm-1-deadbeef-abcdef"}},
					loggingv1.ElasticsearchRoleMaster: {"ready": {"elasticsearch-cdm-1-deadbeef-abcdef"}},
				},
			},
		},
		{
			desc: "single node pruning",
			cluster: &loggingv1.Elasticsearch{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "elasticsearch",
					Namespace: "openshift-logging",
				},
			},
			deployments: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "elasticsearch-cdm-1-deadbeef",
						Namespace: "openshift-logging",
					},
				},
			},
			pods: []runtime.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "elasticsearch-cdm-1-deadbeef-abcdef",
						Namespace: "openshift-logging",
						Labels: map[string]string{
							"component":    "elasticsearch",
							"cluster-name": "elasticsearch",
							"node-name":    "elasticsearch-cdm-1-deadbeef",
						},
					},
				},
			},
			missingDpl: []string{"elasticsearch-cdm-1-deadbeef"},
			status: &loggingv1.ElasticsearchStatus{
				Nodes: []loggingv1.ElasticsearchNodeStatus{
					{DeploymentName: "elasticsearch-cdm-1-deadbeef"},
				},
				Pods: map[loggingv1.ElasticsearchNodeRole]loggingv1.PodStateMap{
					loggingv1.ElasticsearchRoleClient: {"ready": {"elasticsearch-cdm-1-deadbeef-abcdef"}},
					loggingv1.ElasticsearchRoleData:   {"ready": {"elasticsearch-cdm-1-deadbeef-abcdef"}},
					loggingv1.ElasticsearchRoleMaster: {"ready": {"elasticsearch-cdm-1-deadbeef-abcdef"}},
				},
			},
			wantStatus: &loggingv1.ElasticsearchStatus{
				Nodes: []loggingv1.ElasticsearchNodeStatus{},
				Pods: map[loggingv1.ElasticsearchNodeRole]loggingv1.PodStateMap{
					loggingv1.ElasticsearchRoleClient: {"ready": {"elasticsearch-cdm-1-deadbeef-abcdef"}},
					loggingv1.ElasticsearchRoleData:   {"ready": {"elasticsearch-cdm-1-deadbeef-abcdef"}},
					loggingv1.ElasticsearchRoleMaster: {"ready": {"elasticsearch-cdm-1-deadbeef-abcdef"}},
				},
			},
		},
		{
			desc: "single node pruning including pods",
			cluster: &loggingv1.Elasticsearch{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "elasticsearch",
					Namespace: "openshift-logging",
				},
			},
			deployments: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "elasticsearch-cdm-1-deadbeef",
						Namespace: "openshift-logging",
					},
				},
			},
			pods: []runtime.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "elasticsearch-cdm-1-deadbeef-abcdef",
						Namespace: "openshift-logging",
						Labels: map[string]string{
							"component":    "elasticsearch",
							"cluster-name": "elasticsearch",
							"node-name":    "elasticsearch-cdm-1-deadbeef",
						},
					},
				},
			},
			missingDpl:  []string{"elasticsearch-cdm-1-deadbeef"},
			missingPods: []string{"elasticsearch-cdm-1-deadbeef-abcdef"},
			status: &loggingv1.ElasticsearchStatus{
				Nodes: []loggingv1.ElasticsearchNodeStatus{
					{DeploymentName: "elasticsearch-cdm-1-deadbeef"},
				},
				Pods: map[loggingv1.ElasticsearchNodeRole]loggingv1.PodStateMap{
					loggingv1.ElasticsearchRoleClient: {"ready": {"elasticsearch-cdm-1-deadbeef-abcdef"}},
					loggingv1.ElasticsearchRoleData:   {"ready": {"elasticsearch-cdm-1-deadbeef-abcdef"}},
					loggingv1.ElasticsearchRoleMaster: {"ready": {"elasticsearch-cdm-1-deadbeef-abcdef"}},
				},
			},
			wantStatus: &loggingv1.ElasticsearchStatus{
				Nodes: []loggingv1.ElasticsearchNodeStatus{},
				Pods: map[loggingv1.ElasticsearchNodeRole]loggingv1.PodStateMap{
					loggingv1.ElasticsearchRoleClient: {"ready": {}},
					loggingv1.ElasticsearchRoleData:   {"ready": {}},
					loggingv1.ElasticsearchRoleMaster: {"ready": {}},
				},
			},
		},
	}
	for _, test := range tests {
		test := test

		// Populate fake client with api server objects
		client := newFakeClient(test.pods, test.deployments, test.missingPods, test.missingDpl)

		// Populate nodes in operator memory
		key := nodeMapKey(test.cluster.Name, test.cluster.Namespace)
		nodes[key] = populateNodes(test.cluster.Name, test.deployments, client)

		// Define new elasticsearch CR request
		er := &ElasticsearchRequest{client: client, cluster: test.cluster}

		err := er.pruneMissingNodes(test.status)
		if err != test.wantErr {
			t.Errorf("got: %s, want: %s", err, test.wantErr)
		}

		if diff := cmp.Diff(test.status, test.wantStatus); diff != "" {
			t.Errorf("diff: %s", diff)
		}
	}
}

func newFakeClient(pods, deployments []runtime.Object, missingPods, missingDpl []string) client.Client {
	var objs []runtime.Object
	for _, dpl := range deployments {
		isMissing := false
		for _, missing := range missingDpl {
			if missing == dpl.(*appsv1.Deployment).Name {
				isMissing = true
				break
			}
		}

		if !isMissing {
			objs = append(objs, dpl)
		}
	}

	for _, pod := range pods {
		isMissing := false
		for _, missing := range missingPods {
			if missing == pod.(*corev1.Pod).Name {
				isMissing = true
				break
			}
		}

		if !isMissing {
			objs = append(objs, pod)
		}
	}
	return fake.NewFakeClient(objs...)
}

func populateNodes(clusterName string, objs []runtime.Object, client client.Client) []NodeTypeInterface {
	nodes := []NodeTypeInterface{}
	for _, dpl := range objs {
		dpl := dpl.(*appsv1.Deployment)
		node := &deploymentNode{
			clusterName: clusterName,
			self:        *dpl,
			client:      client,
		}
		nodes = append(nodes, node)
	}
	return nodes
}
