package preprocessor

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// NullCoalesceProcessor handles the ?? operator for null coalescing
// Transforms: value ?? default → null-safe default value handling
// Supports both Option<T> types and raw Go pointers (*T)
// Optimizes simple cases with inline code, complex cases with IIFE
type NullCoalesceProcessor struct {
	typeDetector *TypeDetector
	tmpCounter   int
	mappings     []Mapping
}

// CoalesceComplexity represents the complexity level of a null coalesce expression
type CoalesceComplexity int

const (
	ComplexitySimple  CoalesceComplexity = iota // Inline code
	ComplexityComplex                            // IIFE with intermediate vars
)

// NewNullCoalesceProcessor creates a new null coalescing preprocessor
func NewNullCoalesceProcessor() *NullCoalesceProcessor {
	return &NullCoalesceProcessor{
		typeDetector: NewTypeDetector(),
		tmpCounter:   0,
		mappings:     []Mapping{},
	}
}

// Name returns the processor name
func (n *NullCoalesceProcessor) Name() string {
	return "null_coalescing"
}

// Process transforms null coalescing operators
func (n *NullCoalesceProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	// Parse source for type detection
	n.typeDetector.ParseSource(source)

	// Reset state
	n.tmpCounter = 1
	n.mappings = []Mapping{}

	lines := strings.Split(string(source), "\n")
	var output bytes.Buffer

	inputLineNum := 0
	outputLineNum := 1

	for inputLineNum < len(lines) {
		line := lines[inputLineNum]

		// Process the line
		transformed, newMappings, err := n.processLine(line, inputLineNum+1, outputLineNum)
		if err != nil {
			return nil, nil, fmt.Errorf("line %d: %w", inputLineNum+1, err)
		}

		output.WriteString(transformed)
		if inputLineNum < len(lines)-1 {
			output.WriteByte('\n')
		}

		// Add mappings
		if len(newMappings) > 0 {
			n.mappings = append(n.mappings, newMappings...)
		}

		// Update output line count
		newlineCount := strings.Count(transformed, "\n")
		linesOccupied := newlineCount + 1
		outputLineNum += linesOccupied

		inputLineNum++
	}

	return output.Bytes(), n.mappings, nil
}

// processLine processes a single line for null coalescing
func (n *NullCoalesceProcessor) processLine(line string, originalLineNum int, outputLineNum int) (string, []Mapping, error) {
	// Check if line contains ?? operator
	if !strings.Contains(line, "??") {
		return line, nil, nil
	}

	// Find all ?? expressions in the line
	result := line
	var allMappings []Mapping

	// Process from right to left to preserve positions
	positions := findNullCoalescePositions(line)

	// Process in reverse order to maintain string positions
	for i := len(positions) - 1; i >= 0; i-- {
		pos := positions[i]

		// Extract left and right operands
		left := line[pos.leftStart:pos.leftEnd]
		right := line[pos.rightStart:pos.rightEnd]
		fullStart := pos.leftStart
		fullEnd := pos.rightEnd

		// Handle chained ?? (a ?? b ?? c)
		// Collect all chained expressions
		chain := []string{left, right}
		currentPos := i
		chainLeftEnd := pos.leftEnd // Track the left end of the current chain's first element

		// Look for more ?? in chain (going backwards in positions array)
		for currentPos > 0 {
			nextPos := positions[currentPos-1]
			// Check if this is part of the same chain (right operand of previous equals left of current chain)
			if nextPos.rightEnd == chainLeftEnd {
				// This is part of chain: prepend the left operand
				chainLeft := line[nextPos.leftStart:nextPos.leftEnd]
				chain = append([]string{chainLeft}, chain...)
				fullStart = nextPos.leftStart
				chainLeftEnd = nextPos.leftEnd // Update for next iteration
				currentPos--
				i-- // Skip this position in outer loop
			} else {
				break
			}
		}

		// Classify complexity
		complexity := n.classifyComplexity(chain)

		// Detect type of leftmost operand
		leftType := n.typeDetector.DetectType(strings.TrimSpace(chain[0]))

		// Generate replacement code
		replacement, mappings := n.generateCoalesceCode(chain, complexity, leftType, originalLineNum, outputLineNum)

		// Replace in result
		result = result[:fullStart] + replacement + result[fullEnd:]

		// Adjust mappings for replacement location
		for _, m := range mappings {
			m.OriginalColumn += fullStart
			allMappings = append(allMappings, m)
		}
	}

	return result, allMappings, nil
}

// nullCoalescePosition represents a ?? operator position in a line
type nullCoalescePosition struct {
	leftStart  int
	leftEnd    int
	opStart    int // Position of ??
	opEnd      int
	rightStart int
	rightEnd   int
}

