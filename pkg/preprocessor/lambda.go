package preprocessor

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/MadAppGang/dingo/pkg/config"
	dingoerrors "github.com/MadAppGang/dingo/pkg/errors"
)

// Package-level compiled regexes for lambda transformations
var (
	// TypeScript arrow syntax patterns
	// Pattern 1: Single param without parens (identifier only): x => expr
	// Captures: prefix, param (body extracted via balanced delimiter tracking)
	singleParamArrow = regexp.MustCompile(`(^|[^.\w])([a-zA-Z_][a-zA-Z0-9_]*)\s*=>`)

	// Pattern 2: Params with parens (with or without types): (x) => expr, (x, y) => expr, (x: int) => expr
	// Captures: prefix, params, optional return type (body extracted via balanced delimiter tracking)
	multiParamArrow = regexp.MustCompile(`(^|[^.\w])\(([^)]*)\)\s*(?::\s*([^=>\s]+))?\s*=>`)

	// Rust pipe syntax patterns
	// Pattern 1: Single or multi param with pipes: |x| expr, |x, y| expr, |x: int| expr
	// Captures: prefix, params, optional return type (->) (body extracted via balanced delimiter tracking)
	rustPipe = regexp.MustCompile(`(^|[^.\w])\|([^|]*)\|\s*(?:->\s*([^\s{]+))?`)
)

// LambdaStyle represents the lambda syntax style
type LambdaStyle int

const (
	// StyleTypeScript uses TypeScript/JavaScript arrow syntax: x => expr
	StyleTypeScript LambdaStyle = iota
	// StyleRust uses Rust pipe syntax: |x| expr
	StyleRust
)

// LambdaProcessor converts lambda syntax to Go function literals
// Supports two styles (config-driven):
// - TypeScript arrows: x => expr, (x) => expr, (x, y) => expr, (x: int) => expr
// - Rust pipes: |x| expr, |x, y| expr, |x: int| expr, |x: int| -> bool { ... }
type LambdaProcessor struct {
	style              LambdaStyle
	errors             []*dingoerrors.EnhancedError
	strictTypeChecking bool // TODO(v1.1): Enable strict type checking via dingo.toml config
}

// NewLambdaProcessor creates a new lambda processor with default style (TypeScript)
// Strict type checking is disabled by default for backward compatibility
func NewLambdaProcessor() *LambdaProcessor {
	return &LambdaProcessor{
		style:              StyleTypeScript,
		strictTypeChecking: false, // Disabled by default
	}
}

// NewLambdaProcessorWithConfig creates a new lambda processor with config-driven style
func NewLambdaProcessorWithConfig(cfg *config.Config) *LambdaProcessor {
	style := StyleTypeScript // Default
	strictChecking := false  // Default

	if cfg != nil {
		if cfg.Features.LambdaStyle == "rust" {
			style = StyleRust
		}
		// Future: cfg.Features.StrictLambdaTypes (for now, always false)
		strictChecking = false
	}

	return &LambdaProcessor{
		style:              style,
		strictTypeChecking: strictChecking,
	}
}

// WithStrictTypeChecking enables strict type checking for standalone lambdas
// This is useful for testing and can be enabled via config in the future
func (l *LambdaProcessor) WithStrictTypeChecking(strict bool) *LambdaProcessor {
	l.strictTypeChecking = strict
	return l
}

// Name returns the processor name
func (l *LambdaProcessor) Name() string {
	return "lambda"
}

