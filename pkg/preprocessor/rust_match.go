package preprocessor

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// RustMatchProcessor handles Rust-like pattern matching syntax
// Transforms: match expr { Pattern => expression, ... } → Go switch statement with markers
type RustMatchProcessor struct {
	matchCounter int
	mappings     []Mapping
}

// Pattern-matching regex for Rust-like match expressions
var (
	// Match the entire match expression: match expr { ... }
	matchExprPattern = regexp.MustCompile(`(?s)match\s+([^{]+)\s*\{(.+)\}`)
)

// NewRustMatchProcessor creates a new Rust-like match preprocessor
func NewRustMatchProcessor() *RustMatchProcessor {
	return &RustMatchProcessor{
		matchCounter: 0,
		mappings:     []Mapping{},
	}
}

// getScrutineeVarName returns scrutinee variable name (scrutinee, scrutinee2, scrutinee3, ...)
func (r *RustMatchProcessor) getScrutineeVarName(matchID int) string {
	if matchID == 0 {
		return "scrutinee"
	}
	return fmt.Sprintf("scrutinee%d", matchID+1)
}

// getResultVarName returns result variable name (result, result2, result3, ...)
func (r *RustMatchProcessor) getResultVarName(matchID int) string {
	if matchID == 0 {
		return "result"
	}
	return fmt.Sprintf("result%d", matchID+1)
}

// isGeneratedResultVar checks if a variable name is a generated result variable
func (r *RustMatchProcessor) isGeneratedResultVar(varName string) bool {
	return varName == "result" || strings.HasPrefix(varName, "result")
}

// Name returns the processor name
func (r *RustMatchProcessor) Name() string {
	return "rust_match"
}

// Process transforms Rust-like match expressions
func (r *RustMatchProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	r.mappings = []Mapping{}
	r.matchCounter = 0

	input := string(source)
	lines := strings.Split(input, "\n")

	var output bytes.Buffer
	inputLineNum := 0
	outputLineNum := 1

	for inputLineNum < len(lines) {
		line := lines[inputLineNum]

		// Check if this line starts a match expression (not in comments)
		// Must be "match " followed by identifier (not in middle of word, not in comment)
		trimmed := strings.TrimSpace(line)
		isMatchExpr := false
		if !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "/*") {
			// FIX: Only detect match expressions that start with match keyword
			// This prevents reprocessing generated code like panic("unreachable: match is exhaustive")
			// Valid patterns: "match expr", "let x = match", "var y = match", "return match"
			if strings.HasPrefix(trimmed, "match ") ||
				strings.HasPrefix(trimmed, "let ") && strings.Contains(trimmed, " match ") ||
				strings.HasPrefix(trimmed, "var ") && strings.Contains(trimmed, " match ") ||
				strings.HasPrefix(trimmed, "return ") && strings.Contains(trimmed, " match ") {
				isMatchExpr = true
			}
		}

		if isMatchExpr {
			// Collect the entire match expression (may span multiple lines)
			matchExpr, linesConsumed := r.collectMatchExpression(lines, inputLineNum)
			if matchExpr != "" {
				// Transform the match expression
				transformed, newMappings, err := r.transformMatch(matchExpr, inputLineNum+1, outputLineNum)
				if err != nil {
					return nil, nil, fmt.Errorf("line %d: %w", inputLineNum+1, err)
				}

				output.WriteString(transformed)
				r.mappings = append(r.mappings, newMappings...)

				// Update line counters
				inputLineNum += linesConsumed
				outputLineNum += strings.Count(transformed, "\n")

				// Add newline if not at end
				if inputLineNum < len(lines) {
					output.WriteByte('\n')
					outputLineNum++
				}
				continue
			}
		}

		// Not a match expression, pass through as-is
		output.WriteString(line)
		if inputLineNum < len(lines)-1 {
			output.WriteByte('\n')
		}
		inputLineNum++
		outputLineNum++
	}

	return output.Bytes(), r.mappings, nil
}

// isAlphanumeric checks if a rune is alphanumeric or underscore
func isAlphanumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

// collectMatchExpression collects a complete match expression across multiple lines
// Returns: (matchExpression, linesConsumed)
func (r *RustMatchProcessor) collectMatchExpression(lines []string, startLine int) (string, int) {
	var buf bytes.Buffer
	braceDepth := 0
	linesConsumed := 0

	for i := startLine; i < len(lines); i++ {
		line := lines[i]
		buf.WriteString(line)
		linesConsumed++

		// Track brace depth
		for _, ch := range line {
			if ch == '{' {
				braceDepth++
			} else if ch == '}' {
				braceDepth--
				if braceDepth == 0 {
					// Complete match expression
					return buf.String(), linesConsumed
				}
			}
		}

		// Add newline if more lines to come (C7 FIX: Preserve newlines for proper formatting)
		if i < len(lines)-1 {
			buf.WriteByte('\n')
		}
	}

	// Incomplete match expression (missing closing brace)
	return "", 0
}

// transformMatch transforms a Rust-like match expression to Go switch
func (r *RustMatchProcessor) transformMatch(matchExpr string, originalLine int, outputLine int) (string, []Mapping, error) {
	// Extract scrutinee and arms using boundary-aware parsing instead of regex
	// This fixes the issue where DOTALL flag (.+) matches across all newlines until EOF
	// in files with multiple match expressions
	scrutinee, armsText, err := r.extractScrutineeAndArms(matchExpr)
	if err != nil {
		return "", nil, fmt.Errorf("extracting match components: %w", err)
	}

	// Check if match expression is in assignment context and extract variable name
	// If the entire match expression starts with "let x = match" or "var x = match",
	// we need to handle it specially using Variable Hoisting pattern
	isInAssignment, assignmentVar := r.extractAssignmentVar(matchExpr)

	// Check if scrutinee is a tuple expression
	isTuple, tupleElements, err := r.detectTuple(scrutinee)
	if err != nil {
		return "", nil, err
	}

	if isTuple {
		// Parse tuple pattern arms
		tupleArms, err := r.parseTupleArms(armsText)
		if err != nil {
			return "", nil, fmt.Errorf("parsing tuple pattern arms: %w", err)
		}

		// Generate tuple match (elements extraction + pattern info)
		result, mappings := r.generateTupleMatch(tupleElements, tupleArms, originalLine, outputLine)
		return result, mappings, nil
	}

	// Parse pattern arms (non-tuple)
	arms, err := r.parseArms(armsText)
	if err != nil {
		return "", nil, fmt.Errorf("parsing pattern arms: %w", err)
	}

	// Generate Go switch statement
	result, mappings := r.generateSwitch(scrutinee, arms, originalLine, outputLine, isInAssignment, assignmentVar)
	return result, mappings, nil
}

// extractScrutineeAndArms extracts the scrutinee expression and arms text from a match expression
// using boundary-aware parsing instead of regex to avoid DOTALL flag issues
// This properly separates "match expr { arms }" without capturing beyond the closing brace
func (r *RustMatchProcessor) extractScrutineeAndArms(matchExpr string) (scrutinee string, armsText string, err error) {
	matchExpr = strings.TrimSpace(matchExpr)

	// Find the opening brace for the arms
	// We need to find the first { that comes after the match keyword and expression
	matchKeywordIdx := strings.Index(matchExpr, "match")
	if matchKeywordIdx == -1 {
		return "", "", fmt.Errorf("no match keyword found")
	}

	// Find the opening brace - it's the first { after the expression
	braceIdx := -1
	for i := matchKeywordIdx + len("match"); i < len(matchExpr); i++ {
		if matchExpr[i] == '{' {
			// Found the opening brace for the arms
			braceIdx = i
			break
		}
	}

	if braceIdx == -1 {
		return "", "", fmt.Errorf("no opening brace found in match expression")
	}

	// Scrutinee is everything between "match" and the opening brace
	scrutineeStart := matchKeywordIdx + len("match")
	scrutinee = strings.TrimSpace(matchExpr[scrutineeStart:braceIdx])

	// Arms text is between the braces
	// Use depth-aware search starting from braceIdx to find the matching closing brace
	// This ensures we don't stop at a } from a nested block expression
	armsStart := braceIdx + 1
	armsEnd := -1
	depth := 1 // Start with depth 1 because we're already past the opening brace

	for i := braceIdx + 1; i < len(matchExpr); i++ {
		if matchExpr[i] == '{' {
			depth++
		} else if matchExpr[i] == '}' {
			depth--
			if depth == 0 {
				// Found the matching closing brace for the arms
				armsEnd = i
				break
			}
		}
	}

	if armsEnd == -1 {
		return "", "", fmt.Errorf("no closing brace found in match expression")
	}

	armsText = strings.TrimSpace(matchExpr[armsStart:armsEnd])

	return scrutinee, armsText, nil
}

