package preprocessor

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"regexp"
	"sort"
	"strings"
)

// Package-level compiled regexes (Issue 2: Regex Performance)
var (
	assignPattern = regexp.MustCompile(`^\s*(let|var)\s+(\w+)\s*=\s*(.+)$`)
	returnPattern = regexp.MustCompile(`^\s*return\s+(.+)$`)
	msgPattern    = regexp.MustCompile(`^(.*\?)\s*"((?:[^"\\]|\\.)*)"`)
)

// ImportTracker manages automatic import detection
// Tracks function calls and determines which standard library packages are needed
type ImportTracker struct {
	needed  map[string]bool   // package path → needed
	aliases map[string]string // funcName → package path
}

// Common standard library functions that require imports
// Maps both bare function names AND qualified calls (pkg.Function)
var stdLibFunctions = map[string]string{
	// os package
	"ReadFile":    "os",
	"WriteFile":   "os",
	"Open":        "os",
	"Create":      "os",
	"Stat":        "os",
	"Remove":      "os",
	"Mkdir":       "os",
	"MkdirAll":    "os",
	"Getwd":       "os",
	"Chdir":       "os",
	"os.ReadFile": "os",
	"os.WriteFile": "os",
	"os.Open":     "os",
	"os.Create":   "os",
	"os.Stat":     "os",
	"os.Remove":   "os",
	"os.Mkdir":    "os",
	"os.MkdirAll": "os",
	"os.Getwd":    "os",
	"os.Chdir":    "os",

	// encoding/json
	"Marshal":      "encoding/json",
	"Unmarshal":    "encoding/json",
	"NewEncoder":   "encoding/json",
	"NewDecoder":   "encoding/json",
	"json.Marshal": "encoding/json",
	"json.Unmarshal": "encoding/json",
	"json.NewEncoder": "encoding/json",
	"json.NewDecoder": "encoding/json",

	// strconv
	"Atoi":         "strconv",
	"Itoa":         "strconv",
	"ParseInt":     "strconv",
	"ParseFloat":   "strconv",
	"ParseBool":    "strconv",
	"FormatInt":    "strconv",
	"FormatFloat":  "strconv",
	"strconv.Atoi": "strconv",
	"strconv.Itoa": "strconv",
	"strconv.ParseInt": "strconv",
	"strconv.ParseFloat": "strconv",
	"strconv.ParseBool": "strconv",
	"strconv.FormatInt": "strconv",
	"strconv.FormatFloat": "strconv",

	// io
	"ReadAll":    "io",
	"io.ReadAll": "io",

	// net/http
	"Get":         "net/http",
	"Post":        "net/http",
	"NewRequest":  "net/http",
	"http.Get":    "net/http",
	"http.Post":   "net/http",
	"http.NewRequest": "net/http",

	// path/filepath
	"Join":          "path/filepath",
	"Base":          "path/filepath",
	"Dir":           "path/filepath",
	"Ext":           "path/filepath",
	"Clean":         "path/filepath",
	"filepath.Join": "path/filepath",
	"filepath.Base": "path/filepath",
	"filepath.Dir":  "path/filepath",
	"filepath.Ext":  "path/filepath",
	"filepath.Clean": "path/filepath",

	// fmt (already tracked via needsFmt, but add for completeness)
	"Sprintf":     "fmt",
	"Fprintf":     "fmt",
	"Printf":      "fmt",
	"Errorf":      "fmt",
	"fmt.Sprintf": "fmt",
	"fmt.Fprintf": "fmt",
	"fmt.Printf":  "fmt",
	"fmt.Errorf":  "fmt",
}

// NewImportTracker creates a new import tracker
func NewImportTracker() *ImportTracker {
	return &ImportTracker{
		needed:  make(map[string]bool),
		aliases: stdLibFunctions,
	}
}

// TrackFunctionCall records a function call for import tracking
func (it *ImportTracker) TrackFunctionCall(funcName string) {
	if pkg, exists := it.aliases[funcName]; exists {
		it.needed[pkg] = true
	}
}

// GetNeededImports returns a sorted list of needed package imports
func (it *ImportTracker) GetNeededImports() []string {
	imports := make([]string, 0, len(it.needed))
	for pkg := range it.needed {
		imports = append(imports, pkg)
	}
	sort.Strings(imports)
	return imports
}

