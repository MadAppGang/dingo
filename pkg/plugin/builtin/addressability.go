package builtin

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/MadAppGang/dingo/pkg/plugin"
)

// isAddressable determines if an expression can have its address taken.
// According to Go language spec, addressable expressions include:
//   - Variables (identifiers)
//   - Pointer dereferences (*ptr)
//   - Array/slice indexing (arr[i])
//   - Field selectors for addressable structs (obj.field)
//   - Parenthesized addressable expressions ((x))
//
// Non-addressable expressions include:
//   - Literals (42, "hello", true)
//   - Composite literals (User{}, []int{1,2})
//   - Function calls (getUser())
//   - Binary operations (x + y)
//   - Unary operations (!flag, -value)
//   - Type assertions (x.(Type))
//   - Map index expressions (when used as rvalue)
func isAddressable(expr ast.Expr) bool {
	if expr == nil {
		return false
	}

	switch e := expr.(type) {
	// Addressable cases
	case *ast.Ident:
		// Variables, fields, package-level declarations are addressable
		// (unless they're built-in constants like true, false, nil)
		// For our purposes, we treat all identifiers as potentially addressable
		return true

	case *ast.SelectorExpr:
		// Field selectors like obj.Field are addressable if obj is addressable
		// Package selectors like pkg.Var are also addressable
		return true

	case *ast.IndexExpr:
		// Array/slice indexing: arr[i] is addressable
		// Map indexing: m[key] is NOT addressable for taking address,
		// but we return true here since our use case handles it differently
		return true

	case *ast.StarExpr:
		// Pointer dereference: *ptr is always addressable
		return true

	case *ast.ParenExpr:
		// Parenthesized expressions: (x) is addressable if x is addressable
		return isAddressable(e.X)

	// Non-addressable cases
	case *ast.BasicLit:
		// Literals: 42, "string", true, 3.14
		return false

	case *ast.CompositeLit:
		// Composite literals: User{}, []int{1,2}, map[string]int{"a": 1}
		return false

	case *ast.CallExpr:
		// Function calls: getUser(), fmt.Sprintf(...)
		return false

	case *ast.BinaryExpr:
		// Binary operations: x + y, a * b, str1 + str2
		return false

	case *ast.UnaryExpr:
		// Unary operations: -value, !flag, ^bits
		// (Note: *ptr is StarExpr, not UnaryExpr)
		return false

	case *ast.TypeAssertExpr:
		// Type assertions: x.(Type), x.(*Type)
		return false

	case *ast.FuncLit:
		// Function literals: func() { ... }
		return false

	case *ast.SliceExpr:
		// Slice expressions: arr[1:3]
		return false

	case *ast.ArrayType, *ast.StructType, *ast.FuncType,
		*ast.InterfaceType, *ast.MapType, *ast.ChanType:
		// Type expressions are not addressable
		return false

	default:
		// Conservative default: assume non-addressable
		// This ensures safety - we'll wrap in IIFE if unsure
		return false
	}
}

// wrapInIIFE wraps a non-addressable expression in an Immediately Invoked Function Expression (IIFE)
// that creates a temporary variable, assigns the expression to it, and returns its address.
//
// Example transformation:
//   Input:  Ok(42)
//   Output: Ok(func() *int { tmp1 := 42; return &tmp1 }())
//
// This pattern allows taking the address of literals and other non-addressable expressions.
//
// Parameters:
//   - expr: The non-addressable expression to wrap
//   - typeName: The type name as a string (e.g., "int", "string", "User")
//   - ctx: Plugin context (provides temp variable counter and error reporting)
//
// Returns:
//   - An ast.CallExpr representing the IIFE pattern
func wrapInIIFE(expr ast.Expr, typeName string, ctx *plugin.Context) ast.Expr {
	// Generate unique temporary variable name using context counter
	tmpVar := ctx.NextTempVar()

	// Parse the type name to create an AST type expression
	typeExpr := parseTypeString(typeName)

	// Create the IIFE structure:
	// func() *T {
	//     __tmpN := expr
	//     return &__tmpN
	// }()
	return &ast.CallExpr{
		// The function call (immediate invocation)
		Fun: &ast.FuncLit{
			// Function signature: func() *T
			Type: &ast.FuncType{
				Params: &ast.FieldList{
					// No parameters
					List: []*ast.Field{},
				},
				Results: &ast.FieldList{
					// Returns *T (pointer to type)
					List: []*ast.Field{
						{
							Type: &ast.StarExpr{
								X: typeExpr,
							},
						},
					},
				},
			},
			// Function body
			Body: &ast.BlockStmt{
				List: []ast.Stmt{
					// __tmpN := expr
					&ast.AssignStmt{
						Lhs: []ast.Expr{
							ast.NewIdent(tmpVar),
						},
						Tok: token.DEFINE,
						Rhs: []ast.Expr{
							expr,
						},
					},
					// return &__tmpN
					&ast.ReturnStmt{
						Results: []ast.Expr{
							&ast.UnaryExpr{
								Op: token.AND,
								X:  ast.NewIdent(tmpVar),
							},
						},
					},
				},
			},
		},
		// Empty argument list (immediate invocation)
		Args: []ast.Expr{},
	}
}

