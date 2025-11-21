package preprocessor

import (
	"strings"
	"testing"
)

func TestSafeNavProcessorV2_Metadata(t *testing.T) {
	p := NewSafeNavProcessor()

	code := `let name: *string
let result = user?.name`

	result, metadata, _, err := p.ProcessV2(code, ModePostAST)
	if err != nil {
		t.Fatalf("ProcessV2 failed: %v", err)
	}

	// Should generate metadata
	if len(metadata) != 1 {
		t.Fatalf("Expected 1 metadata entry, got %d", len(metadata))
	}

	meta := metadata[0]
	if meta.Type != "safe_nav" {
		t.Errorf("Expected type 'safe_nav', got '%s'", meta.Type)
	}

	if meta.OriginalText != "?." {
		t.Errorf("Expected original text '?.', got '%s'", meta.OriginalText)
	}

	if meta.GeneratedMarker != "// dingo:s:0" {
		t.Errorf("Expected marker '// dingo:s:0', got '%s'", meta.GeneratedMarker)
	}

	if meta.ASTNodeType != "CallExpr" {
		t.Errorf("Expected AST node type 'CallExpr', got '%s'", meta.ASTNodeType)
	}

	// Should include marker in generated code
	if !strings.Contains(result, "// dingo:s:0") {
		t.Errorf("Expected marker '// dingo:s:0' in generated code, got:\n%s", result)
	}
}

func TestSafeNavProcessorV2_UniqueMarkers(t *testing.T) {
	p := NewSafeNavProcessor()

	code := `let user: *User
let name = user?.name
let city = user?.address?.city
let zip = user?.address?.zip`

	result, metadata, _, err := p.ProcessV2(code, ModePostAST)
	if err != nil {
		t.Fatalf("ProcessV2 failed: %v", err)
	}

	// Should generate 3 metadata entries (one per line with ?.)
	if len(metadata) != 3 {
		t.Fatalf("Expected 3 metadata entries, got %d", len(metadata))
	}

	// Check markers are unique
	markers := make(map[string]bool)
	for i, meta := range metadata {
		expectedMarker := "// dingo:s:" + string(rune('0'+i))
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

func TestSafeNavProcessorV2_DualMode(t *testing.T) {
	p := NewSafeNavProcessor()

	code := `let user: *User
let name = user?.name`

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
	if !strings.Contains(result, "// dingo:s:0") {
		t.Errorf("Expected marker in output")
	}
}

func TestSafeNavProcessorV2_LegacyMode(t *testing.T) {
	p := NewSafeNavProcessor()

	code := `let user: *User
let name = user?.name`

	result, metadata, sourceMap, err := p.ProcessV2(code, ModeLegacy)
	if err != nil {
		t.Fatalf("ProcessV2 failed: %v", err)
	}

	// Should NOT generate metadata
	if len(metadata) != 0 {
		t.Errorf("Expected 0 metadata entries in Legacy mode, got %d", len(metadata))
	}

	// Should NOT include markers in output
	if strings.Contains(result, "// dingo:s:") {
		t.Errorf("Should not include markers in Legacy mode, got:\n%s", result)
	}

	// Should still generate mappings
	if sourceMap == nil {
		t.Fatal("Expected source map in Legacy mode")
	}
}
