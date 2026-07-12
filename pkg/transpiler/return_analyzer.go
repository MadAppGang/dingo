// Package transpiler provides return type analysis for implicit Result/Option wrapping.
// It uses go/types to accurately determine what type a function returns and what
// type a return expression has, enabling automatic wrapping with Ok/Err/Some/None.
package transpiler

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/MadAppGang/dingo/pkg/typechecker"
)

// ReturnAnalyzer analyzes function return types and return expressions
// to determine if implicit wrapping is needed for Result/Option types.
type ReturnAnalyzer struct {
	checker       *typechecker.Checker
	methodReturns *MethodReturnTypes // AST-based fallback for method return types
}

// NewReturnAnalyzer creates a new return analyzer with type checking support.
func NewReturnAnalyzer(checker *typechecker.Checker) *ReturnAnalyzer {
	return &ReturnAnalyzer{checker: checker}
}

// ReturnTypeInfo contains information about a function's return type.
type ReturnTypeInfo struct {
	// Kind is either "result", "option", or "other"
	Kind string

	// For Result[T, E]:
	TType types.Type // The success type T
	EType types.Type // The error type E

	// For Option[T]:
	// TType is the wrapped type T
	// EType is nil

	// AST expressions for cloning
	TAstExpr ast.Expr // AST expression for T
	EAstExpr ast.Expr // AST expression for E (nil for Option)
}

// isResultOrOptionType checks if an AST expression represents Result or Option type.
// Handles both unqualified (Result, Option) and qualified (dgo.Result, pkg.Option) forms.
func isResultOrOptionType(expr ast.Expr) (bool, string) {
	switch e := expr.(type) {
	case *ast.Ident:
		// Unqualified: Result, Option
		if e.Name == "Result" || e.Name == "Option" {
			return true, e.Name
		}
	case *ast.SelectorExpr:
		// Qualified: dgo.Result, pkg.Option
		if e.Sel.Name == "Result" || e.Sel.Name == "Option" {
			return true, e.Sel.Name
		}
	}
	return false, ""
}

// AnalyzeReturnType analyzes a function declaration to determine if it returns
// Result[T,E] or Option[T], and extracts the type parameters.
//
// Supports both qualified (dgo.Result[T,E]) and unqualified (Result[T,E]) forms.
// Returns nil if the function doesn't return Result or Option.
func (ra *ReturnAnalyzer) AnalyzeReturnType(fn *ast.FuncDecl) *ReturnTypeInfo {
	if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
		return nil // Must have exactly one return value
	}

	resultType := fn.Type.Results.List[0].Type

	// Check for Result[T, E] (IndexListExpr - two type parameters)
	if indexList, ok := resultType.(*ast.IndexListExpr); ok {
		isROrO, typeName := isResultOrOptionType(indexList.X)
		if isROrO && typeName == "Result" && len(indexList.Indices) == 2 {
			info := &ReturnTypeInfo{
				Kind:     "result",
				TAstExpr: indexList.Indices[0],
				EAstExpr: indexList.Indices[1],
			}

			// Get go/types information if checker available
			if ra.checker != nil {
				info.TType = ra.checker.TypeOf(indexList.Indices[0])
				info.EType = ra.checker.TypeOf(indexList.Indices[1])
			}

			return info
		}
	}

	// Check for Option[T] (IndexExpr - one type parameter)
	if indexExpr, ok := resultType.(*ast.IndexExpr); ok {
		isROrO, typeName := isResultOrOptionType(indexExpr.X)
		if isROrO && typeName == "Option" {
			info := &ReturnTypeInfo{
				Kind:     "option",
				TAstExpr: indexExpr.Index,
				EAstExpr: nil,
			}

			if ra.checker != nil {
				info.TType = ra.checker.TypeOf(indexExpr.Index)
			}

			return info
		}
	}

	return nil // Not Result or Option
}