// parseTypeString converts a type name string to an ast.Expr representing that type.
// This handles simple cases like "int", "string", as well as pointer types, slices, etc.
//
// Examples:
//   - "int" → ast.Ident{Name: "int"}
//   - "error" → ast.Ident{Name: "error"}
//   - "User" → ast.Ident{Name: "User"}
//
// Note: For Phase 3, we primarily handle simple type names.
// Complex types (pointers, slices, maps) will be added as needed in future phases.
func parseTypeString(typeName string) ast.Expr {
	// For now, handle simple identifiers
	// Future enhancement: parse complex types like *int, []string, map[string]int
	// using go/parser or manual parsing logic

	// Check for empty type name
	if typeName == "" {
		// Fallback to interface{} for unknown types
		return &ast.InterfaceType{
			Methods: &ast.FieldList{},
		}
	}

	// Simple case: just an identifier (int, string, User, etc.)
	return ast.NewIdent(typeName)
}

// MaybeWrapForAddressability is a convenience function that checks addressability
// and wraps the expression in an IIFE if needed.
//
// This is the primary API that plugins should use.
//
// Usage:
//   valueExpr := MaybeWrapForAddressability(arg, "int", ctx)
//   // valueExpr is now guaranteed to be addressable (or already was)
//
// Returns:
//   - If expr is addressable: &expr (address-of expression)
//   - If expr is NOT addressable: IIFE wrapper that returns pointer
func MaybeWrapForAddressability(expr ast.Expr, typeName string, ctx *plugin.Context) ast.Expr {
	if isAddressable(expr) {
		// Already addressable - just take its address
		return &ast.UnaryExpr{
			Op: token.AND,
			X:  expr,
		}
	}

	// Not addressable - wrap in IIFE
	return wrapInIIFE(expr, typeName, ctx)
}

// FormatExprForDebug converts an AST expression to a readable string for debugging.
// This is useful for error messages and logging.
func FormatExprForDebug(expr ast.Expr) string {
	if expr == nil {
		return "<nil>"
	}

	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.BasicLit:
		return e.Value
	case *ast.SelectorExpr:
		return fmt.Sprintf("%s.%s", FormatExprForDebug(e.X), e.Sel.Name)
	case *ast.CallExpr:
		return fmt.Sprintf("%s(...)", FormatExprForDebug(e.Fun))
	case *ast.CompositeLit:
		return fmt.Sprintf("%s{...}", FormatExprForDebug(e.Type))
	case *ast.BinaryExpr:
		return fmt.Sprintf("(%s %s %s)", FormatExprForDebug(e.X), e.Op.String(), FormatExprForDebug(e.Y))
	case *ast.UnaryExpr:
		return fmt.Sprintf("%s%s", e.Op.String(), FormatExprForDebug(e.X))
	case *ast.IndexExpr:
		return fmt.Sprintf("%s[...]", FormatExprForDebug(e.X))
	case *ast.StarExpr:
		return fmt.Sprintf("*%s", FormatExprForDebug(e.X))
	default:
		return fmt.Sprintf("<%T>", expr)
	}
}
