package errors

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewEnhancedError(t *testing.T) {
	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.dingo")

	content := `package main

func test() {
    x := 42
    y := x + 1
    return y
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Parse to get positions
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, testFile, content, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Create error at line 4 (x := 42)
	pos := f.Pos() + token.Pos(len("package main\n\nfunc test() {\n    "))

	enhanced := NewEnhancedError(fset, pos, "Non-exhaustive match")

	// Verify basic fields
	if enhanced.Message != "Non-exhaustive match" {
		t.Errorf("Expected message 'Non-exhaustive match', got %q", enhanced.Message)
	}

	if enhanced.Line != 4 {
		t.Errorf("Expected line 4, got %d", enhanced.Line)
	}

	// Verify source lines extracted
	if len(enhanced.SourceLines) == 0 {
		t.Error("Expected source lines to be extracted")
	}

	// Verify highlight line set
	if enhanced.HighlightLine < 0 || enhanced.HighlightLine >= len(enhanced.SourceLines) {
		t.Errorf("Invalid highlight line %d (total lines: %d)", enhanced.HighlightLine, len(enhanced.SourceLines))
	}
}

func TestEnhancedErrorFormat(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "example.dingo")

	content := `result := fetchData()
if result != nil {
    x := result * 2
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()
	file := fset.AddFile(testFile, 1, len(content))

	// Position at "if" keyword (line 2)
	pos := file.Pos(len("result := fetchData()\n"))
	err := NewEnhancedError(fset, pos, "Non-exhaustive match")
	err.Length = 2 // "if"
	err.Annotation = "Missing pattern: Err(_)"
	err.Suggestion = "Add Err case"
	err.MissingItems = []string{"Err(_)"}

	formatted := err.Format()

	// Verify output contains expected elements
	expected := []string{
		"Error: Non-exhaustive match",
		"example.dingo:",
		"^^",
		"Missing pattern: Err(_)",
		"Suggestion: Add Err case",
	}

	for _, exp := range expected {
		if !strings.Contains(formatted, exp) {
			t.Errorf("Expected formatted error to contain %q\nGot:\n%s", exp, formatted)
		}
	}
}

func TestSourceLineExtraction(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "multiline.dingo")

	content := `line 1
line 2
line 3
line 4
line 5
line 6
line 7
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		targetLine    int
		contextLines  int
		expectedLines []string
		expectedIdx   int
	}{
		{
			name:          "middle line with 2 context",
			targetLine:    4,
			contextLines:  2,
			expectedLines: []string{"line 2", "line 3", "line 4", "line 5", "line 6"},
			expectedIdx:   2,
		},
		{
			name:          "first line with 2 context",
			targetLine:    1,
			contextLines:  2,
			expectedLines: []string{"line 1", "line 2", "line 3"},
			expectedIdx:   0,
		},
		{
			name:          "last line with 2 context",
			targetLine:    7,
			contextLines:  2,
			expectedLines: []string{"line 5", "line 6", "line 7"},
			expectedIdx:   2,
		},
		{
			name:          "no context",
			targetLine:    4,
			contextLines:  0,
			expectedLines: []string{"line 4"},
			expectedIdx:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear cache to ensure fresh read
			ClearCache()

			lines, idx, err := extractSourceLines(testFile, tt.targetLine, tt.contextLines)
			if err != nil {
				t.Fatalf("extractSourceLines failed: %v", err)
			}

			if len(lines) != len(tt.expectedLines) {
				t.Errorf("Expected %d lines, got %d", len(tt.expectedLines), len(lines))
			}

			for i, expected := range tt.expectedLines {
				if i >= len(lines) {
					break
				}
				if lines[i] != expected {
					t.Errorf("Line %d: expected %q, got %q", i, expected, lines[i])
				}
			}

			if idx != tt.expectedIdx {
				t.Errorf("Expected highlight index %d, got %d", tt.expectedIdx, idx)
			}
		})
	}
}

func TestSourceCaching(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "cache.dingo")

	content := "line 1\nline 2\nline 3"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Clear cache
	ClearCache()

	// First read - should cache
	lines1, _, _ := extractSourceLines(testFile, 2, 1)

	// Second read - should use cache
	lines2, _, _ := extractSourceLines(testFile, 2, 1)

	// Verify same result
	if len(lines1) != len(lines2) {
		t.Errorf("Cache returned different number of lines: %d vs %d", len(lines1), len(lines2))
	}

	for i := range lines1 {
		if lines1[i] != lines2[i] {
			t.Errorf("Cache returned different line %d: %q vs %q", i, lines1[i], lines2[i])
		}
	}

	// Verify cache actually working by checking sourceCacheMu
	sourceCacheMu.RLock()
	_, cached := sourceCache[testFile]
	sourceCacheMu.RUnlock()

	if !cached {
		t.Error("Expected file to be cached")
	}
}

func TestCaretPositioning(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "caret.dingo")

	content := `    match value {
        Ok(x) => x
    }
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()
	file := fset.AddFile(testFile, 1, len(content))

	// Position at "match" (after 4 spaces)
	pos := file.Pos(4)

	err := NewEnhancedError(fset, pos, "Test error")
	err.Length = 5 // "match"

	formatted := err.Format()

	// Should have 4 spaces + 5 carets
	expectedCaret := "    ^^^^^"
	if !strings.Contains(formatted, expectedCaret) {
		t.Errorf("Expected caret line %q\nGot:\n%s", expectedCaret, formatted)
	}
}

