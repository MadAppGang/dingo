package errors

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnippetBuilder(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "snippet.dingo")

	content := `package main
func test() {
    x := 42
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, testFile, content, 0)
	if err != nil {
		t.Fatal(err)
	}

	pos := f.Pos()

	// Test snippet builder with chaining
	snippet := NewSnippet(fset, pos, "Test error").
		Annotate("Custom annotation").
		Suggest("Try this fix").
		MissingPatterns([]string{"Err(_)"})

	finalErr := snippet.Build()
	enhanced, ok := finalErr.(*EnhancedError)
	if !ok {
		t.Fatal("Expected EnhancedError")
	}

	if enhanced.Annotation != "Missing pattern: Err(_)" {
		t.Errorf("Expected annotation about missing pattern, got %q", enhanced.Annotation)
	}

	if enhanced.Suggestion != "Try this fix" {
		t.Errorf("Expected suggestion 'Try this fix', got %q", enhanced.Suggestion)
	}

	if len(enhanced.MissingItems) != 1 || enhanced.MissingItems[0] != "Err(_)" {
		t.Errorf("Expected missing items [Err(_)], got %v", enhanced.MissingItems)
	}
}

func TestExhaustivenessError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "exhaustive.dingo")

	content := `match result {
    Ok(x) => x
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()
	file := fset.AddFile(testFile, 1, len(content))
	pos := file.Pos(0)

	err := ExhaustivenessError(
		fset,
		pos,
		"result",
		[]string{"Err(_)"},
		[]string{"Ok(x)"},
	)

	enhanced, ok := err.(*EnhancedError)
	if !ok {
		t.Fatal("Expected EnhancedError")
	}

	// Verify message
	if enhanced.Message != "Non-exhaustive match" {
		t.Errorf("Expected 'Non-exhaustive match', got %q", enhanced.Message)
	}

	// Verify annotation
	if !strings.Contains(enhanced.Annotation, "Missing pattern: Err(_)") {
		t.Errorf("Expected annotation about Err(_), got %q", enhanced.Annotation)
	}

	// Verify suggestion structure
	suggestion := enhanced.Suggestion
	expectedParts := []string{
		"Add pattern to handle all cases",
		"match result",
		"Ok(x) => ...",
		"Err(_) => ...  // Add this",
	}

	for _, part := range expectedParts {
		if !strings.Contains(suggestion, part) {
			t.Errorf("Expected suggestion to contain %q\nGot:\n%s", part, suggestion)
		}
	}
}

func TestTupleArityError(t *testing.T) {
	fset := token.NewFileSet()
	file := fset.AddFile("test.dingo", 1, 100)
	pos := file.Pos(10)

	err := TupleArityError(fset, pos, 3, 2)

	enhanced, ok := err.(*EnhancedError)
	if !ok {
		t.Fatal("Expected EnhancedError")
	}

	if !strings.Contains(enhanced.Message, "expected 3 elements, got 2") {
		t.Errorf("Expected arity mismatch message, got %q", enhanced.Message)
	}

	if enhanced.Annotation != "Inconsistent tuple size" {
		t.Errorf("Expected annotation 'Inconsistent tuple size', got %q", enhanced.Annotation)
	}

	if !strings.Contains(enhanced.Suggestion, "3 elements") {
		t.Errorf("Expected suggestion to mention 3 elements, got %q", enhanced.Suggestion)
	}
}

func TestTupleLimitError(t *testing.T) {
	fset := token.NewFileSet()
	file := fset.AddFile("test.dingo", 1, 100)
	pos := file.Pos(10)

	err := TupleLimitError(fset, pos, 8, 6)

	enhanced, ok := err.(*EnhancedError)
	if !ok {
		t.Fatal("Expected EnhancedError")
	}

	if !strings.Contains(enhanced.Message, "limited to 6 elements (found 8)") {
		t.Errorf("Expected limit message, got %q", enhanced.Message)
	}

	if !strings.Contains(enhanced.Annotation, "8 > 6") {
		t.Errorf("Expected annotation with comparison, got %q", enhanced.Annotation)
	}

	if !strings.Contains(enhanced.Suggestion, "nested match") {
		t.Errorf("Expected suggestion about nested match, got %q", enhanced.Suggestion)
	}
}

