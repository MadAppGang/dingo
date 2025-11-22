// Package preprocessor transforms Dingo syntax to valid Go syntax
package preprocessor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MadAppGang/dingo/pkg/config"
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
	cache *FunctionExclusionCache
}

// TransformMetadata holds metadata about a transformation (NOT final mappings)
// This is emitted by preprocessors and used by Post-AST generator to match AST nodes
type TransformMetadata struct {
	Type            string // "error_prop", "type_annot", "enum", etc.
	OriginalLine    int    // Line in .dingo file
	OriginalColumn  int    // Column in .dingo file
	OriginalLength  int    // Length in .dingo file
	OriginalText    string // Original Dingo syntax (e.g., "?")
	GeneratedMarker string // Unique marker in Go code (e.g., "// dingo:e:0")
	ASTNodeType     string // "CallExpr", "FuncDecl", "IfStmt", etc.
}

// ProcessResult holds the result of preprocessing
// Supports both legacy mappings and new Post-AST metadata
type ProcessResult struct {
	Source   []byte              // Transformed Go source code
	Mappings []Mapping           // LEGACY: For backward compatibility
	Metadata []TransformMetadata // NEW: For Post-AST generation
}

// FeatureProcessor defines the interface for individual feature preprocessors
type FeatureProcessor interface {
	// Name returns the feature name for logging/debugging
	Name() string

	// Process transforms the source code and returns:
	// - transformed source
	// - source mappings for error reporting
	// - error if transformation failed
	Process(source []byte) ([]byte, []Mapping, error)
}

// FeatureProcessorV2 is the new interface that supports metadata emission
// Processors can implement this interface to support metadata-based source map generation
type FeatureProcessorV2 interface {
	FeatureProcessor // Embed the old interface for backward compatibility

	// ProcessV2 transforms the source code and returns a ProcessResult
	// This method supports metadata generation
	ProcessV2(source []byte) (ProcessResult, error)
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
		// 4. Tuples ((a, b) = (1, 2)) - BEFORE safe navigation (uses . in field access)
		NewTupleProcessor(),
		// 5. Safe navigation (?.) - BEFORE null coalescing (SafeNav handles ?. before NullCoalesce sees ??)
		NewSafeNavProcessor(),
		// 6. Null coalescing (??) - AFTER safe navigation, BEFORE ternary
		//    CRITICAL: Must run BEFORE TernaryProcessor and ErrorPropProcessor
		NewNullCoalesceProcessor(),
		// 7. Ternary operator (? :) - AFTER null coalescing, BEFORE error propagation
		//    Process ternary BEFORE error prop to cleanly separate ? : from single ?
		NewTernaryProcessor(),
		// 8. Error propagation (expr?) - AFTER ternary (handles remaining ?)
		NewErrorPropProcessor(),
	}

	// 9. Enums (enum Name { ... }) - after error prop, before keywords
	processors = append(processors, NewEnumProcessor())

	// 10. Keywords (let → var) - after pattern match, error prop, and enum
	processors = append(processors, NewKeywordProcessor())

	// 11. Unqualified imports (ReadFile → os.ReadFile) - requires cache
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

// ProcessWithMetadata runs all feature processors and returns both legacy mappings and metadata
func (p *Preprocessor) ProcessWithMetadata() (string, *SourceMap, []TransformMetadata, error) {
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
	allMetadata := []TransformMetadata{}
	neededImports := []string{}

	// Run each processor in sequence
	for _, proc := range p.processors {
		// Check if processor implements V2 interface
		if procV2, ok := proc.(FeatureProcessorV2); ok {
			// Use new ProcessV2 method
			procResult, err := procV2.ProcessV2(result)
			if err != nil {
				return "", nil, nil, fmt.Errorf("%s preprocessing failed: %w", proc.Name(), err)
			}

			// Update result
			result = procResult.Source

			// Merge mappings (legacy support)
			for _, m := range procResult.Mappings {
				sourceMap.AddMapping(m)
			}

			// Collect metadata
			allMetadata = append(allMetadata, procResult.Metadata...)
		} else {
			// Fall back to legacy Process method
			processed, mappings, err := proc.Process(result)
			if err != nil {
				return "", nil, nil, fmt.Errorf("%s preprocessing failed: %w", proc.Name(), err)
			}

			// Update result
			result = processed

			// Merge mappings
			for _, m := range mappings {
				sourceMap.AddMapping(m)
			}
		}

		// Collect needed imports if processor implements ImportProvider
		if importProvider, ok := proc.(ImportProvider); ok {
			imports := importProvider.GetNeededImports()
			neededImports = append(neededImports, imports...)
		}
	}

	// Inject all needed imports at the end (after all transformations complete)
	if len(neededImports) > 0 {
		var importInsertLine, importBlockEndLine int
		var err error
		// CRITICAL FIX: Get both import start and end lines for accurate shifting
		result, importInsertLine, importBlockEndLine, err = injectImportsWithPosition(result, neededImports)
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to inject imports: %w", err)
		}

		// Calculate how many lines the import block occupies
		// importInsertLine is where imports are inserted (after package declaration)
		// importBlockEndLine is where imports end (last line of import block)
		// CRITICAL FIX: Only apply adjustment if imports were actually added
		if importInsertLine > 0 && importBlockEndLine > 0 {
			importBlockSize := importBlockEndLine - importInsertLine + 1

			// Adjust all source mappings to account for added import lines
			adjustMappingsForImports(sourceMap, importBlockSize, importInsertLine)

			// TODO: Adjust metadata line numbers
			// This will be needed when we integrate metadata-based source maps
		}
	}

	return string(result), sourceMap, allMetadata, nil
}

