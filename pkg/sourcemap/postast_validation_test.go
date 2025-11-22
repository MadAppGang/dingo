package sourcemap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/generator"
	"github.com/MadAppGang/dingo/pkg/parser"
	"github.com/MadAppGang/dingo/pkg/preprocessor"
)

// TestSourceMapCompleteness verifies that generated source maps contain both
// transformation mappings and identity mappings
func TestSourceMapCompleteness(t *testing.T) {
	tests := []struct {
		name                    string
		dingoFile               string
		expectedTransformations int // Number of ? operators, etc.
		minIdentityMappings     int // Minimum number of identity mappings
	}{
		{
			name:                    "error_prop_01_simple",
			dingoFile:               "../../tests/golden/error_prop_01_simple.dingo",
			expectedTransformations: 2,   // 2 '?' operators
			minIdentityMappings:     5,   // At least 5 unmapped lines
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. Transpile .dingo file
			goFile, mapFile := transpileDingoFile(t, tt.dingoFile)
			defer os.Remove(goFile)
			defer os.Remove(mapFile)

			// 2. Load generated source map
			sm := loadSourceMapFile(t, mapFile)

			// 3. Count transformation vs identity mappings
			transformCount := 0
			identityCount := 0
			for _, m := range sm.Mappings {
				if m.Name == "identity" {
					identityCount++
				} else {
					transformCount++
				}
			}

			// 4. Verify transformation count
			if transformCount != tt.expectedTransformations {
				t.Errorf("Expected %d transformations, got %d", tt.expectedTransformations, transformCount)
			}

			// 5. Verify minimum identity mappings
			if identityCount < tt.minIdentityMappings {
				t.Errorf("Expected at least %d identity mappings, got %d", tt.minIdentityMappings, identityCount)
			}

			// 6. Verify all original lines are covered (no gaps)
			assertNoGaps(t, sm.Mappings, tt.dingoFile)
		})
	}
}

// TestPositionTranslationAccuracy verifies that specific positions in .dingo
// map to correct positions in .go
func TestPositionTranslationAccuracy(t *testing.T) {
	tests := []struct {
		name            string
		dingoFile       string
		dingoLine       int    // Line in .dingo file (1-based)
		expectedGoLine  int    // Expected line in .go file (1-based)
		expectedSymbol  string // Symbol at that position
	}{
		{
			name:           "error_prop_simple_first_question_mark",
			dingoFile:      "../../tests/golden/error_prop_01_simple.dingo",
			dingoLine:      4,  // os.ReadFile(path)?
			expectedGoLine: 8,  // ✅ ACTUAL CODE LINE (tmp, err := os.ReadFile(path))
			expectedSymbol: "ReadFile",
		},
		{
			name:           "error_prop_simple_second_question_mark",
			dingoFile:      "../../tests/golden/error_prop_01_simple.dingo",
			dingoLine:      10,  // readConfig("config.yaml")?
			expectedGoLine: 17,  // ✅ ACTUAL CODE LINE (tmp, err := readConfig("config.yaml"))
			expectedSymbol: "readConfig",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. Transpile and load source map
			goFile, mapFile := transpileDingoFile(t, tt.dingoFile)
			defer os.Remove(goFile)
			defer os.Remove(mapFile)

			sm := loadSourceMapFile(t, mapFile)

			// 2. Find mapping for the original line
			var foundMapping *preprocessor.Mapping
			for i := range sm.Mappings {
				if sm.Mappings[i].OriginalLine == tt.dingoLine {
					foundMapping = &sm.Mappings[i]
					break
				}
			}

			if foundMapping == nil {
				t.Fatalf("No mapping found for original line %d", tt.dingoLine)
			}

			// 3. Verify generated line matches expected
			if foundMapping.GeneratedLine != tt.expectedGoLine {
				t.Errorf("Expected generated line %d, got %d", tt.expectedGoLine, foundMapping.GeneratedLine)
			}

			// 4. Verify symbol at position (read .go file and check)
			symbol := getSymbolAtLine(t, goFile, tt.expectedGoLine)
			if !strings.Contains(symbol, tt.expectedSymbol) {
				t.Errorf("Expected symbol to contain %q, got %q", tt.expectedSymbol, symbol)
			}
		})
	}
}