// WrapperType determines what wrapper to use for a return expression.
// For Result[T,E]: returns "Ok" if expr is type T, "Err" if expr is type E
// For Option[T]: returns "Some" if expr is non-nil, "None" if expr is nil
// Returns empty string if no wrapping needed (already wrapped).
type WrapperType string

const (
	WrapperOk   WrapperType = "Ok"
	WrapperErr  WrapperType = "Err"
	WrapperSome WrapperType = "Some"
	WrapperNone WrapperType = "None"
	WrapperSkip WrapperType = "" // Already wrapped, skip
)

// DetermineWrapper analyzes a return expression and determines what wrapper
// constructor should be used (Ok, Err, Some, None, or none if already wrapped).
//
// For Result[T,E]:
//   - Returns WrapperOk if expr type matches T
//   - Returns WrapperErr if expr type matches E
//
// For Option[T]:
//   - Returns WrapperNone if expr is nil literal
//   - Returns WrapperSome otherwise
//
// Returns WrapperSkip if expression is already wrapped with dgo.Ok/Err/Some/None.
func (ra *ReturnAnalyzer) DetermineWrapper(expr ast.Expr, returnInfo *ReturnTypeInfo) WrapperType {
	// Check if already wrapped
	if ra.isAlreadyWrapped(expr) {
		return WrapperSkip
	}

	switch returnInfo.Kind {
	case "result":
		return ra.determineResultWrapper(expr, returnInfo)
	case "option":
		return ra.determineOptionWrapper(expr, returnInfo)
	default:
		return WrapperSkip
	}
}

// determineResultWrapper determines Ok vs Err for Result[T,E] returns.
// Uses multiple strategies with fallbacks when type checker is unavailable.
func (ra *ReturnAnalyzer) determineResultWrapper(expr ast.Expr, returnInfo *ReturnTypeInfo) WrapperType {
	// Strategy 0a: Check if expression already returns Result type via type checker
	// This catches function/method calls that return Result[T, E] directly
	if ra.checker != nil {
		exprType := ra.checker.TypeOf(expr)
		if exprType != nil {
			if ra.isResultType(exprType) {
				return WrapperSkip // Already Result[T, E], no wrapping needed
			}
		}
	}

	// Strategy 0b: AST-based detection for Result-returning expressions (fallback)
	// Check for common patterns that return Result even without type checker
	if ra.isResultExpression(expr) {
		return WrapperSkip
	}

	// Strategy 0c: Recognize structurally unambiguous error expressions before
	// applying the conservative unknown-method fallback. Calls such as
	// result.MustErr() are specifically defined to return the Result error type
	// and must be wrapped even when go/types cannot resolve the local dgo package.
	if ra.isErrorExpression(expr) {
		return WrapperErr
	}

	// Strategy 0d: For other method calls with unknown return type (nil or invalid),
	// skip wrapping. This prevents wrapping external calls that may already return
	// Result; callers can wrap ambiguous values explicitly.
	if call, ok := expr.(*ast.CallExpr); ok {
		if _, isSel := call.Fun.(*ast.SelectorExpr); isSel {
			if ra.checker != nil {
				exprType := ra.checker.TypeOf(expr)
				// Skip wrapping if type is nil (unknown) or invalid (unresolved)
				if exprType == nil || isInvalidType(exprType) {
					return WrapperSkip // Unknown type - don't wrap, let user handle it
				}
			}
		}
	}

	// Strategy 1: Use go/types for accurate type comparison
	if ra.checker != nil && returnInfo.EType != nil {
		exprType := ra.checker.TypeOf(expr)
		if exprType != nil {
			// Check if expression type is assignable to E (error type)
			if types.AssignableTo(exprType, returnInfo.EType) {
				return WrapperErr
			}
			// Check if expression type is assignable to T (success type)
			if returnInfo.TType != nil && types.AssignableTo(exprType, returnInfo.TType) {
				return WrapperOk
			}
		}
	}

	// Strategy 2: Heuristic - check for composite literal matching E type
	if compLit, ok := expr.(*ast.CompositeLit); ok {
		if eIdent, ok := returnInfo.EAstExpr.(*ast.Ident); ok {
			if litIdent, ok := compLit.Type.(*ast.Ident); ok {
				if litIdent.Name == eIdent.Name {
					return WrapperErr
				}
			}
		}
	}

	// Strategy 3: Check for error interface (requires type checker)
	// IMPORTANT: Skip invalid types - they may falsely pass implements check
	if ra.checker != nil {
		exprType := ra.checker.TypeOf(expr)
		if exprType != nil && !isInvalidType(exprType) && ra.implementsError(exprType) {
			return WrapperErr
		}
	}

	// Default: assume success type (Ok)
	return WrapperOk
}