// extractAssignmentVar extracts the variable name if match is in assignment context
// Returns: (isInAssignment, variableName)
// Examples:
//   "let x = match ..." -> (true, "x")
//   "var result = match ..." -> (true, "result")
//   "return match ..." -> (true, "__match_result_N") // Auto-generated name
//   "match ..." -> (false, "")
func (r *RustMatchProcessor) extractAssignmentVar(matchExpr string) (bool, string) {
	// Get the text before "match" keyword
	matchIdx := strings.Index(matchExpr, "match")
	if matchIdx == -1 {
		return false, ""
	}

	beforeMatch := strings.TrimSpace(matchExpr[:matchIdx])

	// Check for "return match" pattern
	if beforeMatch == "return" || strings.HasSuffix(beforeMatch, "return") {
		// Generate temporary variable name for return context
		varName := r.getResultVarName(r.matchCounter)
		return true, varName
	}

	// Check if there's an assignment operator before match
	if !strings.Contains(beforeMatch, "=") {
		return false, ""
	}

	// Extract variable name from patterns like:
	//   "let x ="
	//   "var result ="
	//   "x :="

	// Remove any "let" or "var" keywords
	beforeMatch = strings.TrimPrefix(beforeMatch, "let")
	beforeMatch = strings.TrimPrefix(beforeMatch, "var")
	beforeMatch = strings.TrimSpace(beforeMatch)

	// Now we should have something like "x =" or "result ="
	// Remove the "=" and any ":="
	beforeMatch = strings.TrimSuffix(beforeMatch, "=")
	beforeMatch = strings.TrimSuffix(beforeMatch, ":")
	beforeMatch = strings.TrimSpace(beforeMatch)

	// What remains should be the variable name
	varName := beforeMatch
	if varName == "" {
		return false, ""
	}

	return true, varName
}

// patternArm represents a single pattern arm
type patternArm struct {
	pattern    string // Ok(x), Err(e), Some(v), None, _
	binding    string // x, e, v, etc. (empty for None and _)
	guard      string // Guard condition: "x > 0" (optional, empty if no guard)
	expression string // the expression to execute
}

// parseArms parses pattern arms from the match body
// Handles both simple and block expressions:
//   Ok(x) => x * 2,
//   Err(e) => { log(e); return 0 }
func (r *RustMatchProcessor) parseArms(armsText string) ([]patternArm, error) {
	arms := []patternArm{}
	text := strings.TrimSpace(armsText)

	// Parse arms manually to handle nested braces
	i := 0
	for i < len(text) {
		// Skip whitespace
		for i < len(text) && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n' || text[i] == '\r') {
			i++
		}
		if i >= len(text) {
			break
		}

		// Extract pattern + optional guard (everything before =>)
		arrowPos := strings.Index(text[i:], "=>")
		if arrowPos == -1 {
			break // No more arms
		}

		patternAndGuard := strings.TrimSpace(text[i : i+arrowPos])
		i += arrowPos + 2 // Skip =>

		// Split pattern from guard if present
		// Guard syntax: Pattern if condition => expr
		//           or: Pattern where condition => expr
		pattern, guard := r.splitPatternAndGuard(patternAndGuard)

		// Skip whitespace after =>
		for i < len(text) && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n' || text[i] == '\r') {
			i++
		}
		if i >= len(text) {
			return nil, fmt.Errorf("unexpected end after =>")
		}

		// Extract expression (until comma or end)
		var expr string
		if text[i] == '{' {
			// Block expression - find matching }
			braceCount := 1
			start := i
			i++
			for i < len(text) && braceCount > 0 {
				if text[i] == '{' {
					braceCount++
				} else if text[i] == '}' {
					braceCount--
				}
				i++
			}
			expr = strings.TrimSpace(text[start:i])
			// PRIORITY 3 FIX: Skip trailing comma after block expression
			if i < len(text) && text[i] == ',' {
				i++
			}
		} else {
			// Simple expression - find comma or end (respecting strings and nesting)
			start := i
			if start >= len(text) {
				return nil, fmt.Errorf("unexpected end of text after =>")
			}
			exprEnd := r.findExpressionEnd(text, start)
			if exprEnd > start {
				expr = strings.TrimSpace(text[start:exprEnd])
				i = exprEnd
				// Skip comma if present
				if i < len(text) && text[i] == ',' {
					i++
				}
			} else {
				return nil, fmt.Errorf("invalid expression end at position %d", start)
			}
		}

		// Extract binding from pattern (if present)
		// FIX: Use proper paren matching to handle nested patterns like Result_Ok(Value_Int(n))
		binding := ""
		patternName := pattern
		if strings.Contains(pattern, "(") {
			start := strings.Index(pattern, "(")
			// Find MATCHING closing paren (not just first one)
			end := r.findMatchingCloseParen(pattern, start)
			if end > start {
				binding = strings.TrimSpace(pattern[start+1 : end])
				patternName = pattern[:start]
			}
		}

		arms = append(arms, patternArm{
			pattern:    patternName,
			binding:    binding,
			guard:      guard,
			expression: expr,
		})
	}

	if len(arms) == 0 {
		return nil, fmt.Errorf("no pattern arms found")
	}

	return arms, nil
}

// findMatchingCloseParen finds the closing paren that matches the open paren at position start
// Handles nested parens correctly: Result_Ok(Value_Int(n)) -> finds the final )
func (r *RustMatchProcessor) findMatchingCloseParen(text string, start int) int {
	if start >= len(text) || text[start] != '(' {
		return -1
	}

	depth := 1
	i := start + 1

	for i < len(text) && depth > 0 {
		if text[i] == '(' {
			depth++
		} else if text[i] == ')' {
			depth--
		}
		if depth == 0 {
			return i
		}
		i++
	}

	return -1 // No matching close paren
}

// findExpressionEnd finds the end of an expression, respecting string literals and nested structures
// Returns the position of the delimiter (comma) or end of string
// This correctly handles commas inside strings, parentheses, brackets, and braces
func (r *RustMatchProcessor) findExpressionEnd(text string, start int) int {
	i := start
	inString := false
	stringDelim := byte(0)
	depth := 0 // Track nesting depth for (), [], {}

	for i < len(text) {
		ch := text[i]

		// Handle string literals
		if !inString && (ch == '"' || ch == '`') {
			inString = true
			stringDelim = ch
			i++
			continue
		}
		if inString {
			if ch == stringDelim {
				// Check if escaped
				if i > 0 && text[i-1] == '\\' {
					// Escaped quote, stay in string
					i++
					continue
				}
				// End of string
				inString = false
				stringDelim = 0
			}
			i++
			continue
		}

		// Not in string - check for delimiters and nesting
		switch ch {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			// Comma at depth 0 is the delimiter we're looking for
			if depth == 0 {
				return i
			}
		}

		i++
	}

	// Reached end of text
	return i
}