// TestSymbolAtTranslatedPosition verifies that LSP hover would find the correct symbol
func TestSymbolAtTranslatedPosition(t *testing.T) {
	type Position struct {
		Line int
		Col  int
	}

	tests := []struct {
		name            string
		dingoFile       string
		hoverPosition   Position // Where user hovers in .dingo
		expectedSymbol  string   // What gopls should find
		symbolMustExist bool     // Must find symbol (not blank/comment)
	}{
		{
			name:      "hover_on_ReadFile",
			dingoFile: "../../tests/golden/error_prop_01_simple.dingo",
			hoverPosition: Position{
				Line: 4,  // let data = os.ReadFile(path)?
				Col:  18, // Position of "ReadFile"
			},
			expectedSymbol:  "ReadFile",
			symbolMustExist: true,
		},
		{
			name:      "hover_on_os",
			dingoFile: "../../tests/golden/error_prop_01_simple.dingo",
			hoverPosition: Position{
				Line: 4,
				Col:  12, // Position of "os"
			},
			expectedSymbol:  "os",
			symbolMustExist: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. Transpile and load source map
			goFile, mapFile := transpileDingoFile(t, tt.dingoFile)
			defer os.Remove(goFile)
			defer os.Remove(mapFile)

			sm := loadSourceMapFile(t, mapFile)

			// 2. Find mapping for the hover position (simplified - use line mapping)
			var goLine int
			for _, m := range sm.Mappings {
				if m.OriginalLine == tt.hoverPosition.Line {
					goLine = m.GeneratedLine
					break
				}
			}

			if goLine == 0 {
				t.Fatalf("No mapping found for line %d", tt.hoverPosition.Line)
			}

			// 3. Read the .go file at that position
			goContent, err := os.ReadFile(goFile)
			if err != nil {
				t.Fatalf("Failed to read .go file: %v", err)
			}

			lines := strings.Split(string(goContent), "\n")
			if goLine < 1 || goLine > len(lines) {
				t.Fatalf("Translated line %d out of range (1-%d)", goLine, len(lines))
			}

			line := lines[goLine-1] // Convert to 0-based

			// 4. Verify symbol exists at position
			if tt.symbolMustExist {
				// Check line is not empty and not a comment
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					t.Errorf("Translated to BLANK line %d (should be code)", goLine)
				}
				if strings.HasPrefix(trimmed, "//") {
					t.Errorf("Translated to COMMENT line %d: %q (should be code)", goLine, trimmed)
				}
			}

			// 5. Verify expected symbol is on the line
			if !strings.Contains(line, tt.expectedSymbol) {
				t.Errorf("Line %d does not contain symbol %q\nLine: %q",
					goLine, tt.expectedSymbol, line)
			}
		})
	}
}

// TestNoMappingsToComments verifies that transformation mappings NEVER point to comment lines
func TestNoMappingsToComments(t *testing.T) {
	dingoFile := "../../tests/golden/error_prop_01_simple.dingo"

	goFile, mapFile := transpileDingoFile(t, dingoFile)
	defer os.Remove(goFile)
	defer os.Remove(mapFile)

	sm := loadSourceMapFile(t, mapFile)
	goContent, _ := os.ReadFile(goFile)
	lines := strings.Split(string(goContent), "\n")

	// Check all transformation mappings
	for _, m := range sm.Mappings {
		if m.Name == "identity" {
			continue // Skip identity mappings
		}

		// Transformation mapping - must NOT point to comment
		if m.GeneratedLine < 1 || m.GeneratedLine > len(lines) {
			t.Errorf("Mapping line %d out of range", m.GeneratedLine)
			continue
		}

		line := lines[m.GeneratedLine-1]
		trimmed := strings.TrimSpace(line)

		// CRITICAL: Transformation mappings must point to CODE, not comments
		if strings.HasPrefix(trimmed, "//") {
			t.Errorf("Transformation mapping %q points to COMMENT line %d: %q",
				m.Name, m.GeneratedLine, trimmed)
		}

		// Also check it's not blank
		if trimmed == "" {
			t.Errorf("Transformation mapping %q points to BLANK line %d",
				m.Name, m.GeneratedLine)
		}
	}
}

// TestRoundTripTranslation verifies that .dingo → .go → .dingo translation is lossless
func TestRoundTripTranslation(t *testing.T) {
	tests := []struct {
		name      string
		dingoFile string
		testLines []int // Test line numbers in .dingo (1-based)
	}{
		{
			name:      "error_prop_01_simple",
			dingoFile: "../../tests/golden/error_prop_01_simple.dingo",
			testLines: []int{4, 10}, // Two ? operators
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. Transpile and load source map
			goFile, mapFile := transpileDingoFile(t, tt.dingoFile)
			defer os.Remove(goFile)
			defer os.Remove(mapFile)

			sm := loadSourceMapFile(t, mapFile)

			// 2. Build reverse mapping (generated line → original line)
			reverseMap := make(map[int]int)
			for _, m := range sm.Mappings {
				reverseMap[m.GeneratedLine] = m.OriginalLine
			}

			// 3. Test round-trip for each line
			for _, dingoLine := range tt.testLines {
				// Forward: .dingo → .go
				var goLine int
				for _, m := range sm.Mappings {
					if m.OriginalLine == dingoLine {
						goLine = m.GeneratedLine
						break
					}
				}

				if goLine == 0 {
					t.Errorf("No mapping found for dingo line %d", dingoLine)
					continue
				}

				// Reverse: .go → .dingo
				backToDingoLine, exists := reverseMap[goLine]
				if !exists {
					t.Errorf("No reverse mapping for go line %d", goLine)
					continue
				}

				// Verify round-trip accuracy
				if backToDingoLine != dingoLine {
					t.Errorf("Round-trip failed: %d → %d → %d", dingoLine, goLine, backToDingoLine)
				}
			}
		})
	}
}

