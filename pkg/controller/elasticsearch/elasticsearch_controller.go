package elasticsearch

import (
	"context"
	"fmt"
	"log"
	"time"

	loggingv1alpha1 "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1alpha1"
	"github.com/openshift/elasticsearch-operator/pkg/k8shandler"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

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
	err = c.Watch(&source.Kind{Type: &loggingv1alpha1.Elasticsearch{}}, &handler.EnqueueRequestForObject{})
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
func (r *ReconcileElasticsearch) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Printf("Reconciling Elasticsearch %s/%s\n", request.Namespace, request.Name)

	// Fetch the Elasticsearch instance
	instance := &loggingv1alpha1.Elasticsearch{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	err = r.reconsileElasticsearch(instance)
	return reconcile.Result{RequeueAfter: 5 * time.Second}, err
}

// reconsileElasticsearch reconciles the cluster's state to the spec specified
func (r *ReconcileElasticsearch) reconsileElasticsearch(es *loggingv1alpha1.Elasticsearch) (err error) {
	// Ensure existence of services
	logrus.Debugf("Begin Reconcile of Elasticsearch cluster %v", es.ClusterName)
	err = k8shandler.CreateOrUpdateServices(r.client, es)
	if err != nil {
		return fmt.Errorf("Failed to reconcile Services for Elasticsearch cluster: %v", err)
	}

	// Ensure existence of servicesaccount
	serviceAccountName, err := k8shandler.CreateOrUpdateServiceAccount(r.client, es)
	if err != nil {
		return fmt.Errorf("Failed to reconcile ServiceAccount for Elasticsearch cluster: %v", err)
	}

	// Ensure existence of config maps
	configMapName, err := k8shandler.CreateOrUpdateConfigMaps(r.client, es)
	if err != nil {
		return fmt.Errorf("Failed to reconcile ConfigMaps for Elasticsearch cluster: %v", err)
	}

	// Ensure existence of prometheus rules
	if err = k8shandler.CreateOrUpdatePrometheusRules(r.client, es); err != nil {
		return fmt.Errorf("Failed to reconcile PrometheusRules for Elasticsearch cluster: %v", err)
	}

	// Ensure existence of service monitors
	if err = k8shandler.CreateOrUpdateServiceMonitors(r.client, es); err != nil {
		return fmt.Errorf("Failed to reconcile Service Monitors for Elasticsearch cluster: %v", err)
	}

	// TODO: Ensure existence of storage?

	// Ensure Elasticsearch cluster itself is up to spec
	err = k8shandler.CreateOrUpdateElasticsearchCluster(r.client, es, configMapName, serviceAccountName)
	if err != nil {
		return fmt.Errorf("Failed to reconcile Elasticsearch deployment spec: %v", err)
	}

	logrus.Debugf("End Reconcile of Elasticsearch cluster %v", es.ClusterName)
	return nil
}
