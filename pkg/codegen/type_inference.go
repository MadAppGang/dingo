package codegen

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"strings"
)

// extractFunctionSignatures extracts function/method signatures from Dingo source.
// It uses go/scanner to tokenize and extract only the signature portion (func ... { ),
// replacing the body with empty braces {}. This allows go/parser to parse the
// signatures even when the source contains Dingo-specific syntax like match expressions.
//
// IMPORTANT: Only extracts TOP-LEVEL NAMED functions and methods:
//   - `func FunctionName(params) returns { ... }`
//   - `func (receiver) MethodName(params) returns { ... }`
//
// Skips:
//   - `type Foo func(...)` - type alias to function type
//   - `return func(...) {...}` - function literals
//   - `x := func(...) {...}` - anonymous functions assigned to variables
//
// Returns a valid Go source string containing a package declaration and function stubs.
func extractFunctionSignatures(src []byte) string {
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))

	var s scanner.Scanner
	s.Init(file, src, nil, 0)

	var result []byte
	result = append(result, "package p\n"...)

	// State machine for extracting signatures
	// States:
	// 0 = looking for 'func'
	// 1 = found 'func', need to determine if it's a named function/method
	// 2 = in receiver parens, waiting for closing paren
	// 3 = confirmed named function, collecting signature until '{'
	// 4 = inside function body, skip until balanced braces
	// 5 = skip this func entirely (it's a literal or type def)
	state := 0
	braceDepth := 0
	parenDepth := 0
	sigStart := -1

	for {
		pos, tok, _ := s.Scan()
		if tok == token.EOF {
			break
		}

		offset := fset.Position(pos).Offset

		switch state {
		case 0: // Looking for 'func'
			if tok == token.FUNC {
				state = 1
				sigStart = offset
			}

		case 1: // Found 'func', determine what kind
			if tok == token.IDENT {
				// func FunctionName(...) - regular named function
				state = 3
			} else if tok == token.LPAREN {
				// func (... - could be method receiver OR function literal params
				// Need to look ahead: method has IDENT after receiver parens
				state = 2
				parenDepth = 1
			} else {
				// Unexpected token after func - skip this func
				state = 0
				sigStart = -1
			}

		case 2: // Inside receiver parens (or function literal params)
			if tok == token.LPAREN {
				parenDepth++
			} else if tok == token.RPAREN {
				parenDepth--
				if parenDepth == 0 {
					// Parens closed, next token determines if method or literal
					// Stay in state 2 with parenDepth=0 to check next token
				}
			} else if parenDepth == 0 {
				// We're past the parens, check what comes next
				if tok == token.IDENT {
					// func (receiver) MethodName(...) - this is a method
					state = 3
				} else if tok == token.LBRACE {
					// func (params) { - function literal, skip it
					state = 4
					braceDepth = 1
					sigStart = -1
				} else if tok == token.LPAREN {
					// func (receiver) (params) - could be method with no name?
					// Actually this is invalid Go, but let's handle gracefully
					state = 5
					parenDepth = 1
				} else {
					// Something else - skip
					state = 0
					sigStart = -1
				}
			}

		case 3: // Confirmed named function, collecting signature
			if tok == token.LBRACE {
				// Extract signature from sigStart to current position
				sig := src[sigStart:offset]
				result = append(result, sig...)
				result = append(result, " {}\n"...)
				state = 4
				braceDepth = 1
			} else if tok == token.SEMICOLON {
				// Interface method or forward declaration - no body
				sig := src[sigStart:offset]
				result = append(result, sig...)
				result = append(result, " {}\n"...)
				state = 0
				sigStart = -1
			}

		case 4: // Inside function body, skip until braces balance
			if tok == token.LBRACE {
				braceDepth++
			} else if tok == token.RBRACE {
				braceDepth--
				if braceDepth == 0 {
					state = 0
					sigStart = -1
				}
			}

		case 5: // Skip nested parens (for edge cases)
			if tok == token.LPAREN {
				parenDepth++
			} else if tok == token.RPAREN {
				parenDepth--
				if parenDepth == 0 {
					state = 0
					sigStart = -1
				}
			}
		}
	}

	if len(result) <= len("package p\n") {
		return ""
	}

	return string(result)
}