// splitPatternAndGuard splits a pattern arm into pattern and optional guard
// Supports both 'if' and 'where' guard keywords
// Examples:
//   "Ok(x) if x > 0" -> ("Ok(x)", "x > 0")
//   "Ok(x) where x > 0" -> ("Ok(x)", "x > 0")
//   "Ok(x)" -> ("Ok(x)", "")
func (r *RustMatchProcessor) splitPatternAndGuard(patternAndGuard string) (pattern string, guard string) {
	// Strategy: Look for guard keyword (" if " or " where ") that comes after a complete pattern
	// Pattern formats:
	//   - Ok(binding)   - ends with )
	//   - None          - bare identifier
	//   - _             - wildcard

	// We need to find " if " or " where " (with surrounding spaces) that appears after the pattern
	// To avoid false matches like "diff" containing "if", we require the keyword
	// to be surrounded by spaces

	var guardPos int = -1
	var guardKeywordLen int = 0

	// Try " where " first (Swift-style)
	idx := strings.Index(patternAndGuard, " where ")
	if idx != -1 {
		// Found " where " - this could be the guard
		// Validate it's after a complete pattern by checking what comes before
		before := patternAndGuard[:idx]
		if r.isCompletePattern(before) {
			guardPos = idx
			guardKeywordLen = 7 // len(" where ")
		}
	}

	// If no " where ", try " if " (Rust-style)
	if guardPos == -1 {
		idx = strings.Index(patternAndGuard, " if ")
		if idx != -1 {
			// Found " if " - this could be the guard
			// Validate it's after a complete pattern by checking what comes before
			before := patternAndGuard[:idx]
			if r.isCompletePattern(before) {
				guardPos = idx
				guardKeywordLen = 4 // len(" if ")
			}
		}
	}

	if guardPos == -1 {
		// No guard found
		return strings.TrimSpace(patternAndGuard), ""
	}

	// Split at the guard keyword
	pattern = strings.TrimSpace(patternAndGuard[:guardPos])
	guard = strings.TrimSpace(patternAndGuard[guardPos+guardKeywordLen:])

	return pattern, guard
}

// isCompletePattern checks if a string looks like a complete pattern
// (ends with ) for binding patterns, or is a bare identifier/wildcard)
func (r *RustMatchProcessor) isCompletePattern(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}

	// Pattern with binding: Ok(x), Some(value), etc - must end with )
	if strings.Contains(trimmed, "(") {
		return strings.HasSuffix(trimmed, ")")
	}

	// Bare pattern: None, _, Active, etc - just an identifier
	// Check it's not part of a larger expression (no operators like <, >, &&, etc)
	return true
}

// generateSwitch generates Go switch statement with DINGO_MATCH markers
func (r *RustMatchProcessor) generateSwitch(scrutinee string, arms []patternArm, originalLine int, outputLine int, isInAssignment bool, assignmentVar string) (string, []Mapping) {
	var buf bytes.Buffer
	mappings := []Mapping{}

	matchID := r.matchCounter
	r.matchCounter++

	// Create temporary variable for scrutinee
	scrutineeVar := r.getScrutineeVarName(matchID)

	// Variable Hoisting Pattern: If in assignment context, declare result variable with proper type
	if isInAssignment && assignmentVar != "" {
		resultType := r.inferMatchResultType(arms)

		// Line 1: Declare result variable with proper type
		buf.WriteString(fmt.Sprintf("var %s %s\n", assignmentVar, resultType))
		mappings = append(mappings, Mapping{
			OriginalLine:    originalLine,
			OriginalColumn:  1,
			GeneratedLine:   outputLine,
			GeneratedColumn: 1,
			Length:          5,
			Name:            "rust_match",
		})
		outputLine++
	}

	// Line 2: DINGO_MATCH_START marker (MUST BE BEFORE temp var)
	buf.WriteString(fmt.Sprintf("// DINGO_MATCH_START: %s\n", scrutinee))
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          5,
		Name:            "rust_match",
	})
	outputLine++

	// Line 3: Store scrutinee in temporary variable
	buf.WriteString(fmt.Sprintf("%s := %s\n", scrutineeVar, scrutinee))
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          5, // "match"
		Name:            "rust_match",
	})
	outputLine++

	// Line 4: switch statement opening (tag-based switch - CORRECT pattern)
	buf.WriteString(fmt.Sprintf("switch %s.tag {\n", scrutineeVar))
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          5,
		Name:            "rust_match",
	})
	outputLine++

	// Group arms by pattern to handle guards correctly
	// Multiple guards on the same pattern become if-else chains within one case
	groupedArms := r.groupArmsByPattern(arms)

	// Generate case statements for each pattern group
	for _, armGroup := range groupedArms {
		caseLines, caseMappings := r.generateCaseWithGuards(scrutineeVar, armGroup, originalLine, outputLine, isInAssignment, assignmentVar)
		buf.WriteString(caseLines)
		mappings = append(mappings, caseMappings...)
		outputLine += strings.Count(caseLines, "\n")
	}

	// Closing brace for switch
	buf.WriteString("}\n")
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          1,
		Name:            "rust_match",
	})
	outputLine++

	// PRIORITY 3 FIX: Add panic for exhaustiveness (Go doesn't know switch is exhaustive)
	buf.WriteString("panic(\"unreachable: match is exhaustive\")\n")
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          5,
		Name:            "rust_match_panic",
	})
	outputLine++

	// If in assignment context with auto-generated variable (return match), add return statement
	if isInAssignment && assignmentVar != "" && r.isGeneratedResultVar(assignmentVar) {
		buf.WriteString(fmt.Sprintf("return %s\n", assignmentVar))
		mappings = append(mappings, Mapping{
			OriginalLine:    originalLine,
			OriginalColumn:  1,
			GeneratedLine:   outputLine,
			GeneratedColumn: 1,
			Length:          6, // "return"
			Name:            "rust_match_return",
		})
		outputLine++
	}

	// DINGO_MATCH_END marker
	buf.WriteString("// DINGO_MATCH_END\n")
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          1,
		Name:            "rust_match",
	})

	return buf.String(), mappings
}

// inferMatchResultType infers the result type from match arms
// For now, uses simple heuristics based on first arm's pattern
func (r *RustMatchProcessor) inferMatchResultType(arms []patternArm) string {
	if len(arms) == 0 {
		return "interface{}" // Fallback
	}

	// Look at first arm's pattern to infer type
	firstPattern := arms[0].pattern
	switch firstPattern {
	case "Ok", "Err":
		// Result type - need to infer T and E types
		// For now, return generic Result type placeholder
		// TODO: Parse arm expressions to infer exact types
		return "ResultIntError" // Simplified for now
	case "Some", "None":
		// Option type - need to infer T type
		return "OptionInt" // Simplified for now
	default:
		// Custom enum or unknown
		return "interface{}"
	}
}

// armGroup represents a group of arms with the same pattern (different guards)
type armGroup struct {
	pattern string       // The pattern name (Ok, Err, Some, None, etc.)
	arms    []patternArm // All arms with this pattern (may have different guards)
}

// groupArmsByPattern groups pattern arms by their pattern name
// This allows multiple guards on the same pattern to become if-else chains
func (r *RustMatchProcessor) groupArmsByPattern(arms []patternArm) []armGroup {
	groups := make(map[string]*armGroup)
	order := []string{} // Preserve order

	for _, arm := range arms {
		pattern := arm.pattern

		if _, exists := groups[pattern]; !exists {
			groups[pattern] = &armGroup{
				pattern: pattern,
				arms:    []patternArm{},
			}
			order = append(order, pattern)
		}

		groups[pattern].arms = append(groups[pattern].arms, arm)
	}

	// Return in original order
	result := make([]armGroup, 0, len(order))
	for _, pattern := range order {
		result = append(result, *groups[pattern])
	}

	return result
}

