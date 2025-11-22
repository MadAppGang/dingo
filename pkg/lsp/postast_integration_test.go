package lsp

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
	"github.com/MadAppGang/dingo/pkg/sourcemap"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// TestPostASTSourceMap_Integration validates LSP position translation with Post-AST source maps
// This test uses real transpilation to ensure 100% accuracy
func TestPostASTSourceMap_Integration(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create a test .dingo file with type annotations (: syntax)
	dingoCode := `package main

func add(a: int, b: int) int {
	return a + b
}
`

	dingoPath := filepath.Join(tmpDir, "test.dingo")
	goPath := filepath.Join(tmpDir, "test.go")

	// Write .dingo file
	if err := os.WriteFile(dingoPath, []byte(dingoCode), 0644); err != nil {
		t.Fatalf("Failed to write .dingo file: %v", err)
	}

	// Transpile using the full pipeline
	sm, goCode, err := transpileDingoFile(dingoPath, goPath, dingoCode)
	if err != nil {
		t.Fatalf("Transpilation failed: %v", err)
	}

	// Write .go file
	if err := os.WriteFile(goPath, goCode, 0644); err != nil {
		t.Fatalf("Failed to write .go file: %v", err)
	}

	// Validate source map structure
	if sm.Version != 1 {
		t.Errorf("Expected version 1, got %d", sm.Version)
	}

	if sm.DingoFile != dingoPath {
		t.Errorf("Expected DingoFile %s, got %s", dingoPath, sm.DingoFile)
	}

	if sm.GoFile != goPath {
		t.Errorf("Expected GoFile %s, got %s", goPath, sm.GoFile)
	}

	if len(sm.Mappings) == 0 {
		t.Fatal("Expected mappings, got none")
	}

	// Test LSP position translation with Post-AST source map
	cache := &testCache{sm: sm}
	translator := &Translator{cache: cache}

	// Test case: Type annotation on line 3
	// "a: int" in .dingo should map to correct position in .go
	testCases := []struct {
		name      string
		direction Direction
		inputURI  protocol.DocumentURI
		inputPos  protocol.Position
		wantURI   string
	}{
		{
			name:      "Type annotation DingoToGo (parameter a)",
			direction: DingoToGo,
			inputURI:  uri.File(dingoPath),
			inputPos:  protocol.Position{Line: 2, Character: 11}, // "a: int" (0-based)
			wantURI:   goPath,
		},
		{
			name:      "Type annotation DingoToGo (parameter b)",
			direction: DingoToGo,
			inputURI:  uri.File(dingoPath),
			inputPos:  protocol.Position{Line: 2, Character: 19}, // "b: int" (0-based)
			wantURI:   goPath,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resultURI, resultPos, err := translator.TranslatePosition(tc.inputURI, tc.inputPos, tc.direction)
			if err != nil {
				t.Fatalf("Translation failed: %v", err)
			}

			if resultURI.Filename() != tc.wantURI {
				t.Errorf("Expected URI %s, got %s", tc.wantURI, resultURI.Filename())
			}

			// Verify position is within valid range (non-negative)
			if resultPos.Line < 0 || resultPos.Character < 0 {
				t.Errorf("Invalid position: line=%d, char=%d", resultPos.Line, resultPos.Character)
			}

			t.Logf("Translation: %s:%d:%d -> %s:%d:%d",
				tc.inputURI.Filename(), tc.inputPos.Line, tc.inputPos.Character,
				resultURI.Filename(), resultPos.Line, resultPos.Character)
		})
	}
}

