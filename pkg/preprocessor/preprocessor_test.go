package preprocessor

import (
	"fmt"
	"strings"
	"testing"
)

func TestErrorPropagationBasic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "simple assignment",
			input: `package main

func readConfig(path: string) ([]byte, error) {
	let data = os.ReadFile(path)?
	return data, nil
}`,
			expected: `package main

import "os"

func readConfig(path string) ([]byte, error) {
	__tmp0, __err0 := os.ReadFile(path)
	// dingo:s:1
	if __err0 != nil {
		return nil, __err0
	}
	// dingo:e:1
	var data = __tmp0
	return data, nil
}`,
		},
		{
			name: "simple return",
			input: `package main

func parseInt(s: string) (int, error) {
	return strconv.Atoi(s)?
}`,
			expected: `package main

import "strconv"

func parseInt(s string) (int, error) {
	__tmp0, __err0 := strconv.Atoi(s)
	// dingo:s:1
	if __err0 != nil {
		return 0, __err0
	}
	// dingo:e:1
	return __tmp0, nil
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New([]byte(tt.input))
			result, _, err := p.Process()
			if err != nil {
				t.Fatalf("preprocessing failed: %v", err)
			}

			actual := strings.TrimSpace(result)
			expected := strings.TrimSpace(tt.expected)

			if actual != expected {
				t.Errorf("output mismatch:\n=== EXPECTED ===\n%s\n\n=== ACTUAL ===\n%s\n", expected, actual)
			}
		})
	}
}

// TestIMPORTANT1_ErrorMessageEscaping tests IMPORTANT-1 fix:
// Error messages with % characters must be escaped to prevent fmt.Errorf panics
func TestIMPORTANT1_ErrorMessageEscaping(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldHave  string
		shouldntHave string
	}{
		{
			name: "percent in error message",
			input: `package main

func readData(path: string) ([]byte, error) {
	let data = os.ReadFile(path)? "failed: 50% complete"
	return data, nil
}`,
			shouldHave: `fmt.Errorf("failed: 50%% complete: %w"`,
			shouldntHave: `fmt.Errorf("failed: 50% complete: %w"`, // This would panic!
		},
		{
			name: "multiple percents in error message",
			input: `package main

func process() (string, error) {
	return DoWork()? "progress: 25% to 75%"
}`,
			shouldHave: `fmt.Errorf("progress: 25%% to 75%%: %w"`,
			shouldntHave: `fmt.Errorf("progress: 25% to 75%: %w"`, // This would panic!
		},
		{
			name: "percent-w pattern in error message",
			input: `package main

func test() (int, error) {
	return Calc()? "100%w complete"
}`,
			shouldHave: `fmt.Errorf("100%%w complete: %w"`,
			shouldntHave: `fmt.Errorf("100%w complete: %w"`, // Would create %w%w!
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New([]byte(tt.input))
			result, _, err := p.Process()
			if err != nil {
				t.Fatalf("preprocessing failed: %v", err)
			}

			actual := string(result)

			if !strings.Contains(actual, tt.shouldHave) {
				t.Errorf("expected to find:\n%s\n\nActual output:\n%s", tt.shouldHave, actual)
			}

			if strings.Contains(actual, tt.shouldntHave) {
				t.Errorf("should NOT contain (unescaped):\n%s\n\nActual output:\n%s", tt.shouldntHave, actual)
			}
		})
	}
}

// TestIMPORTANT2_TypeAnnotationEnhancement tests IMPORTANT-2 fix:
// Type annotations must handle complex Go types including function types, channels, nested generics
func TestIMPORTANT2_TypeAnnotationEnhancement(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "function type in parameters",
			input: `package main

func process(handler: func(int) error) error {
	return nil
}`,
			expected: `package main

func process(handler func(int) error) error {
	return nil
}`,
		},
		{
			name: "channel with direction",
			input: `package main

func send(ch: <-chan string, out: chan<- int) {
}`,
			expected: `package main

func send(ch <-chan string, out chan<- int) {
}`,
		},
		{
			name: "complex nested generics",
			input: `package main

func lookup(cache: map[string][]interface{}, key: string) {
}`,
			expected: `package main

func lookup(cache map[string][]interface{}, key string) {
}`,
		},
		{
			name: "function returning multiple values",
			input: `package main

func transform(fn: func(a, b int) (string, error)) {
}`,
			expected: `package main

func transform(fn func(a, b int) (string, error)) {
}`,
		},
		{
			name: "nested function types",
			input: `package main

func higher(fn: func() func() error) {
}`,
			expected: `package main

func higher(fn func() func() error) {
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New([]byte(tt.input))
			result, _, err := p.Process()
			if err != nil {
				t.Fatalf("preprocessing failed: %v", err)
			}

			actual := strings.TrimSpace(string(result))
			expected := strings.TrimSpace(tt.expected)

			if actual != expected {
				t.Errorf("output mismatch:\n=== EXPECTED ===\n%s\n\n=== ACTUAL ===\n%s\n", expected, actual)
			}
		})
	}
}

// TestGeminiCodeReviewFixes verifies both IMPORTANT fixes from Gemini code review work together
func TestGeminiCodeReviewFixes(t *testing.T) {
	// This test combines both fixes in a realistic scenario:
	// - IMPORTANT-1: Error message escaping (% → %%)
	// - IMPORTANT-2: Complex type annotations (function types, channels)
	// - Bonus: Ternary detection must ignore : in string literals

	input := `package main

func processData(handler: func([]byte) error, path: string) ([]byte, error) {
	let data = os.ReadFile(path)? "failed: 50% complete"
	return data, nil
}

func fetchConfig(url: string) ([]byte, error) {
	return http.Get(url)? "progress: 25% to 75%"
}`

	p := New([]byte(input))
	result, _, err := p.Process()
	if err != nil {
		t.Fatalf("preprocessing failed: %v", err)
	}

	actual := string(result)

	// Verify critical aspects of the fixes
	if !strings.Contains(actual, `"failed: 50%% complete: %w"`) {
		t.Error("IMPORTANT-1 failed: % not escaped in first error message")
	}
	if !strings.Contains(actual, `"progress: 25%% to 75%%: %w"`) {
		t.Error("IMPORTANT-1 failed: % not escaped in second error message")
	}
	if !strings.Contains(actual, "handler func([]byte) error") {
		t.Error("IMPORTANT-2 failed: function type not handled correctly")
	}
	if !strings.Contains(actual, "url string") {
		t.Error("Type annotation conversion failed")
	}
	// Verify imports were added
	if !strings.Contains(actual, `"fmt"`) {
		t.Error("fmt import not added")
	}
	if !strings.Contains(actual, `"os"`) {
		t.Error("os import not added (for os.ReadFile)")
	}
	if !strings.Contains(actual, `"net/http"`) {
		t.Error("net/http import not added (for http.Get)")
	}
}

// TestSourceMapGeneration verifies that source maps are correctly generated
// for error propagation expansions (1 source line → 7 generated lines)
// AND that mappings are correctly adjusted for added imports
func TestSourceMapGeneration(t *testing.T) {
	input := `package main

func readConfig(path string) ([]byte, error) {
	let data = os.ReadFile(path)?
	return data, nil
}`

	p := New([]byte(input))
	_, sourceMap, err := p.Process()
	if err != nil {
		t.Fatalf("preprocessing failed: %v", err)
	}

	// The error propagation on line 4 should generate 8 mappings:
	// - 1 expr_mapping for the expression (os.ReadFile(path))
	// - 7 error_prop mappings for the error handling expansion
	// All mappings point back to original line 4
	// HOWEVER: With import injection, lines are shifted down by 3 (package + blank + import)

	// Expected mappings (line 4 in input → lines 8-14 in output after import block):
	// CRITICAL: Import block format AFTER code generator reformatting:
	// Line 3: import (
	// Line 4:     "os"
	// Line 5: )
	// Line 6: (blank)
	// So import offset is +4 (lines 3-6), not +3!
	// Mapping 0 (expr_mapping): Line 8 - __tmp0, __err0 := os.ReadFile(path)
	// Mapping 1 (error_prop):   Line 8 - __tmp0, __err0 := os.ReadFile(path)
	// Mapping 2 (error_prop):   Line 9 - // dingo:s:1
	// Mapping 3 (error_prop):   Line 10 - if __err0 != nil {
	// Mapping 4 (error_prop):   Line 11 - return nil, __err0
	// Mapping 5 (error_prop):   Line 12 - }
	// Mapping 6 (error_prop):   Line 13 - // dingo:e:1
	// Mapping 7 (var_name):     Line 14 - var data (the variable name itself)
	// Mapping 8 (error_prop):   Line 14 - var data = __tmp0

	expectedMappings := []struct {
		originalLine  int
		generatedLine int
	}{
		{4, 8},  // expr_mapping: os.ReadFile(path)
		{4, 8},  // error_prop: __tmp0, __err0 := os.ReadFile(path)
		{4, 9},  // error_prop: // dingo:s:1
		{4, 10}, // error_prop: if __err0 != nil {
		{4, 11}, // error_prop: return nil, __err0
		{4, 12}, // error_prop: }
		{4, 13}, // error_prop: // dingo:e:1
		{4, 14}, // var_name: var data (the variable name itself)
		{4, 14}, // error_prop: var data = __tmp0
	}

	if len(sourceMap.Mappings) != len(expectedMappings) {
		t.Errorf("expected %d mappings, got %d", len(expectedMappings), len(sourceMap.Mappings))
		for i, m := range sourceMap.Mappings {
			t.Logf("Mapping %d: orig=%d gen=%d", i, m.OriginalLine, m.GeneratedLine)
		}
		return
	}

	for i, expected := range expectedMappings {
		mapping := sourceMap.Mappings[i]
		if mapping.OriginalLine != expected.originalLine {
			t.Errorf("mapping %d: expected original line %d, got %d",
				i, expected.originalLine, mapping.OriginalLine)
		}
		if mapping.GeneratedLine != expected.generatedLine {
			t.Errorf("mapping %d: expected generated line %d, got %d",
				i, expected.generatedLine, mapping.GeneratedLine)
		}
	}
}

// TestSourceMapMultipleExpansions verifies source maps when multiple
// error propagations occur in the same function
// AND that mappings account for import block offset
func TestSourceMapMultipleExpansions(t *testing.T) {
	input := `package main

func process(path string) ([]byte, error) {
	let data = os.ReadFile(path)?
	let result = Process(data)?
	return result, nil
}`

	p := New([]byte(input))
	_, sourceMap, err := p.Process()
	if err != nil {
		t.Fatalf("preprocessing failed: %v", err)
	}

	// Line 4 expands to 9 mappings (1 expr + 1 var_name + 7 error_prop)
	// Line 5 expands to 9 mappings (1 expr + 1 var_name + 7 error_prop)
	// Total: 18 mappings

	if len(sourceMap.Mappings) != 18 {
		t.Errorf("expected 18 mappings (9+9: 2 expr + 2 var_name + 14 error_prop), got %d", len(sourceMap.Mappings))
		for i, m := range sourceMap.Mappings {
			t.Logf("Mapping %d: orig=%d gen=%d", i, m.OriginalLine, m.GeneratedLine)
		}
		return
	}

	// First expansion: line 4 → 9 mappings (1 expr_mapping + 1 var_name + 7 error_prop)
	// Generated lines: 8-14 (with import offset of +4 due to code generator reformatting)
	const importOffset = 4 // package main + blank + import block (3 lines) + blank

	// Mapping 0: expr_mapping for line 4 (points to line 8)
	if sourceMap.Mappings[0].OriginalLine != 4 {
		t.Errorf("mapping 0: expected original line 4, got %d", sourceMap.Mappings[0].OriginalLine)
	}
	if sourceMap.Mappings[0].GeneratedLine != 8 {
		t.Errorf("mapping 0: expected generated line 8, got %d", sourceMap.Mappings[0].GeneratedLine)
	}

	// Mappings 1-8: error_prop + var_name for line 4 (generated lines 8-14)
	// Note: Some mappings share the same line (e.g., var_name and error_prop both on line 14)
	for i := 1; i < 9; i++ {
		mapping := sourceMap.Mappings[i]
		if mapping.OriginalLine != 4 {
			t.Errorf("mapping %d: expected original line 4, got %d", i, mapping.OriginalLine)
		}
		// Check that generated line is in the expected range (8-14)
		if mapping.GeneratedLine < 8 || mapping.GeneratedLine > 14 {
			t.Errorf("mapping %d: generated line %d out of expected range 8-14",
				i, mapping.GeneratedLine)
		}
	}

	// Second expansion: line 5 → 9 mappings (1 expr_mapping + 1 var_name + 7 error_prop)
	// Generated lines: 15-21

	// Mapping 9: expr_mapping for line 5 (points to line 15)
	if sourceMap.Mappings[9].OriginalLine != 5 {
		t.Errorf("mapping 9: expected original line 5, got %d", sourceMap.Mappings[9].OriginalLine)
	}
	if sourceMap.Mappings[9].GeneratedLine != 15 {
		t.Errorf("mapping 9: expected generated line 15, got %d", sourceMap.Mappings[9].GeneratedLine)
	}

	// Mappings 10-17: error_prop + var_name for line 5 (generated lines 15-21)
	// Note: Some mappings share the same line (e.g., var_name and error_prop both on line 21)
	for i := 10; i < 18; i++ {
		mapping := sourceMap.Mappings[i]
		if mapping.OriginalLine != 5 {
			t.Errorf("mapping %d: expected original line 5, got %d", i, mapping.OriginalLine)
		}
		// Check that generated line is in the expected range (15-21)
		if mapping.GeneratedLine < 15 || mapping.GeneratedLine > 21 {
			t.Errorf("mapping %d: generated line %d out of expected range 15-21",
				i, mapping.GeneratedLine)
		}
	}
}

// TestAutomaticImportDetection verifies that imports are automatically added
func TestAutomaticImportDetection(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedImports []string
	}{
		{
			name: "os.ReadFile import",
			input: `package main

func readConfig(path string) ([]byte, error) {
	let data = os.ReadFile(path)?
	return data, nil
}`,
			expectedImports: []string{"os"},
		},
		{
			name: "strconv.Atoi import",
			input: `package main

func parseInt(s string) (int, error) {
	return strconv.Atoi(s)?
}`,
			expectedImports: []string{"strconv"},
		},
		{
			name: "multiple imports",
			input: `package main

func process(path string, num string) ([]byte, error) {
	let data = os.ReadFile(path)?
	let n = strconv.Atoi(num)?
	return data, nil
}`,
			expectedImports: []string{"os", "strconv"},
		},
		{
			name: "with error message (needs fmt)",
			input: `package main

func readData(path string) ([]byte, error) {
	let data = os.ReadFile(path)? "failed to read"
	return data, nil
}`,
			expectedImports: []string{"fmt", "os"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New([]byte(tt.input))
			result, _, err := p.Process()
			if err != nil {
				t.Fatalf("preprocessing failed: %v", err)
			}

			resultStr := string(result)

			// Verify each expected import is present
			for _, expectedPkg := range tt.expectedImports {
				expectedImport := fmt.Sprintf(`"%s"`, expectedPkg)
				if !strings.Contains(resultStr, expectedImport) {
					t.Errorf("expected import %q not found in output:\n%s", expectedPkg, resultStr)
				}
			}
		})
	}
}

// TestSourceMappingWithImports verifies that source mappings are correctly adjusted
// after import injection
func TestSourceMappingWithImports(t *testing.T) {
	input := `package main

func example(path string) ([]byte, error) {
	let data = os.ReadFile(path)?
	return data, nil
}`

	p := New([]byte(input))
	result, sourceMap, err := p.Process()
	if err != nil {
		t.Fatalf("preprocessing failed: %v", err)
	}

	resultStr := string(result)

	// Verify import was added
	if !strings.Contains(resultStr, `import "os"`) {
		t.Errorf("expected os import, got:\n%s", resultStr)
	}

	// Count lines in result to determine import block size
	resultLines := strings.Split(resultStr, "\n")
	t.Logf("Result has %d lines", len(resultLines))

	// Find the line number where the error propagation expansion starts
	// This should be after: package main, blank line, import "os", blank line
	// So expansion should start around line 5

	// Verify all mappings reference the correct original line (line 4 in input)
	for i, mapping := range sourceMap.Mappings {
		if mapping.OriginalLine != 4 {
			t.Errorf("mapping %d: expected original line 4, got %d", i, mapping.OriginalLine)
		}

		// Generated lines should be >= 5 (after package + import block)
		if mapping.GeneratedLine < 5 {
			t.Errorf("mapping %d: generated line %d is before imports end", i, mapping.GeneratedLine)
		}
	}

	// Should have 9 mappings (1 expr_mapping + 1 var_name + 7 error_prop)
	if len(sourceMap.Mappings) != 9 {
		t.Errorf("expected 9 mappings (1 expr + 1 var_name + 7 error_prop), got %d", len(sourceMap.Mappings))
		for i, m := range sourceMap.Mappings {
			t.Logf("Mapping %d: orig=%d gen=%d", i, m.OriginalLine, m.GeneratedLine)
		}
	}
}

// TestCRITICAL2_MultiValueReturnHandling verifies CRITICAL-2 fix:
// Multi-value returns must preserve all non-error values in success path
func TestCRITICAL2_MultiValueReturnHandling(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name: "two values plus error",
			input: `package main

func parseConfig(data string) (int, string, error) {
	return parseData(data)?
}`,
			shouldContain: []string{
				"__tmp0, __tmp1, __err0 := parseData(data)", // Two temps for non-error values, err0 for error
				`return 0, "", __err0`, // error path with two zero values
				"return __tmp0, __tmp1, nil", // success path with both values
			},
			shouldNotContain: []string{
				"return __tmp0, nil", // WRONG: drops __tmp1
			},
		},
		{
			name: "three values plus error",
			input: `package main

func loadUser(id int) (string, int, bool, error) {
	return fetchUser(id)?
}`,
			shouldContain: []string{
				"__tmp0, __tmp1, __tmp2, __err0 := fetchUser(id)", // Three temps for non-error values, err0 for error
				`return "", 0, false, __err0`, // error path with three zero values
				"return __tmp0, __tmp1, __tmp2, nil", // success path with all three values
			},
			shouldNotContain: []string{
				"return __tmp0, nil", // WRONG: drops values
			},
		},
		{
			name: "single value plus error (regression)",
			input: `package main

func parseInt(s string) (int, error) {
	return strconv.Atoi(s)?
}`,
			shouldContain: []string{
				"__tmp0, __err0 := strconv.Atoi(s)", // One temp for non-error value, err0 for error
				"return 0, __err0", // error path
				"return __tmp0, nil", // success path
			},
			shouldNotContain: []string{
				"__tmp0, __tmp1", // WRONG: too many temps for single value
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New([]byte(tt.input))
			result, _, err := p.Process()
			if err != nil {
				t.Fatalf("preprocessing failed: %v", err)
			}

			resultStr := string(result)

			// Verify required patterns
			for _, pattern := range tt.shouldContain {
				if !strings.Contains(resultStr, pattern) {
					t.Errorf("expected to find pattern:\n%s\n\nActual output:\n%s", pattern, resultStr)
				}
			}

			// Verify forbidden patterns
			for _, pattern := range tt.shouldNotContain {
				if strings.Contains(resultStr, pattern) {
					t.Errorf("should NOT contain pattern:\n%s\n\nActual output:\n%s", pattern, resultStr)
				}
			}
		})
	}
}

// TestCRITICAL2_MultiValueReturnWithMessage verifies multi-value returns work with error messages
func TestCRITICAL2_MultiValueReturnWithMessage(t *testing.T) {
	input := `package main

func getConfig(path string) ([]byte, string, error) {
	return loadConfig(path)? "failed to load config"
}`

	p := New([]byte(input))
	result, _, err := p.Process()
	if err != nil {
		t.Fatalf("preprocessing failed: %v", err)
	}

	resultStr := string(result)

	// Verify correct expansion
	expectedPatterns := []string{
		"__tmp0, __tmp1, __err0 := loadConfig(path)", // Two temps for non-error values, err0 for error
		`fmt.Errorf("failed to load config: %w", __err0)`,
		"return __tmp0, __tmp1, nil",
		`return nil, "", `, // error path with two zero values (first is nil for []byte, second is "" for string)
	}

	for _, pattern := range expectedPatterns {
		if !strings.Contains(resultStr, pattern) {
			t.Errorf("expected pattern not found:\n%s\n\nActual output:\n%s", pattern, resultStr)
		}
	}
}

// TestCRITICAL1_MappingsBeforeImportsNotShifted verifies CRITICAL-1 fix:
// Source mappings for code BEFORE import block must NOT be shifted when imports are injected
func TestCRITICAL1_MappingsBeforeImportsNotShifted(t *testing.T) {
	// This test creates a scenario where:
	// - Package declaration is on line 1
	// - Type definition is on lines 3-5
	// - Error propagation is on line 8 (after type def)
	// When imports are injected, they should go after package declaration
	// The type definition should NOT have its mappings shifted
	// Only content AFTER imports should be shifted

	input := `package main

type Config struct {
	Path string
}

func load(path string) ([]byte, error) {
	let data = os.ReadFile(path)?
	return data, nil
}`

	p := New([]byte(input))
	result, sourceMap, err := p.Process()
	if err != nil {
		t.Fatalf("preprocessing failed: %v", err)
	}

	resultStr := string(result)

	// Verify import was injected
	if !strings.Contains(resultStr, `import "os"`) {
		t.Errorf("expected os import to be injected")
	}

	// Parse result to find import insertion line
	lines := strings.Split(resultStr, "\n")
	importInsertLine := -1
	for i, line := range lines {
		if strings.Contains(line, `import "os"`) {
			importInsertLine = i + 1 // Convert to 1-based
			break
		}
	}

	if importInsertLine == -1 {
		t.Fatalf("could not find import block in output")
	}

	t.Logf("Import block inserted at generated line %d", importInsertLine)
	t.Logf("Total mappings: %d", len(sourceMap.Mappings))

	// CRITICAL CHECK: Verify NO mappings for lines before import insertion
	// have been shifted. If the bug existed, these would be incorrectly offset.
	//
	// Expected behavior:
	// - Original line 1 (package main) → Generated line 1 (NOT shifted)
	// - Original line 3-5 (type Config) → Generated line 3-5 (NOT shifted)
	// - Original line 8 (error prop) → Generated lines ~11-17 (shifted by import block)

	for _, mapping := range sourceMap.Mappings {
		// For this test, we care about the error propagation on original line 8
		// which should be shifted to generated lines AFTER the import block
		if mapping.OriginalLine == 8 {
			if mapping.GeneratedLine < importInsertLine+2 {
				t.Errorf("Error propagation mapping on original line 8 maps to generated line %d, "+
					"but should be AFTER import block (line %d+)",
					mapping.GeneratedLine, importInsertLine)
			}
			// This is the content AFTER imports, shifting is expected and correct
			continue
		}

		// If we had mappings for content BEFORE imports (we don't generate these currently),
		// they should NOT be shifted. This checks the logic is correct.
		if mapping.GeneratedLine < importInsertLine && mapping.OriginalLine != mapping.GeneratedLine {
			t.Errorf("Mapping for content BEFORE imports was incorrectly shifted: "+
				"original line %d → generated line %d (should not be shifted)",
				mapping.OriginalLine, mapping.GeneratedLine)
		}
	}

	// Additional verification: Error propagation should produce 8 mappings
	// (1 expr_mapping + 7 error_prop) all pointing to original line 8
	errorPropMappings := 0
	for _, mapping := range sourceMap.Mappings {
		if mapping.OriginalLine == 8 {
			errorPropMappings++
		}
	}

	if errorPropMappings != 8 {
		t.Errorf("Expected 8 mappings for error propagation (1 expr + 7 error_prop), got %d", errorPropMappings)
		for i, m := range sourceMap.Mappings {
			t.Logf("Mapping %d: orig=%d gen=%d", i, m.OriginalLine, m.GeneratedLine)
		}
	}
}

// TestSourceMapOffsetBeforeImports verifies that source map offset adjustments
// are NOT applied to mappings before the import insertion line.
// This is the negative test for CRITICAL-1 fix (>= to > change).
func TestSourceMapOffsetBeforeImports(t *testing.T) {
	// Simulate the internal behavior of adjustMappingsForImports
	// Create mappings with GeneratedLine values before, at, and after importInsertionLine

	sourceMap := &SourceMap{
		Mappings: []Mapping{
			{OriginalLine: 1, OriginalColumn: 1, GeneratedLine: 1, GeneratedColumn: 1, Length: 7, Name: "package"},  // package main
			{OriginalLine: 3, OriginalColumn: 1, GeneratedLine: 2, GeneratedColumn: 1, Length: 4, Name: "type"},     // type Config (before imports will be inserted)
			{OriginalLine: 7, OriginalColumn: 1, GeneratedLine: 3, GeneratedColumn: 1, Length: 4, Name: "func"},     // func definition (after where imports will be inserted)
			{OriginalLine: 8, OriginalColumn: 1, GeneratedLine: 4, GeneratedColumn: 1, Length: 3, Name: "error_prop"}, // error propagation
		},
	}

	// Import will be inserted at line 2 (after package declaration)
	// We're adding 2 import lines
	importInsertionLine := 2
	numImportLines := 2

	// Call the internal adjustment function
	adjustMappingsForImports(sourceMap, numImportLines, importInsertionLine)

	// Verify results:
	// Mapping 0: GeneratedLine=1 (< 2) → should NOT shift (stay at 1)
	if sourceMap.Mappings[0].GeneratedLine != 1 {
		t.Errorf("Mapping at line 1 (< insertionLine %d) was incorrectly shifted to line %d",
			importInsertionLine, sourceMap.Mappings[0].GeneratedLine)
	}

	// Mapping 1: GeneratedLine=2 (= 2) → CRITICAL TEST: should NOT shift (stay at 2)
	// This tests the >= to > fix!
	if sourceMap.Mappings[1].GeneratedLine != 2 {
		t.Errorf("CRITICAL REGRESSION: Mapping at insertionLine %d was incorrectly shifted to line %d. "+
			"This indicates the >= bug has returned (should use > not >=)",
			importInsertionLine, sourceMap.Mappings[1].GeneratedLine)
	}

	// Mapping 2: GeneratedLine=3 (> 2) → should shift to 5 (3 + 2)
	if sourceMap.Mappings[2].GeneratedLine != 5 {
		t.Errorf("Mapping at line 3 (> insertionLine %d) should shift to 5, got %d",
			importInsertionLine, sourceMap.Mappings[2].GeneratedLine)
	}

	// Mapping 3: GeneratedLine=4 (> 2) → should shift to 6 (4 + 2)
	if sourceMap.Mappings[3].GeneratedLine != 6 {
		t.Errorf("Mapping at line 4 (> insertionLine %d) should shift to 6, got %d",
			importInsertionLine, sourceMap.Mappings[3].GeneratedLine)
	}

	t.Logf("✓ All mappings correctly handled:")
	t.Logf("  Line 1 (< %d): NOT shifted (correct)", importInsertionLine)
	t.Logf("  Line 2 (= %d): NOT shifted (CRITICAL FIX VERIFIED)", importInsertionLine)
	t.Logf("  Line 3 (> %d): Shifted to 5 (correct)", importInsertionLine)
	t.Logf("  Line 4 (> %d): Shifted to 6 (correct)", importInsertionLine)
}

// TestMultiValueReturnEdgeCases verifies edge cases for multi-value returns
// in error propagation beyond the existing error_prop_09_multi_value golden test.
// This adds comprehensive coverage for:
// - 2-value return: (T, error) - baseline case
// - 3-value return: (A, B, error) - verified by golden test
// - 4+ value return: (A, B, C, D, error) - extreme case
// - Mixed types: (string, int, []byte, error) - type variety
// - Correct number of temporaries (__tmp0, __tmp1, etc.)
// - All values returned in success path
func TestMultiValueReturnEdgeCases(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		shouldContain    []string
		shouldNotContain []string
		description      string
	}{
		{
			name: "2-value return (baseline case)",
			input: `package main

func readData(path string) ([]byte, error) {
	return os.ReadFile(path)?
}`,
			shouldContain: []string{
				"__tmp0, __err0 := os.ReadFile(path)",
				"return nil, __err0",
				"return __tmp0, nil",
			},
			shouldNotContain: []string{
				"__tmp1", // Should NOT have a second temp
			},
			description: "Standard Go (T, error) pattern",
		},
		{
			name: "3-value return (verified by golden test)",
			input: `package main

func parseUserData(input string) (string, string, int, error) {
	return extractUserFields(input)?
}

func extractUserFields(data string) (string, string, int, error) {
	return "name", "role", 42, nil
}`,
			shouldContain: []string{
				"__tmp0, __tmp1, __tmp2, __err0 := extractUserFields(input)",
				`return "", "", 0, __err0`,
				"return __tmp0, __tmp1, __tmp2, nil",
			},
			shouldNotContain: []string{
				"__tmp3", // Should NOT have a fourth temp
			},
			description: "Three non-error values plus error",
		},
		{
			name: "4-value return (extreme case)",
			input: `package main

func parseRecord(line string) (string, int, float64, bool, error) {
	return extractFields(line)?
}

func extractFields(line string) (string, int, float64, bool, error) {
	return "name", 42, 3.14, true, nil
}`,
			shouldContain: []string{
				"__tmp0, __tmp1, __tmp2, __tmp3, __err0 := extractFields(line)",
				`return "", 0, 0.0, false, __err0`,
				"return __tmp0, __tmp1, __tmp2, __tmp3, nil",
			},
			shouldNotContain: []string{
				"__tmp4", // Should NOT have a fifth temp
			},
			description: "Four non-error values plus error (extreme case)",
		},
		{
			name: "5-value return (very extreme case)",
			input: `package main

func parseComplexRecord(data string) (string, int, []byte, map[string]int, bool, error) {
	return extractComplexFields(data)?
}

func extractComplexFields(data string) (string, int, []byte, map[string]int, bool, error) {
	return "key", 100, []byte("data"), map[string]int{}, true, nil
}`,
			shouldContain: []string{
				"__tmp0, __tmp1, __tmp2, __tmp3, __tmp4, __err0 := extractComplexFields(data)",
				"return __tmp0, __tmp1, __tmp2, __tmp3, __tmp4, nil",
			},
			shouldNotContain: []string{
				"__tmp5", // Should NOT have a sixth temp
			},
			description: "Five non-error values plus error (very extreme)",
		},
		{
			name: "mixed types (string, int, []byte, error)",
			input: `package main

func readAndParse(path string) (string, int, []byte, error) {
	return processFile(path)?
}

func processFile(path string) (string, int, []byte, error) {
	return "result", 200, []byte("data"), nil
}`,
			shouldContain: []string{
				"__tmp0, __tmp1, __tmp2, __err0 := processFile(path)",
				`return "", 0, nil, __err0`,
				"return __tmp0, __tmp1, __tmp2, nil",
			},
			shouldNotContain: []string{
				"__tmp3", // Should NOT have a fourth temp
			},
			description: "Mixed types: string, int, []byte + error",
		},
		{
			name: "complex types (map, slice, struct pointer, error)",
			input: `package main

type Config struct {
	Name string
}

func loadConfig(path string) (map[string]string, []int, *Config, error) {
	return parseConfig(path)?
}

func parseConfig(path string) (map[string]string, []int, *Config, error) {
	return map[string]string{}, []int{}, &Config{}, nil
}`,
			shouldContain: []string{
				"__tmp0, __tmp1, __tmp2, __err0 := parseConfig(path)",
				"return nil, nil, nil, __err0",
				"return __tmp0, __tmp1, __tmp2, nil",
			},
			shouldNotContain: []string{
				"__tmp3", // Should NOT have a fourth temp
			},
			description: "Complex types: map, slice, struct pointer + error",
		},
		{
			name: "verify correct number of temporaries (3 non-error values)",
			input: `package main

func multi3(s string) (int, int, int, error) {
	return convert3(s)?
}

func convert3(s string) (int, int, int, error) {
	return 1, 2, 3, nil
}`,
			shouldContain: []string{
				"__tmp0, __tmp1, __tmp2, __err0",
				"return 0, 0, 0, __err0",
				"return __tmp0, __tmp1, __tmp2, nil",
			},
			shouldNotContain: []string{
				"__tmp3, __err0", // Should NOT have __tmp3
			},
			description: "Verify exactly 3 temps for 3 non-error values",
		},
		{
			name: "verify correct number of temporaries (4 non-error values)",
			input: `package main

func multi4(s string) (int, int, int, int, error) {
	return convert4(s)?
}

func convert4(s string) (int, int, int, int, error) {
	return 1, 2, 3, 4, nil
}`,
			shouldContain: []string{
				"__tmp0, __tmp1, __tmp2, __tmp3, __err0",
				"return 0, 0, 0, 0, __err0",
				"return __tmp0, __tmp1, __tmp2, __tmp3, nil",
			},
			shouldNotContain: []string{
				"__tmp4, __err0", // Should NOT have __tmp4
			},
			description: "Verify exactly 4 temps for 4 non-error values",
		},
		{
			name: "all values returned in success path (4 values)",
			input: `package main

func processData(input string) (string, int, bool, []byte, error) {
	return parse(input)?
}

func parse(input string) (string, int, bool, []byte, error) {
	return "name", 42, true, []byte("data"), nil
}`,
			shouldContain: []string{
				"__tmp0, __tmp1, __tmp2, __tmp3, __err0 := parse(input)",
				"return __tmp0, __tmp1, __tmp2, __tmp3, nil",
			},
			shouldNotContain: []string{
				"return __tmp0, nil", // WRONG: would drop values
				"return __tmp0, __tmp1, nil", // WRONG: would drop values
				"return __tmp0, __tmp1, __tmp2, nil", // WRONG: would drop __tmp3
			},
			description: "Verify all 4 non-error values returned in success path",
		},
		{
			name: "all zero values in error path (mixed types)",
			input: `package main

func getData() (string, int, bool, []byte, error) {
	return fetch()?
}

func fetch() (string, int, bool, []byte, error) {
	return "", 0, false, nil, nil
}`,
			shouldContain: []string{
				"__tmp0, __tmp1, __tmp2, __tmp3, __err0 := fetch()",
				`return "", 0, false, nil, __err0`,
			},
			shouldNotContain: []string{
				"return nil, __err0", // WRONG: only one zero value
				`return "", 0, __err0`, // WRONG: missing two zero values
			},
			description: "Verify all zero values in error path for mixed types",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New([]byte(tt.input))
			result, _, err := p.Process()
			if err != nil {
				t.Fatalf("preprocessing failed: %v", err)
			}

			resultStr := string(result)

			// Verify required patterns
			for _, pattern := range tt.shouldContain {
				if !strings.Contains(resultStr, pattern) {
					t.Errorf("%s\nExpected to find pattern:\n  %s\n\nActual output:\n%s",
						tt.description, pattern, resultStr)
				}
			}

			// Verify forbidden patterns
			for _, pattern := range tt.shouldNotContain {
				if strings.Contains(resultStr, pattern) {
					t.Errorf("%s\nShould NOT contain pattern:\n  %s\n\nActual output:\n%s",
						tt.description, pattern, resultStr)
				}
			}
		})
	}
}

