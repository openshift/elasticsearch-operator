package controllers

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	loggingv1 "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	"github.com/openshift/elasticsearch-operator/internal/k8shandler"
)

// SecretReconciler reconciles a Secret object
type SecretReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *SecretReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()

	cluster := &loggingv1.Elasticsearch{}
	esName := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
	}

	err := r.Get(context.TODO(), esName, cluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	ok, err := k8shandler.SecretReconcile(cluster, r.Client)
	if !ok {
		return reconcileResult, err
	}
	return ctrl.Result{}, err
}

func esSecretUpdatePredicate(r client.Client) predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			cluster := &loggingv1.Elasticsearch{}
			esName := types.NamespacedName{
				Namespace: e.MetaNew.GetNamespace(),
				Name:      e.MetaNew.GetName(),
			}
			err := r.Get(context.TODO(), esName, cluster)
			if err != nil {
				return false
			}
			return true
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithEventFilter(esSecretUpdatePredicate(r.Client)).
		Complete(r)
}
