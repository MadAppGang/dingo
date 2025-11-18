package lsp

import (
	"testing"

	"go.lsp.dev/protocol"
)

func TestParseTranspileError_ValidError(t *testing.T) {
	dingoPath := "/path/to/example.dingo"
	output := "/path/to/example.dingo:10:15: undefined: foo"

	diagnostic := ParseTranspileError(dingoPath, output)
	if diagnostic == nil {
		t.Fatal("Expected diagnostic, got nil")
	}

	// Check position (0-based)
	if diagnostic.Range.Start.Line != 9 {
		t.Errorf("Expected line 9, got %d", diagnostic.Range.Start.Line)
	}
	if diagnostic.Range.Start.Character != 14 {
		t.Errorf("Expected character 14, got %d", diagnostic.Range.Start.Character)
	}

	// Check severity and source
	if diagnostic.Severity != protocol.DiagnosticSeverityError {
		t.Errorf("Expected error severity, got %v", diagnostic.Severity)
	}
	if diagnostic.Source != "dingo" {
		t.Errorf("Expected source 'dingo', got '%s'", diagnostic.Source)
	}

	// Check message
	if diagnostic.Message != "undefined: foo" {
		t.Errorf("Expected message 'undefined: foo', got '%s'", diagnostic.Message)
	}
}

func TestParseTranspileError_GenericError(t *testing.T) {
	dingoPath := "/path/to/example.dingo"
	output := "error: failed to parse file"

	diagnostic := ParseTranspileError(dingoPath, output)
	if diagnostic == nil {
		t.Fatal("Expected diagnostic, got nil")
	}

	// Should create diagnostic at line 0
	if diagnostic.Range.Start.Line != 0 {
		t.Errorf("Expected line 0, got %d", diagnostic.Range.Start.Line)
	}

	// Check message contains error
	if diagnostic.Message != "error: failed to parse file" {
		t.Errorf("Expected full error message, got '%s'", diagnostic.Message)
	}
}

func TestParseTranspileError_NoError(t *testing.T) {
	dingoPath := "/path/to/example.dingo"
	output := "Transpilation successful"

	diagnostic := ParseTranspileError(dingoPath, output)
	if diagnostic != nil {
		t.Errorf("Expected nil for non-error output, got diagnostic: %+v", diagnostic)
	}
}

func TestParseTranspileError_MultilineError(t *testing.T) {
	dingoPath := "/path/to/example.dingo"
	output := `Build started...
/path/to/example.dingo:25:8: type mismatch
/path/to/example.dingo:30:12: syntax error
Build failed`

	diagnostic := ParseTranspileError(dingoPath, output)
	if diagnostic == nil {
		t.Fatal("Expected diagnostic, got nil")
	}

	// Should parse first error
	if diagnostic.Range.Start.Line != 24 { // 25-1 = 24 (0-based)
		t.Errorf("Expected line 24, got %d", diagnostic.Range.Start.Line)
	}

	if diagnostic.Message != "type mismatch" {
		t.Errorf("Expected 'type mismatch', got '%s'", diagnostic.Message)
	}
}

func TestAutoTranspiler_OnFileChange(t *testing.T) {
	// Note: This is an integration test that requires 'dingo' binary
	// For unit test, we'd need to mock exec.Command (not trivial in Go)
	// Skipping for now - would need exec command mocking
	t.Skip("Integration test - requires 'dingo' binary")
}
