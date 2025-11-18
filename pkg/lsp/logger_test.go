package lsp

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogger_Levels(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		logFunc  func(Logger)
		expected bool // Should message appear?
	}{
		{"debug logs at debug level", "debug", func(l Logger) { l.Debugf("test") }, true},
		{"debug hidden at info level", "info", func(l Logger) { l.Debugf("test") }, false},
		{"info logs at info level", "info", func(l Logger) { l.Infof("test") }, true},
		{"info logs at debug level", "debug", func(l Logger) { l.Infof("test") }, true},
		{"warn always logs", "warn", func(l Logger) { l.Warnf("test") }, true},
		{"error always logs", "info", func(l Logger) { l.Errorf("test") }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			logger := NewLogger(tt.level, buf)

			tt.logFunc(logger)

			output := buf.String()
			hasOutput := strings.Contains(output, "test")

			if hasOutput != tt.expected {
				t.Errorf("Expected output=%v, got output=%v (output: %s)", tt.expected, hasOutput, output)
			}
		})
	}
}

func TestLogger_ParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"debug", LogLevelDebug},
		{"DEBUG", LogLevelDebug},
		{"info", LogLevelInfo},
		{"INFO", LogLevelInfo},
		{"warn", LogLevelWarn},
		{"warning", LogLevelWarn},
		{"error", LogLevelError},
		{"unknown", LogLevelInfo}, // Default
		{"", LogLevelInfo},         // Default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level := parseLogLevel(tt.input)
			if level != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, level)
			}
		})
	}
}

func TestLogger_Format(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger("info", buf)

	logger.Infof("formatted %s %d", "message", 42)

	output := buf.String()
	if !strings.Contains(output, "formatted message 42") {
		t.Errorf("Expected formatted output, got: %s", output)
	}
	if !strings.Contains(output, "[INFO]") {
		t.Errorf("Expected [INFO] prefix, got: %s", output)
	}
	if !strings.Contains(output, "[dingo-lsp]") {
		t.Errorf("Expected [dingo-lsp] prefix, got: %s", output)
	}
}
