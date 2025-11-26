package preprocessor

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// SafeNavProcessor handles the ?. operator for safe navigation
// Transforms: user?.address?.city → null-safe chain with Option/pointer checks
// Supports both Option<T> types and raw Go pointers (*T)
type SafeNavProcessor struct {
	typeDetector *TypeDetector
	tmpCounter   int
	mappings     []Mapping
}

// TypeKind represents the kind of type for safe navigation
type TypeKind int

const (
	TypeUnknown TypeKind = iota // Defer to AST plugin
	TypeOption                  // Option<T> or XOption enum
	TypePointer                 // *T pointer type
	TypeRegular                 // Regular value type (error)
)

// TypeDetector analyzes source code to infer types for safe navigation
type TypeDetector struct {
	// Variable type annotations: "user" → "*User" or "UserOption"
	varTypes map[string]string

	// Struct field types: "User.address" → "*Address"
	fieldTypes map[string]string

	// Enum definitions: "UserOption" → true
	optionTypes map[string]bool
}

// NewTypeDetector creates a new type detector
func NewTypeDetector() *TypeDetector {
	return &TypeDetector{
		varTypes:    make(map[string]string),
		fieldTypes:  make(map[string]string),
		optionTypes: make(map[string]bool),
	}
}

// ParseSource analyzes the source code to extract type information
func (td *TypeDetector) ParseSource(source []byte) {
	sourceStr := string(source)

	td.parseTypeAnnotations(sourceStr)
	td.parseStructFields(sourceStr)
	td.parseEnumDefinitions(sourceStr)
}

// parseTypeAnnotations extracts type annotations from variable declarations
// Matches: let varName: TypeName = ...
//          var varName: TypeName = ...
//          varName: TypeName (function params)
func (td *TypeDetector) parseTypeAnnotations(source string) {
	// Variable declarations with type annotations
	re := regexp.MustCompile(`(?:let|var)\s+(\w+)\s*:\s*([*\w<>\[\]]+)\s*=`)
	matches := re.FindAllStringSubmatch(source, -1)

	for _, match := range matches {
		varName := match[1]
		typeName := match[2]
		td.varTypes[varName] = typeName
	}

	// Function parameters: funcName(param: Type)
	// Note: This is simplified - full parser would handle this better
	paramRe := regexp.MustCompile(`\(\s*(\w+)\s*:\s*([*\w<>\[\]]+)\s*[,)]`)
	paramMatches := paramRe.FindAllStringSubmatch(source, -1)

	for _, match := range paramMatches {
		paramName := match[1]
		typeName := match[2]
		td.varTypes[paramName] = typeName
	}
}

// parseStructFields extracts field types from struct definitions
// Matches: type StructName struct { fieldName TypeName }
func (td *TypeDetector) parseStructFields(source string) {
	// Match struct definitions
	structRe := regexp.MustCompile(`type\s+(\w+)\s+struct\s*\{([^}]+)\}`)
	structMatches := structRe.FindAllStringSubmatch(source, -1)

	for _, match := range structMatches {
		structName := match[1]
		fieldsBlock := match[2]

		// Parse individual fields
		fieldRe := regexp.MustCompile(`(\w+)\s+([*\w<>\[\]]+)`)
		fieldMatches := fieldRe.FindAllStringSubmatch(fieldsBlock, -1)

		for _, fm := range fieldMatches {
			fieldName := fm[1]
			fieldType := fm[2]

			// Store as "StructName.fieldName" → "FieldType"
			key := structName + "." + fieldName
			td.fieldTypes[key] = fieldType
		}
	}
}

// parseEnumDefinitions extracts enum definitions to identify Option types
// Matches: enum XOption { Some(X), None }
func (td *TypeDetector) parseEnumDefinitions(source string) {
	// Match enum definitions
	enumRe := regexp.MustCompile(`enum\s+(\w+)\s*\{([^}]+)\}`)
	enumMatches := enumRe.FindAllStringSubmatch(source, -1)

	for _, match := range enumMatches {
		enumName := match[1]
		variants := match[2]

		// Check if it looks like an Option type (has Some and None variants)
		if strings.Contains(variants, "Some") && strings.Contains(variants, "None") {
			td.optionTypes[enumName] = true
		}
	}
}

