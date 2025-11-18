package preprocessor

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// SwiftMatchProcessor handles Swift-like pattern matching syntax
// Transforms: switch expr { case .Variant(let x) where guard: body } → Go switch statement with markers
// Normalizes to IDENTICAL markers as RustMatchProcessor - plugin sees no difference
type SwiftMatchProcessor struct {
	matchCounter int
	mappings     []Mapping
}

// Pattern-matching regex for Swift-like switch expressions
var (
	// Match the entire switch expression: switch expr { ... }
	// Using non-greedy (.+?) to match minimum content between braces
	switchExprPattern = regexp.MustCompile(`(?s)switch\s+([^{]+)\s*\{(.+?)\}`)
)

// NewSwiftMatchProcessor creates a new Swift-like match preprocessor
func NewSwiftMatchProcessor() *SwiftMatchProcessor {
	return &SwiftMatchProcessor{
		matchCounter: 0,
		mappings:     []Mapping{},
	}
}

// Name returns the processor name
func (s *SwiftMatchProcessor) Name() string {
	return "swift_match"
}

// Process transforms Swift-like switch expressions
func (s *SwiftMatchProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	s.mappings = []Mapping{}
	s.matchCounter = 0

	input := string(source)
	lines := strings.Split(input, "\n")

	var output bytes.Buffer
	inputLineNum := 0
	outputLineNum := 1

	for inputLineNum < len(lines) {
		line := lines[inputLineNum]

		// Check if this line starts a switch expression
		if strings.Contains(line, "switch ") {
			// Collect the entire switch expression (may span multiple lines)
			switchExpr, linesConsumed := s.collectSwitchExpression(lines, inputLineNum)
			if switchExpr != "" {
				// Transform the switch expression
				transformed, newMappings, err := s.transformSwitch(switchExpr, inputLineNum+1, outputLineNum)
				if err != nil {
					return nil, nil, fmt.Errorf("line %d: %w", inputLineNum+1, err)
				}

				output.WriteString(transformed)
				s.mappings = append(s.mappings, newMappings...)

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

		// Not a switch expression, pass through as-is
		output.WriteString(line)
		if inputLineNum < len(lines)-1 {
			output.WriteByte('\n')
		}
		inputLineNum++
		outputLineNum++
	}

	return output.Bytes(), s.mappings, nil
}

// collectSwitchExpression collects a complete switch expression across multiple lines
// Returns: (switchExpression, linesConsumed)
func (s *SwiftMatchProcessor) collectSwitchExpression(lines []string, startLine int) (string, int) {
	var buf bytes.Buffer
	braceDepth := 0
	linesConsumed := 0
	foundSwitch := false

	for i := startLine; i < len(lines); i++ {
		line := lines[i]
		buf.WriteString(line)
		linesConsumed++

		// Track brace depth
		for _, ch := range line {
			if ch == '{' {
				braceDepth++
				foundSwitch = true
			} else if ch == '}' {
				braceDepth--
				if braceDepth == 0 && foundSwitch {
					// Complete switch expression
					return buf.String(), linesConsumed
				}
			}
		}

		// Add newline if more lines to come
		if i < len(lines)-1 {
			buf.WriteByte('\n')
		}
	}

	// Incomplete switch expression (missing closing brace)
	return "", 0
}

// transformSwitch transforms a Swift-like switch expression to Go switch
func (s *SwiftMatchProcessor) transformSwitch(switchExpr string, originalLine int, outputLine int) (string, []Mapping, error) {
	// Extract scrutinee and cases by finding the opening brace
	// collectSwitchExpression has already ensured braces are balanced
	switchKeywordIdx := strings.Index(switchExpr, "switch ")
	if switchKeywordIdx == -1 {
		return "", nil, fmt.Errorf("invalid switch expression: no switch keyword")
	}

	openBraceIdx := strings.Index(switchExpr, "{")
	if openBraceIdx == -1 {
		return "", nil, fmt.Errorf("invalid switch expression: no opening brace")
	}

	// Extract scrutinee (between "switch" and "{")
	scrutineeStart := switchKeywordIdx + len("switch ")
	scrutinee := strings.TrimSpace(switchExpr[scrutineeStart:openBraceIdx])

	// Extract cases text (between { and final })
	// Find matching closing brace by counting depth
	braceDepth := 0
	closeBraceIdx := -1
	for i := openBraceIdx; i < len(switchExpr); i++ {
		if switchExpr[i] == '{' {
			braceDepth++
		} else if switchExpr[i] == '}' {
			braceDepth--
			if braceDepth == 0 {
				closeBraceIdx = i
				break
			}
		}
	}
	if closeBraceIdx == -1 {
		return "", nil, fmt.Errorf("invalid switch expression: no matching closing brace")
	}

	casesText := switchExpr[openBraceIdx+1 : closeBraceIdx]

	// Check if scrutinee is a tuple expression
	isTuple, tupleElements, err := s.detectTuple(scrutinee)
	if err != nil {
		return "", nil, err
	}

	if isTuple {
		// Parse tuple pattern cases
		tupleCases, err := s.parseTupleCases(casesText)
		if err != nil {
			return "", nil, fmt.Errorf("parsing tuple pattern cases: %w", err)
		}

		// Generate tuple match (elements extraction + pattern info)
		result, mappings := s.generateTupleMatch(tupleElements, tupleCases, originalLine, outputLine)
		return result, mappings, nil
	}

	// Parse case arms (non-tuple)
	cases, err := s.parseCases(casesText)
	if err != nil {
		return "", nil, fmt.Errorf("parsing case arms: %w", err)
	}

	// Generate Go switch statement (identical markers to RustMatchProcessor)
	result, mappings := s.generateSwitch(scrutinee, cases, originalLine, outputLine)
	return result, mappings, nil
}

// swiftCase represents a single Swift case arm
type swiftCase struct {
	variant    string // Ok, Err, Some, None
	binding    string // x, e, v, etc. (empty for None)
	guard      string // Guard condition: "x > 0" (optional, empty if no guard)
	expression string // the expression to execute
}

// parseCases parses case arms from the switch body
// Handles both bare statements and braced bodies:
//   case .Ok(let x): handleOk(x)
//   case .Ok(let x): { handleOk(x); return }
func (s *SwiftMatchProcessor) parseCases(casesText string) ([]swiftCase, error) {
	cases := []swiftCase{}
	text := strings.TrimSpace(casesText)

	// Parse case arms manually to handle nested braces and various patterns
	i := 0
	for i < len(text) {
		// Skip whitespace
		for i < len(text) && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n' || text[i] == '\r') {
			i++
		}
		if i >= len(text) {
			break
		}

		// Look for "case " keyword
		if !strings.HasPrefix(text[i:], "case ") {
			break
		}
		i += 5 // Skip "case "

		// Skip whitespace after "case"
		for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
			i++
		}

		// Expect dot prefix for Swift pattern
		if i >= len(text) || text[i] != '.' {
			return nil, fmt.Errorf("expected '.' prefix for Swift case pattern")
		}
		i++ // Skip '.'

		// Extract variant name (capitalized identifier)
		variantStart := i
		for i < len(text) && (isLetter(text[i]) || isDigit(text[i]) || text[i] == '_') {
			i++
		}
		if i == variantStart {
			return nil, fmt.Errorf("expected variant name after '.'")
		}
		variant := text[variantStart:i]

		// Check for binding: (let x)
		binding := ""
		for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
			i++
		}
		if i < len(text) && text[i] == '(' {
			// Has binding
			i++ // Skip '('

			// Skip whitespace
			for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
				i++
			}

			// Expect "let"
			if strings.HasPrefix(text[i:], "let ") {
				i += 4 // Skip "let "

				// Skip whitespace
				for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
					i++
				}

				// Extract binding identifier
				bindingStart := i
				for i < len(text) && (isLetter(text[i]) || isDigit(text[i]) || text[i] == '_') {
					i++
				}
				binding = text[bindingStart:i]

				// Skip whitespace and closing paren
				for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
					i++
				}
				if i >= len(text) || text[i] != ')' {
					return nil, fmt.Errorf("expected ')' after binding")
				}
				i++ // Skip ')'
			}
		}

		// Check for guard: where/if condition
		guard := ""
		for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
			i++
		}

		if strings.HasPrefix(text[i:], "where ") || strings.HasPrefix(text[i:], "if ") {
			// Extract guard keyword length
			guardKeywordLen := 6 // "where "
			if strings.HasPrefix(text[i:], "if ") {
				guardKeywordLen = 3 // "if "
			}
			i += guardKeywordLen

			// Extract guard condition (until colon)
			guardStart := i
			for i < len(text) && text[i] != ':' {
				i++
			}
			if i >= len(text) {
				return nil, fmt.Errorf("expected ':' after guard condition")
			}
			guard = strings.TrimSpace(text[guardStart:i])
		}

		// Expect colon
		for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
			i++
		}
		if i >= len(text) || text[i] != ':' {
			return nil, fmt.Errorf("expected ':' after case pattern")
		}
		i++ // Skip ':'

		// Skip whitespace after colon
		for i < len(text) && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n' || text[i] == '\r') {
			i++
		}

		// Extract case body (until next "case " or end)
		var body string
		if i < len(text) && text[i] == '{' {
			// Braced body - find matching }
			braceCount := 1
			bodyStart := i
			i++
			for i < len(text) && braceCount > 0 {
				if text[i] == '{' {
					braceCount++
				} else if text[i] == '}' {
					braceCount--
				}
				i++
			}
			body = strings.TrimSpace(text[bodyStart:i])
		} else {
			// Bare statement - find next "case " or end
			// Search for newline followed by optional whitespace and "case "
			// BUT: Must track brace depth to avoid matching "case" inside nested switches
			bodyStart := i
			nextCaseIdx := -1
			braceDepth := 0
			for j := i; j < len(text); j++ {
				if text[j] == '{' {
					braceDepth++
				} else if text[j] == '}' {
					braceDepth--
				} else if text[j] == '\n' && braceDepth == 0 {
					// Found newline at top level, check if followed by whitespace + "case "
					k := j + 1
					// Skip whitespace after newline
					for k < len(text) && (text[k] == ' ' || text[k] == '\t' || text[k] == '\r') {
						k++
					}
					// Check for "case " keyword
					if k < len(text) && strings.HasPrefix(text[k:], "case ") {
						nextCaseIdx = j - i // Relative to bodyStart
						break
					}
				}
			}

			if nextCaseIdx == -1 {
				// Last case - take rest of text
				body = strings.TrimSpace(text[bodyStart:])
				i = len(text)
			} else {
				body = strings.TrimSpace(text[bodyStart : i+nextCaseIdx])
				i = i + nextCaseIdx // Position at newline before next case
			}
		}

		// Add case to list
		cases = append(cases, swiftCase{
			variant:    variant,
			binding:    binding,
			guard:      guard,
			expression: body,
		})
	}

	if len(cases) == 0 {
		return nil, fmt.Errorf("no case arms found")
	}

	return cases, nil
}