// Process transforms lambda syntax to Go function literals based on configured style
// TypeScript style:
//   x => x * 2          →  func(x) { return x * 2 }
//   (x) => x * 2        →  func(x) { return x * 2 }
//   (x, y) => x + y     →  func(x, y) { return x + y }
//   (x: int) => x * 2   →  func(x int) int { return x * 2 }
//   (x) => { return x } →  func(x) { return x }
// Rust style:
//   |x| x * 2           →  func(x) { return x * 2 }
//   |x, y| x + y        →  func(x, y) { return x + y }
//   |x: int| x * 2      →  func(x int) { return x * 2 }
//   |x: int| -> bool { x > 0 } →  func(x int) bool { return x > 0 }
func (l *LambdaProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	// Reset errors for this processing run
	l.errors = nil

	lines := bytes.Split(source, []byte("\n"))
	var result bytes.Buffer
	var mappings []Mapping

	for lineNum, line := range lines {
		originalLine := line

		// Process based on configured style
		switch l.style {
		case StyleTypeScript:
			// Process multi-param arrows first (they include single param with parens)
			line = l.processMultiParamArrow(line, lineNum+1)
			// Then process single-param arrows (without parens)
			line = l.processSingleParamArrow(line, lineNum+1)

		case StyleRust:
			// Process Rust pipe syntax
			line = l.processRustPipe(line, lineNum+1)
		}

		result.Write(line)

		// Add source mapping if line was modified
		if !bytes.Equal(originalLine, line) {
			mappings = append(mappings, Mapping{
				OriginalLine:    lineNum + 1,
				GeneratedLine:   lineNum + 1,
				OriginalColumn:  0,
				GeneratedColumn: 0,
				Length:          len(line),
			})
		}

		if lineNum < len(lines)-1 {
			result.WriteByte('\n')
		}
	}

	// If we collected errors, return them
	if len(l.errors) > 0 {
		return nil, nil, l.errors[0] // Return first error for now
	}

	return result.Bytes(), mappings, nil
}

// extractBalancedBody extracts lambda body with balanced delimiter tracking
// Handles nested parentheses, brackets, braces in function calls
// Examples:
//   transform(x, 1, 2)       -> "transform(x, 1, 2)", stops at closing paren
//   foo(bar(x, y), z)        -> "foo(bar(x, y), z)", handles nesting
//   {return x * 2}           -> "{return x * 2}", stops at closing brace
func extractBalancedBody(src string, start int) (body string, end int) {
	if start >= len(src) {
		return "", start
	}

	depth := 0
	inBlock := false

	for i := start; i < len(src); i++ {
		ch := src[i]

		// Track if we're in a block body (starts with {)
		if i == start && ch == '{' {
			inBlock = true
		}

		switch ch {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			// If in block and depth reaches 0, we've found the closing brace
			if inBlock && ch == '}' && depth == 0 {
				return src[start : i+1], i + 1
			}
			// If not in block and depth goes negative, we've hit the enclosing delimiter
			if !inBlock && depth < 0 {
				return strings.TrimSpace(src[start:i]), i
			}
		case ',':
			// Comma at depth 0 (not inside parens/brackets) ends the expression
			if depth == 0 && !inBlock {
				return strings.TrimSpace(src[start:i]), i
			}
		case '\n':
			// Newline at depth 0 (not inside any delimiters) ends the expression
			if depth == 0 && !inBlock {
				return strings.TrimSpace(src[start:i]), i
			}
		}
	}

	// Reached end of string
	return strings.TrimSpace(src[start:]), len(src)
}