// DetectType determines the type kind of a variable
func (td *TypeDetector) DetectType(varName string) TypeKind {
	// 1. Check variable type annotation
	if typeName, ok := td.varTypes[varName]; ok {
		return classifyType(typeName)
	}

	// 2. Check if it's a field access (e.g., "User.address")
	if fieldType, ok := td.fieldTypes[varName]; ok {
		return classifyType(fieldType)
	}

	// 3. Unknown - defer to AST plugin
	return TypeUnknown
}

// classifyType determines the type kind from a type name
func classifyType(typeName string) TypeKind {
	typeName = strings.TrimSpace(typeName)

	// Pointer type: *User, *string, etc.
	if strings.HasPrefix(typeName, "*") {
		return TypePointer
	}

	// Option type by convention: UserOption, StringOption, etc.
	if strings.HasSuffix(typeName, "Option") {
		return TypeOption
	}

	// Option type with underscore: Option_string, Option_User, etc.
	if strings.HasPrefix(typeName, "Option_") {
		return TypeOption
	}

	// Generic Option syntax: Option<T>, Option[T] (future)
	if strings.HasPrefix(typeName, "Option<") || strings.HasPrefix(typeName, "Option[") {
		return TypeOption
	}

	// Regular type (not nullable)
	return TypeRegular
}

// NewSafeNavProcessor creates a new safe navigation preprocessor
func NewSafeNavProcessor() *SafeNavProcessor {
	return &SafeNavProcessor{
		typeDetector: NewTypeDetector(),
		tmpCounter:   1,
		mappings:     []Mapping{},
	}
}

// Name returns the processor name
func (s *SafeNavProcessor) Name() string {
	return "safe_navigation"
}

// Process is the legacy interface method (implements FeatureProcessor)
func (s *SafeNavProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	// Use the old processLine method to get mappings
	s.typeDetector.ParseSource(source)
	s.tmpCounter = 1

	lines := strings.Split(string(source), "\n")
	var output bytes.Buffer
	var allMappings []Mapping

	inputLineNum := 0
	outputLineNum := 1

	for inputLineNum < len(lines) {
		line := lines[inputLineNum]

		transformed, mappings, err := s.processLine(line, inputLineNum+1, outputLineNum)
		if err != nil {
			return nil, nil, fmt.Errorf("line %d: %w", inputLineNum+1, err)
		}

		output.WriteString(transformed)
		if inputLineNum < len(lines)-1 {
			output.WriteByte('\n')
		}

		// Collect mappings
		allMappings = append(allMappings, mappings...)

		// Update output line count
		newlineCount := strings.Count(transformed, "\n")
		linesOccupied := newlineCount + 1
		outputLineNum += linesOccupied

		inputLineNum++
	}

	return output.Bytes(), allMappings, nil
}

// ProcessInternal implements safe navigation transformation with metadata emission
func (s *SafeNavProcessor) ProcessInternal(code string) (string, []TransformMetadata, error) {
	// Parse source for type detection
	s.typeDetector.ParseSource([]byte(code))

	// Reset state
	s.tmpCounter = 1

	var metadata []TransformMetadata
	counter := 0

	lines := strings.Split(code, "\n")
	var output bytes.Buffer

	inputLineNum := 0
	outputLineNum := 1

	for inputLineNum < len(lines) {
		line := lines[inputLineNum]

		// Process the line with metadata
		transformed, meta, err := s.processLineWithMetadata(line, inputLineNum+1, outputLineNum, &counter)
		if err != nil {
			return "", nil, fmt.Errorf("line %d: %w", inputLineNum+1, err)
		}

		output.WriteString(transformed)
		if inputLineNum < len(lines)-1 {
			output.WriteByte('\n')
		}

		// Add metadata if generated
		if meta != nil {
			metadata = append(metadata, *meta)
		}

		// Update output line count
		newlineCount := strings.Count(transformed, "\n")
		linesOccupied := newlineCount + 1
		outputLineNum += linesOccupied

		inputLineNum++
	}

	return output.String(), metadata, nil
}

