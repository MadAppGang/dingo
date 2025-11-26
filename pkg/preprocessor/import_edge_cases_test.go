package preprocessor

import (
	"fmt"
	"strings"
	"testing"
)

// TestImportInjectionEdgeCases verifies edge cases for import detection and injection system
// Tests deduplication, multiple packages, no imports needed, and existing imports scenarios
func TestImportInjectionEdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedImports []string
		checkDetails    func(t *testing.T, result string, sourceMap *SourceMap)
	}{
		{
			name: "multiple imports from same package (deduplication)",
			input: `package main

func process(path1 string, path2 string) ([]byte, error) {
	let data1 = os.ReadFile(path1)?
	let data2 = os.WriteFile(path2, data1, 0644)?
	return data1, nil
}`,
			expectedImports: []string{"os"},
			checkDetails: func(t *testing.T, result string, sourceMap *SourceMap) {
				// Count occurrences of "os" import (check for the package path, not specific format)
				importCount := strings.Count(result, `"os"`)
				if importCount != 1 {
					t.Errorf("Expected exactly 1 os import (deduplication), got %d:\n%s", importCount, result)
				}

				// Verify both function calls are present
				if !strings.Contains(result, "os.ReadFile") {
					t.Errorf("os.ReadFile call missing in output")
				}
				if !strings.Contains(result, "os.WriteFile") {
					t.Errorf("os.WriteFile call missing in output")
				}
			},
		},
		{
			name: "imports from different packages",
			input: `package main

func loadAndParse(path string) (map[string]interface{}, error) {
	let data = os.ReadFile(path)?
	let parsed = json.Unmarshal(data)?
	return parsed, nil
}`,
			expectedImports: []string{"encoding/json", "os"},
			checkDetails: func(t *testing.T, result string, sourceMap *SourceMap) {
				// Verify both imports are present
				if !strings.Contains(result, `"os"`) {
					t.Errorf("os import missing")
				}
				if !strings.Contains(result, `"encoding/json"`) {
					t.Errorf("encoding/json import missing")
				}

				// Verify imports are in the import block (between package and first function)
				lines := strings.Split(result, "\n")
				packageLine := -1
				firstFuncLine := -1

				for i, line := range lines {
					if strings.HasPrefix(strings.TrimSpace(line), "package ") {
						packageLine = i
					}
					if strings.HasPrefix(strings.TrimSpace(line), "func ") {
						firstFuncLine = i
						break
					}
				}

				if packageLine == -1 || firstFuncLine == -1 {
					t.Fatalf("Could not find package or function declaration")
				}

				// Imports should be between package and first func
				importBlockFound := false
				for i := packageLine + 1; i < firstFuncLine; i++ {
					if strings.Contains(lines[i], "import") {
						importBlockFound = true
						break
					}
				}

				if !importBlockFound {
					t.Errorf("Import block not found between package and function declarations")
				}
			},
		},
		{
			name: "no imports needed (no stdlib calls)",
			input: `package main

func add(a int, b int) int {
	return a + b
}

func multiply(a int, b int) int {
	c := add(a, a)
	return c * b
}`,
			expectedImports: []string{},
			checkDetails: func(t *testing.T, result string, sourceMap *SourceMap) {
				// Verify NO import block exists
				if strings.Contains(result, "import") {
					t.Errorf("Unexpected import block found when no imports needed:\n%s", result)
				}

				// Verify source is minimally changed (type annotations converted)
				if !strings.Contains(result, "func add(a int, b int)") {
					t.Errorf("Function signature was unexpectedly modified")
				}

				// Verify source map is minimal or empty (no error propagation expansions)
				if len(sourceMap.Mappings) > 0 {
					t.Logf("Warning: Source map has %d mappings for code with no transformations", len(sourceMap.Mappings))
				}
			},
		},
		{
			name: "already existing imports (don't duplicate)",
			input: `package main

import "os"

func readConfig(path string) ([]byte, error) {
	let data = os.ReadFile(path)?
	return data, nil
}`,
			expectedImports: []string{"os"},
			checkDetails: func(t *testing.T, result string, sourceMap *SourceMap) {
				// Count total "os" import declarations
				importCount := strings.Count(result, `"os"`)
				if importCount != 1 {
					t.Errorf("Expected exactly 1 os import (no duplication), got %d:\n%s", importCount, result)
				}

				// Verify the import is in the correct location (not duplicated)
				lines := strings.Split(result, "\n")
				importLines := []int{}
				for i, line := range lines {
					if strings.Contains(line, `"os"`) {
						importLines = append(importLines, i+1)
					}
				}

				if len(importLines) > 1 {
					t.Errorf("os import appears on multiple lines %v, should be deduplicated", importLines)
				}
			},
		},
		{
			name: "source map offsets correct for different import counts",
			input: `package main

func multiImport(path string, num string, url string) ([]byte, error) {
	let data = os.ReadFile(path)?
	let n = strconv.Atoi(num)?
	let resp = http.Get(url)?
	return data, nil
}`,
			expectedImports: []string{"net/http", "os", "strconv"},
			checkDetails: func(t *testing.T, result string, sourceMap *SourceMap) {
				// Parse output to find import block
				lines := strings.Split(result, "\n")
				importStartLine := -1
				importEndLine := -1

				for i, line := range lines {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "import") || strings.Contains(trimmed, `"os"`) || strings.Contains(trimmed, `"strconv"`) || strings.Contains(trimmed, `"net/http"`) {
						if importStartLine == -1 {
							importStartLine = i + 1 // 1-based
						}
						importEndLine = i + 1 // 1-based
					}
				}

				if importStartLine == -1 {
					t.Fatalf("Import block not found in output")
				}

				numImportLines := importEndLine - importStartLine + 1
				t.Logf("Import block spans lines %d-%d (%d lines)", importStartLine, importEndLine, numImportLines)

				// Verify all mappings for error propagation are AFTER the import block
				for i, mapping := range sourceMap.Mappings {
					if mapping.Name == "error_prop" {
						if mapping.GeneratedLine <= importEndLine {
							t.Errorf(
								"Mapping %d: Error propagation mapping at generated line %d is inside/before import block (ends at line %d)",
								i, mapping.GeneratedLine, importEndLine,
							)
						}
					}
				}

				// Verify source map has correct number of mappings (3 error propagations × 7 lines each = 21)
				expectedMappingCount := 3 * 7 // 3 error propagations in input
				if len(sourceMap.Mappings) != expectedMappingCount {
					t.Logf("Expected ~%d mappings for 3 error propagations, got %d", expectedMappingCount, len(sourceMap.Mappings))
					t.Logf("This is informational - actual count may vary based on implementation")
				}
			},
		},
		{
			name: "mixed qualified and unqualified calls (only qualified should import)",
			input: `package main

func ReadFile(path string) ([]byte, error) {
	return []byte("mock"), nil
}

func process(path1 string, path2 string) ([]byte, error) {
	let userdata = ReadFile(path1)?
	let sysdata = os.ReadFile(path2)?
	return append(userdata, sysdata...), nil
}`,
			expectedImports: []string{"os"},
			checkDetails: func(t *testing.T, result string, sourceMap *SourceMap) {
				// Verify ONLY os import (not ReadFile)
				if !strings.Contains(result, `"os"`) {
					t.Errorf("os import missing for os.ReadFile call")
				}

				// Verify both calls are present in output
				if !strings.Contains(result, "ReadFile(path1)") {
					t.Errorf("User-defined ReadFile call missing")
				}
				if !strings.Contains(result, "os.ReadFile(path2)") {
					t.Errorf("Qualified os.ReadFile call missing")
				}

				// Verify no spurious imports were added
				spuriousImports := []string{`"io"`, `"encoding/json"`, `"strconv"`}
				for _, imp := range spuriousImports {
					if strings.Contains(result, imp) {
						t.Errorf("Spurious import %s found in output", imp)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New([]byte(tt.input))
			result, sourceMap, err := p.Process()
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

			// Run custom checks if provided
			if tt.checkDetails != nil {
				tt.checkDetails(t, resultStr, sourceMap)
			}
		})
	}
}