// generateCaseWithGuards generates a case statement with optional if-else guard chains
func (r *RustMatchProcessor) generateCaseWithGuards(scrutineeVar string, group armGroup, originalLine int, outputLine int, isInAssignment bool, assignmentVar string) (string, []Mapping) {
	var buf bytes.Buffer
	mappings := []Mapping{}

	// Handle wildcard pattern
	if group.pattern == "_" {
		buf.WriteString("default:\n")
		buf.WriteString("\t// DINGO_PATTERN: _\n")

		// Variable Hoisting: Assign to result variable if in assignment context
		if isInAssignment && assignmentVar != "" {
			buf.WriteString(fmt.Sprintf("\t%s = %s\n", assignmentVar, group.arms[0].expression))
		} else {
			buf.WriteString(fmt.Sprintf("\t%s\n", group.arms[0].expression))
		}

		mappings = append(mappings, Mapping{
			OriginalLine:    originalLine,
			OriginalColumn:  1,
			GeneratedLine:   outputLine,
			GeneratedColumn: 1,
			Length:          1,
			Name:            "rust_match_arm",
		})
		return buf.String(), mappings
	}

	// Generate case tag
	tagName := r.getTagName(group.pattern)
	buf.WriteString(fmt.Sprintf("case %s:\n", tagName))
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          len(group.pattern),
		Name:            "rust_match_arm",
	})
	outputLine++

	// Extract binding from first arm
	// PRIORITY 2 FIX: Handle simple nested patterns (one level deep)
	firstArm := group.arms[0]

	// Check if we have multiple arms with DIFFERENT bindings but NO guards
	// This indicates nested pattern syntax like: Result_Ok(Value_Int(n)), Result_Ok(Value_String(s))
	hasNestedPatterns := false
	if len(group.arms) > 1 {
		hasGuards := false
		bindingsDiffer := false
		for i, arm := range group.arms {
			if arm.guard != "" {
				hasGuards = true
			}
			if i > 0 && arm.binding != firstArm.binding && r.isNestedPatternBinding(arm.binding) {
				bindingsDiffer = true
			}
		}
		hasNestedPatterns = bindingsDiffer && !hasGuards
	}

	if hasNestedPatterns {
		// NESTED PATTERN CASE: Generate nested switch
		// Extract outer value and switch on inner patterns
		fieldName := r.getFieldName(group.pattern, 0)
		intermediateVar := fmt.Sprintf("nested%s", group.pattern)
		buf.WriteString(fmt.Sprintf("\t%s := *%s.%s\n", intermediateVar, scrutineeVar, fieldName))
		buf.WriteString(fmt.Sprintf("\tswitch %s.tag {\n", intermediateVar))

		// Group by inner pattern
		innerGroups := make(map[string][]patternArm)
		for _, arm := range group.arms {
			innerPattern, innerBinding := r.parseNestedPattern(arm.binding)
			innerGroups[innerPattern] = append(innerGroups[innerPattern], patternArm{
				pattern:    innerPattern,
				binding:    innerBinding,
				guard:      arm.guard,
				expression: arm.expression,
			})
		}

		// Generate cases for each inner pattern
		var sortedInner []string
		for p := range innerGroups {
			sortedInner = append(sortedInner, p)
		}
		sort.Strings(sortedInner)

		for _, innerPattern := range sortedInner {
			innerArms := innerGroups[innerPattern]
			innerTag := r.getTagName(innerPattern)
			buf.WriteString(fmt.Sprintf("\tcase %s:\n", innerTag))

			for _, arm := range innerArms {
				// Extract innermost binding
				if arm.binding != "" && arm.binding != "_" {
					bindingCode := r.generateBinding(intermediateVar, arm.pattern, arm.binding)
					buf.WriteString(fmt.Sprintf("\t\t%s\n", bindingCode))
				}

				// Expression
				if isInAssignment && assignmentVar != "" {
					buf.WriteString(fmt.Sprintf("\t\t%s = %s\n", assignmentVar, arm.expression))
				} else {
					// FIX T2B: Add return statement for expression-only arms
					exprTrimmed := strings.TrimSpace(arm.expression)
					needsReturn := !strings.HasPrefix(exprTrimmed, "return ") &&
						!strings.HasPrefix(exprTrimmed, "panic(") &&
						!strings.HasPrefix(exprTrimmed, "break") &&
						!strings.HasPrefix(exprTrimmed, "continue") &&
						!strings.HasPrefix(exprTrimmed, "{")

					if needsReturn {
						buf.WriteString(fmt.Sprintf("\t\treturn %s\n", arm.expression))
					} else {
						buf.WriteString(fmt.Sprintf("\t\t%s\n", arm.expression))
					}
				}
			}
		}

		buf.WriteString("\t}\n")
		outputLine += strings.Count(buf.String(), "\n")
	} else {
		// NORMAL CASE: Simple binding or guards on same pattern
		if firstArm.binding != "" && firstArm.binding != "_" {
			// Generate binding extraction with CORRECTED field name
			bindingCode := r.generateBinding(scrutineeVar, group.pattern, firstArm.binding)
			buf.WriteString(fmt.Sprintf("\t%s\n", bindingCode))
			mappings = append(mappings, Mapping{
				OriginalLine:    originalLine,
				OriginalColumn:  1,
				GeneratedLine:   outputLine,
				GeneratedColumn: 1,
				Length:          len(firstArm.binding),
				Name:            "rust_match_binding",
			})
			outputLine++
		}
	}

	// Generate if-else chain for guards (SKIP if nested patterns already handled)
	if hasNestedPatterns {
		// Nested patterns already generated, skip guard loop
		return buf.String(), mappings
	}

	for i, arm := range group.arms {
		// DINGO_PATTERN marker
		patternStr := arm.pattern
		if arm.binding != "" {
			patternStr = fmt.Sprintf("%s(%s)", arm.pattern, arm.binding)
		}
		buf.WriteString(fmt.Sprintf("\t// DINGO_PATTERN: %s", patternStr))

		// Add DINGO_GUARD marker if guard present
		if arm.guard != "" {
			buf.WriteString(fmt.Sprintf(" | DINGO_GUARD: %s", arm.guard))
		}
		buf.WriteString("\n")

		mappings = append(mappings, Mapping{
			OriginalLine:    originalLine,
			OriginalColumn:  1,
			GeneratedLine:   outputLine,
			GeneratedColumn: 1,
			Length:          len(patternStr),
			Name:            "rust_match_arm",
		})
		outputLine++

		// Generate if/else if/else for guard
		exprStr := arm.expression

		if arm.guard != "" {
			// Has guard: wrap in if statement
			if i == 0 {
				buf.WriteString(fmt.Sprintf("\tif %s {\n", arm.guard))
			} else {
				buf.WriteString(fmt.Sprintf("\t} else if %s {\n", arm.guard))
			}

			// Expression body (indented)
			if isInAssignment && assignmentVar != "" {
				buf.WriteString(fmt.Sprintf("\t\t%s = %s\n", assignmentVar, exprStr))
			} else {
				// FIX T2B: Add return statement for expression-only arms
				exprTrimmed := strings.TrimSpace(exprStr)
				needsReturn := !strings.HasPrefix(exprTrimmed, "return ") &&
					!strings.HasPrefix(exprTrimmed, "panic(") &&
					!strings.HasPrefix(exprTrimmed, "break") &&
					!strings.HasPrefix(exprTrimmed, "continue") &&
					!strings.HasPrefix(exprTrimmed, "{")

				if needsReturn {
					buf.WriteString(fmt.Sprintf("\t\treturn %s\n", exprStr))
				} else {
					buf.WriteString(fmt.Sprintf("\t\t%s\n", exprStr))
				}
			}
		} else {
			// No guard: this is the else clause (or standalone if no other guards)
			if i > 0 {
				buf.WriteString("\t} else {\n")
			}

			// Expression body
			indent := "\t"
			if i > 0 {
				indent = "\t\t" // Inside else block
			}

			if isInAssignment && assignmentVar != "" {
				buf.WriteString(fmt.Sprintf("%s%s = %s\n", indent, assignmentVar, exprStr))
			} else {
				// FIX T2B: Add return statement for expression-only arms
				// If expression doesn't already start with return/panic/break/continue, add return
				exprTrimmed := strings.TrimSpace(exprStr)
				needsReturn := !strings.HasPrefix(exprTrimmed, "return ") &&
					!strings.HasPrefix(exprTrimmed, "panic(") &&
					!strings.HasPrefix(exprTrimmed, "break") &&
					!strings.HasPrefix(exprTrimmed, "continue") &&
					!strings.HasPrefix(exprTrimmed, "{") // Block expressions handle their own returns

				if needsReturn {
					buf.WriteString(fmt.Sprintf("%sreturn %s\n", indent, exprStr))
				} else {
					buf.WriteString(fmt.Sprintf("%s%s\n", indent, exprStr))
				}
			}

			// Close else block if we had previous guards
			if i > 0 {
				buf.WriteString("\t}\n")
			}
		}
	}

	// Close the final if/else if chain if there were guards
	if len(group.arms) > 0 && group.arms[len(group.arms)-1].guard != "" {
		buf.WriteString("\t}\n")
	}

	return buf.String(), mappings
}