// isLetter checks if byte is ASCII letter
func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// isDigit checks if byte is ASCII digit
func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// normalizeBody normalizes case body (both bare statements and braced blocks)
// Examples:
//   "handleOk(x)" → "handleOk(x)"
//   "{ handleOk(x); return }" → "{ handleOk(x); return }"
func (s *SwiftMatchProcessor) normalizeBody(body string) string {
	trimmed := strings.TrimSpace(body)

	// Already braced? Keep as-is
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		return trimmed
	}

	// Bare statement - keep as-is (Go switch allows this)
	return trimmed
}

// generateSwitch generates Go switch statement with DINGO_MATCH markers
// CRITICAL: Emits IDENTICAL markers as RustMatchProcessor - plugin sees no difference
func (s *SwiftMatchProcessor) generateSwitch(scrutinee string, cases []swiftCase, originalLine int, outputLine int) (string, []Mapping) {
	var buf bytes.Buffer
	mappings := []Mapping{}

	matchID := s.matchCounter
	s.matchCounter++

	// Create temporary variable for scrutinee
	scrutineeVar := fmt.Sprintf("__match_%d", matchID)

	// Line 1: Store scrutinee in temporary variable
	buf.WriteString(fmt.Sprintf("%s := %s\n", scrutineeVar, scrutinee))
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          6,
		Name:            "swift_match",
	})
	outputLine++

	// Line 2: DINGO_MATCH_START marker (SAME as Rust)
	buf.WriteString(fmt.Sprintf("// DINGO_MATCH_START: %s\n", scrutinee))
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          6,
		Name:            "swift_match",
	})
	outputLine++

	// Line 3: switch statement opening (tag-based switch - IDENTICAL to Rust)
	buf.WriteString(fmt.Sprintf("switch %s.tag {\n", scrutineeVar))
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          6,
		Name:            "swift_match",
	})
	outputLine++

	// Generate case statements for each arm
	for _, caseArm := range cases {
		caseLines, caseMappings := s.generateCase(scrutineeVar, caseArm, originalLine, outputLine)
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
		Name:            "swift_match",
	})
	outputLine++

	// DINGO_MATCH_END marker (SAME as Rust)
	buf.WriteString("// DINGO_MATCH_END\n")
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          1,
		Name:            "swift_match",
	})

	return buf.String(), mappings
}