// processSingleParamArrow handles: x => expr
func (l *LambdaProcessor) processSingleParamArrow(line []byte, lineNum int) []byte {
	// Don't process if it's already inside a func literal
	if bytes.Contains(line, []byte("func(")) {
		return line
	}

	// Use FindAllSubmatchIndex to get submatch positions in the original string
	matches := singleParamArrow.FindAllSubmatchIndex(line, -1)
	if len(matches) == 0 {
		return line
	}

	// Build result by replacing matches from right to left (to preserve indices)
	result := line
	lineStr := string(line)

	for i := len(matches) - 1; i >= 0; i-- {
		loc := matches[i]

		matchStart := loc[0]
		matchEnd := loc[1]

		// Extract submatches from original line
		var prefix, param []byte
		if loc[2] != -1 && loc[3] != -1 {
			prefix = line[loc[2]:loc[3]]
		}
		if loc[4] != -1 && loc[5] != -1 {
			param = line[loc[4]:loc[5]]
		}

		// Extract body using balanced delimiter tracking (starts after =>)
		bodyStart := matchEnd
		// Skip whitespace after =>
		for bodyStart < len(lineStr) && (lineStr[bodyStart] == ' ' || lineStr[bodyStart] == '\t') {
			bodyStart++
		}

		bodyStr, bodyEnd := extractBalancedBody(lineStr, bodyStart)
		body := []byte(bodyStr)

		// Check for type inference failure (only if strict checking is enabled)
		if l.strictTypeChecking {
			// Only error if this is truly standalone (not in a function call context)
			// Function calls have parens before the lambda: .map(x => ...)
			// Standalone: let f = x => ... OR x => ... at start of line
			isInCallContext := matchStart > 0 && line[matchStart-1] == '('

			if len(param) > 0 && !bytes.Contains(param, []byte(" ")) && !isInCallContext {
				// param is just "x" not "x int", and it's not in a call context
				// This indicates missing type information for standalone lambda
				l.addTypeInferenceError(lineNum, matchStart, string(line), string(param))
			}
		}

		var buf bytes.Buffer
		// Write prefix if present
		if len(prefix) > 0 {
			buf.Write(prefix)
		}
		buf.WriteString("func(")
		buf.Write(param)

		// Add TYPE_INFERENCE_NEEDED marker if no type annotation
		// This signals the lambda_type_inference plugin to infer the type
		if !bytes.Contains(param, []byte(" ")) {
			buf.WriteString(" __TYPE_INFERENCE_NEEDED")
		}
		buf.WriteString(")")

		// Process body - wrap in { return ... } if expression, pass through if block
		processedBody := l.processLambdaBody(body)
		buf.Write(processedBody)

		// Replace in result (from matchStart to bodyEnd)
		// Update result and lineStr for next iteration
		newResult := append([]byte(lineStr[:matchStart]), buf.Bytes()...)
		newResult = append(newResult, []byte(lineStr[bodyEnd:])...)
		result = newResult
		lineStr = string(result)
	}

	return result
}

// processMultiParamArrow handles: (x) => expr, (x, y) => expr, (x: int) => expr, (x: int): bool => expr
func (l *LambdaProcessor) processMultiParamArrow(line []byte, lineNum int) []byte {
	// Don't process if it's already inside a func literal
	if bytes.Contains(line, []byte("func(")) {
		return line
	}

	// Use FindAllSubmatchIndex to get submatch positions in the original string
	matches := multiParamArrow.FindAllSubmatchIndex(line, -1)
	if len(matches) == 0 {
		return line
	}

	// Build result by replacing matches from right to left (to preserve indices)
	result := line
	lineStr := string(line)

	for i := len(matches) - 1; i >= 0; i-- {
		loc := matches[i]
		// loc format: [matchStart, matchEnd, group1Start, group1End, group2Start, group2End, ...]

		matchStart := loc[0]
		matchEnd := loc[1]

		// Extract submatches from original line
		var prefix, params, returnType []byte
		if loc[2] != -1 && loc[3] != -1 {
			prefix = line[loc[2]:loc[3]]
		}
		if loc[4] != -1 && loc[5] != -1 {
			params = bytes.TrimSpace(line[loc[4]:loc[5]])
		}
		if len(loc) > 6 && loc[6] != -1 && loc[7] != -1 {
			returnType = bytes.TrimSpace(line[loc[6]:loc[7]])
		}

		// Extract body using balanced delimiter tracking (starts after =>)
		bodyStart := matchEnd
		// Skip whitespace after =>
		for bodyStart < len(lineStr) && (lineStr[bodyStart] == ' ' || lineStr[bodyStart] == '\t') {
			bodyStart++
		}

		bodyStr, bodyEnd := extractBalancedBody(lineStr, bodyStart)
		body := []byte(bodyStr)

		// Check if any parameter lacks type annotation (only if strict checking is enabled)
		if l.strictTypeChecking {
			// Only error if this is truly standalone (not in a function call context)
			// The regex already matches prefix, so check if it's an opening paren
			isInCallContext := len(prefix) > 0 && prefix[len(prefix)-1] == '('

			if l.hasUntypedParams(params) && !isInCallContext {
				l.addTypeInferenceError(lineNum, matchStart, string(line), string(params))
			}
		}

		// Parse parameters to handle type annotations
		processedParams := l.processParams(params)

		var buf bytes.Buffer
		// Write prefix if present
		if len(prefix) > 0 {
			buf.Write(prefix)
		}
		buf.WriteString("func(")
		buf.Write(processedParams)
		buf.WriteString(")")

		// Add return type if specified
		if len(returnType) > 0 {
			buf.WriteString(" ")
			buf.Write(returnType)
		}

		// Process body - wrap in { return ... } if expression, pass through if block
		processedBody := l.processLambdaBody(body)
		buf.Write(processedBody)

		// Replace in result (from matchStart to bodyEnd)
		// Update result and lineStr for next iteration
		newResult := append([]byte(lineStr[:matchStart]), buf.Bytes()...)
		newResult = append(newResult, []byte(lineStr[bodyEnd:])...)
		result = newResult
		lineStr = string(result)
	}

	return result
}

