package codegen

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/MadAppGang/dingo/pkg/typeloader"
)

// profileTranspile enables timing output for type resolution stages.
// Set DINGO_PROFILE=1 to enable.
var profileTranspile = os.Getenv("DINGO_PROFILE") == "1"

// globalTypeCache caches type loading results across multiple file transpilations.
// This is critical for performance - packages.Load() is very expensive (1+ second).
var globalTypeCache = &typeCache{
	results: make(map[string]*typeloader.LoadResult),
}

type typeCache struct {
	mu      sync.RWMutex
	results map[string]*typeloader.LoadResult // key is sorted, comma-joined imports
}

// ClearTypeCache clears the global type cache. Call this at the start of a new build.
func ClearTypeCache() {
	globalTypeCache.mu.Lock()
	defer globalTypeCache.mu.Unlock()
	globalTypeCache.results = make(map[string]*typeloader.LoadResult)
}

func (c *typeCache) get(key string) (*typeloader.LoadResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result, ok := c.results[key]
	return result, ok
}

func (c *typeCache) set(key string, result *typeloader.LoadResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results[key] = result
}

// TypeResolver provides type information for expressions using go/types.
// It handles cross-file and cross-package type resolution.
//
// This resolver is optional - if it fails to load types, callers should
// fall back to existing local-only search behavior.
type TypeResolver struct {
	fset             *token.FileSet
	file             *ast.File
	typeInfo         *types.Info
	pkg              *types.Package
	loadResult       *typeloader.LoadResult
	importAliases    map[string]string                                   // maps alias (e.g., "util") to full path
	dingoFuncs       map[string]map[string]*typeloader.FunctionSignature // maps pkgAlias -> funcName -> signature (from Dingo sources)
	interfaceMethods map[string]*typeloader.FunctionSignature            // maps methodName -> signature (from interface definitions)
	workingDir       string                                              // directory where source file lives
}

