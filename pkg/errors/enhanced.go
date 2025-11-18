package errors

import (
	"bufio"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
)

// EnhancedError provides rustc-style error messages with source snippets
type EnhancedError struct {
	// Basic error information
	Message  string
	Filename string
	Line     int    // 1-indexed
	Column   int    // 1-indexed
	Length   int    // Length of error span (for underline)

	// Source context
	SourceLines   []string // Lines to display (with context)
	HighlightLine int      // Which line in SourceLines has error (0-indexed)

	// Rich diagnostics
	Annotation   string   // Text after ^^^^ ("Missing pattern: Err(_)")
	Suggestion   string   // Multi-line suggestion block
	MissingItems []string // For exhaustiveness: missing patterns
}

// sourceCache caches file contents to avoid repeated reads
var (
	sourceCache   = make(map[string][]string)
	sourceCacheMu sync.RWMutex
)

// NewEnhancedError creates an enhanced error with source context
func NewEnhancedError(
	fset *token.FileSet,
	pos token.Pos,
	message string,
) *EnhancedError {
	if !pos.IsValid() {
		// Fallback for invalid position
		return &EnhancedError{
			Message:  message,
			Filename: "unknown",
			Line:     0,
			Column:   0,
			Length:   1,
		}
	}

	position := fset.Position(pos)

	// Extract source lines (2 lines context before/after)
	sourceLines, highlightIdx := extractSourceLines(position.Filename, position.Line, 2)

	return &EnhancedError{
		Message:       message,
		Filename:      position.Filename,
		Line:          position.Line,
		Column:        position.Column,
		Length:        1, // Default, can be extended
		SourceLines:   sourceLines,
		HighlightLine: highlightIdx,
	}
}

// NewEnhancedErrorSpan creates an enhanced error with a span (start to end position)
func NewEnhancedErrorSpan(
	fset *token.FileSet,
	startPos, endPos token.Pos,
	message string,
) *EnhancedError {
	err := NewEnhancedError(fset, startPos, message)

	// Calculate span length
	if startPos.IsValid() && endPos.IsValid() {
		start := fset.Position(startPos)
		end := fset.Position(endPos)

		// Same line: calculate column difference
		if start.Line == end.Line {
			err.Length = end.Column - start.Column
			if err.Length < 1 {
				err.Length = 1
			}
		}
	}

	return err
}

// WithAnnotation adds an annotation (text after ^^^^)
func (e *EnhancedError) WithAnnotation(annotation string) *EnhancedError {
	e.Annotation = annotation
	return e
}

// WithSuggestion adds a suggestion block
func (e *EnhancedError) WithSuggestion(suggestion string) *EnhancedError {
	e.Suggestion = suggestion
	return e
}

// WithMissingItems adds missing items (for exhaustiveness errors)
func (e *EnhancedError) WithMissingItems(items []string) *EnhancedError {
	e.MissingItems = items
	return e
}

// Format produces rustc-style error message
func (e *EnhancedError) Format() string {
	var buf strings.Builder

	// Header: Error: <message> in <file>:<line>:<col>
	if e.Line > 0 {
		fmt.Fprintf(&buf, "Error: %s in %s:%d:%d\n\n",
			e.Message, filepath.Base(e.Filename), e.Line, e.Column)
	} else {
		fmt.Fprintf(&buf, "Error: %s\n\n", e.Message)
	}

	// Source snippet with line numbers
	if len(e.SourceLines) > 0 && e.Line > 0 {
		startLine := e.Line - e.HighlightLine

		for i, line := range e.SourceLines {
			lineNum := startLine + i

			if i == e.HighlightLine {
				// Error line - show with caret
				fmt.Fprintf(&buf, "  %4d | %s\n", lineNum, line)

				// Caret line:     |     ^^^^^^^ <annotation>
				caretIndent := utf8.RuneCountInString(line[:min(e.Column-1, len(line))])
				caretLen := e.Length
				if caretLen < 1 {
					caretLen = 1
				}

				fmt.Fprintf(&buf, "       | %s%s",
					strings.Repeat(" ", caretIndent),
					strings.Repeat("^", caretLen),
				)

				if e.Annotation != "" {
					fmt.Fprintf(&buf, " %s", e.Annotation)
				}
				fmt.Fprintf(&buf, "\n")
			} else {
				// Context line
				fmt.Fprintf(&buf, "  %4d | %s\n", lineNum, line)
			}
		}

		buf.WriteString("\n")
	}

	// Suggestion section
	if e.Suggestion != "" {
		fmt.Fprintf(&buf, "Suggestion: %s\n", e.Suggestion)
	}

	// Missing items (for exhaustiveness)
	if len(e.MissingItems) > 0 {
		fmt.Fprintf(&buf, "\nMissing patterns: %s\n", strings.Join(e.MissingItems, ", "))
	}

	return buf.String()
}

// Error implements error interface
func (e *EnhancedError) Error() string {
	return e.Format()
}

// extractSourceLines reads source file and extracts lines with context
// Returns the lines and the index of the target line within the slice
func extractSourceLines(filename string, targetLine, contextLines int) ([]string, int) {
	// Try cache first
	sourceCacheMu.RLock()
	allLines, cached := sourceCache[filename]
	sourceCacheMu.RUnlock()

	if !cached {
		// Read file
		file, err := os.Open(filename)
		if err != nil {
			// Graceful fallback - return empty
			return nil, 0
		}
		defer file.Close()

		// Read all lines
		scanner := bufio.NewScanner(file)
		allLines = []string{}
		for scanner.Scan() {
			allLines = append(allLines, scanner.Text())
		}

		if scanner.Err() != nil {
			// Graceful fallback
			return nil, 0
		}

		// Cache for future use
		sourceCacheMu.Lock()
		sourceCache[filename] = allLines
		sourceCacheMu.Unlock()
	}

	// Calculate range (1-indexed to 0-indexed)
	targetIdx := targetLine - 1
	if targetIdx < 0 || targetIdx >= len(allLines) {
		return nil, 0
	}

	start := max(0, targetIdx-contextLines)
	end := min(len(allLines), targetIdx+contextLines+1)

	// Return slice and highlight index within slice
	highlightIdx := targetIdx - start
	return allLines[start:end], highlightIdx
}

// ClearCache clears the source file cache (useful for testing)
func ClearCache() {
	sourceCacheMu.Lock()
	sourceCache = make(map[string][]string)
	sourceCacheMu.Unlock()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
