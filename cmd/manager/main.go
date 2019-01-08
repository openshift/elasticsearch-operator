package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/openshift/elasticsearch-operator/pkg/apis"
	"github.com/openshift/elasticsearch-operator/pkg/controller"
	"github.com/openshift/elasticsearch-operator/pkg/utils"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	"github.com/sirupsen/logrus"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

const (
	// supported log formats
	logFormatLogfmt = "logfmt"
	logFormatJSON   = "json"

	// env vars
	logLevelEnv  = "LOG_LEVEL"
	logFormatEnv = "LOG_FORMAT"
)

var (
	logLevel            string
	logFormat           string
	availableLogLevels  string
	availableLogFormats = []string{
		logFormatLogfmt,
		logFormatJSON,
	}
	// this is a constant, but can't be in the `const` section because
	// the value is a runtime function return value
	defaultLogLevel = logrus.InfoLevel.String()
)

func printVersion() {
	logrus.Printf("Go Version: %s", runtime.Version())
	logrus.Printf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
	logrus.Printf("operator-sdk Version: %v", sdkVersion.Version)
}

func init() {
	// create availableLogLevels
	buf := &bytes.Buffer{}
	comma := ""
	for _, logrusLevel := range logrus.AllLevels {
		buf.WriteString(comma)
		buf.WriteString(logrusLevel.String())
		comma = ", "
	}
	availableLogLevels = buf.String()
	// default values are ""
	// that means that if no arguments are provided env variables take precedence
	// otherwise command-line arguments take precendence
	flagset := flag.CommandLine
	flagset.StringVar(&logLevel, "log-level", "", fmt.Sprintf("Log level to use. Possible values: %s", availableLogLevels))
	flagset.StringVar(&logFormat, "log-format", "", fmt.Sprintf("Log format to use. Possible values: %s", strings.Join(availableLogFormats, ", ")))
	flagset.Parse(os.Args[1:])
}

func initLogger() error {
	// first check cmd arguments, then environment variables
	if logLevel == "" {
		logLevel = utils.LookupEnvWithDefault(logLevelEnv, defaultLogLevel)
	}
	if logFormat == "" {
		logFormat = utils.LookupEnvWithDefault(logFormatEnv, logFormatLogfmt)
	}

	// set log level, default to info level
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return fmt.Errorf("log level '%s' unknown.  Possible values: %v", logLevel, availableLogLevels)
	}
	logrus.SetLevel(level)

	// set log format, default to text formatter
	switch logFormat {
	case logFormatLogfmt:
		logrus.SetFormatter(&logrus.TextFormatter{})
		break
	case logFormatJSON:
		logrus.SetFormatter(&logrus.JSONFormatter{})
		break
	default:
		return fmt.Errorf("log format '%s' unknown, %v are possible values", logFormat, availableLogFormats)
	}
	// log to stdout; logrus defaults to stderr
	logrus.SetOutput(os.Stdout)

	return nil
}

func main() {
	if err := initLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "instantiating elasticsearch controller failed: %v\n", err)
		return
	}
	printVersion()
	flag.Parse()

	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		logrus.Fatalf("failed to get watch namespace: %v", err)
	}

	// TODO: Expose metrics port after SDK uses controller-runtime's dynamic client
	// sdk.ExposeMetricsPort()

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		logrus.Fatal(err)
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{Namespace: namespace})
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Print("Registering Components.")

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		logrus.Fatal(err)
	}

	// Setup all Controllers
	if err := controller.AddToManager(mgr); err != nil {
		logrus.Fatal(err)
	}

	logrus.Print("Starting the Cmd.")

	// Start the Cmd
	logrus.Fatal(mgr.Start(signals.SetupSignalHandler()))
}
