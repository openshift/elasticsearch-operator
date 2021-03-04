package controllers

import (
	"context"
	"time"

	"github.com/openshift/elasticsearch-operator/internal/metrics"

	"github.com/ViaQ/logerr/log"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	loggingv1 "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/internal/k8shandler"
)

// ElasticsearchReconciler reconciles a Elasticsearch object
type ElasticsearchReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Elasticsearch object and makes changes based on the state read
// and what is in the Elasticsearch.Spec
var (
	reconcilePeriod = 30 * time.Second
	// reconcileResult = reconcile.Result{RequeueAfter: reconcilePeriod}
	reconcileResult = ctrl.Result{RequeueAfter: reconcilePeriod}
)

func (r *ElasticsearchReconciler) Reconcile(request ctrl.Request) (ctrl.Result, error) {
	// Fetch the Elasticsearch instance
	cluster := &loggingv1.Elasticsearch{}

	err := r.Get(context.TODO(), request.NamespacedName, cluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Flushing nodes", "objectKey", request.NamespacedName)
			k8shandler.FlushNodes(request.NamespacedName.Name, request.NamespacedName.Namespace)
			k8shandler.RemoveDashboardConfigMap(r.Client)
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if cluster.Spec.ManagementState == loggingv1.ManagementStateUnmanaged {
		// Cluster state changes from Managed -> Unmanaged, so set "unmanaged" as 1 and set "managed" as 0.
		metrics.SetEsClusterManagementStateUnmanaged()
		return ctrl.Result{}, nil
	}
	// Cluster state changes from Unmanaged -> Managed, so set "managed" as 1 and set "unmanaged" as 0.
	metrics.SetEsClusterManagementStateManaged()

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

	if err = k8shandler.Reconcile(cluster, r.Client); err != nil {
		return reconcileResult, err
	}

	return reconcileResult, nil
}

func (r *ElasticsearchReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("elasticsearch-controller").
		For(&loggingv1.Elasticsearch{}).
		Complete(r)
}