// generateCase generates a single case statement
// CRITICAL: Emits IDENTICAL markers as RustMatchProcessor
func (s *SwiftMatchProcessor) generateCase(scrutineeVar string, caseArm swiftCase, originalLine int, outputLine int) (string, []Mapping) {
	var buf bytes.Buffer
	mappings := []Mapping{}

	// Generate case tag (tag-based case - IDENTICAL to Rust)
	tagName := s.getTagName(caseArm.variant)
	buf.WriteString(fmt.Sprintf("case %s:\n", tagName))
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          len(caseArm.variant),
		Name:            "swift_match_arm",
	})
	outputLine++

	// DINGO_PATTERN marker (SAME format as Rust)
	patternStr := caseArm.variant
	if caseArm.binding != "" {
		patternStr = fmt.Sprintf("%s(%s)", caseArm.variant, caseArm.binding)
	}
	buf.WriteString(fmt.Sprintf("\t// DINGO_PATTERN: %s", patternStr))

	// Add DINGO_GUARD marker if guard present (SAME format as Rust)
	if caseArm.guard != "" {
		buf.WriteString(fmt.Sprintf(" | DINGO_GUARD: %s", caseArm.guard))
	}
	buf.WriteString("\n")

	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          len(patternStr),
		Name:            "swift_match_arm",
	})
	outputLine++

	// Extract binding if present (SAME binding code as Rust)
	if caseArm.binding != "" {
		bindingCode := s.generateBinding(scrutineeVar, caseArm.variant, caseArm.binding)
		buf.WriteString(fmt.Sprintf("\t%s\n", bindingCode))
		mappings = append(mappings, Mapping{
			OriginalLine:    originalLine,
			OriginalColumn:  1,
			GeneratedLine:   outputLine,
			GeneratedColumn: 1,
			Length:          len(caseArm.binding),
			Name:            "swift_match_binding",
		})
		outputLine++
	}

	// Case body expression
	exprStr := caseArm.expression
	if strings.HasPrefix(exprStr, "{") && strings.HasSuffix(exprStr, "}") {
		// Block expression: extract inner statements
		innerBlock := strings.TrimSpace(exprStr[1 : len(exprStr)-1])
		formatted := s.formatBlockStatements(innerBlock)
		for _, line := range strings.Split(formatted, "\n") {
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				buf.WriteString(fmt.Sprintf("\t%s\n", trimmed))
			}
		}
	} else {
		// Simple expression: add as-is
		buf.WriteString(fmt.Sprintf("\t%s\n", exprStr))
	}

	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          len(caseArm.expression),
		Name:            "swift_match_expr",
	})

	return buf.String(), mappings
}

