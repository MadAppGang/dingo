package preprocessor

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
)

// TupleProcessor handles tuple literal and destructuring syntax
// Transforms:
//   - Literals: (10, 20) → __TUPLE_2__LITERAL__{hash}(10, 20)
//   - Destructuring: let (x, y) = expr → temp var + assignments
//
// Arity limits:
//   - 0 elements → error: "Empty tuples not supported"
//   - 1 element → error: "Single-element tuples not supported"
//   - 2-12 elements → ok (Tuple2 through Tuple12)
//   - >12 elements → error: "Maximum 12 elements"
type TupleProcessor struct {
	counter  int       // For temporary variable generation (starts at 1, first var is "tmp", then "tmp1", "tmp2"...)
	mappings []Mapping // Source map tracking
}

// NewTupleProcessor creates a new tuple processor
func NewTupleProcessor() *TupleProcessor {
	return &TupleProcessor{
		counter:  1,
		mappings: []Mapping{},
	}
}

// Name returns the processor name
func (t *TupleProcessor) Name() string {
	return "tuples"
}

// Process transforms tuple syntax (literals and destructuring)
func (t *TupleProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	// Reset state
	t.counter = 1
	t.mappings = []Mapping{}

	lines := strings.Split(string(source), "\n")
	var output bytes.Buffer

	inputLineNum := 0
	outputLineNum := 1

	for inputLineNum < len(lines) {
		line := lines[inputLineNum]

		// Process the line
		transformed, newMappings, err := t.processLine(line, inputLineNum+1, outputLineNum)
		if err != nil {
			return nil, nil, fmt.Errorf("line %d: %w", inputLineNum+1, err)
		}

		output.WriteString(transformed)
		if inputLineNum < len(lines)-1 {
			output.WriteByte('\n')
		}

		// Add mappings
		if len(newMappings) > 0 {
			t.mappings = append(t.mappings, newMappings...)
		}

		// Update output line count
		newlineCount := strings.Count(transformed, "\n")
		linesOccupied := newlineCount + 1
		outputLineNum += linesOccupied

		inputLineNum++
	}

	return output.Bytes(), t.mappings, nil
}

// processLine processes a single line for tuple syntax
func (t *TupleProcessor) processLine(line string, originalLineNum int, outputLineNum int) (string, []Mapping, error) {
	// Quick check: does this line have parentheses?
	if !strings.Contains(line, "(") {
		return line, nil, nil
	}

	// Skip comment lines entirely
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "//") {
		return line, nil, nil
	}

	// Check for destructuring pattern: let (x, y, ...) = expr
	if isDestructuring(line) {
		return t.processDestructuring(line, originalLineNum, outputLineNum)
	}

	// Check for tuple literals: (expr, expr, ...)
	// We need to scan for literals and replace them with markers
	transformed, mappings, err := t.processTupleLiterals(line, originalLineNum, outputLineNum)
	if err != nil {
		return "", nil, err
	}

	return transformed, mappings, nil
}

// isDestructuring checks if a line contains destructuring syntax
// Pattern: let (ident, ident, ...) = expr
func isDestructuring(line string) bool {
	trimmed := strings.TrimSpace(line)
	// Look for "let (" pattern
	if !strings.HasPrefix(trimmed, "let ") && !strings.HasPrefix(trimmed, "var ") {
		return false
	}

	// Find opening paren
	parenIdx := strings.Index(trimmed, "(")
	if parenIdx == -1 {
		return false
	}

	// Must be immediately after "let" or "var" (with optional whitespace)
	prefix := strings.TrimSpace(trimmed[:parenIdx])
	if prefix != "let" && prefix != "var" {
		return false
	}

	// Must have = after closing paren
	closeParen := findMatchingParen(trimmed, parenIdx)
	if closeParen == -1 {
		return false
	}

	// Check for = after closing paren
	afterParen := strings.TrimSpace(trimmed[closeParen+1:])
	return strings.HasPrefix(afterParen, "=")
}