// Magic Comment System Documentation
//
// The error propagation processor inserts special marker comments to enable
// accurate source mapping between Dingo source and generated Go code.
//
// Format:
//   // dingo:s:1  - Marks the start of an expanded block (1 original line)
//   // dingo:e:1  - Marks the end of an expanded block (1 original line)
//
// Purpose:
//   When a single line of Dingo code (e.g., "let x = ReadFile(path)?") expands to
//   7 lines of Go code, these markers help the LSP server map error positions back
//   to the original Dingo source line.
//
// Example Expansion:
//   Dingo:  let x = ReadFile(path)?
//   Go:     __tmp0, __err0 := ReadFile(path)
//           // dingo:s:1
//           if __err0 != nil {
//               return nil, __err0
//           }
//           // dingo:e:1
//           var x = __tmp0
//
// The number after 's' and 'e' indicates how many original lines were consumed.
// Currently always 1 since error propagation only processes single-line expressions.
//
// Future Enhancement:
//   These markers will be consumed by the LSP server to provide accurate:
//   - Error message positioning
//   - Breakpoint mapping for debugging
//   - Go-to-definition navigation
//   - Hover information

// ErrorPropProcessor handles the ? operator for error propagation
// Transforms: expr? → full error handling expansion
type ErrorPropProcessor struct {
	tryCounter    int
	lines         []string
	currentFunc   *funcContext
	needsFmt      bool
	importTracker *ImportTracker
	mappings      []Mapping // Store mappings for adjustment after import injection
}

// funcContext tracks the current function for zero value generation
type funcContext struct {
	returnTypes []string
	zeroValues  []string
}

// NewErrorPropProcessor creates a new error propagation preprocessor
func NewErrorPropProcessor() *ErrorPropProcessor {
	return &ErrorPropProcessor{
		tryCounter: 0,
	}
}

// Name returns the processor name
func (e *ErrorPropProcessor) Name() string {
	return "error_propagation"
}

// Process transforms error propagation operators
func (e *ErrorPropProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	// Initialize import tracker
	e.importTracker = NewImportTracker()
	e.mappings = []Mapping{}

	// Split into lines for processing
	e.lines = strings.Split(string(source), "\n")
	e.needsFmt = false

	var output bytes.Buffer
	inputLineNum := 0
	outputLineNum := 1 // Track current output line number (1-based)

	for inputLineNum < len(e.lines) {
		line := e.lines[inputLineNum]

		// Check if this is a function declaration
		if e.isFunctionDeclaration(line) {
			e.currentFunc = e.parseFunctionSignature(inputLineNum)
			e.tryCounter = 0 // Reset counter for each function
		}

		// Process the line, passing the current output line number
		transformed, newMappings := e.processLine(line, inputLineNum+1, outputLineNum)
		output.WriteString(transformed)
		if inputLineNum < len(e.lines)-1 {
			output.WriteByte('\n')
		}

		// Add all mappings from this line
		if len(newMappings) > 0 {
			e.mappings = append(e.mappings, newMappings...)
		}

		// Update output line count
		// The transformed text may contain multiple lines.
		// Count: number of newlines + 1 (for the line itself)
		// This gives us the number of lines the transformation occupies.
		newlineCount := strings.Count(transformed, "\n")
		linesOccupied := newlineCount + 1
		outputLineNum += linesOccupied

		inputLineNum++
	}

	// Return result WITHOUT injecting imports
	// Imports will be injected by the main Preprocessor after all transformations
	return output.Bytes(), e.mappings, nil
}

// GetNeededImports implements the ImportProvider interface
func (e *ErrorPropProcessor) GetNeededImports() []string {
	imports := e.importTracker.GetNeededImports()

	// Add fmt if needed for error messages
	if e.needsFmt {
		// Check if fmt is already in the list
		hasFmt := false
		for _, pkg := range imports {
			if pkg == "fmt" {
				hasFmt = true
				break
			}
		}
		if !hasFmt {
			imports = append(imports, "fmt")
		}
	}

	return imports
}

