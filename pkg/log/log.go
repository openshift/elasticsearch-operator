package log

import (
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	once sync.Once

	// empty logger to prevent nil pointer panics before Init is called
	logger = zapr.NewLogger(zap.NewNop())

	cfg = zap.Config{
		Level:       zap.NewAtomicLevelAt(zap.InfoLevel),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         "json",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	// options
	// disableStacktrace disables all stacktraces by setting the allowed level to 100, which is a number higher than the highest level of logging
	disableStacktrace = zap.AddStacktrace(zap.NewAtomicLevelAt(zapcore.DPanicLevel))
)

// Init initializes the logger. This is required to use logging correctly
// component is the name of the component being used to log messages. Typically this is your application name
// keyValuePairs are default key/value pairs to be used with all logs in the future
func Init(component string, keyValuePairs ...interface{}) {
	once.Do(func() {
		zap.AddCallerSkip(5)
		zl, err := cfg.Build(disableStacktrace, zap.AddCallerSkip(2))
		if err != nil {
			// panic because this is a hard coding problem within the config itself that cannot be handled
			// except by fixing the config struct itself.
			panic(err)
		}

		logger = zapr.NewLogger(zl).
			WithName(component)
		if len(keyValuePairs) > 0 {
			logger = logger.WithValues(keyValuePairs)
		}
	})
}

//proxyLogger is a minimal adapter to Logger to
//facilitate backports to 4.6 due to the difference in
//log libraries.  It does not provide equivalent log
// capabilities
type proxyLogger struct {
	level int
}

func V(verbosity int) proxyLogger {
	return proxyLogger{level: verbosity}
}
func (p proxyLogger) Info(msg string, keysAndValues ...interface{}) {
	if p.level == 0 {
		Info(msg, keysAndValues)
	}
}

// Logger returns the singleton logger that was created via Init
func Logger() logr.Logger {
	return logger
}
func Wrap(err error, msg string) error {
	return fmt.Errorf("%s: %v", msg, err)
}

// Info logs a non-error message with the given key/value pairs as context.
//
// The msg argument should be used to add some constant description to
// the log line.  The key/value pairs can then be used to add additional
// variable information.  The key/value pairs should alternate string
// keys and arbitrary values.
//
// This is a package level function that is a shortcut for log.Logger().Info(...)
func Info(msg string, keysAndValues ...interface{}) {
	logger.Info(msg, keysAndValues...)
}

// Error logs an error, with the given message and key/value pairs as context.
// It functions similarly to calling Info with the "error" named value, but may
// have unique behavior, and should be preferred for logging errors (see the
// package documentations for more information).
//
// The msg field should be used to add context to any underlying error,
// while the err field should be used to attach the actual error that
// triggered this log line, if present.
//
// This is a package level function that is a shortcut for log.Logger().Error(...)
func Error(err error, msg string, keysAndValues ...interface{}) {
	logger.Error(err, msg, keysAndValues...)
}

// WithValues adds some key-value pairs of context to a logger.
// See Info for documentation on how key/value pairs work.
//
// This is a package level function that is a shortcut for log.Logger().WithValues(...)
func WithValues(keysAndValues ...interface{}) logr.Logger {
	return logger.WithValues(keysAndValues...)
}

// WithName adds a new element to the logger's name.
// Successive calls with WithName continue to append
// suffixes to the logger's name.  It's strongly recommended
// that name segments contain only letters, digits, and hyphens
// (see the package documentation for more information).
//
// This is a package level function that is a shortcut for log.Logger().WithName(...)
func WithName(name string) logr.Logger {
	return logger.WithName(name)
}
