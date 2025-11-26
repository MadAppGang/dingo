// Package transform implements AST transformations for Dingo features
package transform

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ast/astutil"

	"github.com/MadAppGang/dingo/pkg/preprocessor"
)

const doubleUnderscore = "__"

// Transformer walks the AST and replaces Dingo placeholder patterns
// with final Go implementations
type Transformer struct {
	fset      *token.FileSet
	sourceMap *preprocessor.SourceMap
	typeInfo  *types.Info
}

// New creates a new transformer
func New(fset *token.FileSet, sourceMap *preprocessor.SourceMap) *Transformer {
	return &Transformer{
		fset:      fset,
		sourceMap: sourceMap,
		typeInfo: &types.Info{
			Types: make(map[ast.Expr]types.TypeAndValue),
			Defs:  make(map[*ast.Ident]types.Object),
			Uses:  make(map[*ast.Ident]types.Object),
		},
	}
}

// Transform applies all transformations to the AST
func (t *Transformer) Transform(file *ast.File) (*ast.File, error) {
	// Step 1: Type check to populate type info
	// (Skipped for now - will add when needed for lambdas)

	// Step 2: Walk and transform
	result := astutil.Apply(file, t.visit, nil)

	if result == nil {
		return file, nil
	}

	// CRITICAL-4 FIX: Safe type assertion with error handling
	if f, ok := result.(*ast.File); ok {
		return f, nil
	}
	return nil, fmt.Errorf("unexpected return type from astutil.Apply: got %T, expected *ast.File", result)
}

// visit is called for each node during AST traversal
func (t *Transformer) visit(cursor *astutil.Cursor) bool {
	node := cursor.Node()
	if node == nil {
		return true
	}

	switch n := node.(type) {
	case *ast.CallExpr:
		// Check for Dingo placeholder function calls
		if ident, ok := n.Fun.(*ast.Ident); ok {
			return t.handlePlaceholderCall(cursor, ident, n)
		}

	case *ast.GenDecl:
		// Check for enum type definitions
		return t.handleGenDecl(cursor, n)
	}

	return true // Continue traversal
}

// handlePlaceholderCall processes calls to Dingo placeholder functions
// IMPORTANT-4 FIX: Added validation to prevent false positives
func (t *Transformer) handlePlaceholderCall(cursor *astutil.Cursor, ident *ast.Ident, call *ast.CallExpr) bool {
	name := ident.Name

	switch {
	// Error propagation is fully handled in preprocessor, no transform needed

	case len(name) >= 15 && name[:15] == "__dingo_lambda_":
		// Lambda: __dingo_lambda_N__(...)
		// IMPORTANT-4 FIX: Validate that this is actually a lambda placeholder
		if !isValidLambdaPlaceholder(call) {
			return true // Not a valid placeholder, skip transformation
		}
		return t.transformLambda(cursor, call)

	case len(name) >= 14 && name[:14] == "__dingo_match_":
		// Pattern match: __dingo_match_N__(...)
		// IMPORTANT-4 FIX: Validate that this is actually a match placeholder
		if !isValidMatchPlaceholder(call) {
			return true // Not a valid placeholder, skip transformation
		}
		return t.transformMatch(cursor, call)

	case len(name) >= 17 && name[:17] == "__dingo_safe_nav_":
		// Safe navigation: __dingo_safe_nav_N__(...)
		// IMPORTANT-4 FIX: Validate that this is actually a safe nav placeholder
		if !isValidSafeNavPlaceholder(call) {
			return true // Not a valid placeholder, skip transformation
		}
		return t.transformSafeNav(cursor, call)
	}

	return true
}

// handleGenDecl processes general declarations (enum types)
func (t *Transformer) handleGenDecl(cursor *astutil.Cursor, decl *ast.GenDecl) bool {
	// Check if this is an enum definition
	// (Will implement when we add sum types)
	return true
}

// NOTE: Error propagation (? operator) is fully handled in pkg/preprocessor/error_prop.go
// This transformer focuses on AST-level features: lambdas, pattern matching, safe navigation

// transformLambda transforms lambda placeholders
func (t *Transformer) transformLambda(cursor *astutil.Cursor, call *ast.CallExpr) bool {
	// TODO: Implement lambda transformation
	//
	// CRITICAL-5: When implementing, you MUST call cursor.Replace(transformedNode)
	// to replace the placeholder node with the actual transformation.
	// Without calling Replace(), the transformation will be a no-op.
	//
	// Example implementation:
	//   transformedNode := &ast.FuncLit{
	//       Type: &ast.FuncType{ /* ... */ },
	//       Body: &ast.BlockStmt{ /* ... */ },
	//   }
	//   cursor.Replace(transformedNode)
	//
	return true
}