// InferReturnTypes analyzes the source to find the enclosing function's return types
// and returns the corresponding zero values for each return position.
// The last return value is always assumed to be the error being propagated.
func InferReturnTypes(src []byte, exprPos int) []string {
	funcDecl := findEnclosingFunction(src, exprPos)
	if funcDecl == nil {
		return []string{"nil"} // Fallback
	}

	returnTypes := parseReturnTypes(funcDecl)
	if len(returnTypes) == 0 {
		return []string{}
	}

	// Convert types to zero values (excluding the last one which is err)
	zeroValues := make([]string, len(returnTypes)-1)
	for i := 0; i < len(returnTypes)-1; i++ {
		zeroValues[i] = zeroValueFor(returnTypes[i])
	}

	return zeroValues
}

// InferReturnTypeNames analyzes the source to find the enclosing function's return types
// and returns the actual type names (not zero values).
// Returns nil if unable to determine types.
func InferReturnTypeNames(src []byte, exprPos int) []string {
	funcDecl := findEnclosingFunction(src, exprPos)
	if funcDecl == nil {
		return nil
	}
	return parseReturnTypes(funcDecl)
}

// CanPropagateError checks if the enclosing function at the given position can propagate errors.
// Returns (true, "") if the function returns error or (T, error) or Result[T,E].
// Returns (false, funcName) if the function has no error return (void, or non-error returns only).
// Returns (true, "") if unable to determine (conservative - don't block compilation).
func CanPropagateError(src []byte, exprPos int) (bool, string) {
	funcDecl := findEnclosingFunction(src, exprPos)
	if funcDecl == nil {
		// Can't determine enclosing function - be conservative, don't block
		return true, ""
	}

	funcName := ""
	if funcDecl.Name != nil {
		funcName = funcDecl.Name.Name
	}

	returnTypes := parseReturnTypes(funcDecl)
	if len(returnTypes) == 0 {
		// Void function - cannot propagate errors
		return false, funcName
	}

	// Check if any return type is "error" or a Result type
	for _, rt := range returnTypes {
		if rt == "error" {
			return true, ""
		}
		if IsResultType(rt) {
			return true, ""
		}
	}

	// Has return types but none is error or Result
	return false, funcName
}

// InferEnclosingFunctionReturnsResult checks if the enclosing function returns a Result type.
// Returns the Result's T type if it does, empty string otherwise.
// For example, for `func foo() Result[User, error]`, returns "User".
func InferEnclosingFunctionReturnsResult(src []byte, exprPos int) string {
	typeNames := InferReturnTypeNames(src, exprPos)
	if len(typeNames) == 0 {
		return ""
	}

	// Check each return type for Result pattern
	for _, typeName := range typeNames {
		if IsResultType(typeName) {
			okType := ExtractResultOkType(typeName)
			if okType != "" {
				return okType
			}
		}
	}
	return ""
}

// InferEnclosingFunctionResultTypes checks if the enclosing function returns a Result type.
// Returns (isResult, okType, errType) where:
//   - isResult: true if the enclosing function returns Result[T, E]
//   - okType: the T type from Result[T, E]
//   - errType: the E type from Result[T, E]
//
// For example, for `func foo() Result[User, AppError]`, returns (true, "User", "AppError").
func InferEnclosingFunctionResultTypes(src []byte, exprPos int) (isResult bool, okType string, errType string) {
	typeNames := InferReturnTypeNames(src, exprPos)
	if len(typeNames) == 0 {
		return false, "", ""
	}

	// Check each return type for Result pattern
	for _, typeName := range typeNames {
		if IsResultType(typeName) {
			okType = ExtractResultOkType(typeName)
			errType = ExtractResultErrType(typeName)
			if okType != "" && errType != "" {
				return true, okType, errType
			}
		}
	}
	return false, "", ""
}

// InferExprReturnsResult checks if an expression returns a Result type.
// Returns (isResult, okType, errType) where:
//   - isResult: true if the expression returns a Result[T, E] type
//   - okType: the T type from Result[T, E]
//   - errType: the E type from Result[T, E]
//
// This is used by error propagation to determine whether to generate:
//   - Result pattern: tmp := expr; if tmp.IsErr() { return dgo.Err[T,E](tmp.MustErr()) }
//   - Tuple pattern: tmp, err := expr; if err != nil { return ..., err }
//
// The function works by:
//  1. Extracting the function/method name from the expression
//  2. Finding the function declaration in the source (Strategy 1)
//  3. If not found, checking if the enclosing function returns Result (Strategy 2 - heuristic)
//  4. Checking if its return type matches Result[T, E] pattern
//
// For cross-file type resolution, use InferExprReturnsResultWithResolver instead.
func InferExprReturnsResult(src []byte, exprBytes []byte, exprPos int) (isResult bool, okType string, errType string) {
	return InferExprReturnsResultWithResolver(src, exprBytes, exprPos, nil)
}

