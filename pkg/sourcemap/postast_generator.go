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

	// Step 2: Build set of generated lines that already have transformations
	// This prevents identity mappings from creating duplicates
	transformedGoLines := make(map[int]bool)
	for _, m := range transformMappings {
		transformedGoLines[m.GeneratedLine] = true
	}

	// Step 3: Generate mappings for unchanged code (identity + heuristics)
	// Pass the set of transformed lines to skip them
	identityMappings := g.matchIdentity(transformedGoLines)

	// Step 4: Combine and sort all mappings
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

	// Get marker line and check if there's code on the same line (inline comment)
	markerLine := g.fset.Position(markerPos).Line

	// Strategy: The marker can be:
	// 1. Inline comment (same line as code): tmp, err := foo() // dingo:e:0
	// 2. Next-line comment (line after code):
	//    tmp, err := foo()
	//    // dingo:e:0

	// First, try to find a statement on the SAME line as marker (inline)
	var statementPos token.Pos
	ast.Inspect(g.goAST, func(n ast.Node) bool {
		if n == nil {
			return true
		}

		nodePos := n.Pos()
		nodeLine := g.fset.Position(nodePos).Line

		if nodeLine != markerLine {
			return true
		}

		// Check for statement types
		switch n.(type) {
		case *ast.AssignStmt, *ast.ExprStmt, *ast.ReturnStmt, *ast.IfStmt, *ast.ForStmt, *ast.DeferStmt:
			if statementPos == token.NoPos {
				statementPos = nodePos
			}
		}

		return true
	})

	// If found statement on same line, return it (inline comment case)
	if statementPos != token.NoPos {
		return statementPos
	}

	// Not inline - marker is on separate line after code
	// Find the statement on the line BEFORE the marker
	targetLine := markerLine - 1

	// Find the statement on the line before the marker
	// Priority: AssignStmt > ExprStmt > other statements
	var assignPos, exprPos, otherPos token.Pos

	ast.Inspect(g.goAST, func(n ast.Node) bool {
		if n == nil {
			return true
		}

		nodePos := n.Pos()
		nodeLine := g.fset.Position(nodePos).Line

		if nodeLine != targetLine {
			return true
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
	file := g.fset.File(markerPos)
	if file != nil && markerLine > 1 {
		lineStart := file.LineStart(markerLine - 1)
		return lineStart
	}

	return token.NoPos
}

// matchIdentity matches unchanged code line-by-line with line offset calculation
// For lines without transformations, provide best-effort mappings accounting for:
// 1. Import block injection (adds lines at top of .go file)
// 2. Other preprocessor changes that shift line numbers
//
// CRITICAL: transformedGoLines contains the generated line numbers that already
// have transformation mappings - we MUST skip these to avoid duplicate mappings
func (g *PostASTGenerator) matchIdentity(transformedGoLines map[int]bool) []preprocessor.Mapping {
	mappings := make([]preprocessor.Mapping, 0)

	// Read .dingo file to get line count
	dingoContent, err := os.ReadFile(g.dingoFilePath)
	if err != nil {
		// If can't read file, return empty (transformations only)
		return mappings
	}

	dingoLines := strings.Split(string(dingoContent), "\n")

	// Read .go file to match content
	goContent, err := os.ReadFile(g.goFilePath)
	if err != nil {
		return mappings
	}
	goLines := strings.Split(string(goContent), "\n")

	// Build set of .dingo lines that have transformations (original lines)
	transformedDingoLines := make(map[int]bool)
	for _, meta := range g.metadata {
		transformedDingoLines[meta.OriginalLine] = true
	}

	// Build line-by-line offset map (handles variable offsets)
	offsetMap := g.buildOffsetMap(dingoLines, goLines)

	// Track generated lines we've already created identity mappings for
	// This prevents multiple identity mappings for the same generated line
	usedGoLines := make(map[int]bool)

	// For each line in .dingo file without transformation:
	// Apply line-specific offset to map to correct .go line
	for dingoLineNum := 1; dingoLineNum <= len(dingoLines); dingoLineNum++ {
		if !transformedDingoLines[dingoLineNum] {
			// Get line-specific offset
			lineOffset, exists := offsetMap[dingoLineNum]
			if !exists {
				// No offset found - try identity mapping
				lineOffset = 0
			}

			// Calculate corresponding .go line (with offset)
			goLineNum := dingoLineNum + lineOffset

			// Verify the line exists in .go file
			if goLineNum < 1 || goLineNum > len(goLines) {
				continue // Skip if out of range
			}

			// CRITICAL: Skip if this .go line already has a transformation mapping
			// This prevents duplicate mappings for the same generated line
			if transformedGoLines[goLineNum] {
				continue
			}

			// CRITICAL: Skip if we've already created an identity mapping for this .go line
			// This handles cases where multiple .dingo lines map to the same .go line
			if usedGoLines[goLineNum] {
				continue
			}

			// Mark this generated line as used
			usedGoLines[goLineNum] = true

			// Create mapping with offset applied
			mapping := preprocessor.Mapping{
				OriginalLine:    dingoLineNum,
				OriginalColumn:  1,
				GeneratedLine:   goLineNum,
				GeneratedColumn: 1,
				Length:          len(dingoLines[dingoLineNum-1]),
				Name:            "identity",
			}
			mappings = append(mappings, mapping)
		}
	}

	return mappings
}

// buildOffsetMap creates a map of line number → offset by matching content
// Handles variable offsets (e.g., lines before/after import block have different offsets)
func (g *PostASTGenerator) buildOffsetMap(dingoLines, goLines []string) map[int]int {
	offsetMap := make(map[int]int)
	usedGoLines := make(map[int]bool) // Track which go lines have been matched

	// Match each dingo line to corresponding go line by content
	for i, dingoLine := range dingoLines {
		dingoLineNum := i + 1 // 1-based
		trimmedDingo := strings.TrimSpace(dingoLine)

		// Skip empty lines - they're ambiguous
		if trimmedDingo == "" {
			continue
		}

		// Search for matching line in .go file (within reasonable range)
		// Start from same line number, search within ±20 lines
		searchStart := maxInt(0, i-5)
		searchEnd := minInt(len(goLines), i+20)

		// Find first unused matching line
		for j := searchStart; j < searchEnd; j++ {
			trimmedGo := strings.TrimSpace(goLines[j])

			if trimmedDingo == trimmedGo {
				goLineNum := j + 1 // 1-based

				// Skip if this go line has already been matched (handles duplicate content)
				if usedGoLines[goLineNum] {
					continue
				}

				// Found match - calculate offset
				offset := goLineNum - dingoLineNum
				offsetMap[dingoLineNum] = offset
				usedGoLines[goLineNum] = true // Mark as used
				break
			}
		}
	}

	return offsetMap
}

// maxInt returns the maximum of two integers
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// minInt returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
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