// TestIMPORTANT1_UserDefinedFunctionsDontTriggerImports verifies IMPORTANT-1 fix:
// User-defined functions with stdlib names must NOT trigger import injection
func TestIMPORTANT1_UserDefinedFunctionsDontTriggerImports(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		expectedImports   []string
		unexpectedImports []string
	}{
		{
			name: "user-defined ReadFile (no qualifier)",
			input: `package main

func ReadFile(path string) error {
	return nil
}

func main() error {
	return ReadFile("/tmp/test")?
}`,
			expectedImports:   []string{}, // No imports should be added
			unexpectedImports: []string{"os"}, // os should NOT be imported
		},
		{
			name: "qualified os.ReadFile (with package qualifier)",
			input: `package main

func main() ([]byte, error) {
	let data = os.ReadFile("/tmp/test")?
	return data, nil
}`,
			expectedImports:   []string{"os"}, // os SHOULD be imported
			unexpectedImports: []string{},
		},
		{
			name: "multiple user-defined functions with stdlib names",
			input: `package main

func ReadFile(path string) error { return nil }
func Marshal(v any) error { return nil }
func Atoi(s string) error { return nil }

func main() error {
	let _ = ReadFile("/tmp")?
	let _ = Marshal("test")?
	return Atoi("42")?
}`,
			expectedImports:   []string{}, // No imports (all user-defined)
			unexpectedImports: []string{"os", "encoding/json", "strconv"},
		},
		{
			name: "mixed user-defined and qualified stdlib calls",
			input: `package main

func ReadFile(path string) error { return nil }

func process(path string) ([]byte, error) {
	let err = ReadFile(path)?
	let data = os.ReadFile(path)?
	let _ = strconv.Atoi("42")?
	return data, nil
}`,
			expectedImports:   []string{"os", "strconv"}, // Only qualified calls
			unexpectedImports: []string{"encoding/json"},
		},
		{
			name: "user-defined http.Get lookalike",
			input: `package main

type http struct{}

func (h http) Get(url string) error { return nil }

func main() error {
	let h = http{}
	return h.Get("https://example.com")?
}`,
			expectedImports:   []string{}, // Method call, not package.Function
			unexpectedImports: []string{"net/http"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New([]byte(tt.input))
			result, _, err := p.Process()
			if err != nil {
				t.Fatalf("preprocessing failed: %v", err)
			}

			resultStr := string(result)

			// Verify expected imports are present
			for _, expectedPkg := range tt.expectedImports {
				expectedImport := fmt.Sprintf(`"%s"`, expectedPkg)
				if !strings.Contains(resultStr, expectedImport) {
					t.Errorf("Expected import %q not found in output:\n%s", expectedPkg, resultStr)
				}
			}

			// Verify unexpected imports are NOT present
			for _, unexpectedPkg := range tt.unexpectedImports {
				unexpectedImport := fmt.Sprintf(`"%s"`, unexpectedPkg)
				if strings.Contains(resultStr, unexpectedImport) {
					t.Errorf("Unexpected import %q found in output (false positive):\n%s", unexpectedPkg, resultStr)
				}
			}

			// Additional check: If we expect NO imports, verify there's no import block at all
			if len(tt.expectedImports) == 0 {
				if strings.Contains(resultStr, "import") {
					t.Errorf("Expected NO imports, but found import block in output:\n%s", resultStr)
				}
			}
		})
	}
}