// processRustPipe handles Rust pipe syntax: |x| expr, |x, y| expr, |x: int| -> bool { ... }
func (l *LambdaProcessor) processRustPipe(line []byte, lineNum int) []byte {
	// Don't process if it's already inside a func literal
	if bytes.Contains(line, []byte("func(")) {
		return line
	}

	// Use FindAllSubmatchIndex to get submatch positions in the original string
	matches := rustPipe.FindAllSubmatchIndex(line, -1)
	if len(matches) == 0 {
		return line
	}

	// Build result by replacing matches from right to left (to preserve indices)
	result := line
	lineStr := string(line)

	for i := len(matches) - 1; i >= 0; i-- {
		loc := matches[i]

		matchStart := loc[0]
		matchEnd := loc[1]

		// Extract submatches from original line
		var prefix, params, returnType []byte
		if loc[2] != -1 && loc[3] != -1 {
			prefix = line[loc[2]:loc[3]]
		}
		if loc[4] != -1 && loc[5] != -1 {
			params = bytes.TrimSpace(line[loc[4]:loc[5]])
		}
		if len(loc) > 6 && loc[6] != -1 && loc[7] != -1 {
			returnType = bytes.TrimSpace(line[loc[6]:loc[7]])
		}

		// Extract body using balanced delimiter tracking (starts after optional return type or |params|)
		bodyStart := matchEnd
		// Skip whitespace after -> or |params|
		for bodyStart < len(lineStr) && (lineStr[bodyStart] == ' ' || lineStr[bodyStart] == '\t') {
			bodyStart++
		}

		bodyStr, bodyEnd := extractBalancedBody(lineStr, bodyStart)
		body := []byte(bodyStr)

		// Check if any parameter lacks type annotation (only if strict checking is enabled)
		if l.strictTypeChecking {
			// Only error if this is truly standalone (not in a function call context)
			isInCallContext := len(prefix) > 0 && prefix[len(prefix)-1] == '('

			if l.hasUntypedParams(params) && !isInCallContext {
				l.addTypeInferenceError(lineNum, matchStart, string(line), string(params))
			}
		}

		// Parse parameters to handle type annotations
		processedParams := l.processParams(params)

		var buf bytes.Buffer
		// Write prefix if present
		if len(prefix) > 0 {
			buf.Write(prefix)
		}
		buf.WriteString("func(")
		buf.Write(processedParams)
		buf.WriteString(")")

		// Add return type if specified
		if len(returnType) > 0 {
			buf.WriteString(" ")
			buf.Write(returnType)
		}

		// Process body - wrap in { return ... } if expression, pass through if block
		processedBody := l.processLambdaBody(body)
		buf.Write(processedBody)

		// Replace in result (from matchStart to bodyEnd)
		// Update result and lineStr for next iteration
		newResult := append([]byte(lineStr[:matchStart]), buf.Bytes()...)
		newResult = append(newResult, []byte(lineStr[bodyEnd:])...)
		result = newResult
		lineStr = string(result)
	}

	return result
}