// Helper functions

// transpileDingoFile transpiles a .dingo file and returns paths to .go and .go.map files
func transpileDingoFile(t *testing.T, dingoPath string) (goPath, mapPath string) {
	t.Helper()

	// Read .dingo source
	dingoSource, err := os.ReadFile(dingoPath)
	if err != nil {
		t.Fatalf("Failed to read .dingo file: %v", err)
	}

	// Parse with preprocessor
	p := parser.NewGoParserInstance()
	parseResult, err := p.ParseFile(dingoPath, dingoSource)
	if err != nil {
		t.Fatalf("Failed to parse .dingo file: %v", err)
	}

	// Generate Go code
	gen, err := generator.NewWithPlugins(parseResult.FileSet, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	goCode, err := gen.Generate(&dingoast.File{File: parseResult.AST})
	if err != nil {
		t.Fatalf("Failed to generate Go code: %v", err)
	}

	// Write temporary .go file
	goPath = filepath.Join(t.TempDir(), filepath.Base(dingoPath)+".go")
	if err := os.WriteFile(goPath, goCode, 0644); err != nil {
		t.Fatalf("Failed to write .go file: %v", err)
	}

	// Get preprocessor metadata
	prep := preprocessor.New(dingoSource)
	_, _, metadata, err := prep.ProcessWithMetadata()
	if err != nil {
		t.Fatalf("Failed to get preprocessor metadata: %v", err)
	}

	// CRITICAL: Use GenerateFromFiles which re-parses the WRITTEN .go file
	// This ensures FileSet positions match the final output, not preprocessor output
	sourceMap, err := GenerateFromFiles(dingoPath, goPath, metadata)
	if err != nil {
		t.Fatalf("Failed to generate source map: %v", err)
	}

	// Write source map
	mapPath = goPath + ".map"
	mapJSON, err := json.MarshalIndent(sourceMap, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal source map: %v", err)
	}

	if err := os.WriteFile(mapPath, mapJSON, 0644); err != nil {
		t.Fatalf("Failed to write source map: %v", err)
	}

	return goPath, mapPath
}

// loadSourceMapFile loads a source map from a .go.map file
func loadSourceMapFile(t *testing.T, mapPath string) *preprocessor.SourceMap {
	t.Helper()

	mapJSON, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatalf("Failed to read source map: %v", err)
	}

	var sm preprocessor.SourceMap
	if err := json.Unmarshal(mapJSON, &sm); err != nil {
		t.Fatalf("Failed to parse source map: %v", err)
	}

	return &sm
}

// assertNoGaps verifies that all lines in the .dingo file have at least one mapping
func assertNoGaps(t *testing.T, mappings []preprocessor.Mapping, dingoPath string) {
	t.Helper()

	// Read .dingo file to count lines
	dingoSource, err := os.ReadFile(dingoPath)
	if err != nil {
		t.Fatalf("Failed to read .dingo file: %v", err)
	}

	dingoLines := len(strings.Split(string(dingoSource), "\n"))

	// Track which lines have mappings
	covered := make(map[int]bool)
	for _, m := range mappings {
		covered[m.OriginalLine] = true
	}

	// Check for gaps (allow empty lines and comment-only lines)
	dingoLineText := strings.Split(string(dingoSource), "\n")
	for line := 1; line <= dingoLines; line++ {
		if line > len(dingoLineText) {
			break
		}

		trimmed := strings.TrimSpace(dingoLineText[line-1])
		// Skip blank lines and comment-only lines
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		// All non-empty, non-comment lines should have mappings
		if !covered[line] {
			t.Errorf("Line %d has no mapping: %q", line, dingoLineText[line-1])
		}
	}
}

// getSymbolAtLine extracts a representative symbol/identifier from a line in the Go file
func getSymbolAtLine(t *testing.T, goPath string, line int) string {
	t.Helper()

	goSource, err := os.ReadFile(goPath)
	if err != nil {
		t.Fatalf("Failed to read .go file: %v", err)
	}

	lines := strings.Split(string(goSource), "\n")
	if line < 1 || line > len(lines) {
		t.Fatalf("Line %d out of range (file has %d lines)", line, len(lines))
	}

	return lines[line-1]
}
