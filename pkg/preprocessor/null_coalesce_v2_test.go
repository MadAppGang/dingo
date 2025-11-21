package preprocessor

import (
	"strings"
	"testing"
)

func TestNullCoalesceProcessorV2_Metadata(t *testing.T) {
	p := NewNullCoalesceProcessor()

	code := `let x = value ?? default`

	result, metadata, _, err := p.ProcessV2(code, ModePostAST)
	if err != nil {
		t.Fatalf("ProcessV2 failed: %v", err)
	}

	// Should generate metadata
	if len(metadata) != 1 {
		t.Fatalf("Expected 1 metadata entry, got %d", len(metadata))
	}

	meta := metadata[0]
	if meta.Type != "null_coalesce" {
		t.Errorf("Expected type 'null_coalesce', got '%s'", meta.Type)
	}

	if meta.OriginalText != "??" {
		t.Errorf("Expected original text '??', got '%s'", meta.OriginalText)
	}

	if meta.GeneratedMarker != "// dingo:c:0" {
		t.Errorf("Expected marker '// dingo:c:0', got '%s'", meta.GeneratedMarker)
	}

	if meta.ASTNodeType != "IfExpr" {
		t.Errorf("Expected AST node type 'IfExpr', got '%s'", meta.ASTNodeType)
	}

	// Should include marker in generated code
	if !strings.Contains(result, "// dingo:c:0") {
		t.Errorf("Expected marker '// dingo:c:0' in generated code, got:\n%s", result)
	}
}

func TestNullCoalesceProcessorV2_UniqueMarkers(t *testing.T) {
	p := NewNullCoalesceProcessor()

	code := `let x = a ?? b
let y = c ?? d
let z = e ?? f`

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
		expectedMarker := "// dingo:c:" + string(rune('0'+i))
		if meta.GeneratedMarker != expectedMarker {
			t.Errorf("Metadata %d: Expected marker '%s', got '%s'", i, expectedMarker, meta.GeneratedMarker)
		}
		if markers[meta.GeneratedMarker] {
			t.Errorf("Duplicate marker: %s", meta.GeneratedMarker)
		}
		markers[meta.GeneratedMarker] = true

		// Verify marker is in output
		if !strings.Contains(result, meta.GeneratedMarker) {
			t.Errorf("Marker '%s' not found in output", meta.GeneratedMarker)
		}
	}
}

func TestNullCoalesceProcessorV2_DualMode(t *testing.T) {
	p := NewNullCoalesceProcessor()

	code := `let x = value ?? default`

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

	// Should include marker in output
	if !strings.Contains(result, "// dingo:c:0") {
		t.Errorf("Expected marker in output")
	}
}

func TestNullCoalesceProcessorV2_LegacyMode(t *testing.T) {
	p := NewNullCoalesceProcessor()

	code := `let x = value ?? default`

	result, metadata, sourceMap, err := p.ProcessV2(code, ModeLegacy)
	if err != nil {
		t.Fatalf("ProcessV2 failed: %v", err)
	}

	// Should NOT generate metadata
	if len(metadata) != 0 {
		t.Errorf("Expected 0 metadata entries in Legacy mode, got %d", len(metadata))
	}

	// Should NOT include markers in output
	if strings.Contains(result, "// dingo:c:") {
		t.Errorf("Should not include markers in Legacy mode, got:\n%s", result)
	}

	// Should still generate mappings
	if sourceMap == nil {
		t.Fatal("Expected source map in Legacy mode")
	}
}
