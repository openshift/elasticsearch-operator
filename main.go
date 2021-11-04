package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	apiruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	configv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/console/v1"
	oauth "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	securityv1 "github.com/openshift/api/security/v1"

	loggingv1 "github.com/openshift/elasticsearch-operator/apis/logging/v1"
	controllers "github.com/openshift/elasticsearch-operator/controllers/logging"
	"github.com/openshift/elasticsearch-operator/internal/metrics"
	"github.com/openshift/elasticsearch-operator/version"

	"github.com/ViaQ/logerr/log"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = apiruntime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(loggingv1.AddToScheme(scheme))

	utilruntime.Must(routev1.AddToScheme(scheme))
	utilruntime.Must(consolev1.AddToScheme(scheme))
	utilruntime.Must(oauth.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))
	utilruntime.Must(configv1.AddToScheme(scheme))
	utilruntime.Must(securityv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe end point binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	flag.Parse()

	log.MustInit("elasticsearch-operator")
	log.Info("starting up...",
		"operator_version", version.Version,
		"go_version", runtime.Version(),
		"go_os", runtime.GOOS,
		"go_arch", runtime.GOARCH,
	)

	namespace, err := getWatchNamespace()
	if err != nil {
		log.Error(err, "Failed to get watch namespace")
		os.Exit(1)
	}

	ll := log.WithValues("namespace", namespace)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Namespace:              namespace,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "d471c3b1.openshift.io",
		Logger:                 ll,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.ElasticsearchReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Elasticsearch"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Elasticsearch")
		os.Exit(1)
	}
	if err = (&controllers.KibanaReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Kibana"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Kibana")
		os.Exit(1)
	}
	if err = (&controllers.SecretReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Secret"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Secret")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	log.Info("Registering custom metrics for Elasticsearch Operator.")
	metrics.RegisterCustomMetrics()

	log.Info("Registring profiling endpoints.")
	err = registerProfiler(mgr)
	if err != nil {
		ll.Error(err, "failed to register extra pprof handler")
		os.Exit(1)
	}

	ll.Info("Starting the manager.")

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		ll.Error(err, "Manager exited non-zero")
		os.Exit(1)
	}
}

func registerProfiler(m ctrl.Manager) error {
	endpoints := map[string]http.HandlerFunc{
		"/debug/pprof/":        pprof.Index,
		"/debug/pprof/cmdline": pprof.Cmdline,
		"/debug/pprof/profile": pprof.Profile,
		"/debug/pprof/symbol":  pprof.Symbol,
		"/debug/pprof/trace":   pprof.Trace,
	}

	for path, handler := range endpoints {
		err := m.AddMetricsExtraHandler(path, handler)
		if err != nil {
			return err
		}
	}

	return nil
}

// getWatchNamespace get the namespace name of the scoped operator
// - https://sdk.operatorframework.io/docs/building-operators/golang/operator-scope/#configuring-namespace-scoped-operators
func getWatchNamespace() (string, error) {
	watchNamespaceEnvVar := "WATCH_NAMESPACE"
	ns, found := os.LookupEnv(watchNamespaceEnvVar)
	if !found {
		return "", fmt.Errorf("%s must be set", watchNamespaceEnvVar)
	}
	return ns, nil
}
