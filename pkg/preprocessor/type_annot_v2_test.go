package preprocessor

import (
	"strings"
	"testing"
)

func TestTypeAnnotProcessorV2_Metadata(t *testing.T) {
	processor := NewTypeAnnotProcessor()
	input := `package main

func add(x: int, y: int) int {
	return x + y
}`

	result, metadata, sourceMap, err := processor.ProcessV2(input, ModePostAST)
	if err != nil {
		t.Fatalf("ProcessV2 failed: %v", err)
	}

	// Should have one metadata entry for the function with type annotations
	if len(metadata) != 1 {
		t.Fatalf("Expected 1 metadata entry, got %d", len(metadata))
	}

	meta := metadata[0]
	if meta.Type != "type_annot" {
		t.Errorf("Expected type 'type_annot', got '%s'", meta.Type)
	}
	if meta.OriginalLine != 3 {
		t.Errorf("Expected original line 3, got %d", meta.OriginalLine)
	}
	if meta.GeneratedMarker != "// dingo:t:0" {
		t.Errorf("Expected marker '// dingo:t:0', got '%s'", meta.GeneratedMarker)
	}
	if meta.ASTNodeType != "FuncDecl" {
		t.Errorf("Expected AST node type 'FuncDecl', got '%s'", meta.ASTNodeType)
	}

	// Result should contain the marker
	if !strings.Contains(result, "// dingo:t:0") {
		t.Errorf("Result should contain marker '// dingo:t:0'")
	}

	// Result should have transformed : to space
	if !strings.Contains(result, "func add(x int, y int)") {
		t.Errorf("Result should have transformed type annotations")
	}

	// sourceMap should be nil in PostAST mode
	if sourceMap != nil {
		t.Errorf("Expected nil sourceMap in PostAST mode, got %v", sourceMap)
	}
}

func TestTypeAnnotProcessorV2_UniqueMarkers(t *testing.T) {
	processor := NewTypeAnnotProcessor()
	input := `package main

func add(x: int, y: int) int {
	return x + y
}

func multiply(a: int, b: int) int {
	return a * b
}`

	result, metadata, _, err := processor.ProcessV2(input, ModePostAST)
	if err != nil {
		t.Fatalf("ProcessV2 failed: %v", err)
	}

	// Should have two metadata entries
	if len(metadata) != 2 {
		t.Fatalf("Expected 2 metadata entries, got %d", len(metadata))
	}

	// Markers should be unique
	if metadata[0].GeneratedMarker != "// dingo:t:0" {
		t.Errorf("Expected first marker '// dingo:t:0', got '%s'", metadata[0].GeneratedMarker)
	}
	if metadata[1].GeneratedMarker != "// dingo:t:1" {
		t.Errorf("Expected second marker '// dingo:t:1', got '%s'", metadata[1].GeneratedMarker)
	}

	// Both markers should be in result
	if !strings.Contains(result, "// dingo:t:0") {
		t.Errorf("Result should contain marker '// dingo:t:0'")
	}
	if !strings.Contains(result, "// dingo:t:1") {
		t.Errorf("Result should contain marker '// dingo:t:1'")
	}
}

func TestTypeAnnotProcessorV2_ReturnArrow(t *testing.T) {
	processor := NewTypeAnnotProcessor()
	input := `package main

func getValue() -> string {
	return "hello"
}`

	result, metadata, _, err := processor.ProcessV2(input, ModePostAST)
	if err != nil {
		t.Fatalf("ProcessV2 failed: %v", err)
	}

	// Should have one metadata entry for the return arrow transformation
	if len(metadata) != 1 {
		t.Fatalf("Expected 1 metadata entry, got %d", len(metadata))
	}

	// Result should have transformed -> to space
	if !strings.Contains(result, "func getValue() string {") {
		t.Errorf("Result should have transformed return arrow, got: %s", result)
	}

	// Should contain marker
	if !strings.Contains(result, "// dingo:t:0") {
		t.Errorf("Result should contain marker '// dingo:t:0'")
	}
}

func TestTypeAnnotProcessorV2_DualMode(t *testing.T) {
	processor := NewTypeAnnotProcessor()
	input := `package main

func add(x: int, y: int) int {
	return x + y
}`

	result, metadata, sourceMap, err := processor.ProcessV2(input, ModeDual)
	if err != nil {
		t.Fatalf("ProcessV2 failed: %v", err)
	}

	// Should have metadata
	if len(metadata) != 1 {
		t.Fatalf("Expected 1 metadata entry, got %d", len(metadata))
	}

	// Should have source map
	if sourceMap == nil {
		t.Fatal("Expected sourceMap in Dual mode, got nil")
	}
	if len(sourceMap.Mappings) == 0 {
		t.Error("Expected non-empty mappings in Dual mode")
	}

	// Should contain marker
	if !strings.Contains(result, "// dingo:t:0") {
		t.Errorf("Result should contain marker in Dual mode")
	}
}

func TestTypeAnnotProcessorV2_LegacyMode(t *testing.T) {
	processor := NewTypeAnnotProcessor()
	input := `package main

func add(x: int, y: int) int {
	return x + y
}`

	result, metadata, sourceMap, err := processor.ProcessV2(input, ModeLegacy)
	if err != nil {
		t.Fatalf("ProcessV2 failed: %v", err)
	}

	// Should NOT have metadata in Legacy mode
	if len(metadata) != 0 {
		t.Errorf("Expected 0 metadata entries in Legacy mode, got %d", len(metadata))
	}

	// Should have source map
	if sourceMap == nil {
		t.Error("Expected sourceMap in Legacy mode, got nil")
	}

	// Should NOT contain markers in Legacy mode
	if strings.Contains(result, "// dingo:t:") {
		t.Errorf("Result should not contain markers in Legacy mode")
	}

	// Should still transform type annotations
	if !strings.Contains(result, "func add(x int, y int)") {
		t.Errorf("Result should have transformed type annotations even in Legacy mode")
	}
}

func TestTypeAnnotProcessorV2_NoTransformation(t *testing.T) {
	processor := NewTypeAnnotProcessor()
	input := `package main

func add(x int, y int) int {
	return x + y
}`

	result, metadata, _, err := processor.ProcessV2(input, ModePostAST)
	if err != nil {
		t.Fatalf("ProcessV2 failed: %v", err)
	}

	// Should have NO metadata since no transformation happened
	if len(metadata) != 0 {
		t.Errorf("Expected 0 metadata entries (no transformation), got %d", len(metadata))
	}

	// Should not contain any markers
	if strings.Contains(result, "// dingo:t:") {
		t.Errorf("Result should not contain markers when no transformation occurred")
	}

	// Result should be unchanged
	if !strings.Contains(result, "func add(x int, y int)") {
		t.Errorf("Result should remain unchanged")
	}
}