// processDestructuring transforms tuple destructuring
// Supports nested patterns: let ((a, b), c) = expr
// Input: let (x, y, z) = expr
// Output:
//   tmp := expr
//   x, y, z := tmp._0, tmp._1, tmp._2
//
// Input: let ((a, b), c) = expr
// Output:
//   tmp := expr
//   tmp1 := tmp._0
//   a, b := tmp1._0, tmp1._1
//   c := tmp._1
func (t *TupleProcessor) processDestructuring(line string, originalLineNum int, outputLineNum int) (string, []Mapping, error) {
	trimmed := strings.TrimSpace(line)
	indent := t.getIndent(line)

	// Find pattern and expression
	parenIdx := strings.Index(trimmed, "(")
	closeParen := findMatchingParen(trimmed, parenIdx)
	if closeParen == -1 {
		return "", nil, fmt.Errorf("unmatched parenthesis in destructuring")
	}

	// Extract pattern (between parens)
	pattern := trimmed[parenIdx+1 : closeParen]

	// Extract expression (after =)
	afterParen := trimmed[closeParen+1:]
	eqIdx := strings.Index(afterParen, "=")
	if eqIdx == -1 {
		return "", nil, fmt.Errorf("missing = in destructuring")
	}
	expr := strings.TrimSpace(afterParen[eqIdx+1:])

	// Parse pattern: identifiers separated by commas (handles nested patterns)
	identifiers := parseDestructurePattern(pattern)
	if len(identifiers) == 0 {
		return "", nil, fmt.Errorf("empty destructuring pattern")
	}

	// Validate arity
	if err := t.validateArity(len(identifiers), originalLineNum); err != nil {
		return "", nil, err
	}

	// Generate temporary variable for top-level tuple
	tmpVar := t.generateTmpVar()

	// Build output
	var buf bytes.Buffer
	mappings := []Mapping{}

	// Line 1: tmp := expr
	buf.WriteString(indent)
	buf.WriteString(tmpVar)
	buf.WriteString(" := ")
	buf.WriteString(expr)
	buf.WriteString("\n")

	mappings = append(mappings, Mapping{
		OriginalLine:    originalLineNum,
		OriginalColumn:  1,
		GeneratedLine:   outputLineNum,
		GeneratedColumn: 1,
		Length:          len(line),
		Name:            "destructure",
	})
	outputLineNum++

	// Process each identifier (may be nested pattern or simple identifier)
	// Track which ones are simple vs nested
	simpleIds := []string{}
	simpleFields := []string{}

	for i, id := range identifiers {
		// Check if this is a nested pattern: starts with (
		if strings.HasPrefix(id, "(") && strings.HasSuffix(id, ")") {
			// Nested pattern - need intermediate variable
			nestedPattern := id[1 : len(id)-1] // Remove outer parens
			nestedIds := parseDestructurePattern(nestedPattern)

			// Generate intermediate variable
			nestedTmpVar := t.generateTmpVar()

			// Extract nested tuple: tmp1 := tmp._i
			buf.WriteString(indent)
			buf.WriteString(nestedTmpVar)
			buf.WriteString(" := ")
			buf.WriteString(fmt.Sprintf("%s._%d", tmpVar, i))
			buf.WriteString("\n")

			mappings = append(mappings, Mapping{
				OriginalLine:    originalLineNum,
				OriginalColumn:  1,
				GeneratedLine:   outputLineNum,
				GeneratedColumn: 1,
				Length:          len(id),
				Name:            "destructure_nested",
			})
			outputLineNum++

			// Destructure nested tuple: a, b := tmp1._0, tmp1._1
			buf.WriteString(indent)
			buf.WriteString(strings.Join(nestedIds, ", "))
			buf.WriteString(" := ")

			nestedFields := []string{}
			for j := range nestedIds {
				nestedFields = append(nestedFields, fmt.Sprintf("%s._%d", nestedTmpVar, j))
			}
			buf.WriteString(strings.Join(nestedFields, ", "))
			buf.WriteString("\n")

			mappings = append(mappings, Mapping{
				OriginalLine:    originalLineNum,
				OriginalColumn:  1,
				GeneratedLine:   outputLineNum,
				GeneratedColumn: 1,
				Length:          len(id),
				Name:            "destructure_nested_assign",
			})
			outputLineNum++
		} else {
			// Simple identifier (including wildcards)
			simpleIds = append(simpleIds, id)
			simpleFields = append(simpleFields, fmt.Sprintf("%s._%d", tmpVar, i))
		}
	}

	// Generate assignment for all simple (non-nested) identifiers
	if len(simpleIds) > 0 {
		buf.WriteString(indent)
		buf.WriteString(strings.Join(simpleIds, ", "))
		buf.WriteString(" := ")
		buf.WriteString(strings.Join(simpleFields, ", "))

		mappings = append(mappings, Mapping{
			OriginalLine:    originalLineNum,
			OriginalColumn:  1,
			GeneratedLine:   outputLineNum,
			GeneratedColumn: 1,
			Length:          len(line),
			Name:            "destructure_assign",
		})
	}

	return buf.String(), mappings, nil
}