// safeNavPosition represents a safe navigation chain position in a line
type safeNavPosition struct {
	baseStart  int
	baseEnd    int
	chainStart int
	chainEnd   int
}

// findSafeNavStarts finds all safe navigation chains in a line
// Note: Uses findCommentStart from null_coalesce.go (shared in same package)
func findSafeNavStarts(line string) []safeNavPosition {
	var positions []safeNavPosition

	// Find comment start position to skip processing inside comments
	commentStart := findCommentStart(line)

	for i := 0; i < len(line); i++ {
		// Skip if we're inside a comment
		if commentStart != -1 && i >= commentStart {
			break
		}

		// Look for ?. pattern
		if i+1 < len(line) && line[i] == '?' && line[i+1] == '.' {
			// Found ?. - now extract the base identifier before it
			baseStart, baseEnd := extractBaseBefore(line, i)
			if baseStart == -1 {
				continue // No valid base identifier
			}

			// Extract the chain after ?.
			chainStart := i
			chainEnd := extractChainAfter(line, i)

			positions = append(positions, safeNavPosition{
				baseStart:  baseStart,
				baseEnd:    baseEnd,
				chainStart: chainStart,
				chainEnd:   chainEnd,
			})

			// Skip past this chain
			i = chainEnd - 1
		}
	}

	return positions
}

// extractBaseBefore extracts the identifier before position i
func extractBaseBefore(line string, pos int) (start, end int) {
	end = pos

	// Skip backwards over whitespace
	for end > 0 && (line[end-1] == ' ' || line[end-1] == '\t') {
		end--
	}

	// Extract identifier backwards
	start = end
	for start > 0 {
		ch := line[start-1]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			start--
		} else {
			break
		}
	}

	if start == end {
		return -1, -1 // No identifier found
	}

	return start, end
}

// extractChainAfter extracts the full ?. chain after position i
// Handles: ?.prop, ?.method(), ?.prop?.method(args), etc.
func extractChainAfter(line string, pos int) int {
	i := pos

	for i < len(line) {
		// Must start with ?.
		if i+1 >= len(line) || line[i] != '?' || line[i+1] != '.' {
			break
		}

		i += 2 // Skip ?.

		// Extract property/method name
		nameStart := i
		for i < len(line) {
			ch := line[i]
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
				i++
			} else {
				break
			}
		}

		if i == nameStart {
			break // No name after ?.
		}

		// Check if it's a method call (has parentheses)
		if i < len(line) && line[i] == '(' {
			// Extract balanced parentheses
			depth := 1
			i++ // Skip opening (

			for i < len(line) && depth > 0 {
				ch := line[i]

				if ch == '(' {
					depth++
				} else if ch == ')' {
					depth--
				} else if ch == '"' || ch == '\'' || ch == '`' {
					// Skip string literals
					quote := ch
					i++
					for i < len(line) {
						if line[i] == quote {
							if i > 0 && line[i-1] != '\\' {
								break
							}
						}
						i++
					}
				}

				i++
			}

			if depth != 0 {
				break // Unbalanced parentheses
			}
		}

		// Check if another ?. follows
		if i < len(line) && i+1 < len(line) && line[i] == '?' && line[i+1] == '.' {
			continue // More chaining
		} else {
			break // End of chain
		}
	}

	return i
}

