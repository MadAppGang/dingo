// Package preprocessor transforms Dingo syntax to valid Go syntax
package preprocessor

import (
	"bytes"
	"fmt"
	"go/parser"
	"go/printer"
	"go/token"
	"sort"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

// Preprocessor orchestrates multiple feature processors to transform
// Dingo source code into valid Go code with semantic placeholders
type Preprocessor struct {
	source     []byte
	processors []FeatureProcessor
}

// FeatureProcessor defines the interface for individual feature preprocessors
type FeatureProcessor interface {
	// Name returns the feature name for logging/debugging
	Name() string

	// Process transforms the source code and returns:
	// - transformed source
	// - source mappings
	// - error if transformation failed
	Process(source []byte) ([]byte, []Mapping, error)
}

// ImportProvider is an optional interface for processors that need to add imports
type ImportProvider interface {
	// GetNeededImports returns list of import paths that should be added
	GetNeededImports() []string
}

// New creates a new preprocessor with all registered features
func New(source []byte) *Preprocessor {
	return &Preprocessor{
		source: source,
		processors: []FeatureProcessor{
			// Order matters! Process in this sequence:
			// 0. Type annotations (: → space) - must be first
			NewTypeAnnotProcessor(),
			// 1. Error propagation (expr?)
			NewErrorPropProcessor(),
			// 2. Keywords (let → var) - after error prop so it doesn't interfere
			NewKeywordProcessor(),
			// 3. Lambdas (|x| expr)
			// NewLambdaProcessor(),
			// 4. Sum types (enum)
			// NewSumTypeProcessor(),
			// 5. Pattern matching (match)
			// NewPatternMatchProcessor(),
			// 6. Operators (ternary, ??, ?.)
			// NewOperatorProcessor(),
		},
	}
}

// Process runs all feature processors in sequence and combines source maps
func (p *Preprocessor) Process() (string, *SourceMap, error) {
	result := p.source
	sourceMap := NewSourceMap()
	neededImports := []string{}

	// Run each processor in sequence
	for _, proc := range p.processors {
		processed, mappings, err := proc.Process(result)
		if err != nil {
			return "", nil, fmt.Errorf("%s preprocessing failed: %w", proc.Name(), err)
		}

		// Update result
		result = processed

		// Merge mappings
		for _, m := range mappings {
			sourceMap.AddMapping(m)
		}

		// Collect needed imports if processor implements ImportProvider
		if importProvider, ok := proc.(ImportProvider); ok {
			imports := importProvider.GetNeededImports()
			neededImports = append(neededImports, imports...)
		}
	}

	// Inject all needed imports at the end (after all transformations complete)
	if len(neededImports) > 0 {
		originalLineCount := strings.Count(string(result), "\n") + 1
		var importInsertLine int
		result, importInsertLine = injectImportsWithPosition(result, neededImports)
		newLineCount := strings.Count(string(result), "\n") + 1
		importLinesAdded := newLineCount - originalLineCount

		// Adjust all source mappings to account for added import lines
		// CRITICAL-2 FIX: Only shift mappings for lines AFTER import insertion point
		if importLinesAdded > 0 {
			adjustMappingsForImports(sourceMap, importLinesAdded, importInsertLine)
		}
	}

	return string(result), sourceMap, nil
}

// ProcessBytes is like Process but returns bytes
func (p *Preprocessor) ProcessBytes() ([]byte, *SourceMap, error) {
	str, sm, err := p.Process()
	if err != nil {
		return nil, nil, err
	}
	return []byte(str), sm, nil
}

// injectImportsWithPosition adds needed imports to the source code and returns the insertion line
// Returns: modified source and the line number where imports were inserted (1-based)
func injectImportsWithPosition(source []byte, needed []string) ([]byte, int) {
	// Parse the source
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", source, parser.ParseComments)
	if err != nil {
		// If parse fails, return original (should not happen after all transformations)
		return source, 1
	}

	// Deduplicate and sort needed imports
	importMap := make(map[string]bool)
	for _, pkg := range needed {
		importMap[pkg] = true
	}

	// Remove packages that are already imported
	for _, imp := range node.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		delete(importMap, path)
	}

	// If no new imports needed, return original
	if len(importMap) == 0 {
		return source, 1
	}

	// Determine import insertion line (after package declaration, before first decl)
	importInsertLine := 1
	if node.Name != nil {
		// Line after package declaration (typically line 1 or 2)
		importInsertLine = fset.Position(node.Name.End()).Line + 1
	}

	// Convert map to sorted slice
	finalImports := make([]string, 0, len(importMap))
	for pkg := range importMap {
		finalImports = append(finalImports, pkg)
	}
	sort.Strings(finalImports)

	// Add each import using astutil
	for _, pkg := range finalImports {
		astutil.AddImport(fset, node, pkg)
	}

	// Generate output with imports
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, node); err != nil {
		return source, importInsertLine
	}

	return buf.Bytes(), importInsertLine
}

// adjustMappingsForImports shifts mapping line numbers to account for added imports
// CRITICAL-2 FIX: Only shifts mappings for lines AFTER the import insertion point
func adjustMappingsForImports(sourceMap *SourceMap, numImportLines int, importInsertionLine int) {
	for i := range sourceMap.Mappings {
		// Only shift mappings for generated lines at or after the import insertion point
		if sourceMap.Mappings[i].GeneratedLine >= importInsertionLine {
			sourceMap.Mappings[i].GeneratedLine += numImportLines
		}
	}
}