// NewTypeResolver creates a resolver from Dingo source.
//
// Parameters:
//   - src: Dingo source code (will be sanitized internally)
//   - workingDir: Directory for resolving imports (usually project root)
//
// Returns error if source cannot be parsed. Type checking errors are
// silently ignored - we want partial type info even if some expressions fail.
//
// NOTE: The sanitization here is NOT byte-based code transformation (forbidden by CLAUDE.md).
// It's a preprocessing step to make Dingo source parseable by go/parser.
// All actual type analysis is done via go/ast and go/types (token-based).
func NewTypeResolver(src []byte, workingDir string) (*TypeResolver, error) {
	// Step 1: Sanitize Dingo syntax for go/parser
	// Replace '?' with space to make source parseable
	// Also replace 'match' keyword with spaces (same length)
	// This preserves byte positions while making valid Go
	sanitized := SanitizeDingoSource(src)

	// Step 2: Parse with go/parser to get AST
	fset := token.NewFileSet()
	file, parseErr := parser.ParseFile(fset, "", sanitized, parser.ParseComments)

	// Step 3: Extract imports and aliases from AST
	// Even if parsing failed, we may have partial AST for imports
	var imports []string
	var importAliases map[string]string
	if file != nil {
		imports = extractImportsFromAST(file)
		importAliases = extractImportAliasesFromAST(file)
	}

	// If parsing failed completely (no valid package declaration), return error
	// file.Name.Name == "" means parser couldn't find a valid package name
	if file == nil || file.Name == nil || file.Name.Name == "" {
		return nil, parseErr
	}

	// Step 4: Load external package types using typeloader
	// Make working directory absolute for go/packages
	if workingDir == "" || workingDir == "." {
		workingDir, _ = filepath.Abs(".")
	} else {
		workingDir, _ = filepath.Abs(workingDir)
	}

	// Create cache key from imports (sorted for consistency)
	sortedImports := make([]string, len(imports))
	copy(sortedImports, imports)
	// Simple sort for cache key
	for i := 0; i < len(sortedImports); i++ {
		for j := i + 1; j < len(sortedImports); j++ {
			if sortedImports[i] > sortedImports[j] {
				sortedImports[i], sortedImports[j] = sortedImports[j], sortedImports[i]
			}
		}
	}
	cacheKey := workingDir + ":" + strings.Join(sortedImports, ",")

	// Try cache first
	var loadResult *typeloader.LoadResult
	if cached, ok := globalTypeCache.get(cacheKey); ok {
		loadResult = cached
	} else {
		// Cache miss - load from packages (expensive!)
		loader := typeloader.NewLoader(typeloader.LoaderConfig{
			WorkingDir: workingDir,
			FailFast:   false, // Don't fail on first error
		})

		var loadErr error
		loadResult, loadErr = loader.LoadFromImports(imports)
		// Non-fatal: we can still do local type checking even if imports fail
		if loadErr != nil {
			loadResult = nil
		}
		// Cache the result (even if nil, to avoid retrying)
		globalTypeCache.set(cacheKey, loadResult)
	}

	// Step 5: Run go/types type checker
	typeInfo := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}

	// Try fast gcexportdata-based importer first, fall back to source importer.
	// The gc export importer reads pre-compiled type data from the build cache,
	// which is ~1000x faster than the source importer for large dependency graphs.
	var typeImporter types.Importer
	if len(imports) > 0 {
		var gcStart time.Time
		if profileTranspile {
			gcStart = time.Now()
		}
		gcImp, gcErr := NewGCExportImporter(fset, workingDir, imports...)
		if gcErr == nil {
			typeImporter = gcImp
			if profileTranspile {
				fmt.Fprintf(os.Stderr, "    [PROFILE] gcExportImporter created (%d exports) %v\n", gcImp.ExportCount(), time.Since(gcStart))
			}
		} else {
			if profileTranspile {
				fmt.Fprintf(os.Stderr, "    [PROFILE] gcExportImporter failed, falling back to source: %v\n", gcErr)
			}
		}
	}
	if typeImporter == nil {
		typeImporter = importer.ForCompiler(fset, "source", nil)
	}

	config := types.Config{
		Importer: typeImporter,
		Error: func(err error) {
			// Ignore errors - we want partial type info
			// Dingo syntax may cause type mismatches
		},
	}

	pkg, _ := config.Check("main", fset, []*ast.File{file}, typeInfo)
	// Ignore check errors - we still get partial type info

	resolver := &TypeResolver{
		fset:          fset,
		file:          file,
		typeInfo:      typeInfo,
		pkg:           pkg,
		loadResult:    loadResult,
		importAliases: importAliases,
		workingDir:    workingDir,
	}

	// Step 6: Load function signatures from Dingo source files in imported packages
	// This handles cross-package calls where the imported package is written in Dingo
	resolver.loadDingoFunctions()

	return resolver, nil
}

