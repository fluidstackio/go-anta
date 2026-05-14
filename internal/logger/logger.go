package logger

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

func init() {
	log = logrus.New()
	
	// Set default configuration
	log.SetOutput(os.Stdout)
	log.SetLevel(logrus.WarnLevel) // Default to only show warnings and above
	
	// Custom formatter for better readability
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "15:04:05",
		PadLevelText:    true,
	})
}

// SetLevel sets the log level
func SetLevel(level string) {
	switch strings.ToLower(level) {
	case "debug":
		log.SetLevel(logrus.DebugLevel)
	case "info":
		log.SetLevel(logrus.InfoLevel)
	case "warn", "warning":
		log.SetLevel(logrus.WarnLevel)
	case "error":
		log.SetLevel(logrus.ErrorLevel)
	case "fatal":
		log.SetLevel(logrus.FatalLevel)
	case "panic":
		log.SetLevel(logrus.PanicLevel)
	case "trace":
		log.SetLevel(logrus.TraceLevel)
	default:
		log.SetLevel(logrus.InfoLevel)
	}
}

// SetVerbose enables debug logging when verbose is true
func SetVerbose(verbose bool) {
	if verbose {
		log.SetLevel(logrus.DebugLevel)
		log.Debug("Verbose logging enabled")
	}
}

// SetOutput sets the log output file
func SetOutput(path string) error {
	if path == "" {
		log.SetOutput(os.Stdout)
		return nil
	}
	
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	
	log.SetOutput(file)
	return nil
}

// Helper functions for common logging. Only the formatted variants
// (Debugf/Infof/Warnf/Errorf) are exposed because that is the entire
// surface the codebase uses.
func Debugf(format string, args ...interface{}) {
	log.Debugf(format, args...)
}

func Infof(format string, args ...interface{}) {
	log.Infof(format, args...)
}

func Warnf(format string, args ...interface{}) {
	log.Warnf(format, args...)
}

func Errorf(format string, args ...interface{}) {
	log.Errorf(format, args...)
}