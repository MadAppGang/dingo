package preprocessor

import (
	"strings"
	"testing"
)

// TestUnqualifiedTransform_Basic tests basic unqualified call transformation
func TestUnqualifiedTransform_Basic(t *testing.T) {
	cache := NewFunctionExclusionCache("/tmp/test")
	// Empty cache = no local functions
	processor := NewUnqualifiedImportProcessor(cache)

	source := []byte(`package main

func main() {
	data, err := ReadFile("test.txt")
	if err != nil {
		Printf("error: %v", err)
	}
}
`)

	result, mappings, err := processor.Process(source)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Check transformations
	resultStr := string(result)
	if !strings.Contains(resultStr, "os.ReadFile") {
		t.Errorf("Expected 'os.ReadFile', got: %s", resultStr)
	}
	if !strings.Contains(resultStr, "fmt.Printf") {
		t.Errorf("Expected 'fmt.Printf', got: %s", resultStr)
	}

	// Check imports
	imports := processor.GetNeededImports()
	if len(imports) != 2 {
		t.Errorf("Expected 2 imports, got %d", len(imports))
	}

	hasOs := false
	hasFmt := false
	for _, imp := range imports {
		if imp == "os" {
			hasOs = true
		}
		if imp == "fmt" {
			hasFmt = true
		}
	}

	if !hasOs {
		t.Errorf("Expected 'os' import")
	}
	if !hasFmt {
		t.Errorf("Expected 'fmt' import")
	}

	// Check mappings
	if len(mappings) != 2 {
		t.Errorf("Expected 2 mappings, got %d", len(mappings))
	}
}

// TestUnqualifiedTransform_LocalFunction tests that local functions are NOT transformed
func TestUnqualifiedTransform_LocalFunction(t *testing.T) {
	cache := NewFunctionExclusionCache("/tmp/test")

	// Simulate scanning that found local ReadFile
	cache.localFunctions = map[string]bool{
		"ReadFile": true,
	}

	processor := NewUnqualifiedImportProcessor(cache)

	source := []byte(`package main

func ReadFile(path string) ([]byte, error) {
	// User-defined ReadFile
	return nil, nil
}

func main() {
	data, err := ReadFile("test.txt")
}
`)

	result, _, err := processor.Process(source)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	resultStr := string(result)

	// Should NOT transform to os.ReadFile
	if strings.Contains(resultStr, "os.ReadFile") {
		t.Errorf("Should not transform local ReadFile, got: %s", resultStr)
	}

	// Should remain as ReadFile
	if !strings.Contains(resultStr, "ReadFile(\"test.txt\")") {
		t.Errorf("Expected unqualified 'ReadFile', got: %s", resultStr)
	}

	// No imports should be added
	imports := processor.GetNeededImports()
	if len(imports) != 0 {
		t.Errorf("Expected 0 imports for local function, got %d", len(imports))
	}
}