// processLine processes a single line
// Returns: (transformed_text, mappings)
func (e *ErrorPropProcessor) processLine(line string, originalLineNum int, outputLineNum int) (string, []Mapping) {
	// Check if line contains ? operator (and not ternary)
	if !strings.Contains(line, "?") {
		return line, nil
	}

	// Check if it's a ternary (has : after ?)
	if e.isTernaryLine(line) {
		return line, nil
	}

	// Pattern: let/var NAME = EXPR? ["message"]
	if matches := assignPattern.FindStringSubmatch(line); matches != nil {
		rightSide := matches[3] // Everything after =
		if strings.Contains(rightSide, "?") {
			expr, errMsg := e.extractExpressionAndMessage(rightSide)
			result, mappings := e.expandAssignment(matches, expr, errMsg, originalLineNum, outputLineNum)
			return result, mappings
		}
	}

	// Pattern: return EXPR? ["message"]
	if matches := returnPattern.FindStringSubmatch(line); matches != nil {
		returnPart := matches[1] // Everything after return
		if strings.Contains(returnPart, "?") {
			expr, errMsg := e.extractExpressionAndMessage(returnPart)
			result, mappings := e.expandReturn(matches, expr, errMsg, originalLineNum, outputLineNum)
			return result, mappings
		}
	}

	// If we can't recognize the pattern, leave as-is
	return line, nil
}

// extractExpressionAndMessage extracts the expression and optional error message
// Input: "ReadFile(path)? \"failed to read\"" → ("ReadFile(path)?", "failed to read")
// Input: "ReadFile(path)?" → ("ReadFile(path)?", "")
func (e *ErrorPropProcessor) extractExpressionAndMessage(line string) (string, string) {
	// Look for ? followed by optional string (handle escaped quotes)
	// This pattern matches: expr? "message with \" escapes"
	if matches := msgPattern.FindStringSubmatch(strings.TrimSpace(line)); matches != nil {
		return matches[1], matches[2]
	}

	// No message, return as-is
	return strings.TrimSpace(line), ""
}

// expandAssignment expands: let x = expr? → full error handling
// Creates mappings for all 7 generated lines back to the original source line
func (e *ErrorPropProcessor) expandAssignment(matches []string, expr string, errMsg string, originalLine int, startOutputLine int) (string, []Mapping) {
	keyword := matches[1]  // "let" or "var"
	varName := matches[2]  // variable name
	exprClean := strings.TrimSpace(strings.TrimSuffix(expr, "?"))

	// Track function call for import detection
	e.trackFunctionCallInExpr(exprClean)

	tmpVar := fmt.Sprintf("__tmp%d", e.tryCounter)
	errVar := fmt.Sprintf("__err%d", e.tryCounter)
	e.tryCounter++

	// Calculate exact position of ? operator for accurate source mapping
	qPos := strings.Index(expr, "?")
	if qPos == -1 {
		qPos = 0 // fallback if ? not found
	}

	// Generate the expansion
	var buf bytes.Buffer
	indent := e.getIndent(matches[0])
	mappings := []Mapping{}

	// Line 1: __tmpN, __errN := expr
	buf.WriteString(indent)
	buf.WriteString(fmt.Sprintf("%s, %s := %s\n", tmpVar, errVar, exprClean))
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  qPos + 1, // 1-based column position of ?
		GeneratedLine:   startOutputLine,
		GeneratedColumn: 1,
		Length:          1, // length of ? operator
		Name:            "error_prop",
	})

	// Line 2: // dingo:s:1
	buf.WriteString(indent)
	buf.WriteString("// dingo:s:1\n")
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  qPos + 1, // 1-based column position of ?
		GeneratedLine:   startOutputLine + 1,
		GeneratedColumn: 1,
		Length:          1, // length of ? operator
		Name:            "error_prop",
	})

	// Line 3: if __errN != nil {
	buf.WriteString(indent)
	buf.WriteString(fmt.Sprintf("if %s != nil {\n", errVar))
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  qPos + 1, // 1-based column position of ?
		GeneratedLine:   startOutputLine + 2,
		GeneratedColumn: 1,
		Length:          1, // length of ? operator
		Name:            "error_prop",
	})

	// Line 4: return zeroValues, wrapped_error
	buf.WriteString(indent)
	buf.WriteString("\t")
	buf.WriteString(e.generateReturnStatement(errVar, errMsg))
	buf.WriteString("\n")
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  qPos + 1, // 1-based column position of ?
		GeneratedLine:   startOutputLine + 3,
		GeneratedColumn: 1,
		Length:          1, // length of ? operator
		Name:            "error_prop",
	})

	// Line 5: }
	buf.WriteString(indent)
	buf.WriteString("}\n")
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  qPos + 1, // 1-based column position of ?
		GeneratedLine:   startOutputLine + 4,
		GeneratedColumn: 1,
		Length:          1, // length of ? operator
		Name:            "error_prop",
	})

	// Line 6: // dingo:e:1
	buf.WriteString(indent)
	buf.WriteString("// dingo:e:1\n")
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  qPos + 1, // 1-based column position of ?
		GeneratedLine:   startOutputLine + 5,
		GeneratedColumn: 1,
		Length:          1, // length of ? operator
		Name:            "error_prop",
	})

	// Line 7: var varName = __tmpN
	buf.WriteString(indent)
	buf.WriteString(fmt.Sprintf("%s %s = %s", keyword, varName, tmpVar))
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  qPos + 1, // 1-based column position of ?
		GeneratedLine:   startOutputLine + 6,
		GeneratedColumn: 1,
		Length:          1, // length of ? operator
		Name:            "error_prop",
	})

	return buf.String(), mappings
}