// processLine processes a single line for safe navigation
func (s *SafeNavProcessor) processLine(line string, originalLineNum int, outputLineNum int) (string, []Mapping, error) {
	// Check if line contains ?. operator
	if !strings.Contains(line, "?.") {
		return line, nil, nil
	}

	// Check if all ?. occurrences are inside comments
	commentStart := findCommentStart(line)
	if commentStart != -1 {
		// Find first ?. position
		firstSafeNav := strings.Index(line, "?.")
		if firstSafeNav >= commentStart {
			// All ?. operators are inside comments, skip processing
			return line, nil, nil
		}
	}

	// Check for trailing ?. operator (error case)
	// But skip if the trailing ?. is inside a comment
	trimmed := strings.TrimSpace(line)
	if strings.HasSuffix(trimmed, "?.") {
		// Check if this trailing ?. is inside a comment
		if commentStart != -1 && len(trimmed) > 0 {
			// Find position of trailing ?. in original line
			trailingPos := strings.LastIndex(line, "?.")
			if trailingPos >= commentStart {
				// Trailing ?. is inside comment, skip error
				return line, nil, nil
			}
		}
		// Find the base identifier
		parts := strings.Split(strings.TrimSpace(line), "?.")
		if len(parts) > 0 {
			base := strings.TrimSpace(parts[0])
			// Extract last word if it's an assignment
			words := strings.Fields(base)
			if len(words) > 0 {
				lastWord := words[len(words)-1]
				return "", nil, fmt.Errorf(
					"trailing safe navigation operator without property: %s?.\n"+
						"  Help: Safe navigation (?.) requires a property or method after it\n"+
						"  Example: user?.name or user?.getName()\n"+
						"  Note: Did you mean error propagation (?) instead of safe navigation (?.)?",
					lastWord)
			}
		}
		return "", nil, fmt.Errorf(
			"trailing safe navigation operator (?.) without property\n" +
				"  Help: Safe navigation (?.) requires a property or method after it\n" +
				"  Note: Did you mean error propagation (?) instead?")
	}

	// Find safe navigation chains using manual parsing
	// We can't use simple regex for method calls with args due to nested parentheses
	result := line
	var allMappings []Mapping

	// Find all occurrences of ?. in the line
	safeNavPositions := findSafeNavStarts(line)

	// Process in reverse order to maintain string positions
	for i := len(safeNavPositions) - 1; i >= 0; i-- {
		pos := safeNavPositions[i]

		// Extract base identifier (before ?.)
		baseStart := pos.baseStart
		baseEnd := pos.baseEnd
		base := line[baseStart:baseEnd]

		// Extract chain starting from ?.
		chainStart := pos.chainStart
		chainEnd := pos.chainEnd
		chain := line[chainStart:chainEnd]

		fullStart := baseStart
		fullEnd := chainEnd

		// Parse the chain into individual properties/methods
		elements := parseSafeNavChain(chain)
		if len(elements) == 0 {
			continue
		}

		// Validate chain
		if err := s.validateChain(base, elements); err != nil {
			return "", nil, err
		}

		// Detect base type
		baseType := s.typeDetector.DetectType(base)

		// Generate code
		replacement, mappings, err := s.generateSafeNavCode(base, elements, baseType, originalLineNum, outputLineNum)
		if err != nil {
			return "", nil, err
		}

		// Replace in result
		result = result[:fullStart] + replacement + result[fullEnd:]

		// Adjust mappings for this replacement
		for _, m := range mappings {
			// Adjust column position based on replacement location
			m.OriginalColumn += fullStart
			allMappings = append(allMappings, m)
		}
	}

	return result, allMappings, nil
}