// generateCase generates a single case statement
func (r *RustMatchProcessor) generateCase(scrutineeVar string, arm patternArm, originalLine int, outputLine int, isInAssignment bool, assignmentVar string) (string, []Mapping) {
	var buf bytes.Buffer
	mappings := []Mapping{}

	// Handle wildcard pattern
	if arm.pattern == "_" {
		buf.WriteString("default:\n")
		buf.WriteString(fmt.Sprintf("\t// DINGO_PATTERN: _\n"))

		// Variable Hoisting: Assign to result variable if in assignment context
		if isInAssignment && assignmentVar != "" {
			buf.WriteString(fmt.Sprintf("\t%s = %s\n", assignmentVar, arm.expression))
		} else {
			// FIX T2B: Add return statement for expression-only arms
			exprTrimmed := strings.TrimSpace(arm.expression)
			needsReturn := !strings.HasPrefix(exprTrimmed, "return ") &&
				!strings.HasPrefix(exprTrimmed, "panic(") &&
				!strings.HasPrefix(exprTrimmed, "break") &&
				!strings.HasPrefix(exprTrimmed, "continue") &&
				!strings.HasPrefix(exprTrimmed, "{")

			if needsReturn {
				buf.WriteString(fmt.Sprintf("\treturn %s\n", arm.expression))
			} else {
				buf.WriteString(fmt.Sprintf("\t%s\n", arm.expression))
			}
		}

		mappings = append(mappings, Mapping{
			OriginalLine:    originalLine,
			OriginalColumn:  1,
			GeneratedLine:   outputLine,
			GeneratedColumn: 1,
			Length:          1,
			Name:            "rust_match_arm",
		})
		return buf.String(), mappings
	}

	// Generate case tag (tag-based case - CORRECT pattern)
	tagName := r.getTagName(arm.pattern)
	buf.WriteString(fmt.Sprintf("case %s:\n", tagName))
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          len(arm.pattern),
		Name:            "rust_match_arm",
	})
	outputLine++

	// DINGO_PATTERN marker
	patternStr := arm.pattern
	if arm.binding != "" {
		patternStr = fmt.Sprintf("%s(%s)", arm.pattern, arm.binding)
	}
	buf.WriteString(fmt.Sprintf("\t// DINGO_PATTERN: %s", patternStr))

	// Add DINGO_GUARD marker if guard present
	if arm.guard != "" {
		buf.WriteString(fmt.Sprintf(" | DINGO_GUARD: %s", arm.guard))
	}
	buf.WriteString("\n")

	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          len(patternStr),
		Name:            "rust_match_arm",
	})
	outputLine++

	// Extract binding if present
	if arm.binding != "" {
		// Generate binding extraction
		// For Ok(x): x := *scrutinee.ok_0
		// For Err(e): e := scrutinee.err_0
		// For Some(v): v := *scrutinee.some_0
		bindingCode := r.generateBinding(scrutineeVar, arm.pattern, arm.binding)
		buf.WriteString(fmt.Sprintf("\t%s\n", bindingCode))
		mappings = append(mappings, Mapping{
			OriginalLine:    originalLine,
			OriginalColumn:  1,
			GeneratedLine:   outputLine,
			GeneratedColumn: 1,
			Length:          len(arm.binding),
			Name:            "rust_match_binding",
		})
		outputLine++
	}

	// Pattern arm expression (C7 FIX: Handle block expressions properly)
	// GUARD TRANSFORMATION: Wrap body in if statement if guard present
	exprStr := arm.expression

	if arm.guard != "" {
		// GUARD CASE: Wrap expression in if statement
		buf.WriteString(fmt.Sprintf("\tif %s {\n", arm.guard))

		if strings.HasPrefix(exprStr, "{") && strings.HasSuffix(exprStr, "}") {
			// Block expression: remove outer braces and preserve formatting
			innerBlock := strings.TrimSpace(exprStr[1 : len(exprStr)-1])
			formatted := r.formatBlockStatements(innerBlock)
			for _, line := range strings.Split(formatted, "\n") {
				if trimmed := strings.TrimSpace(line); trimmed != "" {
					buf.WriteString(fmt.Sprintf("\t\t%s\n", trimmed))
				}
			}
		} else {
			// Simple expression
			if isInAssignment && assignmentVar != "" {
				// Variable Hoisting: Assign to result variable instead of returning
				buf.WriteString(fmt.Sprintf("\t\t%s = %s\n", assignmentVar, exprStr))
			} else {
				// Not in assignment context: keep expression as-is
				buf.WriteString(fmt.Sprintf("\t\t%s\n", exprStr))
			}
		}

		buf.WriteString("\t}\n")
	} else {
		// NO GUARD: Normal case body
		if strings.HasPrefix(exprStr, "{") && strings.HasSuffix(exprStr, "}") {
			// Block expression: remove outer braces and preserve formatting
			innerBlock := strings.TrimSpace(exprStr[1 : len(exprStr)-1])
			formatted := r.formatBlockStatements(innerBlock)
			for _, line := range strings.Split(formatted, "\n") {
				if trimmed := strings.TrimSpace(line); trimmed != "" {
					buf.WriteString(fmt.Sprintf("\t%s\n", trimmed))
				}
			}
		} else {
			// Simple expression
			if isInAssignment && assignmentVar != "" {
				// Variable Hoisting: Assign to result variable instead of returning
				buf.WriteString(fmt.Sprintf("\t%s = %s\n", assignmentVar, exprStr))
			} else {
				// Not in assignment context: keep expression as-is
				buf.WriteString(fmt.Sprintf("\t%s\n", exprStr))
			}
		}
	}

	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          len(arm.expression),
		Name:            "rust_match_expr",
	})

	return buf.String(), mappings
}

// getTagName converts pattern name to Go tag constant name (CamelCase)
// Ok → ResultTagOk, Err → ResultTagErr, Some → OptionTagSome, None → OptionTagNone
// ResultOk → ResultTagOk, OptionSome → OptionTagSome (constructor-style names)
// Status_Pending → StatusTagPending (for custom enums)
func (r *RustMatchProcessor) getTagName(pattern string) string {
	switch pattern {
	case "Ok":
		return "ResultTagOk"
	case "Err":
		return "ResultTagErr"
	case "Some":
		return "OptionTagSome"
	case "None":
		return "OptionTagNone"
	// Constructor-style names: ResultOk, ResultErr, OptionSome, OptionNone
	case "ResultOk":
		return "ResultTagOk"
	case "ResultErr":
		return "ResultTagErr"
	case "OptionSome":
		return "OptionTagSome"
	case "OptionNone":
		return "OptionTagNone"
	default:
		// Custom enum variant: EnumName_Variant → EnumNameTagVariant
		// Example: Value_Int → ValueTagInt
		if idx := strings.Index(pattern, "_"); idx > 0 {
			enumName := pattern[:idx]
			variantName := pattern[idx+1:]
			return enumName + "Tag" + variantName
		}
		// Bare variant name (shouldn't happen in well-formed Dingo code)
		return pattern + "Tag"
	}
}

