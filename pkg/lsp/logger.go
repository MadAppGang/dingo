package lsp

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

// LogLevel represents logging verbosity
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// Logger provides configurable logging for the LSP server
type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}

// StandardLogger implements Logger interface with standard library
type StandardLogger struct {
	level  LogLevel
	logger *log.Logger
}

// NewLogger creates a new logger with the specified level
// levelStr can be: "debug", "info", "warn", "error"
func NewLogger(levelStr string, output io.Writer) Logger {
	if output == nil {
		output = os.Stderr
	}

	level := parseLogLevel(levelStr)
	return &StandardLogger{
		level:  level,
		logger: log.New(output, "[dingo-lsp] ", log.Ldate|log.Ltime|log.Lshortfile),
	}
}

func parseLogLevel(levelStr string) LogLevel {
	switch strings.ToLower(levelStr) {
	case "debug":
		return LogLevelDebug
	case "info":
		return LogLevelInfo
	case "warn", "warning":
		return LogLevelWarn
	case "error":
		return LogLevelError
	default:
		return LogLevelInfo
	}
}

func (l *StandardLogger) Debugf(format string, args ...interface{}) {
	if l.level <= LogLevelDebug {
		l.logger.Output(2, fmt.Sprintf("[DEBUG] "+format, args...))
	}
}

func (l *StandardLogger) Infof(format string, args ...interface{}) {
	if l.level <= LogLevelInfo {
		l.logger.Output(2, fmt.Sprintf("[INFO] "+format, args...))
	}
}

func (l *StandardLogger) Warnf(format string, args ...interface{}) {
	if l.level <= LogLevelWarn {
		l.logger.Output(2, fmt.Sprintf("[WARN] "+format, args...))
	}
}

func (l *StandardLogger) Errorf(format string, args ...interface{}) {
	if l.level <= LogLevelError {
		l.logger.Output(2, fmt.Sprintf("[ERROR] "+format, args...))
	}
}

func (l *StandardLogger) Fatalf(format string, args ...interface{}) {
	l.logger.Output(2, fmt.Sprintf("[FATAL] "+format, args...))
	os.Exit(1)
}