func TestUTF8Handling(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "utf8.dingo")

	// Content with multi-byte UTF-8 characters
	content := `    let emoji = "😀"
    match result {
        Ok(x) => x
    }
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()
	file := fset.AddFile(testFile, 1, len(content))

	// Position at line 2, after spaces
	pos := file.Pos(len("    let emoji = \"😀\"\n    "))

	err := NewEnhancedError(fset, pos, "Test UTF-8")
	formatted := err.Format()

	// Should handle UTF-8 correctly (no panic, reasonable output)
	if !strings.Contains(formatted, "Test UTF-8") {
		t.Errorf("UTF-8 handling failed:\n%s", formatted)
	}
}

func TestInvalidPosition(t *testing.T) {
	fset := token.NewFileSet()

	// Invalid position
	err := NewEnhancedError(fset, token.NoPos, "Invalid position test")

	if err.Filename != "unknown" {
		t.Errorf("Expected filename 'unknown', got %q", err.Filename)
	}

	if err.Line != 0 {
		t.Errorf("Expected line 0, got %d", err.Line)
	}

	// Should not panic
	formatted := err.Format()
	if !strings.Contains(formatted, "Invalid position test") {
		t.Error("Expected message in formatted output")
	}
}

func TestGracefulFallback(t *testing.T) {
	// Non-existent file
	fset := token.NewFileSet()
	file := fset.AddFile("/nonexistent/file.dingo", 1, 100)
	pos := file.Pos(10)

	err := NewEnhancedError(fset, pos, "File not found")

	// Should not panic, should return empty source lines
	if err.SourceLines != nil && len(err.SourceLines) > 0 {
		t.Error("Expected empty source lines for non-existent file")
	}

	// Should still format without panic
	formatted := err.Format()
	if !strings.Contains(formatted, "File not found") {
		t.Error("Expected message in formatted output")
	}
}

func TestEnhancedErrorSpan(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "span.dingo")

	content := "match result { Ok(x) => x }"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()
	file := fset.AddFile(testFile, 1, len(content))

	// Span from "match" to "result"
	startPos := file.Pos(0)
	endPos := file.Pos(12) // "match result"

	err := NewEnhancedErrorSpan(fset, startPos, endPos, "Test span")

	// Should calculate length
	if err.Length < 10 {
		t.Errorf("Expected span length >= 10, got %d", err.Length)
	}

	formatted := err.Format()
	if !strings.Contains(formatted, strings.Repeat("^", err.Length)) {
		t.Errorf("Expected %d carets in output:\n%s", err.Length, formatted)
	}
}

func TestWithAnnotation(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.dingo")
	content := "x := 42\ny := x + 1\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()
	file := fset.AddFile(testFile, 1, len(content))
	pos := file.Pos(10)

	err := NewEnhancedError(fset, pos, "Test message")
	_ = err.WithAnnotation("Custom annotation")

	if err.Annotation != "Custom annotation" {
		t.Errorf("Expected annotation 'Custom annotation', got %q", err.Annotation)
	}

	formatted := err.Format()
	if !strings.Contains(formatted, "Custom annotation") {
		t.Error("Formatted output should contain annotation")
	}
}

func TestWithSuggestion(t *testing.T) {
	fset := token.NewFileSet()
	file := fset.AddFile("test.dingo", 1, 100)
	pos := file.Pos(10)

	err := NewEnhancedError(fset, pos, "Test message")
	_ = err.WithSuggestion("Try this fix")

	if err.Suggestion != "Try this fix" {
		t.Errorf("Expected suggestion 'Try this fix', got %q", err.Suggestion)
	}

	formatted := err.Format()
	if !strings.Contains(formatted, "Suggestion: Try this fix") {
		t.Error("Formatted output should contain suggestion")
	}
}

func TestWithMissingItems(t *testing.T) {
	fset := token.NewFileSet()
	file := fset.AddFile("test.dingo", 1, 100)
	pos := file.Pos(10)

	err := NewEnhancedError(fset, pos, "Non-exhaustive match")
	_ = err.WithMissingItems([]string{"Err(_)", "None"})

	if len(err.MissingItems) != 2 {
		t.Errorf("Expected 2 missing items, got %d", len(err.MissingItems))
	}

	formatted := err.Format()
	if !strings.Contains(formatted, "Err(_)") || !strings.Contains(formatted, "None") {
		t.Error("Formatted output should contain missing items")
	}
}