// generateBinding generates binding extraction code (CamelCase field names)
func (r *RustMatchProcessor) generateBinding(scrutinee string, pattern string, binding string) string {
	switch pattern {
	case "Ok", "ResultOk":
		// For Result<T,E>, Ok value is stored in ok field (pointer to T)
		return fmt.Sprintf("%s := *%s.ok", binding, scrutinee)
	case "Err", "ResultErr":
		// For Result<T,E>, Err value is stored in err field (pointer to E)
		return fmt.Sprintf("%s := *%s.err", binding, scrutinee)
	case "Some", "OptionSome":
		// For Option<T>, Some value is stored in some field (pointer to T)
		return fmt.Sprintf("%s := *%s.some", binding, scrutinee)
	case "None", "OptionNone":
		// None has no value to extract
		return ""
	default:
		// Custom enum variant: check if this is a multi-field tuple binding
		// Multi-field tuple bindings contain commas, e.g., "path, body"
		if strings.Contains(binding, ",") {
			// Multi-field tuple: split binding and generate extraction for each field
			bindings := strings.Split(binding, ",")
			var extractions []string

			// Extract base variant name
			variantName := pattern
			if idx := strings.LastIndex(pattern, "_"); idx != -1 {
				variantName = pattern[idx+1:]
			}
			baseName := strings.ToLower(variantName)

			for i, b := range bindings {
				b = strings.TrimSpace(b)
				if b == "_" {
					continue // Skip wildcard bindings
				}
				// Use correct naming: first field no suffix, then 1, 2, 3
				var fieldName string
				if i == 0 {
					fieldName = baseName
				} else {
					fieldName = fmt.Sprintf("%s%d", baseName, i)
				}
				extractions = append(extractions, fmt.Sprintf("%s := *%s.%s", b, scrutinee, fieldName))
			}
			return strings.Join(extractions, "\n\t")
		}

		// Single-field tuple: extract variant name and lowercase it
		// Pattern may be "Request_Get" → field name is "get"
		variantName := pattern
		if idx := strings.LastIndex(pattern, "_"); idx != -1 {
			// Extract variant after last underscore: "Request_Get" -> "Get"
			variantName = pattern[idx+1:]
		}
		fieldName := strings.ToLower(variantName)

		// Check if field is a pointer (most custom enum fields are pointers)
		// For now, assume pointer dereference for custom enums with bindings
		if binding != "_" {
			return fmt.Sprintf("%s := *%s.%s", binding, scrutinee, fieldName)
		}
		return fmt.Sprintf("%s := %s.%s", binding, scrutinee, fieldName)
	}
}

// generateTupleBinding generates binding extraction code for tuple patterns (CamelCase)
// BUG FIX #2: New function to extract tuple element values
// Examples:
//   elemVar="elem0", variant="Ok", binding="x"
//   -> "x := *elem0.ok"
func (r *RustMatchProcessor) generateTupleBinding(elemVar string, variant string, binding string) string {
	switch variant {
	case "Ok", "ResultOk":
		// For Result<T,E>, Ok value is stored in ok field (pointer to T)
		return fmt.Sprintf("%s := *%s.ok", binding, elemVar)
	case "Err", "ResultErr":
		// For Result<T,E>, Err value is stored in err field (pointer to E)
		return fmt.Sprintf("%s := *%s.err", binding, elemVar)
	case "Some", "OptionSome":
		// For Option<T>, Some value is stored in some field (pointer to T)
		return fmt.Sprintf("%s := *%s.some", binding, elemVar)
	case "None", "OptionNone":
		// None has no value to extract
		return ""
	default:
		// Custom enum variant: lowercase field name
		// Example: Status_Pending -> "pending"
		variantName := variant
		if idx := strings.LastIndex(variant, "_"); idx != -1 {
			variantName = variant[idx+1:]
		}
		fieldName := strings.ToLower(variantName)
		return fmt.Sprintf("%s := *%s.%s", binding, elemVar, fieldName)
	}
}

// formatBlockStatements formats block statements preserving newlines
func (r *RustMatchProcessor) formatBlockStatements(block string) string {
	// Newlines are now preserved, just return the block as-is
	return block
}

// GetNeededImports implements the ImportProvider interface
func (r *RustMatchProcessor) GetNeededImports() []string {
	// Rust match syntax doesn't require additional imports
	return []string{}
}

// detectTuple checks if scrutinee is a tuple expression: (expr1, expr2, ...)
// Returns: (isTuple, elements, error)
func (r *RustMatchProcessor) detectTuple(scrutinee string) (bool, []string, error) {
	trimmed := strings.TrimSpace(scrutinee)

	// Must start/end with parens
	if !strings.HasPrefix(trimmed, "(") || !strings.HasSuffix(trimmed, ")") {
		return false, nil, nil // Not a tuple
	}

	// Parse elements
	inner := trimmed[1 : len(trimmed)-1]
	elements := r.splitTupleElements(inner)

	// Enforce 6-element limit (USER DECISION)
	if len(elements) > 6 {
		return false, nil, fmt.Errorf(
			"tuple patterns limited to 6 elements (found %d)",
			len(elements),
		)
	}

	// Must have at least 2 elements to be a tuple
	if len(elements) < 2 {
		return false, nil, nil
	}

	return true, elements, nil
}

// splitTupleElements splits tuple elements on commas (respects nested parens/brackets)
func (r *RustMatchProcessor) splitTupleElements(s string) []string {
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

// tuplePatternArm represents a tuple pattern arm
type tuplePatternArm struct {
	patterns   []tupleElementPattern // One per tuple element
	guard      string                // Guard condition (optional)
	expression string                // Expression to execute
}

// tupleElementPattern represents one element in a tuple pattern
type tupleElementPattern struct {
	variant string // Ok, Err, Some, None, _ (wildcard)
	binding string // x, e, v (optional - empty for None/_)
}

// parseTupleArms parses tuple pattern arms from match body
// Example: (Ok(x), Err(e)) => expr1, (Ok(a), Ok(b)) if guard => expr2
func (r *RustMatchProcessor) parseTupleArms(armsText string) ([]tuplePatternArm, error) {
	arms := []tuplePatternArm{}
	text := strings.TrimSpace(armsText)

	i := 0
	for i < len(text) {
		// Skip whitespace
		for i < len(text) && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n' || text[i] == '\r') {
			i++
		}
		if i >= len(text) {
			break
		}

		// Expect tuple pattern: (Pattern1, Pattern2, ...)
		if text[i] != '(' {
			return nil, fmt.Errorf("expected tuple pattern at position %d", i)
		}

		// Find matching close paren
		parenDepth := 1
		tupleStart := i
		i++
		for i < len(text) && parenDepth > 0 {
			if text[i] == '(' {
				parenDepth++
			} else if text[i] == ')' {
				parenDepth--
			}
			i++
		}
		tuplePatternStr := text[tupleStart:i]

		// Parse tuple elements
		tupleElements, err := r.parseTuplePattern(tuplePatternStr)
		if err != nil {
			return nil, fmt.Errorf("parsing tuple pattern: %w", err)
		}

		// Skip whitespace
		for i < len(text) && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n' || text[i] == '\r') {
			i++
		}

		// Check for guard (if)
		guard := ""
		if i < len(text) && strings.HasPrefix(text[i:], "if ") {
			i += 3 // skip "if "

			// Extract guard condition (until =>)
			arrowPos := strings.Index(text[i:], "=>")
			if arrowPos == -1 {
				return nil, fmt.Errorf("expected => after guard")
			}
			guard = strings.TrimSpace(text[i : i+arrowPos])
			i += arrowPos
		}

		// Expect =>
		if !strings.HasPrefix(text[i:], "=>") {
			return nil, fmt.Errorf("expected => at position %d", i)
		}
		i += 2

		// Skip whitespace after =>
		for i < len(text) && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n' || text[i] == '\r') {
			i++
		}

		// Extract expression (until comma or end)
		var expr string
		if text[i] == '{' {
			// Block expression
			braceCount := 1
			start := i
			i++
			for i < len(text) && braceCount > 0 {
				if text[i] == '{' {
					braceCount++
				} else if text[i] == '}' {
					braceCount--
				}
				i++
			}
			expr = strings.TrimSpace(text[start:i])
		} else {
			// Simple expression - find comma or end (respecting strings and nesting)
			start := i
			exprEnd := r.findExpressionEnd(text, i)
			expr = strings.TrimSpace(text[start:exprEnd])
			i = exprEnd
		}

		// Skip comma
		if i < len(text) && text[i] == ',' {
			i++
		}

		arms = append(arms, tuplePatternArm{
			patterns:   tupleElements,
			guard:      guard,
			expression: expr,
		})
	}

	if len(arms) == 0 {
		return nil, fmt.Errorf("no tuple pattern arms found")
	}

	return arms, nil
}