// processTupleLiterals finds and transforms tuple literals in a line
// Distinguishes tuples from function calls and grouping expressions
func (t *TupleProcessor) processTupleLiterals(line string, originalLineNum int, outputLineNum int) (string, []Mapping, error) {
	result := line
	mappings := []Mapping{}
	offset := 0 // Track how much the string has grown/shrunk

	// Scan for potential tuple literals
	for i := 0; i < len(result); i++ {
		if result[i] != '(' {
			continue
		}

		// Check if this is a tuple literal (not a function call or grouping)
		isTuple, elements, closeParen := t.detectTuple(result, i)
		if !isTuple {
			// Skip to closing paren to avoid re-processing
			if closeParen > i {
				i = closeParen
			}
			continue
		}

		// Validate arity
		arity := len(elements)
		if err := t.validateArity(arity, originalLineNum); err != nil {
			return "", nil, err
		}

		// Extract the full tuple expression
		tupleExpr := result[i : closeParen+1]

		// Generate marker
		marker := t.generateMarker(arity, tupleExpr)

		// Replace in result
		replacement := marker
		result = result[:i] + replacement + result[closeParen+1:]

		// Create mapping
		mappings = append(mappings, Mapping{
			OriginalLine:    originalLineNum,
			OriginalColumn:  i + 1 - offset, // Adjust for previous replacements
			GeneratedLine:   outputLineNum,
			GeneratedColumn: i + 1,
			Length:          len(tupleExpr),
			Name:            "tuple_literal",
		})

		// Update offset and position
		offset += len(replacement) - len(tupleExpr)
		i += len(replacement) - 1 // -1 because loop will increment
	}

	return result, mappings, nil
}