// determineOptionWrapper determines Some vs None for Option[T] returns.
func (ra *ReturnAnalyzer) determineOptionWrapper(expr ast.Expr, returnInfo *ReturnTypeInfo) WrapperType {
	// Check for nil literal
	if ident, ok := expr.(*ast.Ident); ok {
		if ident.Name == "nil" {
			return WrapperNone
		}
	}

	// Check if expression is already an Option[T] type (e.g., returning a variable of type Option[T])
	// Strategy 1: Use type checker if available
	if ra.checker != nil {
		exprType := ra.checker.TypeOf(expr)
		if exprType != nil {
			// Check if the expression type is Option (named type check)
			if named, ok := exprType.(*types.Named); ok {
				typeName := named.Obj().Name()
				if typeName == "Option" {
					return WrapperSkip // Already Option[T], no wrapping needed
				}
			}
		}
	}

	// Strategy 2: AST-based check for known Option[T] variable names
	// Check if expr is an identifier that was declared with Option[T] type
	if ident, ok := expr.(*ast.Ident); ok {
		// This is a simple heuristic - if the variable was declared as Option[T],
		// the type checker would catch it above. Here we do a simpler check.
		if ident.Obj != nil {
			if decl, ok := ident.Obj.Decl.(*ast.ValueSpec); ok {
				if decl.Type != nil {
					// Check if the declared type is Option[T]
					if isOptionType(decl.Type) {
						return WrapperSkip
					}
				}
			}
		}
	}

	// All non-nil, non-Option values wrapped with Some
	return WrapperSome
}

// isOptionType checks if a type expression is Option[T] or dgo.Option[T]
func isOptionType(expr ast.Expr) bool {
	indexExpr, ok := expr.(*ast.IndexExpr)
	if !ok {
		return false
	}
	switch x := indexExpr.X.(type) {
	case *ast.Ident:
		return x.Name == "Option"
	case *ast.SelectorExpr:
		if ident, ok := x.X.(*ast.Ident); ok {
			return ident.Name == "dgo" && x.Sel.Name == "Option"
		}
	}
	return false
}

// isResultType checks if a go/types Type is a Result[T, E] type.
// Works with both dgo.Result and locally aliased Result types.
func (ra *ReturnAnalyzer) isResultType(t types.Type) bool {
	if t == nil {
		return false
	}

	// Check for named type (Result is a named generic type)
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}

	// Check the type name
	typeName := named.Obj().Name()
	return typeName == "Result"
}