// parseTuplePattern parses a single tuple pattern: (Ok(x), Err(e), _)
func (r *RustMatchProcessor) parseTuplePattern(tupleStr string) ([]tupleElementPattern, error) {
	// Remove outer parens
	tupleStr = strings.TrimSpace(tupleStr)
	if !strings.HasPrefix(tupleStr, "(") || !strings.HasSuffix(tupleStr, ")") {
		return nil, fmt.Errorf("invalid tuple pattern: %s", tupleStr)
	}
	inner := tupleStr[1 : len(tupleStr)-1]

	// Split on commas (respecting nested parens)
	elementStrs := r.splitTupleElements(inner)

	elements := make([]tupleElementPattern, len(elementStrs))
	for i, elemStr := range elementStrs {
		elemStr = strings.TrimSpace(elemStr)

		// Wildcard
		if elemStr == "_" {
			elements[i] = tupleElementPattern{
				variant: "_",
				binding: "",
			}
			continue
		}

		// Pattern with binding: Ok(x), Err(e), Some(v)
		if strings.Contains(elemStr, "(") {
			start := strings.Index(elemStr, "(")
			end := strings.Index(elemStr, ")")
			if end <= start {
				return nil, fmt.Errorf("invalid pattern: %s", elemStr)
			}
			variant := strings.TrimSpace(elemStr[:start])
			binding := strings.TrimSpace(elemStr[start+1 : end])
			elements[i] = tupleElementPattern{
				variant: variant,
				binding: binding,
			}
		} else {
			// Pattern without binding: None
			elements[i] = tupleElementPattern{
				variant: elemStr,
				binding: "",
			}
		}
	}

	return elements, nil
}

// generateTupleMatch generates Go code for tuple pattern matching
func (r *RustMatchProcessor) generateTupleMatch(tupleElements []string, arms []tuplePatternArm, originalLine int, outputLine int) (string, []Mapping) {
	var buf bytes.Buffer
	mappings := []Mapping{}

	r.matchCounter++

	arity := len(tupleElements)

	// Line 1: DINGO_MATCH_START marker
	scrutineeRepr := "(" + strings.Join(tupleElements, ", ") + ")"
	buf.WriteString(fmt.Sprintf("// DINGO_MATCH_START: %s\n", scrutineeRepr))
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          5,
		Name:            "rust_match",
	})
	outputLine++

	// Line 2: Extract tuple elements into temp vars
	var elemVars []string
	for i := 0; i < arity; i++ {
		elemVars = append(elemVars, fmt.Sprintf("elem%d", i))
	}
	buf.WriteString(fmt.Sprintf("%s := %s\n",
		strings.Join(elemVars, ", "),
		strings.Join(tupleElements, ", "),
	))
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          5,
		Name:            "rust_match",
	})
	outputLine++

	// Generate DINGO_TUPLE_PATTERN marker with pattern summary
	patternSummary := r.generateTuplePatternSummary(arms)
	buf.WriteString(fmt.Sprintf("// DINGO_TUPLE_PATTERN: %s | ARITY: %d\n", patternSummary, arity))
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          5,
		Name:            "rust_match",
	})
	outputLine++

	// BUG FIX #2 & #4: Generate NESTED switches for tuple patterns
	// This prevents duplicate case errors and properly handles all combinations
	nestedSwitch, nestedMappings := r.generateNestedTupleSwitches(elemVars, arms, originalLine, outputLine)
	buf.WriteString(nestedSwitch)
	mappings = append(mappings, nestedMappings...)
	outputLine += strings.Count(nestedSwitch, "\n")

	// DINGO_MATCH_END marker
	buf.WriteString("// DINGO_MATCH_END\n")
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          1,
		Name:            "rust_match",
	})

	return buf.String(), mappings
}

// generateTuplePatternSummary creates a summary string for DINGO_TUPLE_PATTERN marker
// Example: (Ok, Ok) | (Ok, Err) | (Err, _)
func (r *RustMatchProcessor) generateTuplePatternSummary(arms []tuplePatternArm) string {
	var patterns []string
	for _, arm := range arms {
		var variants []string
		for _, elem := range arm.patterns {
			variants = append(variants, elem.variant)
		}
		patterns = append(patterns, "("+strings.Join(variants, ", ")+")")
	}
	return strings.Join(patterns, " | ")
}

// generateNestedTupleSwitches generates nested switch statements for tuple patterns
// BUG FIX #2 & #4: Creates proper nested switches to avoid duplicate cases
// Example for 2-tuple:
//   switch elem0.tag {
//   case Tag0:
//     switch elem1.tag {
//     case Tag1: ...
//     }
//   }
func (r *RustMatchProcessor) generateNestedTupleSwitches(elemVars []string, arms []tuplePatternArm, originalLine int, outputLine int) (string, []Mapping) {
	var buf bytes.Buffer
	mappings := []Mapping{}

	// Group arms by first element pattern
	groupedArms := make(map[string][]tuplePatternArm)
	for _, arm := range arms {
		firstVariant := arm.patterns[0].variant
		groupedArms[firstVariant] = append(groupedArms[firstVariant], arm)
	}

	// Generate outer switch on first element
	buf.WriteString(fmt.Sprintf("switch %s.tag {\n", elemVars[0]))
	outputLine++

	// PRIORITY 1 FIX: Sort keys for deterministic output
	// Collect variants and sort them (wildcards last)
	var sortedVariants []string
	for variant := range groupedArms {
		sortedVariants = append(sortedVariants, variant)
	}
	sortVariantsInPlace(sortedVariants)

	// Generate cases for each unique first element
	for _, firstVariant := range sortedVariants {
		matchingArms := groupedArms[firstVariant]
		if firstVariant == "_" {
			// Wildcard case
			buf.WriteString("default:\n")
		} else {
			// Specific variant case
			tagName := r.getTagName(firstVariant)
			buf.WriteString(fmt.Sprintf("case %s:\n", tagName))
		}
		outputLine++

		// If tuple has more than 1 element, generate nested switch
		if len(elemVars) > 1 {
			nestedCode, nestedMappings := r.generateNestedSwitchLevel(elemVars, matchingArms, 1, originalLine, outputLine, "\t")
			buf.WriteString(nestedCode)
			mappings = append(mappings, nestedMappings...)
			outputLine += strings.Count(nestedCode, "\n")
		} else {
			// Single element tuple (shouldn't happen, but handle gracefully)
			for _, arm := range matchingArms {
				buf.WriteString(r.generateTupleArmBody(elemVars, arm, "\t"))
				outputLine++
			}
		}
	}

	// Close outer switch
	buf.WriteString("}\n")
	outputLine++

	// Add panic for exhaustiveness (Go doesn't know switch is exhaustive)
	buf.WriteString("\tpanic(\"unreachable: match is exhaustive\")\n")
	outputLine++

	return buf.String(), mappings
}

// generateNestedSwitchLevel generates switch statement for tuple element at given depth
// depth=1 means second element, depth=2 means third element, etc.
func (r *RustMatchProcessor) generateNestedSwitchLevel(elemVars []string, arms []tuplePatternArm, depth int, originalLine int, outputLine int, indent string) (string, []Mapping) {
	var buf bytes.Buffer
	mappings := []Mapping{}

	// Group arms by pattern at this depth
	groupedArms := make(map[string][]tuplePatternArm)
	for _, arm := range arms {
		variant := arm.patterns[depth].variant
		groupedArms[variant] = append(groupedArms[variant], arm)
	}

	// If this is the last element, generate arm bodies directly
	if depth == len(elemVars)-1 {
		// Last level - generate switch with arm bodies
		buf.WriteString(fmt.Sprintf("%sswitch %s.tag {\n", indent, elemVars[depth]))
		outputLine++

		// PRIORITY 1 FIX: Sort keys for deterministic output
		var sortedVariants []string
		for variant := range groupedArms {
			sortedVariants = append(sortedVariants, variant)
		}
		sortVariantsInPlace(sortedVariants)

		for _, variant := range sortedVariants {
			matchingArms := groupedArms[variant]
			if variant == "_" {
				buf.WriteString(fmt.Sprintf("%sdefault:\n", indent))
			} else {
				tagName := r.getTagName(variant)
				buf.WriteString(fmt.Sprintf("%scase %s:\n", indent, tagName))
			}
			outputLine++

			// Generate arm body (bindings + expression)
			for _, arm := range matchingArms {
				body := r.generateTupleArmBody(elemVars, arm, indent+"\t")
				buf.WriteString(body)
				outputLine += strings.Count(body, "\n")
			}
		}

		buf.WriteString(fmt.Sprintf("%s}\n", indent))
		outputLine++
	} else {
		// Not last level - generate switch with nested switches
		buf.WriteString(fmt.Sprintf("%sswitch %s.tag {\n", indent, elemVars[depth]))
		outputLine++

		// PRIORITY 1 FIX: Sort keys for deterministic output
		var sortedVariants []string
		for variant := range groupedArms {
			sortedVariants = append(sortedVariants, variant)
		}
		sortVariantsInPlace(sortedVariants)

		for _, variant := range sortedVariants {
			matchingArms := groupedArms[variant]
			if variant == "_" {
				buf.WriteString(fmt.Sprintf("%sdefault:\n", indent))
			} else {
				tagName := r.getTagName(variant)
				buf.WriteString(fmt.Sprintf("%scase %s:\n", indent, tagName))
			}
			outputLine++

			// Recurse to next depth
			nestedCode, nestedMappings := r.generateNestedSwitchLevel(elemVars, matchingArms, depth+1, originalLine, outputLine, indent+"\t")
			buf.WriteString(nestedCode)
			mappings = append(mappings, nestedMappings...)
			outputLine += strings.Count(nestedCode, "\n")
		}

		buf.WriteString(fmt.Sprintf("%s}\n", indent))
		outputLine++
	}

	return buf.String(), mappings
}

