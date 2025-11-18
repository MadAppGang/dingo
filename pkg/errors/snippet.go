package errors

import (
	"fmt"
	"go/token"
	"strings"
)

// SnippetBuilder helps build error messages with code snippets
type SnippetBuilder struct {
	fset    *token.FileSet
	pos     token.Pos
	message string
	err     *EnhancedError
}

// NewSnippet creates a new snippet builder
func NewSnippet(fset *token.FileSet, pos token.Pos, message string) *SnippetBuilder {
	return &SnippetBuilder{
		fset:    fset,
		pos:     pos,
		message: message,
		err:     NewEnhancedError(fset, pos, message),
	}
}

// NewSnippetSpan creates a new snippet builder with a span
func NewSnippetSpan(fset *token.FileSet, startPos, endPos token.Pos, message string) *SnippetBuilder {
	return &SnippetBuilder{
		fset:    fset,
		pos:     startPos,
		message: message,
		err:     NewEnhancedErrorSpan(fset, startPos, endPos, message),
	}
}

// Annotate adds an annotation (appears after ^^^^)
func (s *SnippetBuilder) Annotate(format string, args ...interface{}) *SnippetBuilder {
	s.err.Annotation = fmt.Sprintf(format, args...)
	return s
}

// Suggest adds a suggestion
func (s *SnippetBuilder) Suggest(format string, args ...interface{}) *SnippetBuilder {
	s.err.Suggestion = fmt.Sprintf(format, args...)
	return s
}

// MissingPatterns adds missing pattern information (for exhaustiveness)
func (s *SnippetBuilder) MissingPatterns(patterns []string) *SnippetBuilder {
	s.err.MissingItems = patterns
	if len(patterns) > 0 {
		s.err.Annotation = fmt.Sprintf("Missing pattern: %s", strings.Join(patterns, ", "))
	}
	return s
}

// Build returns the final enhanced error
func (s *SnippetBuilder) Build() error {
	return s.err
}

// Common error builders for pattern matching

// ExhaustivenessError creates an error for non-exhaustive match
func ExhaustivenessError(
	fset *token.FileSet,
	pos token.Pos,
	scrutinee string,
	missing []string,
	existingPatterns []string,
) error {
	err := NewEnhancedError(fset, pos, "Non-exhaustive match")
	err.Length = len("match") // Highlight the "match" keyword
	err.Annotation = fmt.Sprintf("Missing pattern: %s", strings.Join(missing, ", "))

	// Build suggestion
	var suggestion strings.Builder
	suggestion.WriteString("Add pattern to handle all cases:\n")
	suggestion.WriteString(fmt.Sprintf("    match %s {\n", scrutinee))

	// Show existing patterns
	for _, pattern := range existingPatterns {
		suggestion.WriteString(fmt.Sprintf("        %s => ...,\n", pattern))
	}

	// Show missing patterns with comment
	for _, pattern := range missing {
		suggestion.WriteString(fmt.Sprintf("        %s => ...  // Add this\n", pattern))
	}

	suggestion.WriteString("    }")

	err.Suggestion = suggestion.String()
	err.MissingItems = missing

	return err
}

// TupleArityError creates an error for tuple arity mismatch
func TupleArityError(
	fset *token.FileSet,
	pos token.Pos,
	expected, actual int,
) error {
	message := fmt.Sprintf("Tuple arity mismatch: expected %d elements, got %d", expected, actual)
	err := NewEnhancedError(fset, pos, message)
	err.Annotation = "Inconsistent tuple size"
	err.Suggestion = fmt.Sprintf("Ensure all tuple patterns have %d elements", expected)
	return err
}

// TupleLimitError creates an error for exceeding tuple element limit
func TupleLimitError(
	fset *token.FileSet,
	pos token.Pos,
	actual, limit int,
) error {
	message := fmt.Sprintf("Tuple patterns limited to %d elements (found %d)", limit, actual)
	err := NewEnhancedError(fset, pos, message)
	err.Annotation = fmt.Sprintf("Too many tuple elements (%d > %d)", actual, limit)
	err.Suggestion = "Consider splitting into nested match expressions or using fewer tuple elements"
	return err
}

// GuardSyntaxError creates an error for invalid guard syntax
func GuardSyntaxError(
	fset *token.FileSet,
	pos token.Pos,
	guardStr string,
	parseError error,
) error {
	message := fmt.Sprintf("Invalid guard condition: %s", guardStr)
	err := NewEnhancedError(fset, pos, message)
	err.Annotation = "Guard must be valid Go expression"

	var suggestion strings.Builder
	suggestion.WriteString("Examples of valid guard conditions:\n")
	suggestion.WriteString("    - 'x > 0'\n")
	suggestion.WriteString("    - 'len(s) > 0'\n")
	suggestion.WriteString("    - 'err != nil'\n")
	suggestion.WriteString("    - 'x >= 0 && x < 100'")

	if parseError != nil {
		suggestion.WriteString(fmt.Sprintf("\n\nParse error: %s", parseError.Error()))
	}

	err.Suggestion = suggestion.String()
	return err
}

// PatternTypeMismatchError creates an error for pattern type mismatches
func PatternTypeMismatchError(
	fset *token.FileSet,
	pos token.Pos,
	expected, actual string,
) error {
	message := fmt.Sprintf("Pattern type mismatch: expected %s, got %s", expected, actual)
	err := NewEnhancedError(fset, pos, message)
	err.Annotation = "Type does not match"
	err.Suggestion = fmt.Sprintf("Did you mean to use '%s' instead of '%s'?", expected, actual)
	return err
}

// WildcardError creates an error for misused wildcards
func WildcardError(
	fset *token.FileSet,
	pos token.Pos,
	context string,
) error {
	message := fmt.Sprintf("Wildcard '_' not allowed in %s", context)
	err := NewEnhancedError(fset, pos, message)
	err.Annotation = "Invalid wildcard usage"
	err.Suggestion = "Replace '_' with a named binding or specific pattern"
	return err
}

// NestedMatchError creates an error for problems in nested matches
func NestedMatchError(
	fset *token.FileSet,
	pos token.Pos,
	depth int,
	reason string,
) error {
	message := fmt.Sprintf("Error in nested match (depth %d): %s", depth, reason)
	err := NewEnhancedError(fset, pos, message)
	err.Annotation = "Nested match issue"
	err.Suggestion = "Simplify nested matches or split into separate functions"
	return err
}