// isResultExpression checks if an expression returns a Result type using AST patterns.
// This is a fallback when type checker is unavailable.
//
// Detects:
//   - dgo.Ok[T,E](...), dgo.Err[T,E](...) calls
//   - Method/function calls that return Result (based on AST scan of declarations)
//   - Method calls with common Result-returning patterns (Map, AndThen, etc.)
func (ra *ReturnAnalyzer) isResultExpression(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}

	// Check for dgo.Ok[T,E](...) or dgo.Err[T,E](...) constructors
	// These are handled by isAlreadyWrapped, but include here for completeness
	if ra.isAlreadyWrapped(expr) {
		return true
	}

	// Check for method/function calls
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		methodName := sel.Sel.Name

		// Strategy 1: Check AST-based method return type map
		// This catches methods declared in the same file that return Result
		if ra.methodReturns != nil && ra.methodReturns.ReturnsResult(methodName) {
			return true
		}

		// Strategy 2: Check for common Result-returning method patterns
		// Pattern: receiver.MethodName() where MethodName suggests Result return
		resultPatterns := []string{
			// These methods transform Results but return Result
			"Map", "MapErr", "AndThen", "OrElse",
			// These construct Results
			"ToResult", "AsResult", "IntoResult",
		}
		for _, pattern := range resultPatterns {
			if methodName == pattern {
				return true
			}
		}
	}

	// Check for bare function calls (not method calls)
	if ident, ok := call.Fun.(*ast.Ident); ok {
		// Check AST-based function return type map
		if ra.methodReturns != nil && ra.methodReturns.ReturnsResult(ident.Name) {
			return true
		}
	}

	return false
}

// MethodReturnTypes maps method names to whether they return Result type.
// This is built from AST scanning for fallback when type checker is unavailable.
type MethodReturnTypes struct {
	// methodReturnsResult maps method name to true if it returns Result
	methodReturnsResult map[string]bool
}

// BuildMethodReturnTypes scans an AST file to build a map of method → returnsResult.
// This enables AST-based detection of Result-returning methods.
func BuildMethodReturnTypes(file *ast.File) *MethodReturnTypes {
	m := &MethodReturnTypes{
		methodReturnsResult: make(map[string]bool),
	}

	ast.Inspect(file, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		// Check if this function/method returns Result
		if funcDecl.Type.Results != nil && len(funcDecl.Type.Results.List) == 1 {
			returnType := funcDecl.Type.Results.List[0].Type
			if isResultTypeExpr(returnType) {
				m.methodReturnsResult[funcDecl.Name.Name] = true
			}
		}

		return true
	})

	return m
}

// ReturnsResult checks if a method call returns a Result type based on AST analysis.
func (m *MethodReturnTypes) ReturnsResult(methodName string) bool {
	if m == nil || m.methodReturnsResult == nil {
		return false
	}
	return m.methodReturnsResult[methodName]
}

// isResultTypeExpr checks if an AST type expression is Result[T,E] or dgo.Result[T,E]
func isResultTypeExpr(expr ast.Expr) bool {
	// Check for IndexListExpr (two type params: Result[T, E])
	if indexList, ok := expr.(*ast.IndexListExpr); ok {
		switch x := indexList.X.(type) {
		case *ast.Ident:
			return x.Name == "Result"
		case *ast.SelectorExpr:
			if x.Sel.Name == "Result" {
				return true
			}
		}
	}
	return false
}

// isAlreadyWrapped checks if an expression is already a dgo.Ok/Err/Some/None call.
func (ra *ReturnAnalyzer) isAlreadyWrapped(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}

	// Check for dgo.Ok, dgo.Err, dgo.Some, dgo.None (non-generic form)
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "dgo" {
			name := sel.Sel.Name
			return name == "Ok" || name == "Err" || name == "Some" || name == "None"
		}
	}

	// Check for generic forms: dgo.Ok[T,E], dgo.Err[T,E], dgo.Some[T], dgo.None[T]
	var selectorExpr *ast.SelectorExpr

	// Handle IndexListExpr (two type params: Result)
	if indexList, ok := call.Fun.(*ast.IndexListExpr); ok {
		selectorExpr, _ = indexList.X.(*ast.SelectorExpr)
	}

	// Handle IndexExpr (one type param: Option)
	if indexExpr, ok := call.Fun.(*ast.IndexExpr); ok {
		selectorExpr, _ = indexExpr.X.(*ast.SelectorExpr)
	}

	if selectorExpr != nil {
		if ident, ok := selectorExpr.X.(*ast.Ident); ok && ident.Name == "dgo" {
			name := selectorExpr.Sel.Name
			return name == "Ok" || name == "Err" || name == "Some" || name == "None"
		}
	}

	return false
}