// processLineV2 processes a single line with metadata generation for Post-AST source maps
func (s *SafeNavProcessor) processLineWithMetadata(line string, originalLineNum int, outputLineNum int, markerCounter *int) (string, *TransformMetadata, error) {
	// Check if line contains ?. operator
	if !strings.Contains(line, "?.") {
		return line, nil, nil
	}

	// Check if all ?. occurrences are inside comments
	commentStart := findCommentStart(line)
	if commentStart != -1 {
		firstSafeNav := strings.Index(line, "?.")
		if firstSafeNav >= commentStart {
			return line, nil, nil
		}
	}

	// Check for trailing ?. operator (error case)
	trimmed := strings.TrimSpace(line)
	if strings.HasSuffix(trimmed, "?.") {
		if commentStart != -1 && len(trimmed) > 0 {
			trailingPos := strings.LastIndex(line, "?.")
			if trailingPos >= commentStart {
				return line, nil, nil
			}
		}
		parts := strings.Split(strings.TrimSpace(line), "?.")
		if len(parts) > 0 {
			base := strings.TrimSpace(parts[0])
			words := strings.Fields(base)
			if len(words) > 0 {
				lastWord := words[len(words)-1]
				return "", nil, fmt.Errorf(
					"trailing safe navigation operator without property: %s?.\n"+
						"  Help: Safe navigation (?.) requires a property or method after it\n"+
						"  Example: user?.name or user?.getName()\n"+
						"  Note: Did you mean error propagation (?) instead of safe navigation (?.)?",
					lastWord)
			}
		}
		return "", nil, fmt.Errorf(
			"trailing safe navigation operator (?.) without property\n" +
				"  Help: Safe navigation (?.) requires a property or method after it\n" +
				"  Note: Did you mean error propagation (?) instead?")
	}

	// Find safe navigation chains
	result := line
	var meta *TransformMetadata

	safeNavPositions := findSafeNavStarts(line)

	// Only create ONE metadata entry for the FIRST transformation
	firstTransform := true

	// Process in reverse order to maintain string positions
	for i := len(safeNavPositions) - 1; i >= 0; i-- {
		pos := safeNavPositions[i]

		baseStart := pos.baseStart
		baseEnd := pos.baseEnd
		base := line[baseStart:baseEnd]

		chainStart := pos.chainStart
		chainEnd := pos.chainEnd
		chain := line[chainStart:chainEnd]

		fullStart := baseStart
		fullEnd := chainEnd

		// Parse the chain into individual properties/methods
		elements := parseSafeNavChain(chain)
		if len(elements) == 0 {
			continue
		}

		// Validate chain
		if err := s.validateChain(base, elements); err != nil {
			return "", nil, err
		}

		// Detect base type
		baseType := s.typeDetector.DetectType(base)

		// Create marker for metadata
		marker := fmt.Sprintf("// dingo:s:%d", *markerCounter)

		// Generate code with marker inserted
		replacement, _, err := s.generateSafeNavCodeWithMarker(base, elements, baseType, marker, originalLineNum, outputLineNum)
		if err != nil {
			return "", nil, err
		}

		// Create metadata for first transformation only
		if firstTransform {
			meta = &TransformMetadata{
				Type:            "safe_nav",
				OriginalLine:    originalLineNum,
				OriginalColumn:  chainStart + 1,
				OriginalLength:  2, // length of ?.
				OriginalText:    "?.",
				GeneratedMarker: marker,
				ASTNodeType:     "CallExpr",
			}
			*markerCounter++
			firstTransform = false
		}

		// Replace in result
		result = result[:fullStart] + replacement + result[fullEnd:]
	}

	return result, meta, nil
}

// ChainElement represents a property or method in a safe navigation chain
type ChainElement struct {
	Name       string   // Property/method name
	IsMethod   bool     // true if method call, false if property
	Args       []string // Method arguments (empty for properties)
	RawArgs    string   // Raw argument string (for forwarding)
}

// parseSafeNavChain parses "?.address?.getCity()" into ChainElements
func parseSafeNavChain(chain string) []ChainElement {
	var elements []ChainElement

	// Split by ?. operator
	parts := strings.Split(chain, "?.")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check if it's a method call (has parentheses)
		if idx := strings.Index(part, "("); idx != -1 {
			// Method call: extract name and arguments
			methodName := part[:idx]
			argsStr := part[idx+1:]

			// Remove closing parenthesis
			if endIdx := strings.LastIndex(argsStr, ")"); endIdx != -1 {
				argsStr = argsStr[:endIdx]
			}

			// Parse arguments
			args := parseMethodArgs(argsStr)

			elements = append(elements, ChainElement{
				Name:     methodName,
				IsMethod: true,
				Args:     args,
				RawArgs:  argsStr,
			})
		} else {
			// Property access
			elements = append(elements, ChainElement{
				Name:     part,
				IsMethod: false,
				Args:     nil,
				RawArgs:  "",
			})
		}
	}

	return elements
}