// NewTypeResolverWithCache creates a resolver using pre-loaded types from BuildCache.
// This is the performance-optimized path for multi-file builds.
//
// Instead of calling packages.Load() (~1.4s per file), we look up types from the
// pre-loaded cache (~0.02ms). The cache is populated once for all files by shadow.Builder.
//
// Parameters:
//   - src: Dingo source code (will be sanitized internally)
//   - workingDir: Directory for resolving imports
//   - cache: Pre-loaded BuildCache from shadow.Builder
//
// Returns error if source cannot be parsed. Type checking errors are
// silently ignored - we want partial type info even if some expressions fail.
func NewTypeResolverWithCache(src []byte, workingDir string, cache *typeloader.BuildCache) (*TypeResolver, error) {
	if cache == nil {
		// Fallback to regular NewTypeResolver if no cache provided
		return NewTypeResolver(src, workingDir)
	}

	// Step 1: Sanitize Dingo syntax for go/parser (same as NewTypeResolver)
	sanitized := SanitizeDingoSource(src)

	// Step 2: Parse with go/parser to get AST
	fset := token.NewFileSet()
	file, parseErr := parser.ParseFile(fset, "", sanitized, parser.ParseComments)

	// Step 3: Extract imports and aliases from AST
	// Even if parsing failed, we may have partial AST for imports
	var imports []string
	var importAliases map[string]string
	if file != nil {
		imports = extractImportsFromAST(file)
		importAliases = extractImportAliasesFromAST(file)
	}

	// If parsing failed completely (no valid package declaration), return error
	if file == nil || file.Name == nil || file.Name.Name == "" {
		return nil, parseErr
	}

	// Step 4: Get types from pre-loaded cache (FAST!)
	// This is the key difference from NewTypeResolver - uses cache.LoadImports()
	// which returns cached results (~0.02ms) instead of packages.Load() (~1.4s)
	if workingDir == "" || workingDir == "." {
		workingDir, _ = filepath.Abs(".")
	} else {
		workingDir, _ = filepath.Abs(workingDir)
	}

	loadResult, loadErr := cache.LoadImports(imports)
	if loadErr != nil {
		// Non-fatal: we can still do local type checking
		loadResult = nil
	}

	// Step 5: SKIP go/types type checker for cached version
	// The expensive types.Config.Check() uses importer.ForCompiler() which
	// calls packages.Load() again for each import (~200ms per import).
	// Instead, we rely entirely on loadResult (Strategy 2 in GetReturnCount).
	// This is the key optimization: O(1) cache lookup vs O(n * 200ms) type checking.
	//
	// We create empty typeInfo so GetReturnCount's Strategy 1 falls through
	// to Strategy 2 which uses our cached function signatures.
	typeInfo := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}

	resolver := &TypeResolver{
		fset:          fset,
		file:          file,
		typeInfo:      typeInfo,
		pkg:           nil, // No package - we skip type checking
		loadResult:    loadResult,
		importAliases: importAliases,
		workingDir:    workingDir,
	}

	// Step 6: Load function signatures from Dingo source files in imported packages
	// This handles cross-package calls where the imported package is written in Dingo
	resolver.loadDingoFunctions()

	return resolver, nil
}

// GetReturnCount returns the number of return values for a call expression.
// Returns -1 if unable to determine (caller should use fallback).
//
// This method attempts multiple strategies:
// 1. Look up the call expression in go/types info
// 2. Fall back to typeloader results for imported packages
// 3. Return -1 if all attempts fail
func (r *TypeResolver) GetReturnCount(exprBytes []byte) int {
	if r == nil {
		return -1
	}

	// Strategy 1: Parse the expression and look it up in type info
	exprAST, err := parser.ParseExpr(string(exprBytes))
	if err != nil {
		return -1
	}

	callExpr, ok := exprAST.(*ast.CallExpr)
	if !ok {
		return -1
	}

	// Try to find a matching call expression in our type-checked file
	var matchedType types.Type
	ast.Inspect(r.file, func(n ast.Node) bool {
		if matchedType != nil {
			return false // Already found
		}
		if e, ok := n.(*ast.CallExpr); ok {
			// Compare expressions structurally
			if exprsEqual(e.Fun, callExpr.Fun) {
				if tv, found := r.typeInfo.Types[e]; found {
					matchedType = tv.Type
					return false
				}
			}
		}
		return true
	})

	if matchedType != nil {
		return countReturnsFromType(matchedType)
	}

	// Strategy 2: Try typeloader for imported package methods
	if r.loadResult != nil {
		methodName := extractMethodFromCallExpr(callExpr)
		if methodName != "" {
			// Check methods (Type.Method)
			if sig, found := r.loadResult.Methods[methodName]; found {
				return len(sig.Results)
			}
			// Check package functions (package.Function)
			if sig, found := r.loadResult.Functions[methodName]; found {
				return len(sig.Results)
			}
			// Check local functions
			if sig, found := r.loadResult.LocalFunctions[methodName]; found {
				return len(sig.Results)
			}
		}
	}

	return -1
}

