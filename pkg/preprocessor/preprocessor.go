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

	"github.com/MadAppGang/dingo/pkg/config"
	"golang.org/x/tools/go/ast/astutil"
)

// Preprocessor orchestrates multiple feature processors to transform
// Dingo source code into valid Go code with semantic placeholders
type Preprocessor struct {
	source     []byte
	processors []FeatureProcessor
	oldConfig  *Config        // Deprecated: Legacy preprocessor-specific config
	config     *config.Config // Main Dingo configuration

	// Package-wide cache (optional, for unqualified import inference)
	// When present, enables early bailout optimization and local function exclusion
	cache      *FunctionExclusionCache
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

// New creates a new preprocessor with all registered features and default config
func New(source []byte) *Preprocessor {
	return NewWithMainConfig(source, nil)
}

// NewWithConfig creates a new preprocessor with legacy config (deprecated)
// Use NewWithMainConfig instead
func NewWithConfig(source []byte, legacyConfig *Config) *Preprocessor {
	// Convert legacy config to main config
	cfg := config.DefaultConfig()
	if legacyConfig != nil && legacyConfig.MultiValueReturnMode == "single" {
		// Map legacy mode to main config (feature not in main config yet)
	}
	return NewWithMainConfig(source, cfg)
}

// NewWithMainConfig creates a new preprocessor with main Dingo configuration
func NewWithMainConfig(source []byte, cfg *config.Config) *Preprocessor {
	return newWithConfigAndCache(source, cfg, nil)
}

// newWithConfigAndCache is the internal constructor that accepts an optional cache
func newWithConfigAndCache(source []byte, cfg *config.Config, cache *FunctionExclusionCache) *Preprocessor {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	processors := []FeatureProcessor{
		// Order matters! Process in this sequence:
		// 0. Generic syntax (<> → []) - must be FIRST before type annotations
		NewGenericSyntaxProcessor(),
		// 1. Pattern matching (match) - MUST run BEFORE lambdas (both use =>)
		//    Match arms: Pattern => Expression (structural context)
		//    Lambdas: params => expression (expression context)
		NewRustMatchProcessor(),
		// 2. Lambdas (x => expr, |x| expr) - AFTER pattern matching
		NewLambdaProcessorWithConfig(cfg),
		// 3. Type annotations (: → space) - after lambdas, after generic syntax
		NewTypeAnnotProcessor(),
		// 4. Safe navigation (?.) - BEFORE null coalescing (SafeNav handles ?. before NullCoalesce sees ??)
		NewSafeNavProcessor(),
		// 5. Null coalescing (??) - AFTER safe navigation, BEFORE error propagation
		//    CRITICAL: Must run BEFORE ErrorPropProcessor to avoid transforming ?? into error handling
		NewNullCoalesceProcessor(),
		// 6. Error propagation (expr?) - AFTER null coalescing (both use ?)
		NewErrorPropProcessor(),
	}

	// 7. Enums (enum Name { ... }) - after error prop, before keywords
	processors = append(processors, NewEnumProcessor())

	// 8. Keywords (let → var) - after pattern match, error prop, and enum
	processors = append(processors, NewKeywordProcessor())

	// 9. Unqualified imports (ReadFile → os.ReadFile) - requires cache
	if cache != nil {
		processors = append(processors, NewUnqualifiedImportProcessor(cache))
	}

	return &Preprocessor{
		source:     source,
		config:     cfg,
		oldConfig:  nil, // No longer used
		processors: processors,
		cache:      cache,
	}
}

// Process runs all feature processors in sequence and combines source maps
func (p *Preprocessor) Process() (string, *SourceMap, error) {
	// Early bailout optimization (GPT-5.1): If cache indicates no unqualified imports
	// in this package, skip expensive symbol resolution for unqualified import processors
	skipUnqualifiedProcessing := false
	if p.cache != nil && !p.cache.HasUnqualifiedImports() {
		// This package has no unqualified stdlib calls, skip that processing
		skipUnqualifiedProcessing = true
	}
	_ = skipUnqualifiedProcessing // TODO: Use when UnqualifiedImportProcessor is integrated

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
		var err error
		// IMPORTANT-2 FIX: injectImportsWithPosition now returns errors instead of silent fallback
		result, importInsertLine, err = injectImportsWithPosition(result, neededImports)
		if err != nil {
			return "", nil, fmt.Errorf("failed to inject imports: %w", err)
		}
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

// GetCache returns the function exclusion cache (if present)
// Returns nil if preprocessor was created without a cache
func (p *Preprocessor) GetCache() *FunctionExclusionCache {
	return p.cache
}

// HasCache returns true if this preprocessor has a package-wide cache
func (p *Preprocessor) HasCache() bool {
	return p.cache != nil
}

// injectImportsWithPosition adds needed imports to the source code and returns the insertion line
// IMPORTANT-2 FIX: Now returns errors instead of silently falling back to original source
// Returns: modified source, insertion line (1-based), and error
func injectImportsWithPosition(source []byte, needed []string) ([]byte, int, error) {
	// Parse the source
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", source, parser.ParseComments)
	if err != nil {
		// IMPORTANT-2 FIX: Return error instead of silently falling back
		return nil, 0, fmt.Errorf("failed to parse source for import injection: %w", err)
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
		return source, 1, nil
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
		// IMPORTANT-2 FIX: Return error instead of silently falling back
		return nil, 0, fmt.Errorf("failed to print AST with imports: %w", err)
	}

	return buf.Bytes(), importInsertLine, nil
}

// adjustMappingsForImports shifts mapping line numbers to account for added imports
// CRITICAL-2 FIX: Only shifts mappings for lines AFTER the import insertion point
func adjustMappingsForImports(sourceMap *SourceMap, numImportLines int, importInsertionLine int) {
	for i := range sourceMap.Mappings {
		// CRITICAL-2 FIX: Only shift mappings for lines AFTER import insertion
		//
		// importInsertionLine is the line number (1-based) where imports are inserted
		// (typically line 2 or 3, right after the package declaration).
		//
		// We use > (not >=) to exclude the insertion line itself. Mappings AT the
		// insertion line are for package-level declarations BEFORE the imports, and
		// should NOT be shifted.
		//
		// Example:
		//   Line 1: package main
		//   Line 2: [IMPORTS INSERTED HERE] ← importInsertionLine = 2
		//   Line 3: func foo() { ... } (shifts to line 5 if 2 imports added)
		//
		// Mappings with GeneratedLine=1 or 2 stay as-is.
		// Mappings with GeneratedLine=3+ are shifted by numImportLines.
		if sourceMap.Mappings[i].GeneratedLine > importInsertionLine {
			sourceMap.Mappings[i].GeneratedLine += numImportLines
		}
	}
}