// expandReturn expands: return expr? → full error handling
// Creates mappings for all 7 generated lines back to the original source line
func (e *ErrorPropProcessor) expandReturn(matches []string, expr string, errMsg string, originalLine int, startOutputLine int) (string, []Mapping) {
	exprClean := strings.TrimSpace(strings.TrimSuffix(expr, "?"))

	// Track function call for import detection
	e.trackFunctionCallInExpr(exprClean)

	tmpVar := fmt.Sprintf("__tmp%d", e.tryCounter)
	errVar := fmt.Sprintf("__err%d", e.tryCounter)
	e.tryCounter++

	// Calculate exact position of ? operator for accurate source mapping
	qPos := strings.Index(expr, "?")
	if qPos == -1 {
		qPos = 0 // fallback if ? not found
	}

	// Generate the expansion
	var buf bytes.Buffer
	indent := e.getIndent(matches[0])
	mappings := []Mapping{}

	// Line 1: __tmpN, __errN := expr
	buf.WriteString(indent)
	buf.WriteString(fmt.Sprintf("%s, %s := %s\n", tmpVar, errVar, exprClean))
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  qPos + 1, // 1-based column position of ?
		GeneratedLine:   startOutputLine,
		GeneratedColumn: 1,
		Length:          1, // length of ? operator
		Name:            "error_prop",
	})

	// Line 2: // dingo:s:1
	buf.WriteString(indent)
	buf.WriteString("// dingo:s:1\n")
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  qPos + 1, // 1-based column position of ?
		GeneratedLine:   startOutputLine + 1,
		GeneratedColumn: 1,
		Length:          1, // length of ? operator
		Name:            "error_prop",
	})

	// Line 3: if __errN != nil {
	buf.WriteString(indent)
	buf.WriteString(fmt.Sprintf("if %s != nil {\n", errVar))
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  qPos + 1, // 1-based column position of ?
		GeneratedLine:   startOutputLine + 2,
		GeneratedColumn: 1,
		Length:          1, // length of ? operator
		Name:            "error_prop",
	})

	// Line 4: return zeroValues, wrapped_error
	buf.WriteString(indent)
	buf.WriteString("\t")
	buf.WriteString(e.generateReturnStatement(errVar, errMsg))
	buf.WriteString("\n")
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  qPos + 1, // 1-based column position of ?
		GeneratedLine:   startOutputLine + 3,
		GeneratedColumn: 1,
		Length:          1, // length of ? operator
		Name:            "error_prop",
	})

	// Line 5: }
	buf.WriteString(indent)
	buf.WriteString("}\n")
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  qPos + 1, // 1-based column position of ?
		GeneratedLine:   startOutputLine + 4,
		GeneratedColumn: 1,
		Length:          1, // length of ? operator
		Name:            "error_prop",
	})

	// Line 6: // dingo:e:1
	buf.WriteString(indent)
	buf.WriteString("// dingo:e:1\n")
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  qPos + 1, // 1-based column position of ?
		GeneratedLine:   startOutputLine + 5,
		GeneratedColumn: 1,
		Length:          1, // length of ? operator
		Name:            "error_prop",
	})

	// Line 7: return __tmpN, nil (complete tuple for functions returning multiple values)
	buf.WriteString(indent)
	// CRITICAL-3 LIMITATION: Multi-value return handling
	//
	// PROBLEM: When calling a function that returns (A, B, error), the current code generates:
	//   __tmp, __err := funcCall()  // Only captures one value + error
	//   return __tmp, nil           // Missing the second value
	//
	// CORRECT CODE should be:
	//   __tmp1, __tmp2, __err := funcCall()
	//   return __tmp1, __tmp2, nil
	//
	// LIMITATION: At the preprocessor level (text-based), we don't have type information
	// to determine function signatures. Proper fix requires:
	// 1. AST-level type checking (go/types package)
	// 2. Parsing function signatures to count return values
	// 3. Generating correct number of tmp variables
	//
	// CURRENT BEHAVIOR: Assumes single value + error returns
	// WORKAROUND: Users must use multi-value returns in assignment context, not return context
	//
	// TODO (Phase 3): Move error propagation to AST transformer with type info
	var returnVals []string
	returnVals = append(returnVals, tmpVar)

	// Add nil for error position (last return value)
	if e.currentFunc != nil && len(e.currentFunc.returnTypes) > 1 {
		returnVals = append(returnVals, "nil")
	}
	buf.WriteString(fmt.Sprintf("return %s", strings.Join(returnVals, ", ")))
	mappings = append(mappings, Mapping{
		OriginalLine:    originalLine,
		OriginalColumn:  qPos + 1, // 1-based column position of ?
		GeneratedLine:   startOutputLine + 6,
		GeneratedColumn: 1,
		Length:          1, // length of ? operator
		Name:            "error_prop",
	})

	return buf.String(), mappings
}