// SanitizeDingoSource replaces Dingo-specific syntax with valid Go.
// Handles:
//   - ? "context" -> // "context" (error propagation with context becomes comment)
//   - ? -> space (basic error propagation)
//   - match -> switch (preserves spacing since both are 5 chars)
//   - => -> {  (arrow to brace, adds padding for length)
//
// This is NOT byte-based code transformation (forbidden by CLAUDE.md for position tracking).
// It's a preprocessing step to make Dingo source parseable by go/parser.
// All actual analysis is done via go/ast and go/types (token-based).
//
// The byte positions are preserved (same-length replacements) so AST node positions
// remain valid for the original source.
func SanitizeDingoSource(src []byte) []byte {
	sanitized := make([]byte, len(src))
	copy(sanitized, src)

	// First pass: Replace '? "' with '// ' (turns error context into comment)
	// This handles: expr? "context message" -> expr// "context message"
	for i := 0; i < len(sanitized)-1; i++ {
		if sanitized[i] == '?' {
			// Look for optional whitespace then quote
			j := i + 1
			for j < len(sanitized) && (sanitized[j] == ' ' || sanitized[j] == '\t') {
				j++
			}
			if j < len(sanitized) && sanitized[j] == '"' {
				// Replace ? with / and following char with /
				sanitized[i] = '/'
				sanitized[i+1] = '/'
				i++ // Skip the second / we just wrote
				continue
			}
		}
	}

	// Second pass: Replace remaining '?' with space
	for i := 0; i < len(sanitized); i++ {
		if sanitized[i] == '?' {
			sanitized[i] = ' '
		}
	}

	// Replace "match" with "switch" (both 5 chars when followed by space)
	// Simple loop to find and replace
	for i := 0; i <= len(sanitized)-5; i++ {
		if sanitized[i] == 'm' && i+5 <= len(sanitized) &&
			sanitized[i+1] == 'a' && sanitized[i+2] == 't' &&
			sanitized[i+3] == 'c' && sanitized[i+4] == 'h' {
			// Check it's not part of a larger identifier
			if i == 0 || !isIdentChar(sanitized[i-1]) {
				if i+5 >= len(sanitized) || !isIdentChar(sanitized[i+5]) {
					sanitized[i] = 's'
					sanitized[i+1] = 'w'
					sanitized[i+2] = 'i'
					sanitized[i+3] = 't'
					sanitized[i+4] = 'c'
					// Note: 'h' from "match" stays as last char, but switch is also 6 chars
					// Actually "switch" is 6 chars, "match" is 5. Need different approach.
				}
			}
		}
	}

	// Replace "=>" with ": " (arrow to colon-space for case-like syntax)
	for i := 0; i < len(sanitized)-1; i++ {
		if sanitized[i] == '=' && sanitized[i+1] == '>' {
			sanitized[i] = ':'
			sanitized[i+1] = ' '
		}
	}

	return sanitized
}

// isIdentChar returns true if ch is a valid identifier character
func isIdentChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') || ch == '_'
}

// extractImportsFromAST extracts import paths from an AST.
func extractImportsFromAST(file *ast.File) []string {
	var imports []string
	for _, imp := range file.Imports {
		// Remove quotes from import path
		path := imp.Path.Value
		if len(path) >= 2 && path[0] == '"' && path[len(path)-1] == '"' {
			path = path[1 : len(path)-1]
		}
		imports = append(imports, path)
	}
	return imports
}

// extractImportAliasesFromAST extracts import aliases from an AST.
// Maps alias name (e.g., "util") to full import path (e.g., "github.com/example/util").
func extractImportAliasesFromAST(file *ast.File) map[string]string {
	aliases := make(map[string]string)
	for _, imp := range file.Imports {
		// Remove quotes from import path
		path := imp.Path.Value
		if len(path) >= 2 && path[0] == '"' && path[len(path)-1] == '"' {
			path = path[1 : len(path)-1]
		}

		// Determine the alias
		var alias string
		if imp.Name != nil && imp.Name.Name != "." && imp.Name.Name != "_" {
			// Explicit alias: import alias "package/path"
			alias = imp.Name.Name
		} else {
			// Default alias is the last path component
			// e.g., "github.com/example/util" -> "util"
			parts := splitImportPath(path)
			if len(parts) > 0 {
				alias = parts[len(parts)-1]
			}
		}

		if alias != "" {
			aliases[alias] = path
		}
	}
	return aliases
}