// findNullCoalescePositions finds all ?? operators in a line
func findNullCoalescePositions(line string) []nullCoalescePosition {
	var positions []nullCoalescePosition

	// Find comment start position
	commentStart := findCommentStart(line)

	i := 0
	for i < len(line) {
		// Skip if we're inside a comment
		if commentStart != -1 && i >= commentStart {
			break
		}

		// Look for ?? pattern
		if i+1 < len(line) && line[i] == '?' && line[i+1] == '?' {
			opStart := i
			opEnd := i + 2

			// Extract left operand (go backwards)
			leftEnd := i
			// Skip whitespace backwards to get actual end of left operand (excluding trailing spaces)
			for leftEnd > 0 && (line[leftEnd-1] == ' ' || line[leftEnd-1] == '\t') {
				leftEnd--
			}
			leftStart := extractOperandBefore(line, leftEnd)
			if leftStart == -1 {
				i++
				continue
			}

			// Extract right operand (go forwards)
			rightStart := opEnd
			// Skip whitespace
			for rightStart < len(line) && (line[rightStart] == ' ' || line[rightStart] == '\t') {
				rightStart++
			}

			rightEnd := extractOperandAfter(line, rightStart)
			if rightEnd == -1 {
				i++
				continue
			}

			positions = append(positions, nullCoalescePosition{
				leftStart:  leftStart,
				leftEnd:    leftEnd,
				opStart:    opStart,
				opEnd:      opEnd,
				rightStart: rightStart,
				rightEnd:   rightEnd,
			})

			// Skip past this operator
			i = opEnd
		} else {
			i++
		}
	}

	return positions
}

// findCommentStart finds the start position of a comment (// or /*) in a line
// Returns -1 if no comment found
// NOTE: Does not handle string literals containing comment markers
func findCommentStart(line string) int {
	inString := false
	stringChar := byte(0)

	for i := 0; i < len(line); i++ {
		ch := line[i]

		// Track string state
		if !inString {
			if ch == '"' || ch == '\'' || ch == '`' {
				inString = true
				stringChar = ch
			}
		} else {
			// Inside string - check for closing quote
			if ch == stringChar {
				// Check if escaped
				if i > 0 && line[i-1] == '\\' {
					// Count consecutive backslashes
					backslashCount := 0
					for j := i - 1; j >= 0 && line[j] == '\\'; j-- {
						backslashCount++
					}
					// If odd number of backslashes, quote is escaped
					if backslashCount%2 == 1 {
						continue
					}
				}
				inString = false
			}
			continue
		}

		// Check for comment markers (only when not in string)
		if !inString {
			// Single-line comment
			if i+1 < len(line) && line[i] == '/' && line[i+1] == '/' {
				return i
			}
			// Multi-line comment start
			if i+1 < len(line) && line[i] == '/' && line[i+1] == '*' {
				return i
			}
		}
	}

	return -1
}

// extractOperandBefore extracts an operand before a position
// Handles: identifiers, literals, function calls, safe nav chains
func extractOperandBefore(line string, end int) int {
	// Skip whitespace backwards
	for end > 0 && (line[end-1] == ' ' || line[end-1] == '\t') {
		end--
	}

	if end == 0 {
		return -1
	}

	// Check what we're ending with
	lastChar := line[end-1]

	// Case 1: String literal
	if lastChar == '"' || lastChar == '\'' || lastChar == '`' {
		quote := lastChar
		start := end - 2
		for start >= 0 {
			if line[start] == quote {
				// Check if escaped
				if start > 0 && line[start-1] == '\\' {
					start--
					continue
				}
				return start
			}
			start--
		}
		return -1 // Unclosed string
	}

	// Case 2: Function call or method chain (ends with ))
	if lastChar == ')' {
		depth := 1
		start := end - 2
		for start >= 0 && depth > 0 {
			if line[start] == ')' {
				depth++
			} else if line[start] == '(' {
				depth--
			}
			start--
		}

		if depth != 0 {
			return -1 // Unbalanced
		}

		// Continue backwards to get function/method name (including obj.method() chains)
		start++ // Adjust for loop overshoot
		for start > 0 {
			ch := line[start-1]
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
				start--
			} else if ch == '.' || ch == '?' {
				// Method chain or safe nav - continue backwards
				start--
			} else {
				break
			}
		}
		return start
	}

	// Case 3: Number literal OR identifier ending with digit (e.g., opt1)
	// We need to check if there's a letter/underscore before the digits
	if lastChar >= '0' && lastChar <= '9' {
		start := end - 1
		hasDecimal := false
		for start > 0 {
			ch := line[start-1]
			if ch >= '0' && ch <= '9' {
				start--
			} else if ch == '.' && !hasDecimal {
				hasDecimal = true
				start--
			} else {
				break
			}
		}

		// Check if there's an identifier character before the number
		// If so, this is an identifier (like opt1), not a number literal
		if start > 0 {
			ch := line[start-1]
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' {
				// This is an identifier, not a number - continue scanning backwards
				start--
				for start > 0 {
					ch := line[start-1]
					if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
						start--
					} else {
						break
					}
				}
			}
		}

		return start
	}

	// Case 4: Identifier (including safe nav chains like user?.name)
	start := end
	for start > 0 {
		ch := line[start-1]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			start--
		} else if ch == '.' || ch == '?' {
			// Safe nav chain or property access
			start--
		} else {
			break
		}
	}

	if start == end {
		return -1 // No identifier
	}

	return start
}

