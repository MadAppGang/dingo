// Package plugin provides logging utilities
package plugin

import (
	"fmt"
	"io"
	"os"
	"time"
)

// LogLevel represents the logging level
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// DefaultLogger provides a simple logger implementation
type DefaultLogger struct {
	level  LogLevel
	output io.Writer
	prefix string
}

// NewDefaultLogger creates a new default logger
func NewDefaultLogger(level LogLevel) *DefaultLogger {
	return &DefaultLogger{
		level:  level,
		output: os.Stderr,
		prefix: "[dingo] ",
	}
}

// SetOutput sets the output writer
func (l *DefaultLogger) SetOutput(w io.Writer) {
	l.output = w
}

// SetPrefix sets the log prefix
func (l *DefaultLogger) SetPrefix(prefix string) {
	l.prefix = prefix
}

// Debug logs a debug message
func (l *DefaultLogger) Debug(format string, args ...interface{}) {
	if l.level <= LogLevelDebug {
		l.log("DEBUG", format, args...)
	}
}

// Info logs an info message
func (l *DefaultLogger) Info(format string, args ...interface{}) {
	if l.level <= LogLevelInfo {
		l.log("INFO", format, args...)
	}
}

// Warn logs a warning message
func (l *DefaultLogger) Warn(format string, args ...interface{}) {
	if l.level <= LogLevelWarn {
		l.log("WARN", format, args...)
	}
}

// Error logs an error message
func (l *DefaultLogger) Error(format string, args ...interface{}) {
	if l.level <= LogLevelError {
		l.log("ERROR", format, args...)
	}
}

func (l *DefaultLogger) log(level string, format string, args ...interface{}) {
	timestamp := time.Now().Format("15:04:05")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.output, "%s%s [%s] %s\n", l.prefix, timestamp, level, msg)
}

// NoOpLogger is a logger that does nothing (for production)
type NoOpLogger struct{}

// NewNoOpLogger creates a new no-op logger
func NewNoOpLogger() *NoOpLogger {
	return &NoOpLogger{}
}

// Debug does nothing
func (l *NoOpLogger) Debug(format string, args ...interface{}) {}

// Info does nothing
func (l *NoOpLogger) Info(format string, args ...interface{}) {}

// Warn does nothing
func (l *NoOpLogger) Warn(format string, args ...interface{}) {}

// Error does nothing
func (l *NoOpLogger) Error(format string, args ...interface{}) {}