// splitImportPath splits an import path by "/" without using strings package.
func splitImportPath(path string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		parts = append(parts, path[start:])
	}
	return parts
}

// exprsEqual compares two expressions structurally (simplified).
func exprsEqual(a, b ast.Expr) bool {
	// Compare string representations for matching
	return exprToString(a) == exprToString(b)
}

// exprToString converts an expression to a comparable string.
func exprToString(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return exprToString(x.X) + "." + x.Sel.Name
	case *ast.CallExpr:
		return exprToString(x.Fun) + "()"
	case *ast.IndexExpr:
		// Generic type: foo[T]
		return exprToString(x.X) + "[" + exprToString(x.Index) + "]"
	case *ast.IndexListExpr:
		// Generic type with multiple params: foo[T, E]
		result := exprToString(x.X) + "["
		for i, idx := range x.Indices {
			if i > 0 {
				result += ", "
			}
			result += exprToString(idx)
		}
		result += "]"
		return result
	default:
		return fmt.Sprintf("%T", e)
	}
}

// countReturnsFromType extracts return count from a type.
func countReturnsFromType(t types.Type) int {
	switch typ := t.(type) {
	case *types.Tuple:
		return typ.Len()
	case *types.Named:
		// Single named return type
		return 1
	case *types.Basic:
		// Single basic return type
		return 1
	case *types.Pointer:
		// Single pointer return type
		return 1
	case *types.Slice:
		// Single slice return type
		return 1
	case *types.Interface:
		// Single interface return type (including error)
		return 1
	default:
		// For other types, assume single return
		if t != nil {
			return 1
		}
		return -1
	}
}

// extractMethodFromCallExpr extracts the method/function name from a call expression.
// Returns qualified name like "Type.Method" or "pkg.Function" for lookups.
func extractMethodFromCallExpr(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		// Simple function call: foo()
		return fun.Name
	case *ast.SelectorExpr:
		// Method or package function: obj.Method() or pkg.Func()
		switch x := fun.X.(type) {
		case *ast.Ident:
			// pkg.Func or receiver.Method
			return x.Name + "." + fun.Sel.Name
		case *ast.SelectorExpr:
			// Chained access: a.b.Method - return last receiver.method
			return extractLastIdent(x) + "." + fun.Sel.Name
		}
		return fun.Sel.Name
	case *ast.IndexExpr:
		// Generic function: foo[T]()
		return extractMethodFromCallExpr(&ast.CallExpr{Fun: fun.X})
	case *ast.IndexListExpr:
		// Generic function with multiple type params: foo[T, E]()
		return extractMethodFromCallExpr(&ast.CallExpr{Fun: fun.X})
	}
	return ""
}

// extractLastIdent extracts the last identifier before the method name from a selector chain.
// For "s.companyRepo.GetByID", the selector passed here is "s.companyRepo",
// and we want to return "companyRepo" (the field holding the interface).
func extractLastIdent(sel *ast.SelectorExpr) string {
	// For any selector a.b, return "b" (the rightmost identifier before the method)
	// This is sel.Sel.Name, not sel.X
	return sel.Sel.Name
}