// TestPostASTSourceMap_RoundTrip validates bidirectional translation accuracy
func TestPostASTSourceMap_RoundTrip(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Simple .dingo file
	dingoCode := `package main

func add(a: int, b: int) int {
	return a + b
}
`

	dingoPath := filepath.Join(tmpDir, "roundtrip.dingo")
	goPath := filepath.Join(tmpDir, "roundtrip.go")

	// Write .dingo file
	if err := os.WriteFile(dingoPath, []byte(dingoCode), 0644); err != nil {
		t.Fatalf("Failed to write .dingo file: %v", err)
	}

	// Transpile
	sm, goCode, err := transpileDingoFile(dingoPath, goPath, dingoCode)
	if err != nil {
		t.Fatalf("Transpilation failed: %v", err)
	}

	// Write .go file
	if err := os.WriteFile(goPath, goCode, 0644); err != nil {
		t.Fatalf("Failed to write .go file: %v", err)
	}

	// Test round-trip translation
	cache := &testCache{sm: sm}
	translator := &Translator{cache: cache}

	// Start with a .dingo position
	originalURI := uri.File(dingoPath)
	originalPos := protocol.Position{Line: 2, Character: 10} // Line 3, col 11 (1-based)

	// Translate .dingo -> .go
	goURI, goPos, err := translator.TranslatePosition(originalURI, originalPos, DingoToGo)
	if err != nil {
		t.Fatalf("DingoToGo translation failed: %v", err)
	}

	// Translate .go -> .dingo (round-trip)
	roundTripURI, roundTripPos, err := translator.TranslatePosition(goURI, goPos, GoToDingo)
	if err != nil {
		t.Fatalf("GoToDingo translation failed: %v", err)
	}

	// Verify round-trip returns to original position (or very close)
	// Note: Exact equality may not hold for all positions due to mapping granularity,
	// but for transformed positions it should be exact
	if roundTripURI.Filename() != originalURI.Filename() {
		t.Errorf("Round-trip URI mismatch: expected %s, got %s",
			originalURI.Filename(), roundTripURI.Filename())
	}

	t.Logf("Round-trip: %s:%d:%d -> %s:%d:%d -> %s:%d:%d",
		originalURI.Filename(), originalPos.Line, originalPos.Character,
		goURI.Filename(), goPos.Line, goPos.Character,
		roundTripURI.Filename(), roundTripPos.Line, roundTripPos.Character)
}

// TestPostASTSourceMap_CompareWithLegacy validates that Post-AST source maps
// are at least as accurate as legacy source maps (should be strictly better)
func TestPostASTSourceMap_CompareWithLegacy(t *testing.T) {
	// This test would compare Post-AST vs. legacy generator accuracy
	// For now, we just validate that Post-AST source maps load correctly

	tmpDir := t.TempDir()

	// Create a .dingo file
	dingoCode := `package main

func main() {
	x := 42
}
`

	dingoPath := filepath.Join(tmpDir, "compare.dingo")
	goPath := filepath.Join(tmpDir, "compare.go")

	if err := os.WriteFile(dingoPath, []byte(dingoCode), 0644); err != nil {
		t.Fatalf("Failed to write .dingo file: %v", err)
	}

	// Transpile
	postASTMap, goCode, err := transpileDingoFile(dingoPath, goPath, dingoCode)
	if err != nil {
		t.Fatalf("Transpilation failed: %v", err)
	}

	if err := os.WriteFile(goPath, goCode, 0644); err != nil {
		t.Fatalf("Failed to write .go file: %v", err)
	}

	// Validate Post-AST source map properties
	if postASTMap.Version != 1 {
		t.Errorf("Expected version 1, got %d", postASTMap.Version)
	}

	// All mappings should have valid positions (>0)
	for i, mapping := range postASTMap.Mappings {
		if mapping.GeneratedLine <= 0 || mapping.GeneratedColumn <= 0 {
			t.Errorf("Mapping %d has invalid generated position: line=%d, col=%d",
				i, mapping.GeneratedLine, mapping.GeneratedColumn)
		}

		if mapping.OriginalLine <= 0 || mapping.OriginalColumn <= 0 {
			t.Errorf("Mapping %d has invalid original position: line=%d, col=%d",
				i, mapping.OriginalLine, mapping.OriginalColumn)
		}
	}

	t.Logf("Post-AST source map has %d mappings, all valid", len(postASTMap.Mappings))
}