// TestUnqualifiedTransform_Ambiguous tests error handling for ambiguous functions
func TestUnqualifiedTransform_Ambiguous(t *testing.T) {
	cache := NewFunctionExclusionCache("/tmp/test")
	processor := NewUnqualifiedImportProcessor(cache)

	// "Open" is ambiguous (os.Open, net.Open)
	source := []byte(`package main

func main() {
	f, err := Open("file.txt")
}
`)

	_, _, err := processor.Process(source)
	if err == nil {
		t.Fatalf("Expected error for ambiguous function 'Open'")
	}

	// Check error message
	errMsg := err.Error()
	if !strings.Contains(errMsg, "ambiguous") {
		t.Errorf("Expected 'ambiguous' in error, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "Open") {
		t.Errorf("Expected 'Open' in error, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "os") || !strings.Contains(errMsg, "net") {
		t.Errorf("Expected package suggestions in error, got: %s", errMsg)
	}
}

// TestUnqualifiedTransform_MultipleImports tests multiple stdlib calls
func TestUnqualifiedTransform_MultipleImports(t *testing.T) {
	cache := NewFunctionExclusionCache("/tmp/test")
	processor := NewUnqualifiedImportProcessor(cache)

	source := []byte(`package main

func main() {
	data := ReadFile("file.txt")
	num := Atoi("42")
	now := Now()
	Printf("Time: %v", now)
}
`)

	result, _, err := processor.Process(source)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	resultStr := string(result)

	// Check all transformations
	expected := map[string]string{
		"os.ReadFile":    "ReadFile",
		"strconv.Atoi":   "Atoi",
		"time.Now":       "Now",
		"fmt.Printf":     "Printf",
	}

	for qualified, unqualified := range expected {
		if !strings.Contains(resultStr, qualified) {
			t.Errorf("Expected '%s' for %s, got: %s", qualified, unqualified, resultStr)
		}
	}

	// Check imports
	imports := processor.GetNeededImports()
	if len(imports) != 4 {
		t.Errorf("Expected 4 imports, got %d: %v", len(imports), imports)
	}

	expectedImports := map[string]bool{
		"os":      false,
		"strconv": false,
		"time":    false,
		"fmt":     false,
	}

	for _, imp := range imports {
		if _, exists := expectedImports[imp]; exists {
			expectedImports[imp] = true
		}
	}

	for pkg, found := range expectedImports {
		if !found {
			t.Errorf("Expected import '%s' not found", pkg)
		}
	}
}

// TestUnqualifiedTransform_AlreadyQualified tests that already-qualified calls are skipped
func TestUnqualifiedTransform_AlreadyQualified(t *testing.T) {
	cache := NewFunctionExclusionCache("/tmp/test")
	processor := NewUnqualifiedImportProcessor(cache)

	source := []byte(`package main

import "os"

func main() {
	data := os.ReadFile("file.txt")
}
`)

	result, _, err := processor.Process(source)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	resultStr := string(result)

	// Should remain as os.ReadFile (not os.os.ReadFile)
	if !strings.Contains(resultStr, "os.ReadFile") {
		t.Errorf("Expected 'os.ReadFile' to remain, got: %s", resultStr)
	}

	// Should not have duplicate qualification
	if strings.Contains(resultStr, "os.os.ReadFile") {
		t.Errorf("Should not have duplicate qualification, got: %s", resultStr)
	}

	// No new imports needed (already qualified)
	imports := processor.GetNeededImports()
	if len(imports) != 0 {
		t.Errorf("Expected 0 new imports for qualified call, got %d", len(imports))
	}
}

// TestUnqualifiedTransform_MixedQualifiedUnqualified tests mix of qualified and unqualified
func TestUnqualifiedTransform_MixedQualifiedUnqualified(t *testing.T) {
	cache := NewFunctionExclusionCache("/tmp/test")
	processor := NewUnqualifiedImportProcessor(cache)

	source := []byte(`package main

func main() {
	// Already qualified
	data1 := os.ReadFile("file1.txt")

	// Unqualified
	data2 := ReadFile("file2.txt")

	// Already qualified
	fmt.Printf("data: %v", data1)

	// Unqualified
	Println("hello")
}
`)

	result, _, err := processor.Process(source)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	resultStr := string(result)

	// Should have both qualified versions
	if !strings.Contains(resultStr, "os.ReadFile(\"file1.txt\")") {
		t.Errorf("Expected 'os.ReadFile(\"file1.txt\")' (already qualified), got: %s", resultStr)
	}
	if !strings.Contains(resultStr, "os.ReadFile(\"file2.txt\")") {
		t.Errorf("Expected 'os.ReadFile(\"file2.txt\")' (transformed), got: %s", resultStr)
	}
	if !strings.Contains(resultStr, "fmt.Printf") {
		t.Errorf("Expected 'fmt.Printf' (already qualified), got: %s", resultStr)
	}
	if !strings.Contains(resultStr, "fmt.Println") {
		t.Errorf("Expected 'fmt.Println' (transformed), got: %s", resultStr)
	}

	// Should only need os and fmt imports (not duplicates)
	imports := processor.GetNeededImports()
	if len(imports) != 2 {
		t.Errorf("Expected 2 imports, got %d: %v", len(imports), imports)
	}
}

// TestUnqualifiedTransform_NoStdlib tests source with no stdlib calls
func TestUnqualifiedTransform_NoStdlib(t *testing.T) {
	cache := NewFunctionExclusionCache("/tmp/test")
	processor := NewUnqualifiedImportProcessor(cache)

	source := []byte(`package main

func myFunc() {
	x := 42
	y := x + 1
}
`)

	result, mappings, err := processor.Process(source)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Source should remain unchanged
	if string(result) != string(source) {
		t.Errorf("Source should remain unchanged, got: %s", string(result))
	}

	// No mappings
	if len(mappings) != 0 {
		t.Errorf("Expected 0 mappings, got %d", len(mappings))
	}

	// No imports
	imports := processor.GetNeededImports()
	if len(imports) != 0 {
		t.Errorf("Expected 0 imports, got %d", len(imports))
	}
}

// TestUnqualifiedTransform_OnlyLocalFunctions tests source with only local functions
func TestUnqualifiedTransform_OnlyLocalFunctions(t *testing.T) {
	cache := NewFunctionExclusionCache("/tmp/test")
	cache.localFunctions = map[string]bool{
		"MyFunc":   true,
		"DoStuff":  true,
		"ReadFile": true, // Shadows os.ReadFile
	}

	processor := NewUnqualifiedImportProcessor(cache)

	source := []byte(`package main

func main() {
	MyFunc()
	DoStuff()
	ReadFile("test.txt")
}
`)

	result, _, err := processor.Process(source)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Should remain unchanged (all local)
	if string(result) != string(source) {
		t.Errorf("Source should remain unchanged for local functions, got: %s", string(result))
	}

	// No imports
	imports := processor.GetNeededImports()
	if len(imports) != 0 {
		t.Errorf("Expected 0 imports, got %d", len(imports))
	}
}

// TestCalculatePosition tests line/column calculation
func TestCalculatePosition(t *testing.T) {
	tests := []struct {
		name   string
		source string
		offset int
		line   int
		col    int
	}{
		{
			name:   "first character",
			source: "hello",
			offset: 0,
			line:   1,
			col:    1,
		},
		{
			name:   "second line first char",
			source: "hello\nworld",
			offset: 6,
			line:   2,
			col:    1,
		},
		{
			name:   "second line middle",
			source: "hello\nworld",
			offset: 9,
			line:   2,
			col:    4,
		},
		{
			name:   "multiline",
			source: "package main\n\nfunc main() {\n\tx := 42\n}",
			offset: 14, // 'f' in func
			line:   3,
			col:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line, col := calculatePosition([]byte(tt.source), tt.offset)
			if line != tt.line || col != tt.col {
				t.Errorf("calculatePosition(%q, %d) = (%d, %d), want (%d, %d)",
					tt.source, tt.offset, line, col, tt.line, tt.col)
			}
		})
	}
}

// TestIsAlreadyQualified tests the qualified detection logic
func TestIsAlreadyQualified(t *testing.T) {
	cache := NewFunctionExclusionCache("/tmp/test")
	processor := NewUnqualifiedImportProcessor(cache)

	tests := []struct {
		name     string
		source   string
		funcPos  int
		expected bool
	}{
		{
			name:     "qualified",
			source:   "os.ReadFile(",
			funcPos:  3, // Position of 'R' in ReadFile
			expected: true,
		},
		{
			name:     "unqualified",
			source:   "ReadFile(",
			funcPos:  0, // Position of 'R' in ReadFile
			expected: false,
		},
		{
			name:     "qualified with spaces",
			source:   "os . ReadFile(",
			funcPos:  5, // Position of 'R' in ReadFile
			expected: true,
		},
		{
			name:     "start of file",
			source:   "ReadFile(",
			funcPos:  0,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.isAlreadyQualified([]byte(tt.source), tt.funcPos)
			if result != tt.expected {
				t.Errorf("isAlreadyQualified(%q, %d) = %v, want %v",
					tt.source, tt.funcPos, result, tt.expected)
			}
		})
	}
}
