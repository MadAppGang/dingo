package preprocessor

import (
	"strings"
	"testing"
)

func TestEnumProcessorV2_Metadata(t *testing.T) {
	p := NewEnumProcessor()

	code := `enum Color {
	Red,
	Green,
	Blue,
}`

	result, metadata, _, err := p.ProcessV2(code, ModePostAST)
	if err != nil {
		t.Fatalf("ProcessV2 failed: %v", err)
	}

	// Should generate metadata
	if len(metadata) != 1 {
		t.Fatalf("Expected 1 metadata entry, got %d", len(metadata))
	}

	meta := metadata[0]
	if meta.Type != "enum" {
		t.Errorf("Expected type 'enum', got '%s'", meta.Type)
	}

	if meta.GeneratedMarker != "// dingo:n:0" {
		t.Errorf("Expected marker '// dingo:n:0', got '%s'", meta.GeneratedMarker)
	}

	if meta.ASTNodeType != "TypeSpec" {
		t.Errorf("Expected AST node type 'TypeSpec', got '%s'", meta.ASTNodeType)
	}

	// Should include marker in generated code
	if !strings.Contains(result, "// dingo:n:0") {
		t.Errorf("Expected marker '// dingo:n:0' in generated code, got:\n%s", result)
	}

	// Should generate sum type
	if !strings.Contains(result, "type ColorTag uint8") {
		t.Errorf("Expected sum type to be generated")
	}
}

func TestEnumProcessorV2_UniqueMarkers(t *testing.T) {
	p := NewEnumProcessor()

	code := `enum Color {
	Red,
	Green,
}

enum Shape {
	Circle,
	Square,
}

enum Size {
	Small,
	Large,
}`

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
		expectedMarker := "// dingo:n:" + string(rune('0'+i))
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

func TestEnumProcessorV2_DualMode(t *testing.T) {
	p := NewEnumProcessor()

	code := `enum Color {
	Red,
	Green,
}`

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

	// Should include marker in output
	if !strings.Contains(result, "// dingo:n:0") {
		t.Errorf("Expected marker in output")
	}

	// Should generate sum type
	if !strings.Contains(result, "type ColorTag uint8") {
		t.Errorf("Expected sum type to be generated")
	}
}

func TestEnumProcessorV2_LegacyMode(t *testing.T) {
	p := NewEnumProcessor()

	code := `enum Color {
	Red,
	Green,
}`

	result, metadata, sourceMap, err := p.ProcessV2(code, ModeLegacy)
	if err != nil {
		t.Fatalf("ProcessV2 failed: %v", err)
	}

	// Should NOT generate metadata
	if len(metadata) != 0 {
		t.Errorf("Expected 0 metadata entries in Legacy mode, got %d", len(metadata))
	}

	// Should NOT include markers in output
	if strings.Contains(result, "// dingo:n:") {
		t.Errorf("Should not include markers in Legacy mode, got:\n%s", result)
	}

	// Should still generate sum type
	if !strings.Contains(result, "type ColorTag uint8") {
		t.Errorf("Expected sum type to be generated even in Legacy mode")
	}

	// Should still generate mappings
	if sourceMap == nil {
		t.Fatal("Expected source map in Legacy mode")
	}
}