// InferExprReturnsResultWithResolver checks if an expression returns a Result type.
// If resolver is provided, it uses cross-file type resolution for imported packages.
//
// Returns (isResult, okType, errType) where:
//   - isResult: true if the expression returns a Result[T, E] type
//   - okType: the T type from Result[T, E]
//   - errType: the E type from Result[T, E]
func InferExprReturnsResultWithResolver(src []byte, exprBytes []byte, exprPos int, resolver *TypeResolver) (isResult bool, okType string, errType string) {
	// Strategy 0: Try resolver first if available (handles cross-file/cross-package)
	if resolver != nil {
		isResult, okType, errType = resolver.GetReturnTypeInfo(exprBytes)
		if isResult {
			return isResult, okType, errType
		}
		// Resolver didn't find it - fall through to local search
	}
	// Parse the expression to extract function name
	exprStr := string(exprBytes)
	methodName := extractMethodName(exprStr)

	// Strategy 1: Find the method/function declaration in the current file
	// This searches for both top-level functions AND methods with receivers.
	//
	// IMPORTANT: Only match for:
	// - Direct function calls: foo()
	// - Method calls on self: v.validateWithCache() where v is the receiver
	//
	// DO NOT match for:
	// - Method calls on fields: s.companyRepo.Update() where companyRepo is a field
	//   These have different types and matching by name causes collisions.
	if methodName != "" && !isMethodCallOnField(exprStr) {
		returnType := findMethodReturnType(src, methodName)
		if returnType != "" {
			// Found the method definition in current file
			if IsResultType(returnType) {
				okType = ExtractResultOkType(returnType)
				errType = ExtractResultErrType(returnType)
				return true, okType, errType
			}
			// Method found but doesn't return Result - use tuple pattern
			return false, "", ""
		}
	}

	// Strategy 1b: Check interface methods in the CURRENT source file
	// This handles cases like f.Refresh() where JWKSFetcher interface is defined in the same file.
	if methodName != "" {
		localInterfaceMethods := parseLocalInterfaceMethods(src)
		if returnType, found := localInterfaceMethods[methodName]; found {
			if IsResultType(returnType) {
				okType = ExtractResultOkType(returnType)
				errType = ExtractResultErrType(returnType)
				return true, okType, errType
			}
			// Found but doesn't return Result - use tuple pattern
			return false, "", ""
		}
	}

	// Strategy 2: Be conservative - default to tuple pattern
	// We can't reliably determine if a cross-file Dingo function returns Result[T,E]
	// or (T, error) without parsing the target package's source.
	//
	// The tuple pattern (T, error) is the Go standard and works for most cases.
	// Result-returning functions in the SAME file are already found by Strategy 1.
	// For cross-file Result calls, users should ensure the function is defined in the same file
	// or the caller should also use tuple pattern for interop.
	//
	// REMOVED: Aggressive heuristic that assumed Result pattern based on enclosing function.
	// This caused issues when mixing Result-returning and tuple-returning functions.

	// No method definition found - use tuple pattern as safe default
	return false, "", ""
}