// extractOperandAfter extracts an operand after a position
// Handles: identifiers, literals, function calls, method chains, safe nav
func extractOperandAfter(line string, start int) int {
	if start >= len(line) {
		return -1
	}

	ch := line[start]

	// Case 1: String literal
	if ch == '"' || ch == '\'' || ch == '`' {
		quote := ch
		end := start + 1
		for end < len(line) {
			if line[end] == quote {
				// Check if escaped
				if end > 0 && line[end-1] == '\\' {
					end++
					continue
				}
				return end + 1
			}
			end++
		}
		return -1 // Unclosed string
	}

	// Case 2: Number literal (positive only - don't parse - as operator)
	if ch >= '0' && ch <= '9' {
		end := start + 1
		hasDecimal := false
		for end < len(line) {
			ch := line[end]
			if ch >= '0' && ch <= '9' {
				end++
			} else if ch == '.' && !hasDecimal && end+1 < len(line) && line[end+1] >= '0' && line[end+1] <= '9' {
				hasDecimal = true
				end++
			} else {
				break
			}
		}
		return end
	}

	// Case 3: Identifier with method chains, safe nav, function calls
	if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' {
		end := start + 1
		// Scan identifier with chaining
		for end < len(line) {
			ch := line[end]
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
				end++
			} else if ch == '(' {
				// Function call - extract balanced parens
				depth := 1
				end++
				for end < len(line) && depth > 0 {
					if line[end] == '(' {
						depth++
					} else if line[end] == ')' {
						depth--
					}
					end++
				}
				if depth != 0 {
					return -1 // Unbalanced
				}
				// After function call, continue to check for more chaining
			} else if ch == '.' {
				// Property access or method call
				end++
				// Continue parsing identifier/method after dot
			} else if ch == '?' && end+1 < len(line) && line[end+1] == '.' {
				// Safe navigation ?.
				end += 2
				// Continue parsing identifier/method after ?.
			} else {
				// Stop at operators like +, -, ??, etc.
				break
			}
		}
		return end
	}

	return -1
}

// classifyComplexity determines if a coalesce expression is simple or complex
func (n *NullCoalesceProcessor) classifyComplexity(chain []string) CoalesceComplexity {
	// Chained ?? (more than 2 operands) → IIFE
	if len(chain) > 2 {
		return ComplexityComplex
	}

	// Check left operand
	left := strings.TrimSpace(chain[0])
	if !n.isSimpleOperand(left) {
		return ComplexityComplex
	}

	// Check right operand
	right := strings.TrimSpace(chain[1])
	if !n.isSimpleOperand(right) {
		return ComplexityComplex
	}

	return ComplexitySimple
}

// isSimpleOperand checks if an operand is simple (identifier or literal)
func (n *NullCoalesceProcessor) isSimpleOperand(operand string) bool {
	operand = strings.TrimSpace(operand)

	// Empty → not simple
	if operand == "" {
		return false
	}

	// Check if it's a single identifier
	if isIdentifier(operand) {
		return true
	}

	// Check if it's a literal
	if isLiteral(operand) {
		return true
	}

	// Complex: function calls, operators, safe nav chains
	return false
}

// isIdentifier checks if a string is a valid identifier
func isIdentifier(s string) bool {
	if s == "" {
		return false
	}

	// Must start with letter or underscore
	first := s[0]
	if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || first == '_') {
		return false
	}

	// Rest must be alphanumeric or underscore
	for i := 1; i < len(s); i++ {
		ch := s[i]
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_') {
			return false
		}
	}

	return true
}

// isLiteral checks if a string is a literal value
func isLiteral(s string) bool {
	s = strings.TrimSpace(s)

	// String literal
	if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) ||
		(strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) ||
		(strings.HasPrefix(s, "`") && strings.HasSuffix(s, "`")) {
		return true
	}

	// Number literal
	if regexp.MustCompile(`^\d+(\.\d+)?$`).MatchString(s) {
		return true
	}

	// Boolean literal
	if s == "true" || s == "false" {
		return true
	}

	// nil literal
	if s == "nil" {
		return true
	}

	return false
}