// TestConfigSingleValueReturnModeEnforcement verifies that the "single" mode for
// MultiValueReturnMode correctly enforces only single non-error returns.
func TestConfigSingleValueReturnModeEnforcement(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errorMsg    string
	}{
		{
			name: "multi-value return in single mode - expect error",
			input: `package main

func parseData() (int, string, error) {
	return fetchData()?
}
func fetchData() (int, string, error) {
	return 1, "test", nil
}`,
			expectError: true,
			errorMsg:    "multi-value error propagation not allowed in 'single' mode",
		},
		{
			name: "single-value return in single mode - no error",
			input: `package main

func parseData() (int, error) {
	return fetchData()?
}
func fetchData() (int, error) {
	return 1, nil
}`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			config.MultiValueReturnMode = "single" // Set mode to single

			p := NewWithConfig([]byte(tt.input), config)
			_, _, err := p.Process()

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected an error, but got none")
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain %q, but got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, but got: %v", err)
				}
			}
		})
	}
}

// TestUserFunctionShadowingNoImport verifies Issue #3 fix (IMPORTANT-1) at the ImportTracker level:
// User-defined functions with stdlib names must NOT trigger import injection.
// This test directly checks the ImportTracker.needed map to ensure no false positives.
func TestUserFunctionShadowingNoImport(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		shouldTrack       map[string]bool // Functions that SHOULD trigger imports
		shouldNotTrack    []string        // Package names that should NOT appear
	}{
		{
			name: "user function named ReadFile - no os import",
			input: `package main

func ReadFile(path string) ([]byte, error) {
	return []byte("mock"), nil
}

func main() ([]byte, error) {
	return ReadFile("/tmp/test")?
}`,
			shouldTrack:    map[string]bool{}, // No imports should be tracked
			shouldNotTrack: []string{"os"},
		},
		{
			name: "user function named Atoi - no strconv import",
			input: `package main

func Atoi(s string) (int, error) {
	return 42, nil
}

func parse(s string) (int, error) {
	return Atoi(s)?
}`,
			shouldTrack:    map[string]bool{}, // No imports should be tracked
			shouldNotTrack: []string{"strconv"},
		},
		{
			name: "qualified os.ReadFile call - SHOULD import os",
			input: `package main

func load(path string) ([]byte, error) {
	return os.ReadFile(path)?
}`,
			shouldTrack:    map[string]bool{"os.ReadFile": true}, // SHOULD track os import
			shouldNotTrack: []string{},
		},
		{
			name: "mixed user-defined and qualified stdlib",
			input: `package main

func ReadFile(path string) ([]byte, error) {
	return []byte("user"), nil
}

func Atoi(s string) (int, error) {
	return 0, nil
}

func process(path string, num string) ([]byte, error) {
	let _ = ReadFile(path)?
	let _ = Atoi(num)?
	let data = os.ReadFile(path)?
	let n = strconv.Atoi(num)?
	return data, nil
}`,
			shouldTrack: map[string]bool{
				"os.ReadFile":  true, // Qualified calls SHOULD track
				"strconv.Atoi": true,
			},
			shouldNotTrack: []string{}, // User-defined should NOT appear in tracker
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create processor directly to check internal state
			proc := NewErrorPropProcessorWithConfig(DefaultConfig())

			// Process the input
			_, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("preprocessing failed: %v", err)
			}

			// CRITICAL CHECK: Verify imports via GetNeededImports()
			neededImports := proc.GetNeededImports()

			// Create a map of needed imports for easier checking
			neededMap := make(map[string]bool)
			for _, imp := range neededImports {
				neededMap[imp] = true
			}

			// Check that expected packages are in the imports
			for funcName := range tt.shouldTrack {
				// funcName is like "os.ReadFile", we need to check if "os" is imported
				parts := strings.Split(funcName, ".")
				if len(parts) > 0 {
					expectedPkg := parts[0]
					// Map package names to import paths
					var expectedImport string
					switch expectedPkg {
					case "os":
						expectedImport = "os"
					case "strconv":
						expectedImport = "strconv"
					case "json":
						expectedImport = "encoding/json"
					case "http":
						expectedImport = "net/http"
					case "filepath":
						expectedImport = "path/filepath"
					default:
						expectedImport = expectedPkg
					}

					if !neededMap[expectedImport] {
						t.Errorf("Expected import %q for function call %q, but it wasn't in GetNeededImports()",
							expectedImport, funcName)
						t.Logf("Needed imports: %v", neededImports)
					}
				}
			}

			// Check that user-defined functions did NOT trigger imports
			for _, pkgName := range tt.shouldNotTrack {
				// Map package name to import path
				var importPath string
				switch pkgName {
				case "os":
					importPath = "os"
				case "strconv":
					importPath = "strconv"
				case "json":
					importPath = "encoding/json"
				default:
					importPath = pkgName
				}

				if neededMap[importPath] {
					t.Errorf("Package %q should NOT be imported (user-defined function with same name), "+
						"but found %q in GetNeededImports()", pkgName, importPath)
					t.Logf("All needed imports: %v", neededImports)
				}
			}
		})
	}
}