// parseLocalInterfaceMethods extracts interface method signatures from the current source file.
// Returns a map of method name to return type string.
// This enables detection of Result-returning methods defined in interfaces within the same file.
//
// Key insight: Interface declarations are typically at the TOP of the file, before function
// bodies that may contain Dingo-specific syntax (match, ?). We extract just the part before
// the first "func " to get parseable Go code containing the interface definitions.
func parseLocalInterfaceMethods(src []byte) map[string]string {
	result := make(map[string]string)

	// Find the first "\nfunc " to extract only package + imports + type declarations
	// This avoids parsing function bodies which may contain Dingo-specific syntax
	funcIdx := bytes.Index(src, []byte("\nfunc "))
	var partial []byte
	if funcIdx == -1 {
		partial = src // No functions, parse the whole thing
	} else {
		partial = src[:funcIdx]
	}

	// Parse the partial source
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", partial, parser.SkipObjectResolution)
	if err != nil {
		return result
	}

	// Find all interface type declarations
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}

			// Extract methods from interface
			if interfaceType.Methods != nil {
				for _, method := range interfaceType.Methods.List {
					funcType, ok := method.Type.(*ast.FuncType)
					if !ok {
						continue
					}

					if len(method.Names) == 0 {
						continue
					}
					methodName := method.Names[0].Name

					// Extract first return type
					if funcType.Results != nil && len(funcType.Results.List) > 0 {
						returnType := exprToTypeName(funcType.Results.List[0].Type)
						result[methodName] = returnType
					}
				}
			}
		}
	}

	return result
}

// isKnownTupleMethod checks if a method name is a known stdlib method that returns (T, error).
// This is used to avoid incorrectly treating stdlib calls as Result-returning.
func isKnownTupleMethod(methodName string) bool {
	// Common Go stdlib methods that return (T, error) or just error
	// This list covers the most common patterns
	knownTupleMethods := map[string]bool{
		// io operations
		"Read": true, "Write": true, "Close": true, "Seek": true,
		"ReadAll": true, "ReadFile": true, "WriteFile": true,
		"Copy": true, "CopyN": true, "ReadFrom": true, "WriteTo": true,
		// os operations
		"Open": true, "Create": true, "Remove": true, "Rename": true,
		"Mkdir": true, "MkdirAll": true, "Stat": true, "Lstat": true,
		"Getwd": true, "Chdir": true, "Chmod": true, "Chown": true,
		// encoding/json, encoding/xml
		"Marshal": true, "Unmarshal": true, "Encode": true, "Decode": true,
		// net/http
		"Get": true, "Post": true, "Do": true, "NewRequest": true,
		"ListenAndServe": true, "ListenAndServeTLS": true,
		// database/sql
		"Query": true, "QueryRow": true, "Exec": true, "Prepare": true,
		"Begin": true, "Commit": true, "Rollback": true, "Scan": true,
		// context
		"WithCancel": true, "WithTimeout": true, "WithDeadline": true,
		// strconv
		"Atoi": true, "ParseInt": true, "ParseFloat": true, "ParseBool": true,
		// time
		"Parse": true, "ParseDuration": true, "LoadLocation": true,
		// regexp
		"Compile": true, "MustCompile": true, "Match": true, "MatchString": true,
		// sync
		"Wait": true, "Lock": true, "Unlock": true,
		// fmt - these don't return error as second value typically, but good to know
		"Println": true, "Printf": true, "Sprintf": true,
		// common config/init patterns that return tuple
		"Load": true, "Init": true, "Setup": true, "Configure": true,
	}

	// Check direct match
	if knownTupleMethods[methodName] {
		return true
	}

	// Check for common naming patterns that suggest tuple returns
	lower := strings.ToLower(methodName)

	// Methods starting with common tuple-return prefixes
	// NOTE: "new" is intentionally excluded because project-local methods like
	// NewJWKSFetcher might return Result[T, E] instead of (T, error).
	// We only want to match stdlib patterns here.
	tuplePrefixes := []string{
		"get", "set", "read", "write", "load", "save", "fetch", "send",
		"parse", "format", "encode", "decode", "marshal", "unmarshal",
		"open", "close", "create", "delete", "update", "insert",
		"connect", "disconnect", "dial", "listen", "accept",
		// "new" removed - too broad, catches project methods returning Result
	}
	for _, prefix := range tuplePrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}

	return false
}

// getEnclosingFunctionReturnType finds the enclosing function and returns its return type as a string.
// Returns empty string if not found or if function has multiple return values.
func getEnclosingFunctionReturnType(src []byte, exprPos int) string {
	funcDecl := findEnclosingFunction(src, exprPos)
	if funcDecl == nil {
		return ""
	}

	returnTypes := parseReturnTypes(funcDecl)
	if len(returnTypes) == 1 {
		return returnTypes[0]
	}
	return ""
}

