// Package generator generates Go source code from AST
package generator

import (
	"bytes"
	"go/format"
	"go/printer"
	"go/token"

	dingoast "github.com/yourusername/dingo/pkg/ast"
)

// Generator generates Go source code from a Dingo AST
type Generator struct {
	fset *token.FileSet
}

// New creates a new generator
func New(fset *token.FileSet) *Generator {
	return &Generator{fset: fset}
}

// Generate converts a Dingo AST to Go source code
func (g *Generator) Generate(file *dingoast.File) ([]byte, error) {
	// For now, we just print the go/ast.File directly
	// Later, we'll add transformations for Dingo-specific nodes

	var buf bytes.Buffer

	// Use go/printer to generate source code from AST
	cfg := printer.Config{
		Mode:     printer.TabIndent | printer.UseSpaces,
		Tabwidth: 8,
	}

	if err := cfg.Fprint(&buf, g.fset, file.File); err != nil {
		return nil, err
	}

	// Format the generated code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// If formatting fails, return unformatted code
		// This helps with debugging malformed output
		return buf.Bytes(), nil
	}

	return formatted, nil
}