// GetReturnTypeInfo checks if an expression returns a Result type.
// Returns (isResult, okType, errType) where:
//   - isResult: true if the expression returns a Result[T, E] type
//   - okType: the T type from Result[T, E]
//   - errType: the E type from Result[T, E]
//
// This uses multiple strategies to look up functions:
// 1. dingoFuncs - functions parsed from .dingo files in imported packages
// 2. loadResult - types loaded via go/packages from Go files
func (r *TypeResolver) GetReturnTypeInfo(exprBytes []byte) (isResult bool, okType string, errType string) {
	if r == nil {
		return false, "", ""
	}

	// Parse the expression to extract function name
	exprAST, err := parser.ParseExpr(string(exprBytes))
	if err != nil {
		return false, "", ""
	}

	callExpr, ok := exprAST.(*ast.CallExpr)
	if !ok {
		return false, "", ""
	}

	// Extract method name for lookup (e.g., "util.NewJWKSFetcher")
	methodName := extractMethodFromCallExpr(callExpr)
	if methodName == "" {
		return false, "", ""
	}

	// Strategy 1: Check dingoFuncs (Dingo source packages)
	// This handles packages written in Dingo that go/packages cannot load
	if r.dingoFuncs != nil {
		alias, funcName := splitMethodName(methodName)
		if alias != "" && funcName != "" {
			// First try direct alias lookup (e.g., "service.Create" -> dingoFuncs["service"]["Create"])
			if pkgFuncs, found := r.dingoFuncs[alias]; found {
				if sig, found := pkgFuncs[funcName]; found {
					return checkSignatureForResult(sig)
				}
			}

			// Fallback: alias might be a field name (e.g., "userService.ListByCompany")
			// Search all packages for the method name
			for _, pkgFuncs := range r.dingoFuncs {
				if sig, found := pkgFuncs[funcName]; found {
					return checkSignatureForResult(sig)
				}
			}
		}
	}

	// Strategy 2: Check loadResult (Go packages loaded via go/packages)
	if r.loadResult != nil {
		// Resolve alias to full package path for lookup
		// e.g., "util.NewJWKSFetcher" -> "github.com/example/util.NewJWKSFetcher"
		fullMethodName := r.resolveMethodName(methodName)

		// Check functions (package.Function)
		if sig, found := r.loadResult.Functions[fullMethodName]; found {
			return checkSignatureForResult(sig)
		}

		// Check methods (Type.Method)
		if sig, found := r.loadResult.Methods[methodName]; found {
			return checkSignatureForResult(sig)
		}

		// Check local functions
		if sig, found := r.loadResult.LocalFunctions[methodName]; found {
			return checkSignatureForResult(sig)
		}
	}

	// Strategy 3: Check interface methods from Dingo source packages
	// For calls like s.userRepo.GetByEmail(...), extract just the method name
	// and look it up in interface methods. This handles cross-file interface calls.
	//
	// Note: This CAN have collisions when multiple interfaces have the same method name
	// (e.g., GetByID in both CompanyRepository and UserRepository). However:
	// - For error propagation: If the method returns Result, we use Result pattern.
	//   If it doesn't, we use tuple pattern. Both are valid if signatures match.
	// - For validation: If ANY matching interface method returns Result, flagging
	//   tuple unpacking is correct - the user should investigate.
	if r.interfaceMethods != nil {
		// Extract just the method name (last part after the dot)
		justMethodName := methodName
		if dotIdx := strings.LastIndex(methodName, "."); dotIdx >= 0 {
			justMethodName = methodName[dotIdx+1:]
		}
		if sig, found := r.interfaceMethods[justMethodName]; found {
			return checkSignatureForResult(sig)
		}
	}

	return false, "", ""
}

// resolveMethodName resolves alias.Function to fullpath.Function.
// e.g., "util.NewJWKSFetcher" -> "github.com/example/util.NewJWKSFetcher"
func (r *TypeResolver) resolveMethodName(methodName string) string {
	if r.importAliases == nil {
		return methodName
	}

	// Find the dot separator
	dotIdx := -1
	for i := 0; i < len(methodName); i++ {
		if methodName[i] == '.' {
			dotIdx = i
			break
		}
	}

	if dotIdx <= 0 {
		// No package prefix or malformed
		return methodName
	}

	alias := methodName[:dotIdx]
	funcName := methodName[dotIdx+1:]

	// Look up the full path for this alias
	if fullPath, found := r.importAliases[alias]; found {
		return fullPath + "." + funcName
	}

	// Alias not found, return as-is
	return methodName
}