// isErrorExpression detects error expressions using AST patterns (no type checker needed).
// This provides fallback error detection when type inference is disabled.
//
// Detects:
//   - Error variable names: err, error, userErr, etc.
//   - Error constructor calls: fmt.Errorf, errors.New, errors.Wrap, etc.
//   - Result.MustErr() calls: result.MustErr(), r.MustErr()
//   - Address-of error expressions: &myError
func (ra *ReturnAnalyzer) isErrorExpression(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		// Check for error-like variable names
		name := e.Name
		if name == "err" || name == "error" {
			return true
		}
		// Check for variables with "err" or "error" in the name
		// (e.g., userErr, validationError, dbError)
		lower := name
		if len(lower) > 0 {
			firstLower := lower[0] >= 'a' && lower[0] <= 'z'
			if firstLower && (containsSubstring(lower, "err") || containsSubstring(lower, "error")) {
				return true
			}
		}
	case *ast.CallExpr:
		// Check for error constructor functions and Result.MustErr() calls
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			funcName := sel.Sel.Name
			// Common error constructors: fmt.Errorf, errors.New, errors.Wrap, etc.
			switch funcName {
			case "Errorf", "New", "Wrap", "Wrapf", "WithMessage", "WithStack":
				return true
			// Result.MustErr() returns the error type
			case "MustErr", "UnwrapErr":
				return true
			}
		}
	case *ast.UnaryExpr:
		// Address-of error expression: &myError
		if e.Op == token.AND {
			return ra.isErrorExpression(e.X)
		}
	}
	return false
}

// containsSubstring checks if s contains substr (case-insensitive for ASCII).
func containsSubstring(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			c1, c2 := s[i+j], substr[j]
			// Simple ASCII lowercase
			if c1 >= 'A' && c1 <= 'Z' {
				c1 = c1 + ('a' - 'A')
			}
			if c2 >= 'A' && c2 <= 'Z' {
				c2 = c2 + ('a' - 'A')
			}
			if c1 != c2 {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// isInvalidType checks if a type is an "invalid type" from go/types.
// Invalid types occur when the type checker can't resolve a type (e.g., external package).
// These types can falsely pass interface implementation checks.
func isInvalidType(t types.Type) bool {
	if t == nil {
		return true
	}

	// Check the string representation for "invalid type"
	typeStr := types.TypeString(t, nil)
	if typeStr == "invalid type" {
		return true
	}

	// Also check pointer/slice/map to invalid types
	switch u := t.(type) {
	case *types.Pointer:
		return isInvalidType(u.Elem())
	case *types.Slice:
		return isInvalidType(u.Elem())
	case *types.Array:
		return isInvalidType(u.Elem())
	case *types.Map:
		return isInvalidType(u.Key()) || isInvalidType(u.Elem())
	case *types.Basic:
		return u.Kind() == types.Invalid
	}

	return false
}

// implementsError checks if a type implements the error interface.
func (ra *ReturnAnalyzer) implementsError(t types.Type) bool {
	// Get the error interface type from the universe scope
	// The error interface has a single method: Error() string
	errorType := types.Universe.Lookup("error")
	if errorType == nil {
		return false
	}

	errorInterface, ok := errorType.Type().Underlying().(*types.Interface)
	if !ok {
		return false
	}

	// Check if t implements error interface
	return types.Implements(t, errorInterface)
}

// GetTypeString returns a string representation of a type for debugging.
func (ra *ReturnAnalyzer) GetTypeString(t types.Type) string {
	if t == nil {
		return "<nil>"
	}
	return types.TypeString(t, nil)
}
