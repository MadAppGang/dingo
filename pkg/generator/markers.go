// Package generator - marker injection utilities
package generator

import (
	"fmt"
	"regexp"
	"strings"
)

// Plugin IDs for marker generation:
// 1 = error_propagation (? operator)
// 2 = result_type (Result<T, E>)
// 3 = option_type (Option<T>)
// 4 = pattern_matching (match expressions)
// 5 = sum_types (enum)

// MarkerInjector handles injection of DINGO:GENERATED markers into Go source code
type MarkerInjector struct {
	enabled bool
}

// NewMarkerInjector creates a new marker injector
func NewMarkerInjector(enabled bool) *MarkerInjector {
	return &MarkerInjector{
		enabled: enabled,
	}
}

// InjectMarkers injects DINGO:GENERATED markers into generated Go code
// This is a post-processing step that runs after AST generation
func (m *MarkerInjector) InjectMarkers(source []byte) ([]byte, error) {
	if !m.enabled {
		return source, nil
	}

	sourceStr := string(source)

	// Check if markers are already present (added by preprocessor)
	// If so, skip injection to avoid duplicates
	if strings.Contains(sourceStr, "// dingo:s:") || strings.Contains(sourceStr, "// dingo:e:") {
		return source, nil
	}

	// Pattern to detect error propagation generated code
	// Looks for: if __err0 != nil { return ... }
	errorCheckPattern := regexp.MustCompile(`(?m)(^[ \t]*if __err\d+ != nil \{[^}]*return[^}]*\}[ \t]*\n)`)

	// Inject markers around error propagation blocks
	// Using plugin ID 1 for error_propagation
	result := errorCheckPattern.ReplaceAllStringFunc(sourceStr, func(match string) string {
		// Extract indentation from the if statement
		indent := ""
		if idx := strings.Index(match, "if"); idx > 0 {
			indent = match[:idx]
		}

		startMarker := fmt.Sprintf("%s// dingo:s:1\n", indent)
		endMarker := fmt.Sprintf("%s// dingo:e:1\n", indent)

		return startMarker + match + endMarker
	})

	return []byte(result), nil
}

// injectErrorPropagationMarkers wraps error propagation blocks with markers
