package logger

import (
	"os"

	"github.com/charmbracelet/log"
)

// StdOutLogger implements the Logger interface using charmbracelet/log
// It provides formatted console output with different log levels. Every line
// leads with its event, event=<name>, taken from an "event" argument, and
// event=log when a message has none
type StdOutLogger struct {
	logger *log.Logger
}

// NewStdOutLogger creates a new StdOutLogger with default settings
func NewStdOutLogger() *StdOutLogger {
	logger := log.New(os.Stdout)
	logger.SetLevel(log.DebugLevel)
	return &StdOutLogger{logger: logger}
}

// NewStdOutLoggerWithOptions creates a new StdOutLogger with custom options
func NewStdOutLoggerWithOptions(level log.Level, styles *log.Styles) *StdOutLogger {
	logger := log.New(os.Stdout)
	logger.SetLevel(level)
	if styles != nil {
		logger.SetStyles(styles)
	}
	return &StdOutLogger{logger: logger}
}

// Ensure StdOutLogger implements the Logger interface
var _ Logger = (*StdOutLogger)(nil)

// Info logs an informational message
func (l *StdOutLogger) Info(msg string, args ...interface{}) {
	msg, args = leadWithEvent(msg, args)
	if len(args) == 0 {
		l.logger.Info(msg)
		return
	}
	l.logger.With(args...).Info(msg)
}

// Debug logs a debug message
func (l *StdOutLogger) Debug(msg string, args ...interface{}) {
	msg, args = leadWithEvent(msg, args)
	if len(args) == 0 {
		l.logger.Debug(msg)
		return
	}
	l.logger.With(args...).Debug(msg)
}

// Warn logs a warning message
func (l *StdOutLogger) Warn(msg string, args ...interface{}) {
	msg, args = leadWithEvent(msg, args)
	if len(args) == 0 {
		l.logger.Warn(msg)
		return
	}
	l.logger.With(args...).Warn(msg)
}

// Error logs an error message
func (l *StdOutLogger) Error(msg string, args ...interface{}) {
	msg, args = leadWithEvent(msg, args)
	if len(args) == 0 {
		l.logger.Error(msg)
		return
	}
	l.logger.With(args...).Error(msg)
}

// leadWithEvent returns msg with its event in front and the remaining
// arguments
func leadWithEvent(msg string, args []interface{}) (string, []interface{}) {
	event, msg, args := splitEvent(msg, args)
	return joinMessage(event, msg), args
}
