package preprocessor

import (
	"fmt"
	"regexp"
	"strings"
)

// UnqualifiedImportProcessor transforms unqualified stdlib calls to qualified calls
// and tracks which imports need to be added.
//
// Example transformations:
//   ReadFile(path) → os.ReadFile(path) (adds "os" import)
//   Printf("hello") → fmt.Printf("hello") (adds "fmt" import)
//
// The processor uses:
// - FunctionExclusionCache to skip local user-defined functions
// - StdlibRegistry to determine which package a function belongs to
// - Conservative error handling for ambiguous functions
type UnqualifiedImportProcessor struct {
	cache         *FunctionExclusionCache
	neededImports map[string]bool // Package paths to import
	pattern       *regexp.Regexp  // Matches unqualified function calls
}

// NewUnqualifiedImportProcessor creates a new processor for the given package
func NewUnqualifiedImportProcessor(cache *FunctionExclusionCache) *UnqualifiedImportProcessor {
	// Pattern: Capitalized function call (e.g., ReadFile(...), Printf(...))
	// Matches: [Word boundary][Capital letter][alphanumeric]*[whitespace]*[(]
	// This captures stdlib-style function names
	pattern := regexp.MustCompile(`\b([A-Z][a-zA-Z0-9]*)\s*\(`)

	return &UnqualifiedImportProcessor{
		cache:         cache,
		neededImports: make(map[string]bool),
		pattern:       pattern,
	}
}

// Name returns the processor name for logging
func (p *UnqualifiedImportProcessor) Name() string {
	return "UnqualifiedImportProcessor"
}

// Process transforms unqualified stdlib calls to qualified calls
// Returns:
//   - Transformed source code
//   - Source mappings for LSP
//   - Error if transformation fails (e.g., ambiguous function)
func (p *UnqualifiedImportProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	// Reset state for this run
	p.neededImports = make(map[string]bool)

	var result strings.Builder
	var mappings []Mapping
	lastEnd := 0

	matches := p.pattern.FindAllSubmatchIndex(source, -1)
	for _, match := range matches {
		// match[0], match[1]: Full match (e.g., "ReadFile(")
		// match[2], match[3]: Captured group (e.g., "ReadFile")

		funcNameStart := match[2]
		funcNameEnd := match[3]
		funcName := string(source[funcNameStart:funcNameEnd])

		// Check if this is a local function (skip transformation)
		if p.cache.IsLocalSymbol(funcName) {
			continue
		}

		// Check if this is a method declaration (e.g., func (r Result) Map(...))
		if p.isMethodDeclaration(source, funcNameStart) {
			continue
		}

		// Check if already qualified or is a method call (e.g., "os.ReadFile" or "result.Map")
		if p.isAlreadyQualified(source, funcNameStart) {
			continue
		}

		// Look up in stdlib registry
		pkg, err := GetPackageForFunction(funcName)
		if err != nil {
			// Ambiguous function
			return nil, nil, fmt.Errorf("%w (at position %d)", err, funcNameStart)
		}

		if pkg == "" {
			// Not a stdlib function, skip
			continue
		}

		// Transform: funcName → pkg.funcName
		// Write everything before this match
		result.Write(source[lastEnd:funcNameStart])

		// Calculate line/column for mapping
		origLine, origCol := calculatePosition(source, funcNameStart)
		genLine, genCol := calculatePosition([]byte(result.String()), result.Len())

		// Extract package alias from import path
		// For "encoding/json" → "json"
		// For "os" → "os"
		pkgAlias := pkg
		if idx := strings.LastIndex(pkg, "/"); idx != -1 {
			pkgAlias = pkg[idx+1:]
		}

		// Write qualified name using package alias
		qualified := pkgAlias + "." + funcName
		result.WriteString(qualified)

		// Track import
		p.neededImports[pkg] = true

		// Create source mapping
		// Original: funcName at (origLine, origCol)
		// Generated: pkg.funcName at (genLine, genCol)
		// Length: len(qualified)
		mappings = append(mappings, Mapping{
			GeneratedLine:   genLine,
			GeneratedColumn: genCol,
			OriginalLine:    origLine,
			OriginalColumn:  origCol,
			Length:          len(qualified),
			Name:            fmt.Sprintf("unqualified:%s", funcName),
		})

		lastEnd = funcNameEnd
	}

	// Write remaining source
	result.Write(source[lastEnd:])

	return []byte(result.String()), mappings, nil
}

// GetNeededImports returns the list of import paths that should be added
// Implements the ImportProvider interface
func (p *UnqualifiedImportProcessor) GetNeededImports() []string {
	imports := make([]string, 0, len(p.neededImports))
	for pkg := range p.neededImports {
		imports = append(imports, pkg)
	}
	return imports
}

// isAlreadyQualified checks if a function is already qualified (e.g., os.ReadFile)
// by looking for a preceding identifier followed by a dot
func (p *UnqualifiedImportProcessor) isAlreadyQualified(source []byte, funcPos int) bool {
	if funcPos == 0 {
		return false
	}

	// Look backwards for '.'
	i := funcPos - 1

	// Skip whitespace
	for i >= 0 && (source[i] == ' ' || source[i] == '\t' || source[i] == '\n') {
		i--
	}

	if i < 0 {
		return false
	}

	// Check if immediately preceded by '.'
	if source[i] != '.' {
		return false
	}

	// Found '.', this is always a method call or qualified identifier
	// Examples: result.Map(...), AndThen(...).Map(...), pkg.Function(...)
	// All of these are valid and should be skipped by UnqualifiedImportProcessor
	return true
}

// isMethodDeclaration checks if a function name is part of a method declaration
// by looking for the pattern: func (receiver Type) MethodName(
func (p *UnqualifiedImportProcessor) isMethodDeclaration(source []byte, funcPos int) bool {
	if funcPos < 6 { // Need at least "func ("
		return false
	}

	// Look backwards from function name to find "func ("
	i := funcPos - 1

	// Skip whitespace before function name
	for i >= 0 && (source[i] == ' ' || source[i] == '\t' || source[i] == '\n') {
		i--
	}

	if i < 0 {
		return false
	}

	// Look for closing paren of receiver: ) MethodName
	if source[i] != ')' {
		return false
	}

	// Found ')', now look for opening paren and 'func' keyword
	parenDepth := 1
	i--

	// Skip backward through receiver declaration: (r Result)
	for i >= 0 && parenDepth > 0 {
		if source[i] == ')' {
			parenDepth++
		} else if source[i] == '(' {
			parenDepth--
		}
		i--
	}

	if parenDepth != 0 || i < 0 {
		return false
	}

	// Now i should be just before '(', skip whitespace
	for i >= 0 && (source[i] == ' ' || source[i] == '\t' || source[i] == '\n') {
		i--
	}

	if i < 3 { // Need at least "func"
		return false
	}

	// Check for 'func' keyword
	if i >= 3 &&
		source[i-3] == 'f' &&
		source[i-2] == 'u' &&
		source[i-1] == 'n' &&
		source[i] == 'c' {
		// Verify 'func' is a complete word (preceded by whitespace or start of file)
		if i == 3 {
			return true
		}
		if i > 3 && !isIdentifierChar(source[i-4]) {
			return true
		}
	}

	return false
}

// calculatePosition calculates line and column from byte offset
// Lines and columns are 1-indexed
func calculatePosition(source []byte, offset int) (line, col int) {
	line = 1
	col = 1

	for i := 0; i < offset && i < len(source); i++ {
		if source[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}

	return line, col
}