// getTagName converts variant name to Go tag constant name
// IDENTICAL to RustMatchProcessor implementation
func (s *SwiftMatchProcessor) getTagName(variant string) string {
	switch variant {
	case "Ok":
		return "ResultTagOk"
	case "Err":
		return "ResultTagErr"
	case "Some":
		return "OptionTagSome"
	case "None":
		return "OptionTagNone"
	default:
		// Custom enum variant: capitalize first letter if needed
		return variant + "Tag"
	}
}

// generateBinding generates binding extraction code
// IDENTICAL to RustMatchProcessor implementation
func (s *SwiftMatchProcessor) generateBinding(scrutinee string, variant string, binding string) string {
	switch variant {
	case "Ok":
		// For Result<T,E>, Ok value is stored in ok_0 field (pointer to T)
		return fmt.Sprintf("%s := *%s.ok_0", binding, scrutinee)
	case "Err":
		// For Result<T,E>, Err value is stored in err_0 field (E)
		return fmt.Sprintf("%s := %s.err_0", binding, scrutinee)
	case "Some":
		// For Option<T>, Some value is stored in some_0 field (pointer to T)
		return fmt.Sprintf("%s := *%s.some_0", binding, scrutinee)
	default:
		// Custom enum variant: assume field name is lowercased variant name + _0
		fieldName := strings.ToLower(variant) + "_0"
		return fmt.Sprintf("%s := %s.%s", binding, scrutinee, fieldName)
	}
}

