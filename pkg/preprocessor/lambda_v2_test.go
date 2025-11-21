package preprocessor

import (
	"strings"
	"testing"
)

func TestLambdaProcessorV2_Metadata(t *testing.T) {
	p := NewLambdaProcessor()

	code := `let f = x => x * 2`

	result, metadata, _, err := p.ProcessV2(code, ModePostAST)
	if err != nil {
		t.Fatalf("ProcessV2 failed: %v", err)
	}

	// Should generate metadata
	if len(metadata) != 1 {
		t.Fatalf("Expected 1 metadata entry, got %d", len(metadata))
	}

	meta := metadata[0]
	if meta.Type != "lambda" {
		t.Errorf("Expected type 'lambda', got '%s'", meta.Type)
	}

	if meta.GeneratedMarker != "// dingo:l:0" {
		t.Errorf("Expected marker '// dingo:l:0', got '%s'", meta.GeneratedMarker)
	}

	if meta.ASTNodeType != "FuncLit" {
		t.Errorf("Expected AST node type 'FuncLit', got '%s'", meta.ASTNodeType)
	}

	// Code should be transformed to func literal
	if !strings.Contains(result, "func(x") {
		t.Errorf("Expected lambda to be transformed to func literal, got:\n%s", result)
	}
}

func TestLambdaProcessorV2_UniqueMarkers(t *testing.T) {
	p := NewLambdaProcessor()

	code := `let f1 = x => x * 2
let f2 = y => y + 1
let f3 = z => z - 1`

	result, metadata, _, err := p.ProcessV2(code, ModePostAST)
	if err != nil {
		t.Fatalf("ProcessV2 failed: %v", err)
	}

	// Should generate 3 metadata entries with unique markers
	if len(metadata) != 3 {
		t.Fatalf("Expected 3 metadata entries, got %d", len(metadata))
	}

	// Check markers are unique
	markers := make(map[string]bool)
	for i, meta := range metadata {
		expectedMarker := "// dingo:l:" + string(rune('0'+i))
		if meta.GeneratedMarker != expectedMarker {
			t.Errorf("Metadata %d: Expected marker '%s', got '%s'", i, expectedMarker, meta.GeneratedMarker)
		}
		if markers[meta.GeneratedMarker] {
			t.Errorf("Duplicate marker: %s", meta.GeneratedMarker)
		}
		markers[meta.GeneratedMarker] = true
	}

	// Verify all transformations occurred
	if !strings.Contains(result, "func(x") {
		t.Errorf("Expected lambda transformations in result")
	}
}

func TestLambdaProcessorV2_DualMode(t *testing.T) {
	p := NewLambdaProcessor()

	code := `let f = x => x * 2`

	result, metadata, sourceMap, err := p.ProcessV2(code, ModeDual)
	if err != nil {
		t.Fatalf("ProcessV2 failed: %v", err)
	}

	// Should generate BOTH metadata AND mappings
	if len(metadata) != 1 {
		t.Errorf("Expected 1 metadata entry, got %d", len(metadata))
	}

	if sourceMap == nil {
		t.Fatal("Expected source map, got nil")
	}

	if len(sourceMap.Mappings) == 0 {
		t.Error("Expected mappings in source map")
	}

	// Should include lambda transformation
	if !strings.Contains(result, "func(x") {
		t.Errorf("Expected lambda transformation in output")
	}
}

func TestLambdaProcessorV2_LegacyMode(t *testing.T) {
	p := NewLambdaProcessor()

	code := `let f = x => x * 2`

	result, metadata, sourceMap, err := p.ProcessV2(code, ModeLegacy)
	if err != nil {
		t.Fatalf("ProcessV2 failed: %v", err)
	}

	// Should NOT generate metadata
	if len(metadata) != 0 {
		t.Errorf("Expected 0 metadata entries in Legacy mode, got %d", len(metadata))
	}

	// Should still transform lambda
	if !strings.Contains(result, "func(x") {
		t.Errorf("Expected lambda to be transformed even in Legacy mode")
	}

	// Should still generate mappings
	if sourceMap == nil {
		t.Fatal("Expected source map in Legacy mode")
	}
}

func TestLambdaProcessorV2_RustStyle(t *testing.T) {
	p := NewLambdaProcessorWithConfig(nil)
	p.style = StyleRust

	code := `let f = |x| x * 2`

	result, metadata, _, err := p.ProcessV2(code, ModePostAST)
	if err != nil {
		t.Fatalf("ProcessV2 failed: %v", err)
	}

	// Should generate metadata
	if len(metadata) != 1 {
		t.Fatalf("Expected 1 metadata entry, got %d", len(metadata))
	}

	// Should transform to func literal
	if !strings.Contains(result, "func(x") {
		t.Errorf("Expected Rust-style lambda to be transformed, got:\n%s", result)
	}
}