// findMethodReturnType searches the source for a function/method with the given name
// and returns its return type as a string, or empty string if not found.
//
// NOTE: This function extracts function signatures using go/scanner to handle
// Dingo-specific syntax (match expressions, ?, etc.) that go/parser cannot handle.
// Function signatures are valid Go syntax, so we extract them and parse those.
func findMethodReturnType(src []byte, methodName string) string {
	// Extract function signatures (valid Go) from Dingo source
	signatures := extractFunctionSignatures(src)
	if signatures == "" {
		return ""
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", signatures, parser.ParseComments)
	if err != nil {
		return ""
	}

	var returnType string
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}
		if fn.Name.Name == methodName {
			if fn.Type.Results != nil && fn.Type.Results.NumFields() > 0 {
				// Get the first return type
				returnType = exprToTypeName(fn.Type.Results.List[0].Type)
			}
			return false
		}
		return true
	})

	return returnType
}

// ExtractResultErrType extracts the E from Result[T, E] or dgo.Result[T, E]
// Returns empty string if not a Result type or extraction fails
func ExtractResultErrType(typeStr string) string {
	// Find "Result[" in the string
	resultIdx := -1
	for i := 0; i <= len(typeStr)-7; i++ {
		if typeStr[i:i+7] == "Result[" {
			resultIdx = i + 7
			break
		}
	}
	if resultIdx == -1 {
		return ""
	}

	// Find the comma that separates T and E (at depth 1)
	// Track both brackets [] and parentheses () for tuple types like (string, string)
	bracketDepth := 1
	parenDepth := 0
	commaIdx := -1
	for i := resultIdx; i < len(typeStr); i++ {
		switch typeStr[i] {
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
			if bracketDepth == 0 {
				// No comma found, invalid Result type
				return ""
			}
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case ',':
			// Only match comma at top level (bracket depth 1, not inside parens)
			if bracketDepth == 1 && parenDepth == 0 {
				commaIdx = i
				break
			}
		}
		if commaIdx != -1 {
			break
		}
	}
	if commaIdx == -1 {
		return ""
	}

	// Extract from comma+1 to closing bracket
	// Skip leading space after comma
	start := commaIdx + 1
	for start < len(typeStr) && typeStr[start] == ' ' {
		start++
	}

	// Find closing bracket at depth 1
	bracketDepth = 1
	for i := start; i < len(typeStr); i++ {
		switch typeStr[i] {
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
			if bracketDepth == 0 {
				return typeStr[start:i]
			}
		}
	}
	return ""
}

// findEnclosingFunction finds the function declaration that contains exprPos
func findEnclosingFunction(src []byte, exprPos int) *ast.FuncDecl {
	// First try: parse as complete file
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err == nil {
		var enclosing *ast.FuncDecl
		ast.Inspect(f, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				start := fset.Position(fn.Pos()).Offset
				end := fset.Position(fn.End()).Offset
				if start <= exprPos && exprPos <= end {
					enclosing = fn
				}
			}
			return true
		})
		if enclosing != nil {
			return enclosing
		}
	}

	// Second try: wrap as function body and parse
	wrapped := "package p\n" + string(src)
	fset = token.NewFileSet()
	f, err = parser.ParseFile(fset, "", wrapped, parser.ParseComments)
	if err == nil {
		var enclosing *ast.FuncDecl
		ast.Inspect(f, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				enclosing = fn
			}
			return true
		})
		if enclosing != nil {
			return enclosing
		}
	}

	// Third try: line-based fallback
	return findEnclosingFunctionFallback(src, exprPos)
}