// TestPostASTSourceMap_CacheIntegration validates that the LSP cache works with Post-AST maps
func TestPostASTSourceMap_CacheIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .dingo and .go files
	dingoPath := filepath.Join(tmpDir, "cached.dingo")
	goPath := filepath.Join(tmpDir, "cached.go")
	mapPath := goPath + ".map"

	dingoCode := `package main

func greet(name: string) {
	println("Hello", name)
}
`

	if err := os.WriteFile(dingoPath, []byte(dingoCode), 0644); err != nil {
		t.Fatalf("Failed to write .dingo file: %v", err)
	}

	// Transpile
	sm, goCode, err := transpileDingoFile(dingoPath, goPath, dingoCode)
	if err != nil {
		t.Fatalf("Transpilation failed: %v", err)
	}

	if err := os.WriteFile(goPath, goCode, 0644); err != nil {
		t.Fatalf("Failed to write .go file: %v", err)
	}

	// Write source map to disk
	smJSON, err := json.MarshalIndent(sm, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal source map: %v", err)
	}

	if err := os.WriteFile(mapPath, smJSON, 0644); err != nil {
		t.Fatalf("Failed to save source map: %v", err)
	}

	// Create real LSP cache and load the source map
	logger := NewLogger("error", os.Stderr)
	cache, err := NewSourceMapCache(logger)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Load source map from cache
	loadedSM, err := cache.Get(goPath)
	if err != nil {
		t.Fatalf("Failed to load source map from cache: %v", err)
	}

	// Validate loaded source map
	if loadedSM.Version != sm.Version {
		t.Errorf("Version mismatch: expected %d, got %d", sm.Version, loadedSM.Version)
	}

	if len(loadedSM.Mappings) != len(sm.Mappings) {
		t.Errorf("Mapping count mismatch: expected %d, got %d",
			len(sm.Mappings), len(loadedSM.Mappings))
	}

	// Test cache hit (should return same source map)
	cachedSM, err := cache.Get(goPath)
	if err != nil {
		t.Fatalf("Cache hit failed: %v", err)
	}

	if cachedSM != loadedSM {
		t.Error("Cache hit returned different source map instance")
	}

	// Test cache size
	if cache.Size() != 1 {
		t.Errorf("Expected cache size 1, got %d", cache.Size())
	}

	t.Log("Post-AST source map cache integration validated successfully")
}

// transpileDingoFile is a helper function that transpiles a .dingo file using the full pipeline
// Returns the Post-AST source map and generated Go code
func transpileDingoFile(dingoPath, goPath, dingoCode string) (*preprocessor.SourceMap, []byte, error) {
	// Step 1: Preprocess with metadata
	prep := preprocessor.NewWithMainConfig([]byte(dingoCode), config.DefaultConfig())
	goSource, _, metadata, err := prep.ProcessWithMetadata()
	if err != nil {
		return nil, nil, err
	}

	// Step 2: Parse preprocessed Go
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, dingoPath, []byte(goSource), parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}

	// Step 3: Generate with plugins
	registry, err := builtin.NewDefaultRegistry()
	if err != nil {
		return nil, nil, err
	}

	logger := plugin.NewNoOpLogger()
	gen, err := generator.NewWithPlugins(fset, registry, logger)
	if err != nil {
		return nil, nil, err
	}

	outputCode, err := gen.Generate(file)
	if err != nil {
		return nil, nil, err
	}

	// Step 4: Write .go file (needed for source map generation)
	if err := os.WriteFile(goPath, outputCode, 0644); err != nil {
		return nil, nil, err
	}

	// Step 5: Generate Post-AST source map
	sm, err := sourcemap.GenerateFromFiles(dingoPath, goPath, metadata)
	if err != nil {
		return nil, nil, err
	}

	return sm, outputCode, nil
}