// detectTuple determines if a '(' starts a tuple literal
// Returns: (isTuple, elements, closeParenIndex)
func (t *TupleProcessor) detectTuple(line string, startIdx int) (bool, []string, int) {
	// Find matching closing paren
	closeParen := findMatchingParen(line, startIdx)
	if closeParen == -1 {
		return false, nil, -1
	}

	// Check if preceded by identifier (function call) or } (IIFE pattern)
	if startIdx > 0 {
		prevChar := line[startIdx-1]
		// If previous character is letter, digit, or underscore → function call
		if isIdentifierChar(prevChar) {
			return false, nil, closeParen
		}
		// If previous character is } → IIFE pattern (e.g., }())
		if prevChar == '}' {
			return false, nil, closeParen
		}
	}

	// Extract content between parens
	content := line[startIdx+1 : closeParen]
	trimmed := strings.TrimSpace(content)

	// Empty parens → not a tuple (will be caught by arity validation)
	if trimmed == "" {
		return true, []string{}, closeParen
	}

	// Must contain comma to be a tuple (otherwise it's grouping)
	if !strings.Contains(content, ",") {
		return false, nil, closeParen
	}

	// CRITICAL FIX: Check for function return type signature
	// Pattern: "func name() (type1, type2)" or just ") (type1, type2)"
	if isFunctionReturnType(line, startIdx) {
		return false, nil, closeParen
	}

	// CRITICAL FIX: Check for multi-return "return" statement
	// Pattern: "return (x, y)" where function signature has multi-return
	if isMultiReturnStatement(line, startIdx) {
		return false, nil, closeParen
	}

	// Parse elements
	elements := parseElements(content)

	// Filter out empty elements (e.g., from trailing comma)
	filtered := []string{}
	for _, elem := range elements {
		trimmed := strings.TrimSpace(elem)
		if trimmed != "" {
			// SCOPE REDUCTION: Detect nested tuples and reject them
			// Check if element contains parentheses (indicates nested tuple)
			if containsBalancedParens(trimmed) {
				// This is a nested tuple - not supported in Phase 8
				return false, nil, closeParen
			}
			filtered = append(filtered, trimmed)
		}
	}

	return true, filtered, closeParen
}

// parseElements splits tuple content by commas, respecting nested parens and strings
func parseElements(content string) []string {
	elements := []string{}
	current := ""
	parenDepth := 0
	inString := false
	escaped := false

	for _, ch := range content {
		if escaped {
			current += string(ch)
			escaped = false
			continue
		}

		if ch == '\\' {
			escaped = true
			current += string(ch)
			continue
		}

		if ch == '"' && !inString {
			inString = true
			current += string(ch)
			continue
		}

		if ch == '"' && inString {
			inString = false
			current += string(ch)
			continue
		}

		if inString {
			current += string(ch)
			continue
		}

		// Not in string - track paren depth
		if ch == '(' {
			parenDepth++
			current += string(ch)
			continue
		}

		if ch == ')' {
			parenDepth--
			current += string(ch)
			continue
		}

		// Comma at depth 0 → element separator
		if ch == ',' && parenDepth == 0 {
			elements = append(elements, current)
			current = ""
			continue
		}

		current += string(ch)
	}

	// Add last element (even if empty - caller will filter)
	elements = append(elements, current)

	return elements
}

// parseDestructurePattern parses identifiers from destructuring pattern
// Supports wildcards (_) and nested patterns like ((a, b), c)
func parseDestructurePattern(pattern string) []string {
	// Reuse the proven parseTuplePattern logic from rust_match.go
	// This handles nested patterns correctly: ((a, b), c) -> ["(a, b)", "c"]

	// Split on commas while respecting nested parentheses
	parts := splitTupleElements(pattern)
	identifiers := []string{}

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			identifiers = append(identifiers, trimmed)
		}
	}

	return identifiers
}