// processParams handles parameter list parsing and type annotation conversion
// Handles: x, y → x __TYPE_INFERENCE_NEEDED, y __TYPE_INFERENCE_NEEDED
//          x: int → x int
//          x: int, y: string → x int, y string
func (l *LambdaProcessor) processParams(params []byte) []byte {
	if len(params) == 0 {
		return params
	}

	// Split by comma to handle multiple parameters
	paramList := bytes.Split(params, []byte(","))
	var processed [][]byte

	for _, param := range paramList {
		param = bytes.TrimSpace(param)

		// Check if parameter has type annotation (contains :)
		if bytes.Contains(param, []byte(":")) {
			// Split on : and convert to Go syntax
			parts := bytes.SplitN(param, []byte(":"), 2)
			if len(parts) == 2 {
				name := bytes.TrimSpace(parts[0])
				typeName := bytes.TrimSpace(parts[1])

				var buf bytes.Buffer
				buf.Write(name)
				buf.WriteString(" ")
				buf.Write(typeName)
				processed = append(processed, buf.Bytes())
				continue
			}
		}

		// No type annotation - add TYPE_INFERENCE_NEEDED marker
		var buf bytes.Buffer
		buf.Write(param)
		buf.WriteString(" __TYPE_INFERENCE_NEEDED")
		processed = append(processed, buf.Bytes())
	}

	return bytes.Join(processed, []byte(", "))
}

// hasUntypedParams checks if any parameter in the list lacks a type annotation
func (l *LambdaProcessor) hasUntypedParams(params []byte) bool {
	if len(params) == 0 {
		return false
	}

	// Split by comma to check each parameter
	paramList := bytes.Split(params, []byte(","))
	for _, param := range paramList {
		param = bytes.TrimSpace(param)
		if len(param) == 0 {
			continue
		}

		// Check if parameter has type annotation (contains :)
		if !bytes.Contains(param, []byte(":")) {
			// No type annotation found
			return true
		}
	}

	return false
}

// addTypeInferenceError creates and stores a type inference error
func (l *LambdaProcessor) addTypeInferenceError(lineNum, column int, lineText, params string) {
	// Create error message based on style
	var exampleSyntax string
	switch l.style {
	case StyleTypeScript:
		if strings.Contains(params, ",") {
			exampleSyntax = "(x: int, y: int) => x + y"
		} else {
			exampleSyntax = "(x: int) => x * 2"
		}
	case StyleRust:
		if strings.Contains(params, ",") {
			exampleSyntax = "|x: int, y: int| x + y"
		} else {
			exampleSyntax = "|x: int| x * 2"
		}
	}

	message := "Cannot infer lambda parameter type"
	annotation := "Missing type annotation"
	suggestion := fmt.Sprintf("Add explicit type annotation. Example: %s", exampleSyntax)

	// Create enhanced error (without token.FileSet for preprocessor)
	// We'll create a simple error structure
	err := &dingoerrors.EnhancedError{
		Message:     message,
		Filename:    "source.dingo", // Will be set by caller if available
		Line:        lineNum,
		Column:      column + 1, // Convert 0-indexed to 1-indexed
		Length:      len(params),
		SourceLines: []string{lineText},
		HighlightLine: 0,
		Annotation:  annotation,
		Suggestion:  suggestion,
	}

	l.errors = append(l.errors, err)
}

// processLambdaBody handles lambda body transformation
// If body starts with {, it's a block body - pass through as-is
// Otherwise, wrap in { return ... }
func (l *LambdaProcessor) processLambdaBody(body []byte) []byte {
	trimmed := bytes.TrimSpace(body)

	// Check if body is already a block (starts with {)
	if bytes.HasPrefix(trimmed, []byte("{")) {
		// Block body - pass through with space prefix
		return append([]byte(" "), trimmed...)
	}

	// Expression body - wrap in { return ... }
	var buf bytes.Buffer
	buf.WriteString(" { return ")
	buf.Write(trimmed)
	buf.WriteString(" }")
	return buf.Bytes()
}
