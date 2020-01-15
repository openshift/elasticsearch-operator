package proxyconfig

import (
	"context"
	"time"

	"github.com/openshift/elasticsearch-operator/pkg/k8shandler/kibana"

	configv1 "github.com/openshift/api/config/v1"
	loggingv1 "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/pkg/constants"

	"github.com/openshift/elasticsearch-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	log             = logf.Log.WithName("controller_proxyconfig")
	reconcilePeriod = 30 * time.Second
	reconcileResult = reconcile.Result{RequeueAfter: reconcilePeriod}
)

// Add creates a new ClusterLogging Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	if err := configv1.Install(mgr.GetScheme()); err != nil {
		return &ReconcileProxyConfig{}
	}

	return &ReconcileProxyConfig{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("proxyconfig-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to the additional trust bundle configmap in "openshift-logging".
	pred := predicate.Funcs{
		UpdateFunc:  func(e event.UpdateEvent) bool { return handleConfigMap(e.MetaNew) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return handleConfigMap(e.Meta) },
		CreateFunc:  func(e event.CreateEvent) bool { return handleConfigMap(e.Meta) },
		GenericFunc: func(e event.GenericEvent) bool { return handleConfigMap(e.Meta) },
	}
	if err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForObject{}, pred); err != nil {
		return err
	}

	// Watch for changes to the proxy resource.
	if err = c.Watch(&source.Kind{Type: &configv1.Proxy{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileProxyConfig{}

// ReconcileProxyConfig reconciles a ClusterLogging object
type ReconcileProxyConfig struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a cluster-scoped named "cluster" as well as
// trusted CA bundle configmap objects for the collector and the visualization resources.
// When the user configured and/or system certs are updated, the change is propagated to the
// configmap objects and this reconciler triggers to restart those pods.
func (r *ReconcileProxyConfig) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	proxyNamespacedName := types.NamespacedName{Name: constants.ProxyName}
	proxyConfig := &configv1.Proxy{}
	if request.NamespacedName == proxyNamespacedName || utils.ContainsString(constants.ReconcileForGlobalProxyList, request.Name) {
		if err := r.client.Get(context.TODO(), proxyNamespacedName, proxyConfig); err != nil {
			if apierrors.IsNotFound(err) {
				// Request object not found, could have been deleted after reconcile request.
				// Return and don't requeue
				return reconcile.Result{}, nil
			}
			// Error reading the object - just return without requeuing.
			return reconcile.Result{}, err
		}
	} else {
		return reconcile.Result{}, nil
	}

	kibanaInstance := &loggingv1.Kibana{}
	err := r.client.Get(context.TODO(), request.NamespacedName, kibanaInstance)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if kibanaInstance.Spec.ManagementState == loggingv1.ManagementStateUnmanaged {
		return reconcile.Result{}, nil
	}

	if err := kibana.ReconcileKibana(kibanaInstance, r.client, proxyConfig); err != nil {
		// Failed to reconcile - requeuing.
		return reconcileResult, err
	}

	return reconcile.Result{}, nil
}

func handleConfigMap(meta metav1.Object) bool {
	return utils.ContainsString(constants.ReconcileForGlobalProxyList, meta.GetName())
}