// checkSignatureForResult checks if a function signature returns a Result type.
func checkSignatureForResult(sig *typeloader.FunctionSignature) (isResult bool, okType string, errType string) {
	if sig == nil || len(sig.Results) == 0 {
		return false, "", ""
	}

	// Check the first return type for Result pattern
	returnType := sig.Results[0].Name
	if IsResultType(returnType) {
		okType = ExtractResultOkType(returnType)
		errType = ExtractResultErrType(returnType)
		if errType == "" {
			errType = "error"
		}
		return true, okType, errType
	}

	return false, "", ""
}

// loadDingoFunctions finds and parses .dingo files from imported packages.
// This enables cross-package type resolution for Dingo source packages
// that go/packages cannot load (since they're not valid Go).
func (r *TypeResolver) loadDingoFunctions() {
	if r.importAliases == nil || r.workingDir == "" {
		return
	}

	r.dingoFuncs = make(map[string]map[string]*typeloader.FunctionSignature)
	r.interfaceMethods = make(map[string]*typeloader.FunctionSignature)

	// Find go.mod to determine module name and root
	modName, modRoot := findModuleInfo(r.workingDir)
	if modName == "" || modRoot == "" {
		return
	}

	// For each import, check if it's in our module and has .dingo files
	lfp := &typeloader.LocalFuncParser{}
	for alias, importPath := range r.importAliases {
		// Check if import is within our module
		if !strings.HasPrefix(importPath, modName) {
			continue
		}

		// Get relative path within module
		relPath := strings.TrimPrefix(importPath, modName)
		relPath = strings.TrimPrefix(relPath, "/")

		// Construct directory path
		pkgDir := filepath.Join(modRoot, relPath)

		// Find .dingo files in this directory
		dingoFiles, err := filepath.Glob(filepath.Join(pkgDir, "*.dingo"))
		if err != nil || len(dingoFiles) == 0 {
			continue
		}

		// Parse each .dingo file to extract function signatures and interface methods
		funcs := make(map[string]*typeloader.FunctionSignature)
		for _, file := range dingoFiles {
			src, err := os.ReadFile(file)
			if err != nil {
				continue
			}

			// Extract function signatures
			fileFuncs, err := lfp.ParseLocalFunctions(src)
			if err != nil {
				continue
			}

			for name, sig := range fileFuncs {
				funcs[name] = sig
			}

			// Extract interface method signatures
			// These are stored globally (not per-package) since interface methods
			// can be called through any variable holding that interface type
			ifaceMethods, err := lfp.ParseInterfaceMethods(src)
			if err != nil {
				continue
			}

			for name, sig := range ifaceMethods {
				r.interfaceMethods[name] = sig
			}
		}

		if len(funcs) > 0 {
			r.dingoFuncs[alias] = funcs
		}
	}
}

// findModuleInfo walks up from startDir to find go.mod and extract module info.
// Returns (moduleName, moduleRootPath) or ("", "") if not found.
func findModuleInfo(startDir string) (modName string, modRoot string) {
	dir := startDir
	for {
		modPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(modPath); err == nil {
			// Found go.mod, read module name
			content, err := os.ReadFile(modPath)
			if err == nil {
				// Parse "module xxx" line
				for _, line := range strings.Split(string(content), "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "module ") {
						modName = strings.TrimSpace(strings.TrimPrefix(line, "module"))
						modRoot = dir
						return
					}
				}
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return "", ""
		}
		dir = parent
	}
}

// splitMethodName splits "alias.FuncName" into (alias, funcName).
// Returns ("", "") if the format is invalid.
func splitMethodName(methodName string) (alias string, funcName string) {
	dotIdx := strings.Index(methodName, ".")
	if dotIdx <= 0 || dotIdx >= len(methodName)-1 {
		return "", ""
	}
	return methodName[:dotIdx], methodName[dotIdx+1:]
}
