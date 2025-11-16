// Package ast defines a File wrapper that can contain Dingo-specific nodes
package ast

import (
	"go/ast"
)

// File represents a Dingo source file
// It wraps go/ast.File but stores Dingo nodes separately
type File struct {
	*ast.File                          // Embed standard Go AST
	DingoNodes map[ast.Node]DingoNode  // Maps go/ast positions to Dingo nodes
}

// DingoNode is a marker interface for Dingo-specific AST nodes
type DingoNode interface {
	ast.Node
	IsDingoNode() bool
}

// Implement DingoNode for all our custom types
func (e *ErrorPropagationExpr) IsDingoNode() bool  { return true }
func (n *NullCoalescingExpr) IsDingoNode() bool    { return true }
func (t *TernaryExpr) IsDingoNode() bool           { return true }
func (l *LambdaExpr) IsDingoNode() bool            { return true }
func (r *ResultType) IsDingoNode() bool            { return true }
func (o *OptionType) IsDingoNode() bool            { return true }
func (e *EnumDecl) IsDingoNode() bool              { return true }
func (m *MatchExpr) IsDingoNode() bool             { return true }

// NewFile creates a new Dingo file
func NewFile(goFile *ast.File) *File {
	return &File{
		File:       goFile,
		DingoNodes: make(map[ast.Node]DingoNode),
	}
}

// AddDingoNode associates a Dingo node with a go/ast placeholder
func (f *File) AddDingoNode(placeholder ast.Node, dingoNode DingoNode) {
	f.DingoNodes[placeholder] = dingoNode
}

// GetDingoNode retrieves the Dingo node for a given go/ast node
func (f *File) GetDingoNode(node ast.Node) (DingoNode, bool) {
	dn, ok := f.DingoNodes[node]
	return dn, ok
}

// HasDingoNodes returns true if this file contains any Dingo-specific nodes
func (f *File) HasDingoNodes() bool {
	return len(f.DingoNodes) > 0
}

// RemoveDingoNode removes a Dingo node from the map (cleanup after transformation)
func (f *File) RemoveDingoNode(node ast.Node) {
	delete(f.DingoNodes, node)
}
