// Package sourcemap provides source map generation for Dingo → Go transpilation
package sourcemap

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"

	"github.com/MadAppGang/dingo/pkg/preprocessor"
)

// PostASTGenerator generates source maps AFTER go/printer using FileSet as truth
// This eliminates systematic line drift errors from prediction-based approaches
type PostASTGenerator struct {
	dingoFilePath string
	goFilePath    string
	fset          *token.FileSet // From go/parser (single source of truth)
	goAST         *ast.File      // From go/parser
	metadata      []preprocessor.TransformMetadata
}

// NewPostASTGenerator creates a generator from transpilation output
// This should be called AFTER go/printer has written the final .go file
func NewPostASTGenerator(
	dingoPath, goPath string,
	fset *token.FileSet,
	goAST *ast.File,
	metadata []preprocessor.TransformMetadata,
) *PostASTGenerator {
	return &PostASTGenerator{
		dingoFilePath: dingoPath,
		goFilePath:    goPath,
		fset:          fset,
		goAST:         goAST,
		metadata:      metadata,
	}
}

// Generate creates source map from ACTUAL AST positions (ground truth)
// This is the core Phase 1 implementation - uses FileSet positions, no predictions
func (g *PostASTGenerator) Generate() (*preprocessor.SourceMap, error) {
	sm := preprocessor.NewSourceMap()
	sm.DingoFile = g.dingoFilePath
	sm.GoFile = g.goFilePath

	// Step 1: Generate mappings for transformed code (using markers)
	transformMappings := g.matchTransformations()

	// Step 2: Generate mappings for unchanged code (identity + heuristics)
	identityMappings := g.matchIdentity()

	// Step 3: Combine and sort all mappings
	allMappings := append(transformMappings, identityMappings...)
	sort.Slice(allMappings, func(i, j int) bool {
		if allMappings[i].GeneratedLine != allMappings[j].GeneratedLine {
			return allMappings[i].GeneratedLine < allMappings[j].GeneratedLine
		}
		return allMappings[i].GeneratedColumn < allMappings[j].GeneratedColumn
	})

	// Add to source map
	for _, m := range allMappings {
		sm.AddMapping(m)
	}

	return sm, nil
}

// matchTransformations matches metadata to AST nodes using markers
// Returns mappings using ACTUAL positions from FileSet (no prediction)
func (g *PostASTGenerator) matchTransformations() []preprocessor.Mapping {
	mappings := make([]preprocessor.Mapping, 0, len(g.metadata))

	for _, meta := range g.metadata {
		// Find the AST node by marker comment
		pos := g.findMarkerPosition(meta.GeneratedMarker)
		if pos == token.NoPos {
			// Marker not found - skip this transformation
			// (Could happen if preprocessor didn't add marker correctly)
			continue
		}

		// Extract ACTUAL position from FileSet (GROUND TRUTH)
		actualPos := g.fset.Position(pos)

		// Create mapping: original_pos → generated_pos
		mapping := preprocessor.Mapping{
			OriginalLine:    meta.OriginalLine,
			OriginalColumn:  meta.OriginalColumn,
			GeneratedLine:   actualPos.Line,
			GeneratedColumn: actualPos.Column,
			Length:          meta.OriginalLength,
			Name:            meta.Type,
		}

		mappings = append(mappings, mapping)
	}

	return mappings
}

// findMarkerPosition searches for a marker comment in the AST
// Returns the position of the ACTUAL CODE LINE before the marker, not the marker itself
func (g *PostASTGenerator) findMarkerPosition(marker string) token.Pos {
	if marker == "" {
		return token.NoPos
	}

	var markerPos token.Pos

	// Search through all comment groups to find the marker
	for _, cg := range g.goAST.Comments {
		for _, c := range cg.List {
			if strings.Contains(c.Text, marker) {
				markerPos = c.Pos()
				break
			}
		}
		if markerPos != token.NoPos {
			break
		}
	}

	if markerPos == token.NoPos {
		return token.NoPos
	}

	// CRITICAL FIX: The marker comment is AFTER the actual code
	// We need to find the statement BEFORE the marker
	markerLine := g.fset.Position(markerPos).Line
	targetLine := markerLine - 1

	// Find the statement on the line before the marker
	// Priority: AssignStmt > ExprStmt > other statements
	// (Because error propagation typically transforms assignments)
	var assignPos, exprPos, otherPos token.Pos

	ast.Inspect(g.goAST, func(n ast.Node) bool {
		if n == nil {
			return true
		}

		// Get position and check if it's on our target line
		nodePos := n.Pos()
		nodeLine := g.fset.Position(nodePos).Line

		if nodeLine != targetLine {
			return true // Keep searching
		}

		// Check statement types in priority order
		switch n.(type) {
		case *ast.AssignStmt:
			if assignPos == token.NoPos {
				assignPos = nodePos
			}
		case *ast.ExprStmt:
			if exprPos == token.NoPos {
				exprPos = nodePos
			}
		case *ast.ReturnStmt, *ast.IfStmt, *ast.ForStmt, *ast.DeferStmt:
			if otherPos == token.NoPos {
				otherPos = nodePos
			}
		}

		return true
	})

	// Return best match in priority order
	if assignPos != token.NoPos {
		return assignPos
	}
	if exprPos != token.NoPos {
		return exprPos
	}
	if otherPos != token.NoPos {
		return otherPos
	}

	// Fallback: If we can't find the exact statement, return position
	// on the line before the marker (column 1)
	// This handles edge cases where AST inspection might miss something
	file := g.fset.File(markerPos)
	if file != nil && markerLine > 1 {
		// Get position at start of previous line
		lineStart := file.LineStart(markerLine - 1)
		return lineStart
	}

	return token.NoPos
}

// matchIdentity matches unchanged code line-by-line (heuristics)
// For lines without transformations, provide identity or best-effort mappings
func (g *PostASTGenerator) matchIdentity() []preprocessor.Mapping {
	mappings := make([]preprocessor.Mapping, 0)

	// Read .dingo file to get line count
	dingoContent, err := os.ReadFile(g.dingoFilePath)
	if err != nil {
		// If can't read file, return empty (transformations only)
		return mappings
	}

	dingoLines := strings.Split(string(dingoContent), "\n")

	// Build set of lines that have transformations
	transformedLines := make(map[int]bool)
	for _, meta := range g.metadata {
		transformedLines[meta.OriginalLine] = true
	}

	// For each line in .dingo file without transformation:
	// Use identity mapping (line N → line N)
	// This is a simple heuristic - Phase 2 will improve this
	for lineNum := 1; lineNum <= len(dingoLines); lineNum++ {
		if !transformedLines[lineNum] {
			// Identity mapping (heuristic)
			mapping := preprocessor.Mapping{
				OriginalLine:    lineNum,
				OriginalColumn:  1,
				GeneratedLine:   lineNum,
				GeneratedColumn: 1,
				Length:          len(dingoLines[lineNum-1]),
				Name:            "identity",
			}
			mappings = append(mappings, mapping)
		}
	}

	return mappings
}

// GenerateFromFiles is a convenience function that parses the .go file
// and generates source maps in one step (for testing/simple use cases)
func GenerateFromFiles(
	dingoPath, goPath string,
	metadata []preprocessor.TransformMetadata,
) (*preprocessor.SourceMap, error) {
	// Parse .go file to get FileSet and AST
	fset := token.NewFileSet()
	goAST, err := parser.ParseFile(fset, goPath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Go file: %w", err)
	}

	// Create generator
	gen := NewPostASTGenerator(dingoPath, goPath, fset, goAST, metadata)

	// Generate source map
	return gen.Generate()
}
