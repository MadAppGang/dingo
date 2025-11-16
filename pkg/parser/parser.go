// Package parser provides the interface for parsing Dingo source code
package parser

import (
	"go/token"
	dingoast "github.com/yourusername/dingo/pkg/ast"
)

// Parser is the interface that all Dingo parsers must implement
// This allows us to swap parser implementations (participle -> tree-sitter later)
type Parser interface {
	// ParseFile parses a single Dingo source file
	// Returns a dingoast.File which wraps go/ast.File and contains Dingo-specific nodes
	ParseFile(fset *token.FileSet, filename string, src []byte) (*dingoast.File, error)

	// ParseExpr parses a single expression (useful for REPL, testing)
	ParseExpr(fset *token.FileSet, expr string) (dingoast.DingoNode, error)
}

// Mode controls parser behavior
type Mode uint

const (
	// ParseComments tells the parser to include comments in the AST
	ParseComments Mode = 1 << iota

	// Trace enables parser debugging output
	Trace

	// AllErrors reports all errors (not just the first 10)
	AllErrors
)

// ParseFile is a convenience function that uses the default parser
func ParseFile(fset *token.FileSet, filename string, src []byte, mode Mode) (*dingoast.File, error) {
	p := NewParser(mode)
	return p.ParseFile(fset, filename, src)
}

// ParseExpr is a convenience function that parses an expression
func ParseExpr(fset *token.FileSet, expr string) (dingoast.DingoNode, error) {
	p := NewParser(0)
	return p.ParseExpr(fset, expr)
}

// NewParser creates a new parser instance with the given mode
// For now, this returns a participle-based parser
// Later, we can switch to tree-sitter or other implementations
func NewParser(mode Mode) Parser {
	return newParticipleParser(mode)
}