// transformMatch transforms pattern matching placeholders
func (t *Transformer) transformMatch(cursor *astutil.Cursor, call *ast.CallExpr) bool {
	// TODO: Implement pattern matching transformation
	//
	// CRITICAL-5: When implementing, you MUST call cursor.Replace(transformedNode)
	// to replace the placeholder node with the actual transformation.
	// Without calling Replace(), the transformation will be a no-op.
	//
	// Example implementation:
	//   transformedNode := &ast.TypeSwitchStmt{
	//       /* ... pattern match to type switch conversion ... */
	//   }
	//   cursor.Replace(transformedNode)
	//
	return true
}

// transformSafeNav transforms safe navigation placeholders
func (t *Transformer) transformSafeNav(cursor *astutil.Cursor, call *ast.CallExpr) bool {
	// TODO: Implement safe navigation transformation
	//
	// CRITICAL-5: When implementing, you MUST call cursor.Replace(transformedNode)
	// to replace the placeholder node with the actual transformation.
	// Without calling Replace(), the transformation will be a no-op.
	//
	// Example implementation:
	//   transformedNode := &ast.IfStmt{
	//       /* ... safe nav to nil check conversion ... */
	//   }
	//   cursor.Replace(transformedNode)
	//
	return true
}

// analyzeContext determines the context where an expression appears
// (assignment, return statement, standalone, etc.)
type ExprContext int

const (
	ContextUnknown ExprContext = iota
	ContextAssignment
	ContextReturn
	ContextStandalone
	ContextCondition
)

// getExprContext analyzes the context of an expression
// TransformError wraps transformation errors with context
type TransformError struct {
	Node ast.Node
	Msg  string
	Err  error
}

func (e *TransformError) Error() string {
	return fmt.Sprintf("transform error at %v: %s: %v", e.Node, e.Msg, e.Err)
}

func (e *TransformError) Unwrap() error {
	return e.Err
}

// IMPORTANT-4 FIX: Placeholder validation functions
// These functions validate placeholder structure to prevent false positives
// from user-defined functions that happen to start with reserved prefixes

// isValidLambdaPlaceholder validates that a call expression is actually a lambda placeholder
// Expected structure: __dingo_lambda_N__(closure_params...)
func isValidLambdaPlaceholder(call *ast.CallExpr) bool {
	// Lambda placeholders must have at least one argument (the lambda body or closure)
	// When we implement lambdas, we'll have specific argument patterns
	// For now, just verify it has the function call structure
	if call.Fun == nil {
		return false
	}

	// Check that the function identifier matches the pattern __dingo_lambda_<number>__
	if ident, ok := call.Fun.(*ast.Ident); ok {
		name := ident.Name
		if len(name) < 17 { // "__dingo_lambda_" + at least 1 digit + "__"
			return false
		}
		// Verify it ends with "__"
		if len(name) >= 2 && name[len(name)-2:] != doubleUnderscore {
			return false
		}
		return true
	}

	return false
}

// isValidMatchPlaceholder validates that a call expression is actually a match placeholder
// Expected structure: __dingo_match_N__(discriminant, pattern_handlers...)
func isValidMatchPlaceholder(call *ast.CallExpr) bool {
	// Match placeholders must have at least 2 arguments:
	// 1. The discriminant (value to match)
	// 2. At least one pattern handler
	if len(call.Args) < 2 {
		return false
	}

	// Check function identifier pattern
	if ident, ok := call.Fun.(*ast.Ident); ok {
		name := ident.Name
		if len(name) < 16 { // "__dingo_match_" + at least 1 digit + "__"
			return false
		}
		// Verify it ends with "__"
		if len(name) >= 2 && name[len(name)-2:] != doubleUnderscore {
			return false
		}
		return true
	}

	return false
}

// isValidSafeNavPlaceholder validates that a call expression is actually a safe nav placeholder
// Expected structure: __dingo_safe_nav_N__(receiver, method_chain...)
func isValidSafeNavPlaceholder(call *ast.CallExpr) bool {
	// Safe nav placeholders must have at least 1 argument (the receiver)
	if len(call.Args) < 1 {
		return false
	}

	// Check function identifier pattern
	if ident, ok := call.Fun.(*ast.Ident); ok {
		name := ident.Name
		if len(name) < 19 { // "__dingo_safe_nav_" + at least 1 digit + "__"
			return false
		}
		// Verify it ends with "__"
		if len(name) >= 2 && name[len(name)-2:] != doubleUnderscore {
			return false
		}
		return true
	}

	return false
}