// parseMethodArgs parses method arguments with balanced parentheses
// Handles nested calls, string literals (including raw strings and rune literals), and comments
func parseMethodArgs(argsStr string) []string {
	argsStr = strings.TrimSpace(argsStr)
	if argsStr == "" {
		return []string{}
	}

	var args []string
	var currentArg strings.Builder
	depth := 0
	inString := false
	stringChar := byte(0)
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(argsStr); i++ {
		ch := argsStr[i]

		// Handle line comments
		if !inString && !inBlockComment && i+1 < len(argsStr) && ch == '/' && argsStr[i+1] == '/' {
			inLineComment = true
			currentArg.WriteByte(ch)
			continue
		}

		// Handle block comments
		if !inString && !inLineComment && i+1 < len(argsStr) && ch == '/' && argsStr[i+1] == '*' {
			inBlockComment = true
			currentArg.WriteByte(ch)
			continue
		}

		// End block comment
		if inBlockComment && i > 0 && argsStr[i-1] == '*' && ch == '/' {
			inBlockComment = false
			currentArg.WriteByte(ch)
			continue
		}

		// End line comment (newline)
		if inLineComment && ch == '\n' {
			inLineComment = false
			currentArg.WriteByte(ch)
			continue
		}

		// Skip processing if in comment
		if inLineComment || inBlockComment {
			currentArg.WriteByte(ch)
			continue
		}

		// Handle string literals (including raw strings and single-quoted strings)
		switch ch {
		case '"', '`', '\'': // Regular strings, raw strings, and single-quoted strings
			if !inString {
				inString = true
				stringChar = ch
			} else if ch == stringChar {
				// Check if escaped (but raw strings ` can't be escaped)
				if (ch == '"' || ch == '\'') && i > 0 && argsStr[i-1] == '\\' {
					// Escaped quote - include it
					currentArg.WriteByte(ch)
					continue
				}
				inString = false
				stringChar = 0
			}
			currentArg.WriteByte(ch)

		case '(':
			if !inString {
				depth++
			}
			currentArg.WriteByte(ch)

		case ')':
			if !inString {
				depth--
			}
			currentArg.WriteByte(ch)

		case ',':
			if depth == 0 && !inString {
				// Argument separator
				args = append(args, strings.TrimSpace(currentArg.String()))
				currentArg.Reset()
			} else {
				currentArg.WriteByte(ch)
			}

		default:
			currentArg.WriteByte(ch)
		}
	}

	// Add final argument
	if currentArg.Len() > 0 {
		args = append(args, strings.TrimSpace(currentArg.String()))
	}

	return args
}

// validateChain validates a safe navigation chain
func (s *SafeNavProcessor) validateChain(base string, elements []ChainElement) error {
	// Check for trailing ?.
	if len(elements) == 0 {
		return fmt.Errorf("trailing ?. operator without property: %s?.", base)
	}

	// Check for invalid element names
	for _, elem := range elements {
		if elem.Name == "" {
			return fmt.Errorf("empty property/method in safe navigation chain")
		}

		// Name must be a valid identifier
		if !regexp.MustCompile(`^[a-zA-Z_]\w*$`).MatchString(elem.Name) {
			return fmt.Errorf("invalid property/method name in safe navigation: %s", elem.Name)
		}
	}

	return nil
}

// generateSafeNavCode generates the expanded safe navigation code
func (s *SafeNavProcessor) generateSafeNavCode(base string, elements []ChainElement, baseType TypeKind, originalLine int, outputLine int) (string, []Mapping, error) {
	// Determine mode based on base type
	switch baseType {
	case TypeOption:
		code, mappings := s.generateOptionMode(base, elements, originalLine, outputLine)
		return code, mappings, nil
	case TypePointer:
		code, mappings := s.generatePointerMode(base, elements, originalLine, outputLine)
		return code, mappings, nil
	case TypeUnknown:
		// Generate placeholder for AST plugin to resolve
		code, mappings := s.generateInferPlaceholder(base, elements, originalLine, outputLine)
		return code, mappings, nil
	case TypeRegular:
		// Error: cannot use ?. on non-nullable type
		// Provide clear error message with help text
		return "", nil, fmt.Errorf(
			"safe navigation requires nullable type\n"+
				"  Variable '%s' is not Option<T> or pointer type (*T)\n"+
				"  Help: Use Option<T> for nullable values, or use pointer type (*T)\n"+
				"  Note: If this is a pointer/Option, ensure type annotation is explicit",
			base)
	}

	return "", nil, nil
}