// findEnclosingFunctionFallback uses go/scanner when go/parser fails.
// This properly tokenizes the source to find the enclosing function declaration.
func findEnclosingFunctionFallback(src []byte, exprPos int) *ast.FuncDecl {
	// Tokenize source up to exprPos using go/scanner
	// Collect FUNC positions along with whether they're named (declarations)
	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))

	type funcInfo struct {
		pos     int
		isNamed bool // true if it's a named function (declaration)
	}
	var funcs []funcInfo

	s.Init(file, src, nil, 0)

	var lastTok token.Token
	var afterFuncSawReceiver bool // tracks if we saw a receiver after FUNC
	for {
		pos, tok, _ := s.Scan()
		if tok == token.EOF {
			break
		}
		offset := fset.Position(pos).Offset
		if offset > exprPos {
			break // Stop when we pass exprPos
		}
		if lastTok == token.FUNC && tok == token.IDENT {
			// Previous token was FUNC, this is a name → function declaration (no receiver)
			if len(funcs) > 0 {
				funcs[len(funcs)-1].isNamed = true
			}
			afterFuncSawReceiver = false
		} else if lastTok == token.FUNC && tok == token.LPAREN {
			// Previous token was FUNC, opening paren → method receiver starts
			afterFuncSawReceiver = true
		} else if afterFuncSawReceiver && lastTok == token.RPAREN && tok == token.IDENT {
			// After receiver closing paren, this IDENT is the method name → method declaration
			if len(funcs) > 0 {
				funcs[len(funcs)-1].isNamed = true
			}
			afterFuncSawReceiver = false
		}
		if tok == token.FUNC {
			funcs = append(funcs, funcInfo{pos: offset, isNamed: false})
			afterFuncSawReceiver = false
		}
		lastTok = tok
	}

	if len(funcs) == 0 {
		return nil
	}

	// Find the last NAMED function before exprPos (skip anonymous functions)
	var lastFunc int = -1
	for i := len(funcs) - 1; i >= 0; i-- {
		if funcs[i].isNamed {
			lastFunc = funcs[i].pos
			break
		}
	}

	if lastFunc == -1 {
		// No named functions found, try the last function anyway
		lastFunc = funcs[len(funcs)-1].pos
	}

	// Find the opening brace using a new scanner with new file
	remainder := src[lastFunc:]
	fset2 := token.NewFileSet()
	file2 := fset2.AddFile("", fset2.Base(), len(remainder))
	s.Init(file2, remainder, nil, 0)
	braceOffset := -1
	for {
		pos, tok, _ := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.LBRACE {
			braceOffset = fset2.Position(pos).Offset
			break
		}
	}

	// Extract the function signature
	var funcDecl string
	if braceOffset != -1 {
		funcDecl = string(src[lastFunc : lastFunc+braceOffset])
	} else {
		funcDecl = string(src[lastFunc:exprPos])
	}

	// Parse just the signature
	fset = token.NewFileSet()
	wrapped := funcDecl + " {}"
	f, err := parser.ParseFile(fset, "", "package p\n"+wrapped, 0)
	if err == nil && len(f.Decls) > 0 {
		if fn, ok := f.Decls[0].(*ast.FuncDecl); ok {
			return fn
		}
	}

	return nil
}

// parseReturnTypes extracts return type names from a function declaration
func parseReturnTypes(fn *ast.FuncDecl) []string {
	if fn == nil || fn.Type == nil || fn.Type.Results == nil {
		return nil
	}

	var types []string
	for _, field := range fn.Type.Results.List {
		typeName := exprToTypeName(field.Type)
		// Handle multiple names: (a, b int) counts as 2 returns
		if len(field.Names) > 0 {
			for range field.Names {
				types = append(types, typeName)
			}
		} else {
			types = append(types, typeName)
		}
	}
	return types
}

// exprToTypeName converts an ast.Expr to a type name string
func exprToTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprToTypeName(t.X)
	case *ast.ArrayType:
		return "[]" + exprToTypeName(t.Elt)
	case *ast.MapType:
		return "map[" + exprToTypeName(t.Key) + "]" + exprToTypeName(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.SelectorExpr:
		return exprToTypeName(t.X) + "." + t.Sel.Name
	case *ast.ChanType:
		return "chan " + exprToTypeName(t.Value)
	case *ast.FuncType:
		return "func"
	case *ast.IndexExpr:
		// Generic type with single parameter: Result[T]
		return exprToTypeName(t.X) + "[" + exprToTypeName(t.Index) + "]"
	case *ast.IndexListExpr:
		// Generic type with multiple parameters: Result[T, E]
		result := exprToTypeName(t.X) + "["
		for i, idx := range t.Indices {
			if i > 0 {
				result += ", "
			}
			result += exprToTypeName(idx)
		}
		result += "]"
		return result
	default:
		return "interface{}"
	}
}