// Process runs all feature processors in sequence and combines source maps
// This is the legacy method that returns only source maps (for backward compatibility)
func (p *Preprocessor) Process() (string, *SourceMap, error) {
	// Delegate to ProcessWithMetadata and discard metadata
	result, sourceMap, _, err := p.ProcessWithMetadata()
	return result, sourceMap, err
}

// DEPRECATED: Old Process implementation kept for reference during migration
// Will be removed after all callers migrate to ProcessWithMetadata
func (p *Preprocessor) processLegacy() (string, *SourceMap, error) {
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
		var importInsertLine, importBlockEndLine int
		var err error
		// CRITICAL FIX: Get both import start and end lines for accurate shifting
		result, importInsertLine, importBlockEndLine, err = injectImportsWithPosition(result, neededImports)
		if err != nil {
			return "", nil, fmt.Errorf("failed to inject imports: %w", err)
		}

		// Calculate how many lines the import block occupies
		// importInsertLine is where imports are inserted (after package declaration)
		// importBlockEndLine is where imports end (last line of import block)
		// CRITICAL FIX: Only apply adjustment if imports were actually added
		if importInsertLine > 0 && importBlockEndLine > 0 {
			// Calculate the number of lines added by the import block
			//
			// Example - multi-line import:
			// BEFORE import injection (preprocessed code):
			//   Line 1: package main
			//   Line 2: [blank]
			//   Line 3: func readConfig(...) {
			//   Line 4:     tmp, err := os.ReadFile(path)  ← mapping says gen_line=4
			//
			// AFTER import injection:
			//   Line 1: package main
			//   Line 2: [blank]
			//   Line 3: import (             ← importInsertLine is BEFORE this (line 2)
			//   Line 4:     "os"
			//   Line 5: )                    ← importBlockEndLine = 5
			//   Line 6: [blank line added by go/printer]
			//   Line 7: func readConfig(...) {
			//   Line 8:     tmp, err := os.ReadFile(path)  ← should be gen_line=8
			//
			// Calculation:
			//   importInsertLine = 2 (line after package, before imports start)
			//   importBlockEndLine = 5 (last line of import block)
			//   Shift needed = 8 - 4 = 4 lines
			//   Formula: importBlockEndLine - importInsertLine + 1 = 5 - 2 + 1 = 4 ✓
			//
			// The +1 accounts for the blank line that go/printer adds after the import block
			importBlockSize := importBlockEndLine - importInsertLine + 1

			// Adjust all source mappings to account for added import lines
			adjustMappingsForImports(sourceMap, importBlockSize, importInsertLine)
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

// injectImportsWithPosition adds needed imports using TEXT-BASED manipulation
// CRITICAL: This preserves ALL comments including free-floating markers (// dingo:E:N)
// AST-based approaches strip comments, breaking source map generation
// Returns: modified source, import block start line (1-based), import block end line (1-based), and error
func injectImportsWithPosition(source []byte, needed []string) ([]byte, int, int, error) {
	if len(needed) == 0 {
		return source, 0, 0, nil
	}

	// Deduplicate and sort needed imports
	importMap := make(map[string]bool)
	for _, pkg := range needed {
		importMap[pkg] = true
	}

	code := string(source)
	lines := strings.Split(code, "\n")

	// Find package declaration
	packageLineIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") {
			packageLineIdx = i
			break
		}
	}
	if packageLineIdx == -1 {
		return nil, 0, 0, fmt.Errorf("no package declaration found")
	}

	// Find existing import block (if any)
	importBlockStart, importBlockEnd := findImportBlock(lines)

	// Parse existing imports to avoid duplicates
	if importBlockStart >= 0 {
		for i := importBlockStart + 1; i < importBlockEnd; i++ {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "" || trimmed == ")" {
				continue
			}
			// Extract import path: "path" or _ "path" or alias "path"
			importPath := extractImportPath(trimmed)
			if importPath != "" {
				delete(importMap, importPath)
			}
		}
	}

	// If no new imports needed, return original
	if len(importMap) == 0 {
		return source, 0, 0, nil
	}

	// Convert to sorted slice
	finalImports := make([]string, 0, len(importMap))
	for pkg := range importMap {
		finalImports = append(finalImports, pkg)
	}
	sort.Strings(finalImports)

	var result string
	var insertLine, endLine int

	if importBlockStart >= 0 {
		// Merge with existing import block
		result, insertLine, endLine = mergeImports(lines, finalImports, importBlockStart, importBlockEnd)
	} else {
		// Create new import block after package
		result, insertLine, endLine = insertImports(lines, finalImports, packageLineIdx)
	}

	return []byte(result), insertLine, endLine, nil
}