// generateOptionMode generates safe navigation for Option<T> types
func (s *SafeNavProcessor) generateOptionMode(base string, elements []ChainElement, originalLine int, outputLine int) (string, []Mapping) {
	var buf bytes.Buffer
	var mappings []Mapping

	// Generate IIFE for safe navigation
	buf.WriteString("func() __INFER__ {\n")

	currentVar := base
	outputLinesGenerated := 1 // Start counting from the opening func() line

	// Track the current type through the chain for proper wrapping
	currentType := ""
	if varType, ok := s.typeDetector.varTypes[base]; ok {
		currentType = varType
	}

	for i, elem := range elements {
		// Check if current value is None
		buf.WriteString(fmt.Sprintf("\tif %s.IsNone() {\n", currentVar))
		buf.WriteString("\t\treturn __INFER__None()\n")
		buf.WriteString("\t}\n")
		outputLinesGenerated += 3

		// Unwrap to get the value
		// No-number-first pattern
		tmpVar := ""
		if s.tmpCounter == 1 {
			tmpVar = base
		} else {
			tmpVar = fmt.Sprintf("%s%d", base, s.tmpCounter-1)
		}
		s.tmpCounter++
		buf.WriteString(fmt.Sprintf("\t%s := %s.Unwrap()\n", tmpVar, currentVar))
		outputLinesGenerated++

		// If last element, return it
		if i == len(elements)-1 {
			// Determine the return expression
			returnExpr := ""
			if elem.IsMethod {
				// Method call: call with arguments
				returnExpr = fmt.Sprintf("%s.%s(%s)", tmpVar, elem.Name, elem.RawArgs)
			} else {
				// Property access
				returnExpr = fmt.Sprintf("%s.%s", tmpVar, elem.Name)
			}

			// Bug 2 & 3 fix: Conditional wrapping based on actual field type
			// Determine if the field type is already an Option type
			fieldType := ""
			if currentType != "" {
				// Strip "Option" suffix to get struct type
				structType := strings.TrimSuffix(currentType, "Option")
				// Build field key: "StructType.fieldName"
				fieldKey := structType + "." + elem.Name
				// Look up in TypeDetector
				if ft, ok := s.typeDetector.fieldTypes[fieldKey]; ok {
					fieldType = ft
				}
			}

			// Check if field type ends with "Option" (already an Option type)
			isAlreadyOption := strings.HasSuffix(fieldType, "Option")

			if isAlreadyOption {
				// Bug 2 fix: Already an Option type, return as-is (no double-wrapping)
				buf.WriteString(fmt.Sprintf("\treturn %s\n", returnExpr))
			} else {
				// Bug 3 fix: Plain type or unknown, wrap in Some()
				buf.WriteString(fmt.Sprintf("\treturn __INFER__Some(%s)\n", returnExpr))
			}
			outputLinesGenerated++
		} else {
			// Not last - prepare for next iteration
			if elem.IsMethod {
				// Method call: assign result to currentVar for next iteration
				currentVar = fmt.Sprintf("%s.%s(%s)", tmpVar, elem.Name, elem.RawArgs)
			} else {
				// Property access
				currentVar = fmt.Sprintf("%s.%s", tmpVar, elem.Name)
			}

			// Update currentType for next iteration
			if currentType != "" {
				structType := strings.TrimSuffix(currentType, "Option")
				fieldKey := structType + "." + elem.Name
				if ft, ok := s.typeDetector.fieldTypes[fieldKey]; ok {
					currentType = ft
				}
			}
		}
	}

	buf.WriteString("}()")
	outputLinesGenerated++ // Closing }()

	// Add mapping for the safe navigation chain
	// Note: This single-line original maps to multiple output lines (IIFE pattern)
	// The GeneratedLine points to the start of the IIFE
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1, // Will be adjusted by caller
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          len(base) + len(elements)*2, // Approximate original length
		Name:            fmt.Sprintf("safe_nav_option_%dlines", outputLinesGenerated),
	})

	return buf.String(), mappings
}