// generateReturnStatement generates the return statement with proper zero values
func (e *ErrorPropProcessor) generateReturnStatement(errVar string, errMsg string) string {
	// Get zero values for return types
	var zeroVals []string
	if e.currentFunc != nil && len(e.currentFunc.zeroValues) > 0 {
		zeroVals = e.currentFunc.zeroValues
	} else {
		// Fallback: assume one return value (nil)
		zeroVals = []string{"nil"}
	}

	// Generate error part
	var errPart string
	if errMsg != "" {
		// IMPORTANT-1 FIX: Escape % characters to prevent fmt.Errorf runtime panics
		// Example: "failed: 50% complete" → "failed: 50%% complete"
		escapedMsg := strings.ReplaceAll(errMsg, "%", "%%")

		// Wrap with fmt.Errorf
		e.needsFmt = true
		errPart = fmt.Sprintf(`fmt.Errorf("%s: %%w", %s)`, escapedMsg, errVar)
	} else {
		// Pass through as-is
		errPart = errVar
	}

	// Combine: return zeroVal1, zeroVal2, ..., error
	if len(zeroVals) > 0 {
		// Function returns (T, error) or (T1, T2, ..., error)
		return fmt.Sprintf("return %s, %s", strings.Join(zeroVals, ", "), errPart)
	} else {
		// Function returns only error
		return fmt.Sprintf("return %s", errPart)
	}
}

// isFunctionDeclaration checks if a line is a function declaration
func (e *ErrorPropProcessor) isFunctionDeclaration(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "func ")
}