// findImportBlock locates an existing import block in the source
// Returns start and end line indices (0-based), or -1, -1 if not found
func findImportBlock(lines []string) (start, end int) {
	inImport := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import (") {
			start = i
			inImport = true
		} else if inImport && trimmed == ")" {
			end = i
			return start, end
		} else if strings.HasPrefix(trimmed, "import ") && !strings.Contains(trimmed, "(") {
			// Single import: import "path"
			return i, i
		}
	}
	return -1, -1
}

// extractImportPath extracts the import path from an import line
// Handles: "path", _ "path", alias "path"
func extractImportPath(line string) string {
	// Remove leading/trailing whitespace
	line = strings.TrimSpace(line)

	// Find the quoted string
	startQuote := strings.Index(line, `"`)
	if startQuote == -1 {
		return ""
	}
	endQuote := strings.LastIndex(line, `"`)
	if endQuote <= startQuote {
		return ""
	}

	return line[startQuote+1 : endQuote]
}

// mergeImports adds new imports to an existing import block
func mergeImports(lines []string, newImports []string, start, end int) (string, int, int) {
	// Build new lines with merged imports
	result := make([]string, 0, len(lines)+len(newImports))

	// Copy lines before import block
	result = append(result, lines[:end]...)

	// Add new imports before closing )
	for _, imp := range newImports {
		result = append(result, fmt.Sprintf("\t\"%s\"", imp))
	}

	// Copy lines from closing ) onward
	result = append(result, lines[end:]...)

	// Calculate positions (1-based for return)
	insertLine := start + 2      // Line after "import ("
	endLine := end + len(newImports) + 1 // Adjusted closing )

	return strings.Join(result, "\n"), insertLine, endLine
}

// insertImports creates a new import block after the package declaration
func insertImports(lines []string, imports []string, packageLine int) (string, int, int) {
	// Build import block
	importBlock := []string{
		"",
		"import (",
	}
	for _, imp := range imports {
		importBlock = append(importBlock, fmt.Sprintf("\t\"%s\"", imp))
	}
	importBlock = append(importBlock, ")")

	// Insert after package line
	result := make([]string, 0, len(lines)+len(importBlock))
	result = append(result, lines[:packageLine+1]...)
	result = append(result, importBlock...)
	result = append(result, lines[packageLine+1:]...)

	// Calculate positions (1-based for return)
	insertLine := packageLine + 2        // Line after package declaration
	endLine := packageLine + 1 + len(importBlock) // Last line of import block

	return strings.Join(result, "\n"), insertLine, endLine
}

// adjustMappingsForImports shifts mapping line numbers to account for added imports
// CRITICAL FIX: Shifts mappings for lines AFTER the import insertion point
func adjustMappingsForImports(sourceMap *SourceMap, numImportLines int, importInsertionLine int) {
	for i := range sourceMap.Mappings {
		// CRITICAL FIX: Only shift mappings for lines AFTER import insertion
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
		//   Line 3: func foo() { ... } (shifts to line 7 if 4-line import block added)
		//
		// Mappings with GeneratedLine=1 or 2 stay as-is.
		// Mappings with GeneratedLine=3+ are shifted by numImportLines.
		if sourceMap.Mappings[i].GeneratedLine > importInsertionLine {
			sourceMap.Mappings[i].GeneratedLine += numImportLines
		}
	}
}