// InferExprReturnCount determines how many values an expression returns.
// Returns 1 for single-return expressions (like row.Scan() returning just error),
// Returns 2 for multi-return expressions (like db.Query() returning (*Rows, error)),
// Returns -1 if detection fails (fallback to multi-return assumption).
//
// This function attempts to type-check the expression by:
// 1. Using TypeResolver for cross-file/cross-package type resolution (if provided)
// 2. Extracting the function being called from the expression
// 3. Finding the function declaration in the source
// 4. Counting return values from the signature
//
// For external package functions (like sql.Row.Scan), detection may fail
// and the caller should use the fallback value.
func InferExprReturnCount(src []byte, exprBytes []byte, exprPos int) int {
	// Delegate to the version with optional resolver
	return InferExprReturnCountWithResolver(src, exprBytes, exprPos, nil)
}

// InferExprReturnCountWithResolver determines how many values an expression returns.
// If resolver is provided and can resolve the type, it takes precedence.
// Otherwise falls back to local-only search (existing behavior).
//
// Returns 1 for single-return expressions (like row.Scan() returning just error),
// Returns 2+ for multi-return expressions (like db.Query() returning (*Rows, error)),
// Returns -1 if detection fails (fallback to multi-return assumption).
func InferExprReturnCountWithResolver(src []byte, exprBytes []byte, exprPos int, resolver *TypeResolver) int {
	// Try resolver first if available
	if resolver != nil {
		count := resolver.GetReturnCount(exprBytes)
		if count > 0 {
			return count
		}
		// Fall through to local search if resolver fails
	}

	// Parse the expression using go/parser to extract method name
	exprStr := string(exprBytes)
	methodName := extractMethodName(exprStr)
	if methodName != "" {
		// Check if this method is defined in the current source
		count := findMethodReturnCount(src, methodName)
		if count > 0 {
			return count
		}
	}

	// For external packages, we can't easily determine return count without full type info
	// Return -1 to signal caller should use fallback
	return -1
}

// extractMethodName extracts the method/function name from a call expression
// using go/parser to properly parse the expression AST.
// e.g., "row.Scan(&user.ID)" -> "Scan", "foo()" -> "foo"
func extractMethodName(expr string) string {
	// Parse as Go expression using go/parser
	parsedExpr, err := parser.ParseExpr(expr)
	if err != nil {
		return ""
	}

	// Check if it's a call expression
	callExpr, ok := parsedExpr.(*ast.CallExpr)
	if !ok {
		return ""
	}

	// Extract the function/method name from the call
	return extractFuncName(callExpr.Fun)
}

// isMethodCallOnReceiver checks if an expression is a method call on a receiver (not a direct function call).
// Returns true for: s.repo.GetBySlug(), obj.Method(), pkg.Func()
// Returns false for: foo(), getResult()
//
// This is used to prevent incorrectly matching local functions when the call is a method
// on an external receiver that we can't type-check.
func isMethodCallOnReceiver(expr string) bool {
	// Parse as Go expression using go/parser
	parsedExpr, err := parser.ParseExpr(expr)
	if err != nil {
		return false
	}

	// Check if it's a call expression
	callExpr, ok := parsedExpr.(*ast.CallExpr)
	if !ok {
		return false
	}

	// Check if the function being called is a selector (x.Method)
	_, isSelector := callExpr.Fun.(*ast.SelectorExpr)
	return isSelector
}

// isMethodCallOnField checks if an expression is a method call on a FIELD of the receiver.
// Returns true for: s.repo.GetBySlug(), s.companyRepo.Update()
// Returns false for: foo(), v.validateWithCache(), obj.Method()
//
// The difference from isMethodCallOnReceiver:
// - v.Method() has a simple identifier receiver (v) → this is calling a method on self
// - s.field.Method() has a selector receiver (s.field) → this is calling a method on a field
//
// We use this to avoid matching local methods when calling methods on fields,
// since fields have different types than self.
func isMethodCallOnField(expr string) bool {
	// Parse as Go expression using go/parser
	parsedExpr, err := parser.ParseExpr(expr)
	if err != nil {
		return false
	}

	// Check if it's a call expression
	callExpr, ok := parsedExpr.(*ast.CallExpr)
	if !ok {
		return false
	}

	// Check if the function being called is a selector (x.Method)
	sel, isSelector := callExpr.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return false
	}

	// Check if the receiver (sel.X) is itself a selector (x.field)
	// This means we have x.field.Method() pattern
	_, receiverIsSelector := sel.X.(*ast.SelectorExpr)
	return receiverIsSelector
}