// parseFunctionSignature parses a function signature to extract return types
func (e *ErrorPropProcessor) parseFunctionSignature(startLine int) *funcContext {
	// Collect lines until we find the opening brace
	// Safety limit: search up to 20 lines for opening brace
	var funcText strings.Builder
	foundBrace := false
	maxLines := startLine + 20
	if maxLines > len(e.lines) {
		maxLines = len(e.lines)
	}

	for i := startLine; i < maxLines; i++ {
		funcText.WriteString(e.lines[i])
		funcText.WriteString("\n")

		trimmed := strings.TrimSpace(e.lines[i])
		// Skip comment lines
		if strings.HasPrefix(trimmed, "//") {
			continue
		}

		if idx := strings.Index(trimmed, "{"); idx != -1 {
			foundBrace = true
			break
		}
	}

	if !foundBrace {
		// No brace found - return safe fallback
		return &funcContext{
			returnTypes: []string{},
			zeroValues:  []string{"nil"},
		}
	}

	// Parse as Go function
	fset := token.NewFileSet()
	src := fmt.Sprintf("package p\n%s}", funcText.String())
	file, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		// Failed to parse, use nil fallback
		return &funcContext{
			returnTypes: []string{},
			zeroValues:  []string{"nil"},
		}
	}

	// Extract function declaration
	if len(file.Decls) == 0 {
		return &funcContext{
			returnTypes: []string{},
			zeroValues:  []string{"nil"},
		}
	}

	funcDecl, ok := file.Decls[0].(*ast.FuncDecl)
	if !ok || funcDecl.Type.Results == nil {
		return &funcContext{
			returnTypes: []string{},
			zeroValues:  []string{"nil"},
		}
	}

	// Extract return types
	returnTypes := []string{}
	for _, field := range funcDecl.Type.Results.List {
		typeStr := types.ExprString(field.Type)
		// If field has multiple names, repeat type (rare for returns)
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for i := 0; i < count; i++ {
			returnTypes = append(returnTypes, typeStr)
		}
	}

	// Generate zero values (all except last, which is error)
	zeroValues := []string{}
	for i := 0; i < len(returnTypes)-1; i++ {
		zeroValues = append(zeroValues, getZeroValue(returnTypes[i]))
	}

	return &funcContext{
		returnTypes: returnTypes,
		zeroValues:  zeroValues,
	}
}

// getZeroValue returns the zero value for a given type
// IMPORTANT-4 FIX: Improved handling of edge cases (type aliases, generics, complex types)
func getZeroValue(typ string) string {
	typ = strings.TrimSpace(typ)

	// Empty type → fallback to nil
	if typ == "" {
		return "nil"
	}

	// Built-in types
	zeroMap := map[string]string{
		"int":     "0",
		"int8":    "0",
		"int16":   "0",
		"int32":   "0",
		"int64":   "0",
		"uint":    "0",
		"uint8":   "0",
		"uint16":  "0",
		"uint32":  "0",
		"uint64":  "0",
		"uintptr": "0",
		"float32": "0.0",
		"float64": "0.0",
		"string":  `""`,
		"bool":    "false",
		"error":   "nil",
		"byte":    "0",
		"rune":    "0",
		"complex64":  "0",
		"complex128": "0",
	}

	if zero, ok := zeroMap[typ]; ok {
		return zero
	}

	// IMPORTANT-4 FIX: Handle type parameters (generics like T, K, V)
	// Single uppercase letter or name starting with uppercase followed by constraint
	// Examples: T, K, V, T comparable, K any
	// For generics, we cannot determine zero value at compile time, use nil as fallback
	if len(typ) == 1 && typ[0] >= 'A' && typ[0] <= 'Z' {
		// Single letter generic: T, K, V, etc.
		return "nil" // Safe fallback - will work for most generic constraints
	}

	// Pointer, slice, map, chan, interface → nil
	// IMPORTANT-4 FIX: Check slices, maps BEFORE generic instantiation check
	if strings.HasPrefix(typ, "*") ||
		strings.HasPrefix(typ, "[]") ||
		strings.HasPrefix(typ, "map[") ||
		strings.HasPrefix(typ, "chan ") ||
		strings.HasPrefix(typ, "<-chan ") ||
		strings.HasPrefix(typ, "chan<- ") ||
		typ == "interface{}" ||
		strings.HasPrefix(typ, "interface{") ||
		typ == "any" { // IMPORTANT-4 FIX: Handle 'any' alias for interface{}
		return "nil"
	}

	// Function type → nil
	if strings.HasPrefix(typ, "func(") || strings.HasPrefix(typ, "func (") {
		return "nil"
	}

	// IMPORTANT-4 FIX: Array types [N]T → use composite literal
	// Must check BEFORE generic instantiation check (which also has [...])
	if strings.HasPrefix(typ, "[") && !strings.HasPrefix(typ, "[]") && strings.Contains(typ, "]") {
		// Fixed-size array like [10]int
		return typ + "{}"
	}

	// IMPORTANT-4 FIX: Handle qualified type names (pkg.Type)
	// These should use composite literals
	if strings.Contains(typ, ".") {
		return typ + "{}"
	}

	// IMPORTANT-4 FIX: Handle generic type instantiations like List[int], Map[string, User]
	// Must check AFTER slices/maps/arrays to avoid false positives
	if strings.Contains(typ, "[") && strings.Contains(typ, "]") {
		// Generic type instantiation - use composite literal
		return typ + "{}"
	}

	// IMPORTANT-4 FIX: Custom type → use composite literal for non-pointer types
	// This handles type aliases and custom struct types
	// Examples: MyType, UserID, RequestStatus
	if !strings.HasPrefix(typ, "*") &&
		!strings.HasPrefix(typ, "[]") &&
		!strings.HasPrefix(typ, "map[") &&
		!strings.HasPrefix(typ, "chan ") &&
		!strings.HasPrefix(typ, "<-chan ") &&
		!strings.HasPrefix(typ, "chan<- ") &&
		!strings.HasPrefix(typ, "func(") &&
		!strings.HasPrefix(typ, "func (") &&
		!strings.HasPrefix(typ, "interface{") &&
		typ != "interface{}" &&
		typ != "any" {
		// Check if it looks like a type name (starts with uppercase or contains alphanumeric)
		if len(typ) > 0 && (typ[0] >= 'A' && typ[0] <= 'Z' || typ[0] >= 'a' && typ[0] <= 'z') {
			return typ + "{}"
		}
	}

	// IMPORTANT-4 FIX: Safe fallback for unknown/unparseable types
	// Better to return nil than cause a compilation error
	return "nil"
}

