package preprocessor

import (
	"strings"
	"testing"
)

func TestTypeAnnotProcessor_Metadata(t *testing.T) {
	processor := NewTypeAnnotProcessor()
	input := `package main

func add(x: int, y: int) int {
	return x + y
}`

	result, metadata, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
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

	// Marker should be in metadata, not in output (clean code generation)
	// The marker is used for AST matching via metadata, not embedded in code

	// Result should have transformed : to space
	if !strings.Contains(result, "func add(x int, y int)") {
		t.Errorf("Result should have transformed type annotations")
	}

}

func TestTypeAnnotProcessor_UniqueMarkers(t *testing.T) {
	processor := NewTypeAnnotProcessor()
	input := `package main

func add(x: int, y: int) int {
	return x + y
}

func multiply(a: int, b: int) int {
	return a * b
}`

	_, metadata, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
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

	// Markers should be in metadata, not in output (clean code generation)
}

func TestTypeAnnotProcessor_ReturnArrow(t *testing.T) {
	processor := NewTypeAnnotProcessor()
	input := `package main

func getValue() -> string {
	return "hello"
}`

	result, metadata, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should have one metadata entry for the return arrow transformation
	if len(metadata) != 1 {
		t.Fatalf("Expected 1 metadata entry, got %d", len(metadata))
	}

	// Result should have transformed -> to space
	if !strings.Contains(result, "func getValue() string {") {
		t.Errorf("Result should have transformed return arrow, got: %s", result)
	}

	// Marker should be in metadata, not in output
	if metadata[0].GeneratedMarker != "// dingo:t:0" {
		t.Errorf("Expected marker '// dingo:t:0' in metadata, got '%s'", metadata[0].GeneratedMarker)
	}
}



func TestTypeAnnotProcessor_NoTransformation(t *testing.T) {
	processor := NewTypeAnnotProcessor()
	input := `package main

func add(x int, y int) int {
	return x + y
}`

	result, metadata, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
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