// generateCoalesceCode generates the null coalescing code
func (n *NullCoalesceProcessor) generateCoalesceCode(chain []string, complexity CoalesceComplexity, leftType TypeKind, originalLine int, outputLine int) (string, []Mapping) {
	switch complexity {
	case ComplexitySimple:
		return n.generateInline(chain, leftType, originalLine, outputLine)
	case ComplexityComplex:
		return n.generateIIFE(chain, leftType, originalLine, outputLine)
	}

	return "", nil
}

// generateInline generates inline code for simple cases
func (n *NullCoalesceProcessor) generateInline(chain []string, leftType TypeKind, originalLine int, outputLine int) (string, []Mapping) {
	// Simple case: value ?? default
	left := strings.TrimSpace(chain[0])
	right := strings.TrimSpace(chain[1])

	var buf bytes.Buffer
	var mappings []Mapping

	// Generate based on left type (single-line IIFE to avoid indentation issues)
	// Note: For enum-based Option types, we use __UNWRAP__ placeholder which will be resolved during AST phase
	switch leftType {
	case TypeOption:
		// Option type: check IsSome() and use __UNWRAP__ for value extraction
		buf.WriteString(fmt.Sprintf("func() __INFER__ { if %s.IsSome() { return __UNWRAP__(%s) }; return %s }()", left, left, right))

	case TypePointer:
		// Pointer type: check nil
		buf.WriteString(fmt.Sprintf("func() __INFER__ { if %s != nil { return *%s }; return %s }()", left, left, right))

	case TypeUnknown, TypeRegular:
		// Unknown type: assume Option type (most common case)
		// Generate IIFE with Option checks and __UNWRAP__ placeholder
		buf.WriteString(fmt.Sprintf("func() __INFER__ { if %s.IsSome() { return __UNWRAP__(%s) }; return %s }()", left, left, right))
	}

	// Add mapping
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1, // Will be adjusted by caller
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          len(left) + 4 + len(right), // left ?? right
		Name:            "null_coalesce_inline",
	})

	return buf.String(), mappings
}

// generateIIFE generates IIFE code for complex cases (including chaining)
// Uses single-line format to avoid indentation issues
func (n *NullCoalesceProcessor) generateIIFE(chain []string, leftType TypeKind, originalLine int, outputLine int) (string, []Mapping) {
	var buf bytes.Buffer
	var mappings []Mapping

	// Build single-line IIFE
	buf.WriteString("func() __INFER__ { ")

	// Generate checks for each element in chain
	for i := 0; i < len(chain)-1; i++ {
		operand := strings.TrimSpace(chain[i])

		if i == 0 {
			// First operand: evaluate once and check
			tmpVar := fmt.Sprintf("coalesce%d", n.tmpCounter)
			n.tmpCounter++

			buf.WriteString(fmt.Sprintf("%s := %s; ", tmpVar, operand))

			// Check based on type
			switch leftType {
			case TypeOption:
				buf.WriteString(fmt.Sprintf("if %s.IsSome() { return %s.Unwrap() }; ", tmpVar, tmpVar))

			case TypePointer:
				buf.WriteString(fmt.Sprintf("if %s != nil { return *%s }; ", tmpVar, tmpVar))

			case TypeUnknown, TypeRegular:
				// Generate placeholder check
				buf.WriteString(fmt.Sprintf("if __IS_SOME__(%s) { return __UNWRAP__(%s) }; ", tmpVar, tmpVar))
			}
		} else {
			// Subsequent operands in chain: evaluate and check
			tmpVar := fmt.Sprintf("coalesce%d", n.tmpCounter)
			n.tmpCounter++

			buf.WriteString(fmt.Sprintf("%s := %s; ", tmpVar, operand))

			// All subsequent values treated same as first type
			switch leftType {
			case TypeOption:
				buf.WriteString(fmt.Sprintf("if %s.IsSome() { return %s.Unwrap() }; ", tmpVar, tmpVar))

			case TypePointer:
				buf.WriteString(fmt.Sprintf("if %s != nil { return *%s }; ", tmpVar, tmpVar))

			case TypeUnknown, TypeRegular:
				buf.WriteString(fmt.Sprintf("if __IS_SOME__(%s) { return __UNWRAP__(%s) }; ", tmpVar, tmpVar))
			}
		}
	}

	// Final fallback: return last element
	lastOperand := strings.TrimSpace(chain[len(chain)-1])
	buf.WriteString(fmt.Sprintf("return %s }()", lastOperand))

	// Add mapping
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1, // Will be adjusted by caller
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          20, // Approximate
		Name:            "null_coalesce_iife",
	})

	return buf.String(), mappings
}
