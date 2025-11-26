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
			dingoLine:      6,  // let data = os.ReadFile(filePath)?
			expectedGoLine: 8,  // tmp, err := os.ReadFile(filePath)
			expectedSymbol: "ReadFile",
		},
		{
			name:           "error_prop_simple_second_question_mark",
			dingoFile:      "../../tests/golden/error_prop_01_simple.dingo",
			dingoLine:      12,  // let a = readConfig("config.yaml")?
			expectedGoLine: 17,  // tmp, err := readConfig("config.yaml")
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
				Line: 6,  // let data = os.ReadFile(filePath)?
				Col:  18, // Position of "ReadFile"
			},
			expectedSymbol:  "ReadFile",
			symbolMustExist: true,
		},
		{
			name:      "hover_on_os",
			dingoFile: "../../tests/golden/error_prop_01_simple.dingo",
			hoverPosition: Position{
				Line: 6,  // let data = os.ReadFile(filePath)?
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
// EXPANDED: Now tests BOTH transformed AND untransformed lines to catch identity mapping bugs
func TestRoundTripTranslation(t *testing.T) {
	tests := []struct {
		name        string
		dingoFile   string
		testLines   []int  // Test line numbers in .dingo (1-based)
		description []string // What each line is (for debugging)
	}{
		{
			name:      "error_prop_01_simple",
			dingoFile: "../../tests/golden/error_prop_01_simple.dingo",
			testLines: []int{
				1,  // package main (identity mapping - CRITICAL)
				3,  // func readConfig (identity mapping - CRITICAL for Go to Definition)
				4,  // let data = ... ? (transformation)
				5,  // return data (identity mapping)
				9,  // func test (identity mapping)
				10, // let a = ... ? (transformation)
				11, // println (identity mapping)
			},
			description: []string{
				"package main",
				"func readConfig",
				"? operator",
				"return statement",
				"func test",
				"? operator",
				"println call",
			},
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
			// NOTE: Multiple mappings can have the same generated line (e.g., due to duplicates)
			// We need to handle this properly - for now, use the FIRST mapping (prefer transformations)
			reverseMap := make(map[int]int)
			for _, m := range sm.Mappings {
				// Only set if not already set (first mapping wins)
				if _, exists := reverseMap[m.GeneratedLine]; !exists {
					reverseMap[m.GeneratedLine] = m.OriginalLine
				}
			}

			// 3. Test round-trip for each line
			for i, dingoLine := range tt.testLines {
				desc := ""
				if i < len(tt.description) {
					desc = tt.description[i]
				}

				// Forward: .dingo → .go
				var goLine int
				for _, m := range sm.Mappings {
					if m.OriginalLine == dingoLine {
						goLine = m.GeneratedLine
						break
					}
				}

				if goLine == 0 {
					t.Errorf("No mapping found for dingo line %d (%s)", dingoLine, desc)
					continue
				}

				// Reverse: .go → .dingo
				backToDingoLine, exists := reverseMap[goLine]
				if !exists {
					t.Errorf("No reverse mapping for go line %d (from dingo line %d: %s)", goLine, dingoLine, desc)
					continue
				}

				// Verify round-trip accuracy
				if backToDingoLine != dingoLine {
					t.Errorf("Round-trip failed for %s: dingo %d → go %d → dingo %d (expected %d)",
						desc, dingoLine, goLine, backToDingoLine, dingoLine)

					// Show actual lines for debugging
					dingoContent, _ := os.ReadFile(tt.dingoFile)
					goContent, _ := os.ReadFile(goFile)
					dingoLines := strings.Split(string(dingoContent), "\n")
					goLines := strings.Split(string(goContent), "\n")

					t.Errorf("  Expected: dingo line %d: %q", dingoLine, dingoLines[dingoLine-1])
					t.Errorf("  Got:      dingo line %d: %q", backToDingoLine, dingoLines[backToDingoLine-1])
					t.Errorf("  Via:      go line %d: %q", goLine, goLines[goLine-1])
				}
			}
		})
	}
}

// TestIdentityMappingReverse specifically tests reverse mapping for UNTRANSFORMED lines
// This catches bugs where identity mappings don't account for line shifts (e.g., import blocks)
func TestIdentityMappingReverse(t *testing.T) {
	tests := []struct {
		name              string
		dingoFile         string
		goLine            int    // Line in .go file (1-based)
		expectedDingoLine int    // Expected line in .dingo file (1-based)
		description       string // What this line is
	}{
		{
			name:              "function_definition",
			dingoFile:         "../../tests/golden/error_prop_01_simple.dingo",
			goLine:            7,  // func readConfig in .go
			expectedDingoLine: 5,  // func readConfig in .dingo (line 5: func readConfig...)
			description:       "func readConfig(path string) ([]byte, error)",
		},
		{
			name:              "package_declaration",
			dingoFile:         "../../tests/golden/error_prop_01_simple.dingo",
			goLine:            1,  // package main in .go
			expectedDingoLine: 1,  // package main in .dingo
			description:       "package main",
		},
		{
			name:              "return_statement",
			dingoFile:         "../../tests/golden/error_prop_01_simple.dingo",
			goLine:            14, // return data, nil in .go
			expectedDingoLine: 7,  // return data, nil in .dingo (line 7)
			description:       "return data, nil",
		},
		{
			name:              "second_function",
			dingoFile:         "../../tests/golden/error_prop_01_simple.dingo",
			goLine:            16, // func test in .go
			expectedDingoLine: 11, // func test in .dingo (line 11)
			description:       "func test()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goFile, mapFile := transpileDingoFile(t, tt.dingoFile)
			defer os.Remove(goFile)
			defer os.Remove(mapFile)

			sm := loadSourceMapFile(t, mapFile)

			// Reverse translate: .go → .dingo
			dingoLine, _ := sm.MapToOriginal(tt.goLine, 1)

			if dingoLine != tt.expectedDingoLine {
				t.Errorf("CRITICAL: Go to Definition would jump to WRONG line!")
				t.Errorf("Go line %d (%s)", tt.goLine, tt.description)
				t.Errorf("Expected .dingo line %d, got %d", tt.expectedDingoLine, dingoLine)

				// Read files and show actual lines
				dingoContent, _ := os.ReadFile(tt.dingoFile)
				goContent, _ := os.ReadFile(goFile)
				dingoLines := strings.Split(string(dingoContent), "\n")
				goLines := strings.Split(string(goContent), "\n")

				t.Errorf("")
				t.Errorf("Expected: %q", dingoLines[tt.expectedDingoLine-1])
				if dingoLine >= 1 && dingoLine <= len(dingoLines) {
					t.Errorf("Got:      %q", dingoLines[dingoLine-1])
				}
				t.Errorf("Go line:  %q", goLines[tt.goLine-1])
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

// TestGoToDefinitionReverse verifies reverse translation (Go → Dingo) for LSP "Go to Definition"
// This test exposes the critical bug: identity mappings don't account for import block shifts
func TestGoToDefinitionReverse(t *testing.T) {
	dingoFile := "../../tests/golden/error_prop_01_simple.dingo"

	// Transpile and load source map
	goFile, mapFile := transpileDingoFile(t, dingoFile)
	defer os.Remove(goFile)
	defer os.Remove(mapFile)

	sm := loadSourceMapFile(t, mapFile)

	// Read both files to understand the line shift
	dingoContent, _ := os.ReadFile(dingoFile)
	goContent, _ := os.ReadFile(goFile)
	dingoLines := strings.Split(string(dingoContent), "\n")
	goLines := strings.Split(string(goContent), "\n")

	t.Logf("Dingo file has %d lines, Go file has %d lines", len(dingoLines), len(goLines))

	// TEST CASE: In .go file, "func readConfig" is at line 7
	// In .dingo file, it's at line 5 (after package, empty line, import, empty line)
	// Expected: MapToOriginal(7, 1) → (5, 1)

	goLineWithFunc := 7 // func readConfig in .go file

	// Use MapToOriginal to reverse translate
	originalLine, originalCol := sm.MapToOriginal(goLineWithFunc, 1)

	t.Logf("MapToOriginal(%d, 1) → (%d, %d)", goLineWithFunc, originalLine, originalCol)

	// Debug: Show all mappings to understand the offset
	t.Logf("All mappings:")
	for i, m := range sm.Mappings {
		if i < 15 { // Show first 15 mappings
			t.Logf("  %s: dingo line %d → go line %d", m.Name, m.OriginalLine, m.GeneratedLine)
		}
	}

	// Expected: line 5 (where "func readConfig" actually is in .dingo)
	expectedDingoLine := 5

	if originalLine != expectedDingoLine {
		t.Errorf("CRITICAL BUG: Go to Definition would jump to WRONG line!")
		t.Errorf("Expected original line %d, got %d", expectedDingoLine, originalLine)
		t.Errorf("")
		t.Errorf("Dingo file line %d: %q", expectedDingoLine, dingoLines[expectedDingoLine-1])
		t.Errorf("Dingo file line %d: %q", originalLine, dingoLines[originalLine-1])
		t.Errorf("")
		t.Errorf("Go file line %d: %q", goLineWithFunc, goLines[goLineWithFunc-1])
	}

	// Verify the line we mapped to is actually the function definition
	if originalLine >= 1 && originalLine <= len(dingoLines) {
		dingoLine := dingoLines[originalLine-1]
		if !strings.Contains(dingoLine, "func readConfig") {
			t.Errorf("Mapped line %d is NOT the function definition: %q", originalLine, dingoLine)
		}
	}
}
