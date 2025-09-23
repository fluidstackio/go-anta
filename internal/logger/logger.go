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

// Get returns the logger instance
func Get() *logrus.Logger {
	return log
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

// SetFormatter sets the log formatter
func SetFormatter(format string) {
	switch strings.ToLower(format) {
	case "json":
		log.SetFormatter(&logrus.JSONFormatter{})
	case "text":
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "15:04:05",
			PadLevelText:    true,
		})
	default:
		// Keep current formatter
	}
}

// Helper functions for common logging
func Debug(msg string) {
	log.Debug(msg)
}

func Debugf(format string, args ...interface{}) {
	log.Debugf(format, args...)
}

func Info(msg string) {
	log.Info(msg)
}

func Infof(format string, args ...interface{}) {
	log.Infof(format, args...)
}

func Warn(msg string) {
	log.Warn(msg)
}

func Warnf(format string, args ...interface{}) {
	log.Warnf(format, args...)
}

func Error(msg string) {
	log.Error(msg)
}

func Errorf(format string, args ...interface{}) {
	log.Errorf(format, args...)
}

func Fatal(msg string) {
	log.Fatal(msg)
}

func Fatalf(format string, args ...interface{}) {
	log.Fatalf(format, args...)
}

// WithField creates an entry with a single field
func WithField(key string, value interface{}) *logrus.Entry {
	return log.WithField(key, value)
}

// WithFields creates an entry with multiple fields
func WithFields(fields logrus.Fields) *logrus.Entry {
	return log.WithFields(fields)
}