// splitTupleElements splits elements on commas, respecting nested parens
// This is a helper used by both literal and destructuring parsing
func splitTupleElements(s string) []string {
	var elements []string
	var current strings.Builder
	depth := 0

	for _, ch := range s {
		switch ch {
		case '(', '[', '{':
			depth++
			current.WriteRune(ch)
		case ')', ']', '}':
			depth--
			current.WriteRune(ch)
		case ',':
			if depth == 0 {
				elements = append(elements, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		elements = append(elements, strings.TrimSpace(current.String()))
	}

	return elements
}

// validateArity checks if tuple arity is within valid range
func (t *TupleProcessor) validateArity(arity int, lineNum int) error {
	if arity == 0 {
		return fmt.Errorf("empty tuples are not supported (line %d). Use 'struct{}' if you need a zero-size type", lineNum)
	}

	if arity == 1 {
		return fmt.Errorf("single-element tuples are not supported (line %d). Remove parentheses", lineNum)
	}

	if arity > 12 {
		return fmt.Errorf("tuple has %d elements, maximum is 12 (line %d). Consider using a struct instead", arity, lineNum)
	}

	return nil
}

// generateMarker creates a marker for AST phase
// Format: __TUPLE_{N}__LITERAL__{hash}(expr1, expr2, ...)
func (t *TupleProcessor) generateMarker(arity int, tupleExpr string) string {
	// Generate hash for uniqueness (in case of identical tuples on same line)
	hash := generateHash(tupleExpr)
	return fmt.Sprintf("__TUPLE_%d__LITERAL__%s%s", arity, hash, tupleExpr)
}

// generateHash creates a short hash of the expression
func generateHash(expr string) string {
	h := sha256.Sum256([]byte(expr))
	// Use first 8 chars of hex for brevity
	return fmt.Sprintf("%x", h[:4])
}

// generateTmpVar generates temporary variable names following camelCase convention
// Pattern: tmp, tmp1, tmp2, ... (no-number-first per CLAUDE.md standard)
func (t *TupleProcessor) generateTmpVar() string {
	if t.counter == 1 {
		t.counter++
		return "tmp"
	}
	varName := fmt.Sprintf("tmp%d", t.counter-1)
	t.counter++
	return varName
}

// getIndent extracts leading whitespace from a line
func (t *TupleProcessor) getIndent(line string) string {
	for i, ch := range line {
		if ch != ' ' && ch != '\t' {
			return line[:i]
		}
	}
	return ""
}

// Helper functions

// isFunctionReturnType checks if parens are part of function return type
// Pattern: "func name() (type1, type2)" or ") (type1, type2)"
func isFunctionReturnType(line string, startIdx int) bool {
	// Look backward from startIdx to find what precedes the paren
	before := strings.TrimSpace(line[:startIdx])

	// Case 1: Direct function declaration - "func name()"
	if strings.Contains(before, "func ") && strings.HasSuffix(before, ")") {
		return true
	}

	// Case 2: Multiline function signature - ") (" at start of line
	// This handles cases where return type is on separate line
	if strings.HasSuffix(before, ")") {
		// If the line ONLY contains ") (type1, type2) {", treat as return type
		// This is a multiline function signature continuation
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, ")") {
			return true
		}

		// Also check if "func" is earlier in the line
		if strings.Contains(before, "func ") {
			return true
		}
	}

	return false
}

// isMultiReturnStatement checks if parens are part of "return (x, y)" in multi-return context
// Pattern: "return (expr, expr)"
func isMultiReturnStatement(line string, startIdx int) bool {
	// Look backward from startIdx
	before := strings.TrimSpace(line[:startIdx])

	// Check if immediately preceded by "return"
	if strings.HasSuffix(before, "return") {
		// This is "return (x, y)" - could be multi-return or tuple literal
		// For now, treat as valid Go multi-return (conservative approach)
		// The user can use "return Tuple2(x, y)" for tuple literals
		return true
	}

	return false
}

// containsBalancedParens checks if a string is itself a tuple literal
// (starts with '(' and ends with ')' with balanced parens)
// This is used to detect nested tuples which are not supported in Phase 8
func containsBalancedParens(s string) bool {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) < 2 {
		return false
	}

	// Check if element IS a tuple literal (starts and ends with parens)
	if trimmed[0] != '(' || trimmed[len(trimmed)-1] != ')' {
		return false
	}

	// Verify the parens are balanced and the opening matches the closing
	depth := 0
	for i, ch := range trimmed {
		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
			// If we close to zero before the end, it's not a single tuple
			if depth == 0 && i < len(trimmed)-1 {
				return false
			}
		}
	}

	return depth == 0
}

// findMatchingParen finds the index of the closing paren that matches the opening paren at startIdx
func findMatchingParen(s string, startIdx int) int {
	if startIdx >= len(s) || s[startIdx] != '(' {
		return -1
	}

	depth := 1
	inString := false
	escaped := false

	for i := startIdx + 1; i < len(s); i++ {
		ch := s[i]

		if escaped {
			escaped = false
			continue
		}

		if ch == '\\' {
			escaped = true
			continue
		}

		if ch == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1 // Unmatched
}
