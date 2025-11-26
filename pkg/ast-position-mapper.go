// Package astpositionmapper provides position tracking for AST transformations
package astpositionmapper

import (
	"fmt"
	"go/ast"
	"go/token"
)

// ASTPositionMapper tracks go/ast node positions before and after transformations,
// enabling accurate source map generation. It maintains a mapping between original
// and transformed positions across multiple transformation phases.
type ASTPositionMapper struct {
	// originalPositions stores positions before any transformations
	originalPositions map[string]token.Position

	// transformedPositions stores positions after all transformations
	transformedPositions map[string]token.Position

	// transformations records the sequence of transformations applied
	transformations []TransformationStep
}

// TransformationStep records a single transformation step in the pipeline
type TransformationStep struct {
	// transformName identifies the transformation (e.g., "error_prop", "lambda", "patterns")
	transformName string

	// nodeHashes maps node content hashes to their positions before this step
	beforePositions map[string]token.Position

	// nodeHashes maps node content hashes to their positions after this step
	afterPositions map[string]token.Position
}

// NewASTPositionMapper creates a new position mapper
func NewASTPositionMapper() *ASTPositionMapper {
	return &ASTPositionMapper{
		originalPositions:   make(map[string]token.Position),
		transformedPositions: make(map[string]token.Position),
		transformations:     make([]TransformationStep, 0),
	}
}

// RecordOriginalPositions captures positions of all AST nodes before transformations begin.
// This should be called once at the start of the transformation pipeline.
func (m *ASTPositionMapper) RecordOriginalPositions(fset *token.FileSet, root ast.Node) {
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			return true
		}

		pos := fset.Position(n.Pos())
		hash := nodeHash(n)
		m.originalPositions[hash] = pos

		return true
	})
}

// BeforeTransform records positions before a specific transformation step.
// This should be called at the beginning of each preprocessor transformation.
func (m *ASTPositionMapper) BeforeTransform(transformName string, fset *token.FileSet, root ast.Node) {
	step := TransformationStep{
		transformName:   transformName,
		beforePositions: make(map[string]token.Position),
		afterPositions:  make(map[string]token.Position),
	}

	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			return true
		}

		pos := fset.Position(n.Pos())
		hash := nodeHash(n)
		step.beforePositions[hash] = pos

		return true
	})

	m.transformations = append(m.transformations, step)
}

// AfterTransform records positions after a specific transformation step completes.
// This should be called at the end of each preprocessor transformation.
func (m *ASTPositionMapper) AfterTransform(fset *token.FileSet, root ast.Node) {
	if len(m.transformations) == 0 {
		return // No active transformation step
	}

	step := &m.transformations[len(m.transformations)-1]

	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			return true
		}

		pos := fset.Position(n.Pos())
		hash := nodeHash(n)
		step.afterPositions[hash] = pos

		return true
	})
}

// RecordFinalPositions captures the final positions after all transformations complete.
// This should be called once at the end of the transformation pipeline.
func (m *ASTPositionMapper) RecordFinalPositions(fset *token.FileSet, root ast.Node) {
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			return true
		}

		pos := fset.Position(n.Pos())
		hash := nodeHash(n)
		m.transformedPositions[hash] = pos

		return true
	})
}

// GenerateSourceMappings creates source map mappings from original to final positions.
// This method generates a list of mappings suitable for source map generation.
func (m *ASTPositionMapper) GenerateSourceMappings() []SourceMapping {
	mappings := make([]SourceMapping, 0)

	for hash, originalPos := range m.originalPositions {
		finalPos, exists := m.transformedPositions[hash]
		if !exists {
			continue // Node was removed during transformations
		}

		mapping := SourceMapping{
			OriginalLine:   originalPos.Line,
			OriginalColumn: originalPos.Column,
			FinalLine:      finalPos.Line,
			FinalColumn:    finalPos.Column,
			NodeHash:      hash,
		}

		mappings = append(mappings, mapping)
	}

	return mappings
}

// SourceMapping represents a single position mapping for source map generation
type SourceMapping struct {
	// Original position in the .dingo source file
	OriginalLine   int
	OriginalColumn int

	// Final position in the generated .go file
	FinalLine   int
	FinalColumn int

	// Node hash for tracking purposes
	NodeHash string
}

// GetTransformationHistory returns the sequence of transformation steps applied
func (m *ASTPositionMapper) GetTransformationHistory() []TransformationStep {
	return m.transformations
}

// GetOriginalPosition retrieves the original position for a given node hash
func (m *ASTPositionMapper) GetOriginalPosition(nodeHash string) (token.Position, bool) {
	pos, exists := m.originalPositions[nodeHash]
	return pos, exists
}

// GetFinalPosition retrieves the final transformed position for a given node hash
func (m *ASTPositionMapper) GetFinalPosition(nodeHash string) (token.Position, bool) {
	pos, exists := m.transformedPositions[nodeHash]
	return pos, exists
}

// nodeHash generates a simple content-based hash for AST nodes to track them across transformations.
// This is a basic implementation - for production use, consider more robust hashing.
func nodeHash(n ast.Node) string {
	switch node := n.(type) {
	case *ast.Ident:
		return "ident:" + node.Name
	case *ast.CallExpr:
		if ident, ok := node.Fun.(*ast.Ident); ok {
			return "call:" + ident.Name
		}
		return "call:anonymous"
	case *ast.FuncDecl:
		if node.Name != nil {
			return "func:" + node.Name.Name
		}
		return "func:anonymous"
	case *ast.GenDecl:
		return "gendecl:" + node.Tok.String()
	default:
		// For other node types, use position as a unique identifier
		// This assumes positions don't overlap, which is generally true
		return fmt.Sprintf("node:%d", n.Pos())
	}
}