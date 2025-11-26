// Package sourcemap provides source map generation for Dingo → Go transpilation
package sourcemap

import (
	"encoding/json"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/generator"
	"github.com/MadAppGang/dingo/pkg/parser"
	"github.com/MadAppGang/dingo/pkg/plugin"
	"github.com/MadAppGang/dingo/pkg/plugin/builtin"
	"github.com/MadAppGang/dingo/pkg/preprocessor"
)

// TestPostASTIntegration tests the full pipeline integration:
// Dingo source → Preprocessor (with metadata) → AST Processing → PostASTGenerator → Source Map
func TestPostASTIntegration(t *testing.T) {
	dingoCode := `package main

func test() {
	let x = getValue()?
}
`

	// Create temp directory for test files
	tmpDir := t.TempDir()
	dingoPath := filepath.Join(tmpDir, "test.dingo")
	goPath := filepath.Join(tmpDir, "test.go")
	mapPath := goPath + ".map"

	// Write .dingo file
	if err := os.WriteFile(dingoPath, []byte(dingoCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Step 1: Preprocess with metadata
	prep := preprocessor.NewWithMainConfig([]byte(dingoCode), config.DefaultConfig())
	goSource, _, metadata, err := prep.ProcessWithMetadata()
	if err != nil {
		t.Fatalf("Preprocessing failed: %v", err)
	}

	if len(metadata) == 0 {
		t.Log("Warning: No metadata emitted by preprocessor (expected if no transformations)")
	} else {
		t.Logf("Preprocessor emitted %d metadata entries", len(metadata))
	}

	// Step 2: Parse preprocessed Go
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, dingoPath, []byte(goSource), parser.ParseComments)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Step 3: Generate with plugins
	registry, err := builtin.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	logger := plugin.NewNoOpLogger()
	gen, err := generator.NewWithPlugins(fset, registry, logger)
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	outputCode, err := gen.Generate(file)
	if err != nil {
		t.Fatalf("Generation failed: %v", err)
	}

	// Step 4: Write .go file
	if err := os.WriteFile(goPath, outputCode, 0644); err != nil {
		t.Fatalf("Failed to write Go file: %v", err)
	}

	// Step 5: Generate source map using PostASTGenerator
	postASTGen := NewPostASTGenerator(dingoPath, goPath, fset, file.File, metadata)
	sourceMap, err := postASTGen.Generate()
	if err != nil {
		t.Fatalf("PostASTGenerator failed: %v", err)
	}

	// Verify source map is not empty
	if len(sourceMap.Mappings) == 0 {
		t.Error("Source map has no mappings")
	}

	t.Logf("Generated %d mappings", len(sourceMap.Mappings))

	// Step 6: Write source map
	sourceMapJSON, err := json.MarshalIndent(sourceMap, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal source map: %v", err)
	}

	if err := os.WriteFile(mapPath, sourceMapJSON, 0644); err != nil {
		t.Fatalf("Failed to write source map: %v", err)
	}

	// Step 7: Verify source map can be loaded back
	loadedData, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatalf("Failed to read source map: %v", err)
	}

	loadedMap, err := preprocessor.FromJSON(loadedData)
	if err != nil {
		t.Fatalf("Failed to parse source map: %v", err)
	}

	if len(loadedMap.Mappings) != len(sourceMap.Mappings) {
		t.Errorf("Loaded map has different mapping count: got %d, want %d",
			len(loadedMap.Mappings), len(sourceMap.Mappings))
	}

	t.Logf("✅ Integration test passed")
	t.Logf("   - Dingo file: %s", dingoPath)
	t.Logf("   - Go file: %s", goPath)
	t.Logf("   - Source map: %s", mapPath)
	t.Logf("   - Mappings: %d", len(sourceMap.Mappings))
}

// TestPostASTIntegrationWithMultipleFeatures tests PostASTGenerator with multiple Dingo features
func TestPostASTIntegrationWithMultipleFeatures(t *testing.T) {
	// Skip: Test uses old syntax (Result[T, E] with brackets, match with =>)
	// Current Dingo uses Result<T, E> with angle brackets and different match syntax
	t.Skip("Test uses outdated Dingo syntax - needs update to current syntax")

	dingoCode := `package main

func process(input: string) Result[int, error] {
	let value = parseInt(input)?
	let doubled = value * 2
	return Ok(doubled)
}

func main() {
	result := process("42")
	match result {
		Ok(v) => println(v)
		Err(e) => println("Error:", e)
	}
}
`

	// Create temp directory
	tmpDir := t.TempDir()
	dingoPath := filepath.Join(tmpDir, "multi.dingo")
	goPath := filepath.Join(tmpDir, "multi.go")

	// Write .dingo file
	if err := os.WriteFile(dingoPath, []byte(dingoCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Preprocess with metadata
	prep := preprocessor.NewWithMainConfig([]byte(dingoCode), config.DefaultConfig())
	goSource, _, metadata, err := prep.ProcessWithMetadata()
	if err != nil {
		t.Fatalf("Preprocessing failed: %v", err)
	}

	t.Logf("Preprocessor emitted %d metadata entries", len(metadata))

	// Parse
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, dingoPath, []byte(goSource), parser.ParseComments)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Generate
	registry, err := builtin.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	logger := plugin.NewNoOpLogger()
	gen, err := generator.NewWithPlugins(fset, registry, logger)
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	outputCode, err := gen.Generate(file)
	if err != nil {
		t.Fatalf("Generation failed: %v", err)
	}

	// Write .go file
	if err := os.WriteFile(goPath, outputCode, 0644); err != nil {
		t.Fatalf("Failed to write Go file: %v", err)
	}

	// Generate source map
	postASTGen := NewPostASTGenerator(dingoPath, goPath, fset, file.File, metadata)
	sourceMap, err := postASTGen.Generate()
	if err != nil {
		t.Fatalf("PostASTGenerator failed: %v", err)
	}

	// Verify source map
	if len(sourceMap.Mappings) == 0 {
		t.Error("Source map has no mappings")
	}

	// Count mappings by type
	typeCounts := make(map[string]int)
	for _, m := range sourceMap.Mappings {
		typeCounts[m.Name]++
	}

	t.Logf("✅ Multi-feature test passed")
	t.Logf("   - Total mappings: %d", len(sourceMap.Mappings))
	for typ, count := range typeCounts {
		t.Logf("   - %s: %d", typ, count)
	}
}
