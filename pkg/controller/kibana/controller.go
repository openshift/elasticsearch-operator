package kibana

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	loggingv1 "github.com/openshift/elasticsearch-operator/pkg/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/ViaQ/logerr/log"
	"github.com/openshift/elasticsearch-operator/pkg/elasticsearch"
	"github.com/openshift/elasticsearch-operator/pkg/k8shandler"
	"github.com/openshift/elasticsearch-operator/pkg/k8shandler/kibana"
	"github.com/openshift/elasticsearch-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new Kibana Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileKibana{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// map handlers to be used for all non-kibana CR events
var globalMapHandler = handler.EnqueueRequestsFromMapFunc{
	ToRequests: handler.ToRequestsFunc(getKibanaEvents),
}

var namespacedMapHandler = handler.EnqueueRequestsFromMapFunc{
	ToRequests: handler.ToRequestsFunc(getNamespacedKibanaEvent),
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("kibana-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Kibana
	err = c.Watch(&source.Kind{Type: &loggingv1.Kibana{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for updates to the kibana secret
	secretPred := predicate.Funcs{
		UpdateFunc:  func(e event.UpdateEvent) bool { return handleSecret(e.MetaNew) },
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}

	if err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &namespacedMapHandler, secretPred); err != nil {
		return err
	}

	// Watch for updates to the global proxy config only
	proxyPred := predicate.Funcs{
		UpdateFunc:  func(e event.UpdateEvent) bool { return true },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}

	// Watch for changes to the proxy resource.
	if err = c.Watch(&source.Kind{Type: &configv1.Proxy{}}, &globalMapHandler, proxyPred); err != nil {
		return err
	}

	// Watch for changes to the additional trust bundle configmap
	trustedBundlePred := predicate.Funcs{
		UpdateFunc:  func(e event.UpdateEvent) bool { return handleConfigMap(e.MetaNew) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		CreateFunc:  func(e event.CreateEvent) bool { return handleConfigMap(e.Meta) },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
	if err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &namespacedMapHandler, trustedBundlePred); err != nil {
		return err
	}

	podPred := predicate.Funcs{
		UpdateFunc:  func(e event.UpdateEvent) bool { return handlePod(e.MetaNew) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
	if err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &namespacedMapHandler, podPred); err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileKibana implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileKibana{}

type RegisteredNamespacedNames struct {
	registered []types.NamespacedName
	mux        sync.Mutex
}

var registeredKibanas RegisteredNamespacedNames

// ReconcileKibana reconciles a Kibana object
type ReconcileKibana struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Kibana object and makes changes based on the state read
func (r *ReconcileKibana) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	// get CR
	kibanaInstance := &loggingv1.Kibana{}
	key := types.NamespacedName{
		Name:      request.Name,
		Namespace: request.Namespace,
	}

	err := r.client.Get(context.TODO(), key, kibanaInstance)
	if err != nil {
		if errors.IsNotFound(err) {
			// the CR no longer exists, since it will be cleaned up by the scheduler we don't want to trigger an event for it
			unregisterKibanaNamespacedName(request)
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	if kibanaInstance.Spec.ManagementState == loggingv1.ManagementStateUnmanaged {
		return reconcile.Result{}, nil
	}

	// keep track of the fact that we processed this kibana for future events and for mapping
	registerKibanaNamespacedName(request)

	es, err := k8shandler.GetElasticsearchCR(r.client, request.Namespace)
	if err != nil {
		log.Info("skipping kibana reconciliation", "namespace", request.Namespace, "error", err)
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}

	esClient := elasticsearch.NewClient(es.Name, es.Namespace, r.client)
	proxyCfg, err := kibana.GetProxyConfig(r.client)
	if err != nil {
		return reconcile.Result{}, err
	}

	if err := kibana.Reconcile(kibanaInstance, r.client, esClient, proxyCfg); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// handleSecret returns true if metaname is such that (cr_name) matches or that (cr_name + "-proxy") matches
func handleSecret(meta metav1.Object) bool {

	// iterate over registeredKibanas that match the namespace
	namespace := meta.GetNamespace()

	registeredKibanas.mux.Lock()
	defer registeredKibanas.mux.Unlock()

	for _, kibana := range registeredKibanas.registered {
		if kibana.Namespace == namespace {
			if utils.ContainsString(
				[]string{kibana.Name, fmt.Sprintf("%s-proxy", kibana.Name)},
				meta.GetName(),
			) {
				return true
			}
		}
	}

	return false
}

// handleConfigMap returns true if metaname is contained in the GlobalProxyList constant
func handleConfigMap(meta metav1.Object) bool {

	// iterate over registeredKibanas that match the namespace
	namespace := meta.GetNamespace()

	registeredKibanas.mux.Lock()
	defer registeredKibanas.mux.Unlock()

	for _, kibana := range registeredKibanas.registered {
		if kibana.Namespace == namespace {
			return utils.ContainsString(constants.ReconcileForGlobalProxyList, meta.GetName())
		}
	}

	return false
}

// handlePod returns true if metaname contains a registered kibana name as substring
func handlePod(meta metav1.Object) bool {

	// iterate over registeredKibanas that match the namespace
	namespace := meta.GetNamespace()

	registeredKibanas.mux.Lock()
	defer registeredKibanas.mux.Unlock()

	for _, kibana := range registeredKibanas.registered {
		if kibana.Namespace == namespace {
			return strings.Contains(meta.GetName(), kibana.Name)
		}
	}

	return false
}

func registerKibanaNamespacedName(request reconcile.Request) {
	// check to see if the namespaced name is already registered first
	found := false

	registeredKibanas.mux.Lock()
	defer registeredKibanas.mux.Unlock()

	for _, kibana := range registeredKibanas.registered {
		if kibana == request.NamespacedName {
			found = true
		}
	}

	// if not, add it to registeredKibanas
	if !found {
		log.Info("Registering future events", "name", request.NamespacedName)
		registeredKibanas.registered = append(registeredKibanas.registered, request.NamespacedName)
	}
}

func unregisterKibanaNamespacedName(request reconcile.Request) {
	// look for a namespacedname registered already
	found := false
	index := -1

	registeredKibanas.mux.Lock()
	defer registeredKibanas.mux.Unlock()

	for i, kibana := range registeredKibanas.registered {
		if kibana == request.NamespacedName {
			found = true
			index = i
		}
	}

	// if we find it, remove it from registeredKibanas
	if found {
		log.Info("Unregistering future events", "name", request.NamespacedName)
		registeredKibanas.registered = append(registeredKibanas.registered[:index], registeredKibanas.registered[index+1:]...)
	}
}

// this is used for when we get a proxy config or trusted CA change
// it will return requests for all known kibana CRs
func getKibanaEvents(a handler.MapObject) []reconcile.Request {

	requests := []reconcile.Request{}

	registeredKibanas.mux.Lock()
	defer registeredKibanas.mux.Unlock()

	for _, kibana := range registeredKibanas.registered {
		requests = append(requests, reconcile.Request{NamespacedName: kibana})
	}

	return requests
}

// this is used when we have a secret update
// it will return requests for all known kibana CRs that match
func getNamespacedKibanaEvent(a handler.MapObject) []reconcile.Request {

	namespace := a.Meta.GetNamespace()
	requests := []reconcile.Request{}

	registeredKibanas.mux.Lock()
	defer registeredKibanas.mux.Unlock()

	for _, kibana := range registeredKibanas.registered {
		if kibana.Namespace == namespace {
			requests = append(requests, reconcile.Request{NamespacedName: kibana})
		}
	}

	return requests
}
