// Package ast defines Dingo-specific AST extensions on top of go/ast
//
// Strategy: Reuse 95% of go/ast infrastructure, only define custom nodes
// for Dingo-specific features that don't exist in Go.
//
// Benefits:
// - Leverage go/printer for code generation
// - Use go/token.FileSet for position tracking
// - Reuse go/ast.Walk, go/ast.Inspect
// - Familiar API for Go developers
package ast

import (
	"go/ast"
	"go/token"
)

// ============================================================================
// Dingo-Specific Expression Nodes
// ============================================================================
// These extend go/ast with new expression types for Dingo features

// ErrorPropagationExpr represents the `?` operator (expr?)
// Example: let user = fetchUser(id)?
//
// Implements ast.Expr interface so it can be used anywhere a Go expression can
type ErrorPropagationExpr struct {
	X    ast.Expr  // The expression being propagated (e.g., fetchUser(id))
	QPos token.Pos // Position of the '?' token
}

func (e *ErrorPropagationExpr) Pos() token.Pos { return e.X.Pos() }
func (e *ErrorPropagationExpr) End() token.Pos { return e.QPos + 1 }

// NullCoalescingExpr represents the `??` operator (a ?? b)
// Example: let name = user.name ?? "Unknown"
//
// Implements ast.Expr interface
type NullCoalescingExpr struct {
	X      ast.Expr  // Left operand (nullable value)
	OpPos  token.Pos // Position of '??'
	Y      ast.Expr  // Right operand (default value)
}

func (n *NullCoalescingExpr) Pos() token.Pos { return n.X.Pos() }
func (n *NullCoalescingExpr) End() token.Pos { return n.Y.End() }

// TernaryExpr represents the ternary operator (cond ? then : else)
// Example: let status = age >= 18 ? "adult" : "minor"
//
// Implements ast.Expr interface
type TernaryExpr struct {
	Cond     ast.Expr  // Condition expression
	Question token.Pos // Position of '?'
	Then     ast.Expr  // Expression if true
	Colon    token.Pos // Position of ':'
	Else     ast.Expr  // Expression if false
}

func (t *TernaryExpr) Pos() token.Pos { return t.Cond.Pos() }
func (t *TernaryExpr) End() token.Pos { return t.Else.End() }

// LambdaExpr represents lambda/arrow functions
// Examples:
//   - Rust style: |a, b| a + b
//   - Arrow style: (a, b) => a + b
//   - Kotlin style: { it.age > 18 }
//
// Implements ast.Expr interface
type LambdaExpr struct {
	Pipe   token.Pos       // Position of '|' or '(' or '{'
	Params *ast.FieldList  // Parameters (reuse go/ast.FieldList!)
	Arrow  token.Pos       // Position of '=>' or '->' (if present)
	Body   ast.Expr        // Body (can be ast.BlockStmt or any expression)
	Rpipe  token.Pos       // Position of closing '|' or ')' or '}'
}

func (l *LambdaExpr) Pos() token.Pos { return l.Pipe }
func (l *LambdaExpr) End() token.Pos {
	if l.Rpipe.IsValid() {
		return l.Rpipe + 1
	}
	return l.Body.End()
}

// ============================================================================
// Dingo-Specific Type Nodes
// ============================================================================

// ResultType represents Result<T, E> type
// Example: func fetchUser(id: string) -> Result<User, Error>
//
// Note: This is only used during parsing. The transformer will convert
// Result<T, E> to the appropriate Go struct type.
type ResultType struct {
	Result   token.Pos // Position of 'Result' keyword
	Lbrack   token.Pos // Position of '<'
	Value    ast.Expr  // Value type (T)
	Comma    token.Pos // Position of ','
	Error    ast.Expr  // Error type (E)
	Rbrack   token.Pos // Position of '>'
}

func (r *ResultType) Pos() token.Pos { return r.Result }
func (r *ResultType) End() token.Pos { return r.Rbrack + 1 }

// OptionType represents Option<T> type
// Example: func findUser(id: string) -> Option<User>
//
// Note: This is only used during parsing. The transformer will convert
// Option<T> to the appropriate Go pointer or struct type.
type OptionType struct {
	Option token.Pos // Position of 'Option' keyword
	Lbrack token.Pos // Position of '<'
	Value  ast.Expr  // Value type (T)
	Rbrack token.Pos // Position of '>'
}

func (o *OptionType) Pos() token.Pos { return o.Option }
func (o *OptionType) End() token.Pos { return o.Rbrack + 1 }

// ============================================================================
// Helper Functions
// ============================================================================

// IsDingoNode reports whether a node is a Dingo-specific extension
func IsDingoNode(node ast.Node) bool {
	switch node.(type) {
	case *ErrorPropagationExpr, *NullCoalescingExpr, *TernaryExpr, *LambdaExpr:
		return true
	case *ResultType, *OptionType:
		return true
	default:
		return false
	}
}

// IsDingoType reports whether a type node is Dingo-specific
func IsDingoType(node ast.Node) bool {
	switch node.(type) {
	case *ResultType, *OptionType:
		return true
	default:
		return false
	}
}

// ============================================================================
// Extended Walk for Dingo Nodes
// ============================================================================

// Walk extends go/ast.Walk to handle Dingo-specific nodes
// It delegates to go/ast.Inspect for standard nodes and handles custom nodes
func Walk(node ast.Node, f func(ast.Node) bool) {
	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil {
			return false
		}

		// Call visitor function
		if !f(n) {
			return false
		}

		// Handle Dingo-specific nodes
		switch x := n.(type) {
		case *ErrorPropagationExpr:
			ast.Inspect(x.X, f)
			return false

		case *NullCoalescingExpr:
			ast.Inspect(x.X, f)
			ast.Inspect(x.Y, f)
			return false

		case *TernaryExpr:
			ast.Inspect(x.Cond, f)
			ast.Inspect(x.Then, f)
			ast.Inspect(x.Else, f)
			return false

		case *LambdaExpr:
			if x.Params != nil {
				ast.Inspect(x.Params, f)
			}
			ast.Inspect(x.Body, f)
			return false

		case *ResultType:
			ast.Inspect(x.Value, f)
			ast.Inspect(x.Error, f)
			return false

		case *OptionType:
			ast.Inspect(x.Value, f)
			return false
		}

		// For standard go/ast nodes, continue normal traversal
		return true
	})
}

// ============================================================================
// Notes on Design
// ============================================================================

// Why this hybrid approach works:
//
// 1. Parser Phase:
//    - Dingo source -> Dingo AST (mix of go/ast + custom nodes)
//    - Example: fetchUser(id)? becomes ErrorPropagationExpr{X: CallExpr{...}}
//
// 2. Transform Phase:
//    - Dingo AST -> Pure go/ast
//    - Example: ErrorPropagationExpr -> IfStmt checking for errors
//
// 3. Generation Phase:
//    - Pure go/ast -> Go source code
//    - Use standard go/printer.Fprint()
//
// This means:
// - We get all go/ast tooling for free
// - Custom nodes are isolated and easy to transform
// - Generated Go code uses 100% standard go/ast (can use go/printer directly)
// - Source maps work naturally (both Dingo and Go use token.Pos)