// generatePointerMode generates safe navigation for pointer types
func (s *SafeNavProcessor) generatePointerMode(base string, elements []ChainElement, originalLine int, outputLine int) (string, []Mapping) {
	var buf bytes.Buffer
	var mappings []Mapping

	// Generate IIFE for safe navigation
	buf.WriteString("func() __INFER__ {\n")

	currentVar := base
	outputLinesGenerated := 1 // Start counting from the opening func() line

	for i, elem := range elements {
		// Check if current value is nil
		buf.WriteString(fmt.Sprintf("\tif %s == nil {\n", currentVar))
		buf.WriteString("\t\treturn nil\n")
		buf.WriteString("\t}\n")
		outputLinesGenerated += 3

		// Access the property or method
		if i < len(elements)-1 {
			// Not the last element - create intermediate variable to check next nil
			// CamelCase pattern without underscores
			var tmpVar string
			if i == 0 {
				tmpVar = base + "Tmp"
			} else {
				tmpVar = fmt.Sprintf("%sTmp%d", base, i)
			}
			if elem.IsMethod {
				buf.WriteString(fmt.Sprintf("\t%s := %s.%s(%s)\n", tmpVar, currentVar, elem.Name, elem.RawArgs))
			} else {
				buf.WriteString(fmt.Sprintf("\t%s := %s.%s\n", tmpVar, currentVar, elem.Name))
			}
			currentVar = tmpVar
			outputLinesGenerated++
		} else {
			// Last element - return it
			if elem.IsMethod {
				buf.WriteString(fmt.Sprintf("\treturn %s.%s(%s)\n", currentVar, elem.Name, elem.RawArgs))
			} else {
				buf.WriteString(fmt.Sprintf("\treturn %s.%s\n", currentVar, elem.Name))
			}
			outputLinesGenerated++
		}
	}

	buf.WriteString("}()")
	outputLinesGenerated++ // Closing }()

	// Add mapping for the safe navigation chain
	// Note: This single-line original maps to multiple output lines (IIFE pattern)
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1, // Will be adjusted by caller
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          len(base) + len(elements)*2, // Approximate
		Name:            fmt.Sprintf("safe_nav_pointer_%dlines", outputLinesGenerated),
	})

	return buf.String(), mappings
}

// generateInferPlaceholder generates a placeholder for AST plugin resolution
func (s *SafeNavProcessor) generateInferPlaceholder(base string, elements []ChainElement, originalLine int, outputLine int) (string, []Mapping) {
	var mappings []Mapping

	// Generate placeholder function call that AST plugin will replace
	// Format: __SAFE_NAV_INFER__(base, "prop1", "method2(...)", ...)
	var elemArgs []string
	for _, elem := range elements {
		if elem.IsMethod {
			// Include method with arguments
			elemArgs = append(elemArgs, fmt.Sprintf(`"%s(%s)"`, elem.Name, elem.RawArgs))
		} else {
			// Property access
			elemArgs = append(elemArgs, fmt.Sprintf(`"%s"`, elem.Name))
		}
	}

	placeholder := fmt.Sprintf("__SAFE_NAV_INFER__(%s, %s)", base, strings.Join(elemArgs, ", "))

	// Add mapping
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  1, // Will be adjusted by caller
		GeneratedLine:   outputLine,
		GeneratedColumn: 1,
		Length:          len(base) + len(elements)*2,
		Name:            "safe_nav_infer",
	})

	return placeholder, mappings
}

// generateSafeNavCodeWithMarker generates safe navigation code with marker inserted for metadata tracking
func (s *SafeNavProcessor) generateSafeNavCodeWithMarker(base string, elements []ChainElement, baseType TypeKind, marker string, originalLine int, outputLine int) (string, []Mapping, error) {
	// Generate the code first
	code, mappings, err := s.generateSafeNavCode(base, elements, baseType, originalLine, outputLine)
	if err != nil {
		return "", nil, err
	}

	// For IIFE pattern (Option/Pointer mode), insert marker after opening brace
	// Pattern: "func() __INFER__ {\n" → "func() __INFER__ { " + marker + "\n"
	braceIdx := strings.Index(code, "{\n")
	if braceIdx != -1 {
		// Insert marker after the brace, before the newline
		before := code[:braceIdx+1] // Include the {
		after := code[braceIdx+1:]  // Include the \n and rest
		code = before + " " + marker + after
	} else {
		// For placeholder pattern (__SAFE_NAV_INFER__), prepend marker as comment line
		code = marker + "\n" + code
	}

	return code, mappings, nil
}