// formatBlockStatements formats block statements preserving newlines
func (s *SwiftMatchProcessor) formatBlockStatements(block string) string {
	// Newlines are preserved, return as-is
	return block
}

// GetNeededImports implements the ImportProvider interface
func (s *SwiftMatchProcessor) GetNeededImports() []string {
	// Swift match syntax doesn't require additional imports
	return []string{}
}

// detectTuple checks if scrutinee is a tuple expression: (expr1, expr2, ...)
// Returns: (isTuple, elements, error)
// IDENTICAL to RustMatchProcessor implementation
func (s *SwiftMatchProcessor) detectTuple(scrutinee string) (bool, []string, error) {
	trimmed := strings.TrimSpace(scrutinee)

	// Must start/end with parens
	if !strings.HasPrefix(trimmed, "(") || !strings.HasSuffix(trimmed, ")") {
		return false, nil, nil // Not a tuple
	}

	// Parse elements
	inner := trimmed[1 : len(trimmed)-1]
	elements := s.splitTupleElements(inner)

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
// IDENTICAL to RustMatchProcessor implementation
func (s *SwiftMatchProcessor) splitTupleElements(str string) []string {
	var elements []string
	var current strings.Builder
	depth := 0

	for _, ch := range str {
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

// swiftTupleCase represents a Swift tuple case arm
type swiftTupleCase struct {
	patterns   []swiftTupleElement // One per tuple element
	guard      string              // Guard condition (optional)
	expression string              // Expression to execute
}

// swiftTupleElement represents one element in a Swift tuple pattern
type swiftTupleElement struct {
	variant string // Ok, Err, Some, None, _ (wildcard)
	binding string // x, e, v (optional - empty for None/_)
}

// parseTupleCases parses Swift tuple pattern cases from switch body
// Example: case (.Ok(let x), .Err(let e)): expr1
//          case (.Ok(let a), .Ok(let b)) where guard: expr2
func (s *SwiftMatchProcessor) parseTupleCases(casesText string) ([]swiftTupleCase, error) {
	cases := []swiftTupleCase{}
	text := strings.TrimSpace(casesText)

	i := 0
	for i < len(text) {
		// Skip whitespace
		for i < len(text) && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n' || text[i] == '\r') {
			i++
		}
		if i >= len(text) {
			break
		}

		// Look for "case " keyword
		if !strings.HasPrefix(text[i:], "case ") {
			break
		}
		i += 5 // skip "case "

		// Skip whitespace
		for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
			i++
		}

		// Expect tuple pattern: (.Pattern1, .Pattern2, ...)
		if i >= len(text) || text[i] != '(' {
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
		tupleElements, err := s.parseSwiftTuplePattern(tuplePatternStr)
		if err != nil {
			return nil, fmt.Errorf("parsing tuple pattern: %w", err)
		}

		// Skip whitespace
		for i < len(text) && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n' || text[i] == '\r') {
			i++
		}

		// Check for guard (where/if)
		guard := ""
		if i < len(text) && (strings.HasPrefix(text[i:], "where ") || strings.HasPrefix(text[i:], "if ")) {
			// Find guard keyword
			if strings.HasPrefix(text[i:], "where ") {
				i += 6 // skip "where "
			} else {
				i += 3 // skip "if "
			}

			// Extract guard condition (until :)
			colonPos := strings.Index(text[i:], ":")
			if colonPos == -1 {
				return nil, fmt.Errorf("expected : after guard")
			}
			guard = strings.TrimSpace(text[i : i+colonPos])
			i += colonPos
		}

		// Expect :
		if i >= len(text) || text[i] != ':' {
			return nil, fmt.Errorf("expected : at position %d", i)
		}
		i++

		// Skip whitespace after :
		for i < len(text) && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n' || text[i] == '\r') {
			i++
		}

		// Extract expression
		var expr string
		if i < len(text) && text[i] == '{' {
			// Braced body
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
			// Bare statement - until next "case " or end
			start := i
			nextCase := strings.Index(text[i:], "\ncase ")
			if nextCase == -1 {
				// No more cases - take rest
				expr = strings.TrimSpace(text[start:])
				i = len(text)
			} else {
				expr = strings.TrimSpace(text[start : i+nextCase])
				i += nextCase
			}
		}

		cases = append(cases, swiftTupleCase{
			patterns:   tupleElements,
			guard:      guard,
			expression: expr,
		})
	}

	if len(cases) == 0 {
		return nil, fmt.Errorf("no tuple pattern cases found")
	}

	return cases, nil
}