// extractFuncName extracts the function name from a call's Fun expression
func extractFuncName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		// Simple function call: foo()
		return e.Name
	case *ast.SelectorExpr:
		// Method call: obj.Method() or pkg.Func()
		return e.Sel.Name
	case *ast.IndexExpr:
		// Generic function: foo[T]()
		return extractFuncName(e.X)
	case *ast.IndexListExpr:
		// Generic function with multiple type params: foo[T, U]()
		return extractFuncName(e.X)
	default:
		return ""
	}
}

// findMethodReturnCount searches the source for a function/method with the given name
// and returns its return count, or 0 if not found.
//
// NOTE: This function extracts function signatures using go/scanner to handle
// Dingo-specific syntax (match expressions, ?, etc.) that go/parser cannot handle.
// Function signatures are valid Go syntax, so we extract them and parse those.
func findMethodReturnCount(src []byte, methodName string) int {
	// Extract function signatures (valid Go) from Dingo source
	signatures := extractFunctionSignatures(src)
	if signatures == "" {
		return 0
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", signatures, parser.ParseComments)
	if err != nil {
		return 0
	}

	var returnCount int
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}
		if fn.Name.Name == methodName {
			if fn.Type.Results != nil {
				returnCount = fn.Type.Results.NumFields()
			}
			return false
		}
		return true
	})

	return returnCount
}

// IsResultType checks if a type string represents a Result type (dgo.Result or Result)
func IsResultType(typeStr string) bool {
	// Check for dgo.Result[T, E] or Result[T, E]
	for i := 0; i < len(typeStr); i++ {
		if typeStr[i] == '.' {
			// Has package prefix, check for dgo.Result
			remaining := typeStr[i+1:]
			if len(remaining) >= 6 && remaining[:6] == "Result" {
				return true
			}
		}
	}
	// Check for unqualified Result
	if len(typeStr) >= 6 && typeStr[:6] == "Result" {
		return true
	}
	return false
}

// ExtractResultOkType extracts the T from Result[T, E] or dgo.Result[T, E]
// Returns empty string if not a Result type or extraction fails
func ExtractResultOkType(typeStr string) string {
	// Find "Result[" in the string
	resultIdx := -1
	for i := 0; i <= len(typeStr)-7; i++ {
		if typeStr[i:i+7] == "Result[" {
			resultIdx = i + 7
			break
		}
	}
	if resultIdx == -1 {
		return ""
	}

	// Extract until first comma or ] at top level
	// Track both brackets [] and parentheses () for tuple types like (string, string)
	bracketDepth := 1
	parenDepth := 0
	start := resultIdx
	for i := resultIdx; i < len(typeStr); i++ {
		switch typeStr[i] {
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
			if bracketDepth == 0 {
				return typeStr[start:i]
			}
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case ',':
			// Only match comma at top level (bracket depth 1, not inside parens)
			if bracketDepth == 1 && parenDepth == 0 {
				return typeStr[start:i]
			}
		}
	}
	return ""
}

// zeroValueFor returns the zero value for a given type name.
// NOTE: This operates on type name strings from go/ast (parsed data),
// not source code bytes. Simple prefix checks are used instead of strings package.
func zeroValueFor(typeName string) string {
	switch typeName {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "byte", "rune":
		return "0"
	case "bool":
		return "false"
	case "string":
		return `""`
	case "error":
		return "nil"
	case "interface{}", "any":
		return "nil"
	default:
		// Pointer, slice, map, interface, chan, func
		// Use character checks instead of strings.HasPrefix
		if len(typeName) > 0 && typeName[0] == '*' {
			return "nil" // Pointer
		}
		if len(typeName) >= 2 && typeName[:2] == "[]" {
			return "nil" // Slice
		}
		if len(typeName) >= 4 && typeName[:4] == "map[" {
			return "nil" // Map
		}
		if len(typeName) >= 4 && typeName[:4] == "chan" {
			return "nil" // Channel
		}
		if len(typeName) >= 4 && typeName[:4] == "func" {
			return "nil" // Function
		}
		// Named types from standard library that are nil-able
		// Check for "." in type name (e.g., "http.Client")
		for i := 0; i < len(typeName); i++ {
			if typeName[i] == '.' {
				return "nil" // package.Type - assume nil
			}
		}
		// Struct or named type - use zero value literal
		return typeName + "{}"
	}
}