func TestGuardSyntaxError(t *testing.T) {
	fset := token.NewFileSet()
	file := fset.AddFile("test.dingo", 1, 100)
	pos := file.Pos(10)

	err := GuardSyntaxError(fset, pos, "x >", nil)

	enhanced, ok := err.(*EnhancedError)
	if !ok {
		t.Fatal("Expected EnhancedError")
	}

	if !strings.Contains(enhanced.Message, "Invalid guard condition") {
		t.Errorf("Expected invalid guard message, got %q", enhanced.Message)
	}

	if enhanced.Annotation != "Guard must be valid Go expression" {
		t.Errorf("Expected guard expression annotation, got %q", enhanced.Annotation)
	}

	// Verify suggestion contains examples
	expectedExamples := []string{
		"x > 0",
		"len(s) > 0",
		"err != nil",
	}

	for _, example := range expectedExamples {
		if !strings.Contains(enhanced.Suggestion, example) {
			t.Errorf("Expected suggestion to contain example %q\nGot:\n%s", example, enhanced.Suggestion)
		}
	}
}

func TestPatternTypeMismatchError(t *testing.T) {
	fset := token.NewFileSet()
	file := fset.AddFile("test.dingo", 1, 100)
	pos := file.Pos(10)

	err := PatternTypeMismatchError(fset, pos, "Result<T, E>", "Option<T>")

	enhanced, ok := err.(*EnhancedError)
	if !ok {
		t.Fatal("Expected EnhancedError")
	}

	if !strings.Contains(enhanced.Message, "expected Result<T, E>, got Option<T>") {
		t.Errorf("Expected type mismatch message, got %q", enhanced.Message)
	}

	if !strings.Contains(enhanced.Suggestion, "Did you mean") {
		t.Errorf("Expected 'Did you mean' suggestion, got %q", enhanced.Suggestion)
	}
}

func TestWildcardError(t *testing.T) {
	fset := token.NewFileSet()
	file := fset.AddFile("test.dingo", 1, 100)
	pos := file.Pos(10)

	err := WildcardError(fset, pos, "binding position")

	enhanced, ok := err.(*EnhancedError)
	if !ok {
		t.Fatal("Expected EnhancedError")
	}

	if !strings.Contains(enhanced.Message, "not allowed in binding position") {
		t.Errorf("Expected wildcard error message, got %q", enhanced.Message)
	}

	if enhanced.Annotation != "Invalid wildcard usage" {
		t.Errorf("Expected wildcard annotation, got %q", enhanced.Annotation)
	}
}

func TestNestedMatchError(t *testing.T) {
	fset := token.NewFileSet()
	file := fset.AddFile("test.dingo", 1, 100)
	pos := file.Pos(10)

	err := NestedMatchError(fset, pos, 3, "too deeply nested")

	enhanced, ok := err.(*EnhancedError)
	if !ok {
		t.Fatal("Expected EnhancedError")
	}

	if !strings.Contains(enhanced.Message, "depth 3") {
		t.Errorf("Expected depth in message, got %q", enhanced.Message)
	}

	if !strings.Contains(enhanced.Message, "too deeply nested") {
		t.Errorf("Expected reason in message, got %q", enhanced.Message)
	}

	if !strings.Contains(enhanced.Suggestion, "Simplify") {
		t.Errorf("Expected simplification suggestion, got %q", enhanced.Suggestion)
	}
}

func TestSnippetSpan(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "span.dingo")

	content := "match result { Ok(x) => x }"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()
	file := fset.AddFile(testFile, 1, len(content))

	startPos := file.Pos(0)
	endPos := file.Pos(5) // "match"

	snippet := NewSnippetSpan(fset, startPos, endPos, "Test span")
	err := snippet.Build()

	enhanced, ok := err.(*EnhancedError)
	if !ok {
		t.Fatal("Expected EnhancedError")
	}

	if enhanced.Length < 1 {
		t.Errorf("Expected span length >= 1, got %d", enhanced.Length)
	}
}

func TestMultipleMissingPatterns(t *testing.T) {
	fset := token.NewFileSet()
	file := fset.AddFile("test.dingo", 1, 100)
	pos := file.Pos(10)

	err := ExhaustivenessError(
		fset,
		pos,
		"value",
		[]string{"Err(_)", "None"},
		[]string{"Ok(x)", "Some(y)"},
	)

	enhanced, ok := err.(*EnhancedError)
	if !ok {
		t.Fatal("Expected EnhancedError")
	}

	// Should list both missing patterns
	if !strings.Contains(enhanced.Annotation, "Err(_)") {
		t.Error("Expected Err(_) in annotation")
	}
	if !strings.Contains(enhanced.Annotation, "None") {
		t.Error("Expected None in annotation")
	}

	// Should include both in suggestion
	if !strings.Contains(enhanced.Suggestion, "Err(_) => ...") {
		t.Error("Expected Err(_) in suggestion")
	}
	if !strings.Contains(enhanced.Suggestion, "None => ...") {
		t.Error("Expected None in suggestion")
	}
}
