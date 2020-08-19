package elasticsearch

import (
	"context"
	"time"

	loggingv1 "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/openshift/elasticsearch-operator/pkg/k8shandler"
	"github.com/openshift/elasticsearch-operator/pkg/log"
)

// Add creates a new Elasticsearch Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileElasticsearch{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("elasticsearch-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Elasticsearch
	err = c.Watch(&source.Kind{Type: &loggingv1.Elasticsearch{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileElasticsearch{}

// ReconcileElasticsearch reconciles a Elasticsearch object
type ReconcileElasticsearch struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Elasticsearch object and makes changes based on the state read
// and what is in the Elasticsearch.Spec
var (
	reconcilePeriod = 30 * time.Second
	reconcileResult = reconcile.Result{RequeueAfter: reconcilePeriod}
)

func (r *ReconcileElasticsearch) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Elasticsearch instance
	cluster := &loggingv1.Elasticsearch{}

	err := r.client.Get(context.TODO(),
		request.NamespacedName, cluster)

	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Flushing nodes", "objectKey", request.NamespacedName)
			k8shandler.FlushNodes(request.NamespacedName.Name, request.NamespacedName.Namespace)
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	if cluster.Spec.ManagementState == loggingv1.ManagementStateUnmanaged {
		return reconcile.Result{}, nil
	}

	if cluster.Spec.Spec.Image != "" {
		if cluster.Status.Conditions == nil {
			cluster.Status.Conditions = []loggingv1.ClusterCondition{}
		}
		exists := false
		for _, condition := range cluster.Status.Conditions {
			if condition.Type == loggingv1.CustomImage {
				exists = true
				break
			}
		}
		if !exists {
			cluster.Status.Conditions = append(cluster.Status.Conditions, loggingv1.ClusterCondition{
				Type:               loggingv1.CustomImage,
				Status:             v1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Reason:             "CustomImageUnsupported",
				Message:            "Specifiying a custom image from the custom resource is not supported",
			})
		}

	}

	if err = k8shandler.Reconcile(cluster, r.client); err != nil {
		return reconcileResult, err
	}

	return reconcileResult, nil
}