// parseSwiftTuplePattern parses a single Swift tuple pattern: (.Ok(let x), .Err(let e), _)
func (s *SwiftMatchProcessor) parseSwiftTuplePattern(tupleStr string) ([]swiftTupleElement, error) {
	// Remove outer parens
	tupleStr = strings.TrimSpace(tupleStr)
	if !strings.HasPrefix(tupleStr, "(") || !strings.HasSuffix(tupleStr, ")") {
		return nil, fmt.Errorf("invalid tuple pattern: %s", tupleStr)
	}
	inner := tupleStr[1 : len(tupleStr)-1]

	// Split on commas (respecting nested parens)
	elementStrs := s.splitTupleElements(inner)

	elements := make([]swiftTupleElement, len(elementStrs))
	for i, elemStr := range elementStrs {
		elemStr = strings.TrimSpace(elemStr)

		// Wildcard
		if elemStr == "_" {
			elements[i] = swiftTupleElement{
				variant: "_",
				binding: "",
			}
			continue
		}

		// Swift pattern must start with . prefix
		if !strings.HasPrefix(elemStr, ".") {
			return nil, fmt.Errorf("Swift pattern must start with '.': %s", elemStr)
		}
		elemStr = elemStr[1:] // Remove . prefix

		// Pattern with binding: Ok(let x), Err(let e), Some(let v)
		if strings.Contains(elemStr, "(") {
			start := strings.Index(elemStr, "(")
			end := strings.Index(elemStr, ")")
			if end <= start {
				return nil, fmt.Errorf("invalid pattern: %s", elemStr)
			}
			variant := strings.TrimSpace(elemStr[:start])
			bindingPart := strings.TrimSpace(elemStr[start+1 : end])

			// Extract binding name from "let x" or "var x"
			binding := ""
			if strings.HasPrefix(bindingPart, "let ") {
				binding = strings.TrimSpace(bindingPart[4:])
			} else if strings.HasPrefix(bindingPart, "var ") {
				binding = strings.TrimSpace(bindingPart[4:])
			} else {
				binding = bindingPart // Bare binding without let/var
			}

			elements[i] = swiftTupleElement{
				variant: variant,
				binding: binding,
			}
		} else {
			// Pattern without binding: None
			elements[i] = swiftTupleElement{
				variant: elemStr,
				binding: "",
			}
		}
	}

	return elements, nil
}