// generateTupleArmBody generates the body of a tuple pattern arm (bindings + expression)
func (r *RustMatchProcessor) generateTupleArmBody(elemVars []string, arm tuplePatternArm, indent string) string {
	var buf bytes.Buffer

	// Add DINGO_TUPLE_ARM marker
	var patternStrs []string
	for _, elem := range arm.patterns {
		if elem.binding != "" {
			patternStrs = append(patternStrs, fmt.Sprintf("%s(%s)", elem.variant, elem.binding))
		} else {
			patternStrs = append(patternStrs, elem.variant)
		}
	}
	patternRepr := "(" + strings.Join(patternStrs, ", ") + ")"
	buf.WriteString(fmt.Sprintf("%s// DINGO_TUPLE_ARM: %s", indent, patternRepr))
	if arm.guard != "" {
		buf.WriteString(fmt.Sprintf(" | DINGO_GUARD: %s", arm.guard))
	}
	buf.WriteString("\n")

	// Generate variable bindings for all tuple elements
	for i, elem := range arm.patterns {
		if elem.binding != "" && elem.variant != "_" {
			bindingCode := r.generateTupleBinding(elemVars[i], elem.variant, elem.binding)
			if bindingCode != "" {
				buf.WriteString(fmt.Sprintf("%s%s\n", indent, bindingCode))
			}
		}
	}

	// Add expression (with return statement for expression mode)
	// BUG FIX: Add "return" for match expressions
	// Check if expression is a block or simple expression
	expr := strings.TrimSpace(arm.expression)
	if strings.HasPrefix(expr, "{") && strings.HasSuffix(expr, "}") {
		// Block expression - preserve as-is
		buf.WriteString(fmt.Sprintf("%s%s\n", indent, expr))
	} else {
		// Simple expression - add return statement
		buf.WriteString(fmt.Sprintf("%sreturn %s\n", indent, expr))
	}

	return buf.String()
}

// generateTupleCase generates code for one tuple pattern arm
// BUG FIX #2: Generate variable bindings directly (don't delegate to plugin)
func (r *RustMatchProcessor) generateTupleCase(elemVars []string, arm tuplePatternArm, originalLine int, outputLine int) (string, []Mapping) {
	var buf bytes.Buffer
	mappings := []Mapping{}

	// Generate case for first element only (plugin will expand to nested switches)
	firstElem := arm.patterns[0]

	if firstElem.variant == "_" {
		// Wildcard - default case
		buf.WriteString("default:\n")
	} else {
		// Specific variant
		tagName := r.getTagName(firstElem.variant)
		buf.WriteString(fmt.Sprintf("case %s:\n", tagName))
	}

	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          4,
		Name:            "rust_match_arm",
	})
	outputLine++

	// Add DINGO_TUPLE_ARM marker with full pattern info
	var patternStrs []string
	for _, elem := range arm.patterns {
		if elem.binding != "" {
			patternStrs = append(patternStrs, fmt.Sprintf("%s(%s)", elem.variant, elem.binding))
		} else {
			patternStrs = append(patternStrs, elem.variant)
		}
	}
	patternRepr := "(" + strings.Join(patternStrs, ", ") + ")"

	buf.WriteString(fmt.Sprintf("\t// DINGO_TUPLE_ARM: %s", patternRepr))
	if arm.guard != "" {
		buf.WriteString(fmt.Sprintf(" | DINGO_GUARD: %s", arm.guard))
	}
	buf.WriteString("\n")

	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          len(patternRepr),
		Name:            "rust_match_arm",
	})
	outputLine++

	// BUG FIX #2: Generate variable bindings for ALL tuple elements
	// This fixes the "undefined variable" errors in tuple patterns
	for i, elem := range arm.patterns {
		if elem.binding != "" && elem.variant != "_" {
			// Generate binding: x := *__match_0_elem0.ok_0
			bindingCode := r.generateTupleBinding(elemVars[i], elem.variant, elem.binding)
			buf.WriteString(fmt.Sprintf("\t%s\n", bindingCode))

			mappings = append(mappings, Mapping{
				OriginalLine:    originalLine,
				OriginalColumn:  1,
				GeneratedLine:   outputLine,
				GeneratedColumn: 1,
				Length:          len(elem.binding),
				Name:            "rust_match_binding",
			})
			outputLine++
		}
	}

	// Now add expression (variables are now defined!)
	buf.WriteString(fmt.Sprintf("\t%s\n", arm.expression))

	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          len(arm.expression),
		Name:            "rust_match_expr",
	})

	return buf.String(), mappings
}

// isNestedPatternBinding checks if a binding is itself a pattern (e.g., "Value_Int(n)")
func (r *RustMatchProcessor) isNestedPatternBinding(binding string) bool {
	if binding == "" || binding == "_" {
		return false
	}
	// Check if it has the form Constructor(...)
	return strings.Contains(binding, "(") && strings.Contains(binding, ")")
}

// parseNestedPattern parses a nested pattern like "Value_Int(n)" into ("Value_Int", "n")
func (r *RustMatchProcessor) parseNestedPattern(binding string) (pattern string, innerBinding string) {
	if !strings.Contains(binding, "(") {
		return "", ""
	}

	parenIdx := strings.Index(binding, "(")
	closeIdx := r.findMatchingCloseParen(binding, parenIdx)

	if closeIdx == -1 {
		return "", ""
	}

	pattern = binding[:parenIdx]
	innerBinding = strings.TrimSpace(binding[parenIdx+1 : closeIdx])
	return pattern, innerBinding
}

// getFieldName returns the field name for a pattern (CamelCase, e.g., "Ok" -> "ok", "Err" -> "err")
func (r *RustMatchProcessor) getFieldName(pattern string, fieldIndex int) string {
	switch pattern {
	case "Ok":
		return "ok"
	case "Err":
		return "err"
	case "Some":
		return "some"
	default:
		// Custom enum variant: use lowercase variant name for single-field tuples
		// For multi-field tuples, the caller should use _0, _1, etc. directly
		if idx := strings.LastIndex(pattern, "_"); idx != -1 {
			variantName := pattern[idx+1:]
			return strings.ToLower(variantName)
		}
		return strings.ToLower(pattern)
	}
}

// sortVariantsInPlace sorts variant names in-place for deterministic code generation
// PRIORITY 1 FIX: Ensures switch cases are generated in consistent order
// Sorting rules:
// 1. Named variants sorted alphabetically (Err, Ok, None, Some, etc.)
// 2. Wildcard (_) always last (becomes default case)
func sortVariantsInPlace(variants []string) {
	sort.Slice(variants, func(i, j int) bool {
		// Wildcards always go last
		if variants[i] == "_" {
			return false
		}
		if variants[j] == "_" {
			return true
		}
		// Otherwise, sort alphabetically
		return variants[i] < variants[j]
	})
}