// getIndent extracts leading whitespace from a line
func (e *ErrorPropProcessor) getIndent(line string) string {
	for i, ch := range line {
		if ch != ' ' && ch != '\t' {
			return line[:i]
		}
	}
	return ""
}

// isTernaryLine checks if the line contains a ternary operator
func (e *ErrorPropProcessor) isTernaryLine(line string) bool {
	// Check for ternary pattern: expr ? value : value
	// Important: Must exclude : inside string literals (e.g., error messages)
	qPos := strings.Index(line, "?")
	if qPos == -1 {
		return false
	}

	// Scan after the ? to find : that's NOT in a string literal
	remainder := line[qPos+1:]
	inString := false
	escaped := false

	for _, ch := range remainder {
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

		// Found : outside of string - this is a ternary
		if ch == ':' && !inString {
			return true
		}
	}

	return false
}

// trackFunctionCallInExpr extracts function name from expression and tracks it
// Handles patterns like: FuncName(args), pkg.FuncName(args), obj.Method(args)
//
// IMPORTANT-1 FIX: Now tracks BOTH qualified calls (pkg.Function) and bare function names
// to support patterns like:
//   - http.Get()     → detects "http.Get" and injects "net/http"
//   - filepath.Join() → detects "filepath.Join" and injects "path/filepath"
//   - json.Marshal() → detects "json.Marshal" and injects "encoding/json"
//
// This prevents false positives where user-defined functions with common names
// would incorrectly trigger import injection.
func (e *ErrorPropProcessor) trackFunctionCallInExpr(expr string) {
	// Simple extraction: find identifier before '('
	parenIdx := strings.Index(expr, "(")
	if parenIdx == -1 {
		return
	}

	// Get the part before '('
	beforeParen := strings.TrimSpace(expr[:parenIdx])

	// Split by '.' to handle qualified names (pkg.Func or obj.Method)
	parts := strings.Split(beforeParen, ".")

	// IMPORTANT-1 FIX: Track BOTH the qualified name AND the bare function name
	// This enables detection of both:
	//   1. Qualified calls: http.Get() → "http.Get"
	//   2. Bare calls: ReadFile() → "ReadFile" (but only if in stdLibFunctions)

	if len(parts) >= 2 {
		// Qualified call: try "pkg.Function" pattern first (more specific)
		qualifiedName := strings.Join(parts[len(parts)-2:], ".")
		if e.importTracker != nil {
			e.importTracker.TrackFunctionCall(qualifiedName)
		}
	}

	// Also track the last part (bare function name) as fallback
	if len(parts) > 0 {
		funcName := strings.TrimSpace(parts[len(parts)-1])
		if funcName != "" && e.importTracker != nil {
			e.importTracker.TrackFunctionCall(funcName)
		}
	}
}