// generateTupleMatch generates Go code for Swift tuple pattern matching
// EMITS IDENTICAL MARKERS to RustMatchProcessor
func (s *SwiftMatchProcessor) generateTupleMatch(tupleElements []string, cases []swiftTupleCase, originalLine int, outputLine int) (string, []Mapping) {
	var buf bytes.Buffer
	mappings := []Mapping{}

	matchID := s.matchCounter
	s.matchCounter++

	arity := len(tupleElements)

	// Line 1: DINGO_MATCH_START marker (IDENTICAL to Rust)
	scrutineeRepr := "(" + strings.Join(tupleElements, ", ") + ")"
	buf.WriteString(fmt.Sprintf("// DINGO_MATCH_START: %s\n", scrutineeRepr))
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          6, // "switch"
		Name:            "swift_match",
	})
	outputLine++

	// Line 2: Extract tuple elements into temp vars
	var elemVars []string
	for i := 0; i < arity; i++ {
		elemVars = append(elemVars, fmt.Sprintf("__match_%d_elem%d", matchID, i))
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
		Length:          6,
		Name:            "swift_match",
	})
	outputLine++

	// Generate DINGO_TUPLE_PATTERN marker (IDENTICAL to Rust)
	patternSummary := s.generateTuplePatternSummary(cases)
	buf.WriteString(fmt.Sprintf("// DINGO_TUPLE_PATTERN: %s | ARITY: %d\n", patternSummary, arity))
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          6,
		Name:            "swift_match",
	})
	outputLine++

	// Line 3: switch on first element
	buf.WriteString(fmt.Sprintf("switch %s.tag {\n", elemVars[0]))
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          6,
		Name:            "swift_match",
	})
	outputLine++

	// Generate cases (plugin will transform into nested switches)
	for _, c := range cases {
		caseLines, caseMappings := s.generateSwiftTupleCase(elemVars, c, originalLine, outputLine)
		buf.WriteString(caseLines)
		mappings = append(mappings, caseMappings...)
		outputLine += strings.Count(caseLines, "\n")
	}

	// Closing brace
	buf.WriteString("}\n")
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          1,
		Name:            "swift_match",
	})
	outputLine++

	// DINGO_MATCH_END marker (IDENTICAL to Rust)
	buf.WriteString("// DINGO_MATCH_END\n")
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          1,
		Name:            "swift_match",
	})

	return buf.String(), mappings
}

// generateTuplePatternSummary creates a summary string for DINGO_TUPLE_PATTERN marker
// Example: (Ok, Ok) | (Ok, Err) | (Err, _)
func (s *SwiftMatchProcessor) generateTuplePatternSummary(cases []swiftTupleCase) string {
	var patterns []string
	for _, c := range cases {
		var variants []string
		for _, elem := range c.patterns {
			variants = append(variants, elem.variant)
		}
		patterns = append(patterns, "("+strings.Join(variants, ", ")+")")
	}
	return strings.Join(patterns, " | ")
}

// generateSwiftTupleCase generates code for one Swift tuple pattern case
// EMITS IDENTICAL MARKERS to RustMatchProcessor
func (s *SwiftMatchProcessor) generateSwiftTupleCase(elemVars []string, c swiftTupleCase, originalLine int, outputLine int) (string, []Mapping) {
	var buf bytes.Buffer
	mappings := []Mapping{}

	// Generate case for first element only (plugin will expand to nested switches)
	firstElem := c.patterns[0]

	if firstElem.variant == "_" {
		// Wildcard - default case
		buf.WriteString("default:\n")
	} else {
		// Specific variant
		tagName := s.getTagName(firstElem.variant)
		buf.WriteString(fmt.Sprintf("case %s:\n", tagName))
	}

	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          4,
		Name:            "swift_match_arm",
	})
	outputLine++

	// Add DINGO_TUPLE_ARM marker with full pattern info (IDENTICAL format to Rust)
	var patternStrs []string
	for _, elem := range c.patterns {
		if elem.binding != "" {
			patternStrs = append(patternStrs, fmt.Sprintf("%s(%s)", elem.variant, elem.binding))
		} else {
			patternStrs = append(patternStrs, elem.variant)
		}
	}
	patternRepr := "(" + strings.Join(patternStrs, ", ") + ")"

	buf.WriteString(fmt.Sprintf("\t// DINGO_TUPLE_ARM: %s", patternRepr))
	if c.guard != "" {
		buf.WriteString(fmt.Sprintf(" | DINGO_GUARD: %s", c.guard))
	}
	buf.WriteString("\n")

	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          len(patternRepr),
		Name:            "swift_match_arm",
	})
	outputLine++

	// Plugin will generate nested switches and bindings
	// For now, just add expression
	buf.WriteString(fmt.Sprintf("\t%s\n", c.expression))

	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1,
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          len(c.expression),
		Name:            "swift_match_expr",
	})

	return buf.String(), mappings
}
