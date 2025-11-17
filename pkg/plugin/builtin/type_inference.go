// Package builtin provides built-in Dingo transformation plugins
package builtin

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/token"
	"go/types"
	"reflect"
	"strings"
)

// TypeInferenceService provides centralized type information for all plugins.
//
// IMPORTANT: Cache Lifecycle and Invalidation
//
// The typeCache is keyed by ast.Expr pointer addresses. Since AST transformations
// using astutil.Apply() create NEW node instances, cache entries become stale and
// invalid after any AST mutation. To prevent returning incorrect types or panicking
// on missing entries:
//
//  1. Cache is ONLY valid within a single transformation pass
//  2. Call Refresh() after ANY AST modification to invalidate cache
//  3. The generation counter tracks cache freshness
//  4. Never reuse TypeInferenceService across multiple file transformations
//
// Example correct usage:
//
//	service := NewTypeInferenceService(fset, file, logger)
//	typ, _ := service.InferType(expr)  // Cache miss, type checked
//	typ2, _ := service.InferType(expr) // Cache hit, returns cached
//	// ... AST modified via astutil.Apply ...
//	service.Refresh(modifiedFile)     // Cache invalidated
//	typ3, _ := service.InferType(expr) // Cache miss again (new expr pointer)
//	service.Close()                    // Release resources
type TypeInferenceService struct {
	fset   *token.FileSet
	logger interface{} // plugin.Logger interface stored to avoid circular import

	// Type checker state
	info   *types.Info
	pkg    *types.Package
	config *types.Config

	// Performance cache (STALE after AST mutations - see doc comment above)
	typeCache    map[ast.Expr]types.Type
	cacheEnabled bool
	generation   int // Incremented on Refresh() to detect stale cache usage

	// Synthetic type registry (for Result, Option, etc)
	syntheticTypes map[string]*SyntheticTypeInfo

	// Error collection (populated during type checking)
	errors []error

	// Statistics (for performance monitoring)
	typeChecks int
	cacheHits  int
}

// SyntheticTypeInfo stores information about generated types
type SyntheticTypeInfo struct {
	TypeName   string        // e.g., "Result_int_error"
	Underlying *types.Named  // Type information
	GenDecl    *ast.GenDecl  // Generated AST declaration
}

// TypeInferenceStats contains performance statistics
type TypeInferenceStats struct {
	TypeChecks int
	CacheHits  int
	CacheSize  int
}

// TypeInference is a legacy alias for backward compatibility
type TypeInference = TypeInferenceService

// NewTypeInference creates a new type inference engine for a file
func NewTypeInference(fset *token.FileSet, file *ast.File) (*TypeInference, error) {
	return NewTypeInferenceService(fset, file, nil)
}

// NewTypeInferenceService creates a new type inference service for a file
func NewTypeInferenceService(fset *token.FileSet, file *ast.File, logger interface{}) (*TypeInferenceService, error) {
	// Initialize type information maps
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:     make(map[ast.Node]*types.Scope),
	}

	service := &TypeInferenceService{
		fset:           fset,
		logger:         logger,
		typeCache:      make(map[ast.Expr]types.Type),
		cacheEnabled:   true,
		syntheticTypes: make(map[string]*SyntheticTypeInfo),
		errors:         make([]error, 0),
		typeChecks:     0,
		cacheHits:      0,
	}

	// CRITICAL FIX #5: Configure type checker with error collection, not silent swallowing
	config := &types.Config{
		Importer: importer.Default(),
		Error: func(err error) {
			// Collect errors for later inspection
			service.errors = append(service.errors, err)

			// Log errors at Warn level if logger available
			if logger != nil {
				// Use reflection to call Warn method to avoid import cycle
				if loggerVal := reflect.ValueOf(logger); loggerVal.IsValid() {
					if warnMethod := loggerVal.MethodByName("Warn"); warnMethod.IsValid() {
						warnMethod.Call([]reflect.Value{
							reflect.ValueOf("Type checking error: %v"),
							reflect.ValueOf(err),
						})
					}
				}
			}
		},
	}

	// Extract package name from the file
	packageName := "main"
	if file.Name != nil {
		packageName = file.Name.Name
	}

	// Type-check the file
	pkg, err := config.Check(packageName, fset, []*ast.File{file}, info)
	if err != nil {
		// Continue even with type errors - partial type information is better than none
		// Errors are already collected via config.Error callback
	}

	service.info = info
	service.pkg = pkg
	service.config = config

	return service, nil
}

// Refresh refreshes type information after AST modifications.
//
// CRITICAL: Must be called after any AST transformation that creates new nodes.
// This completely invalidates the typeCache since AST node pointers have changed.
// The generation counter is incremented to help detect stale cache usage patterns.
func (ti *TypeInferenceService) Refresh(file *ast.File) error {
	// CRITICAL FIX #1: Clear ALL cached data since AST nodes have new addresses
	ti.typeCache = make(map[ast.Expr]types.Type)
	ti.errors = make([]error, 0)
	ti.generation++ // Increment to track cache invalidation cycles

	// Reset statistics for this generation
	ti.typeChecks = 0
	ti.cacheHits = 0

	// Re-run type checking
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:     make(map[ast.Node]*types.Scope),
	}

	packageName := "main"
	if file.Name != nil {
		packageName = file.Name.Name
	}

	pkg, err := ti.config.Check(packageName, ti.fset, []*ast.File{file}, info)
	if err != nil {
		// Continue even with type errors - they're collected in ti.errors
	}

	ti.info = info
	ti.pkg = pkg

	return nil
}

// Close releases memory used by type inference maps
func (ti *TypeInferenceService) Close() error {
	// CRITICAL FIX #5: Clear all fields including errors
	if ti.info != nil {
		ti.info.Types = nil
		ti.info.Defs = nil
		ti.info.Uses = nil
		ti.info.Implicits = nil
		ti.info.Selections = nil
		ti.info.Scopes = nil
		ti.info = nil
	}
	ti.typeCache = nil
	ti.syntheticTypes = nil
	ti.errors = nil
	ti.pkg = nil
	ti.config = nil
	return nil
}

// InferFunctionReturnType determines the return type of a function
// Returns the type of the first return value (before the error)
func (ti *TypeInference) InferFunctionReturnType(fn *ast.FuncDecl) (types.Type, error) {
	if fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
		return nil, fmt.Errorf("function has no return values")
	}

	// Get the first return value type
	firstReturn := fn.Type.Results.List[0]
	typ := ti.info.TypeOf(firstReturn.Type)
	if typ == nil {
		return nil, fmt.Errorf("could not determine return type")
	}

	return typ, nil
}

// InferExpressionType determines the type of an expression
func (ti *TypeInference) InferExpressionType(expr ast.Expr) (types.Type, error) {
	typ := ti.info.TypeOf(expr)
	if typ == nil {
		return nil, fmt.Errorf("could not determine expression type")
	}
	return typ, nil
}

// GenerateZeroValue creates an AST expression representing the zero value for a type
func (ti *TypeInference) GenerateZeroValue(typ types.Type) ast.Expr {
	if typ == nil {
		return &ast.Ident{Name: "nil"}
	}

	switch t := typ.(type) {
	case *types.Basic:
		return ti.basicZeroValue(t)
	case *types.Pointer:
		return &ast.Ident{Name: "nil"}
	case *types.Slice:
		return &ast.Ident{Name: "nil"}
	case *types.Map:
		return &ast.Ident{Name: "nil"}
	case *types.Chan:
		return &ast.Ident{Name: "nil"}
	case *types.Interface:
		return &ast.Ident{Name: "nil"}
	case *types.Struct:
		// Generate composite literal: TypeName{}
		return &ast.CompositeLit{
			Type: ti.typeToAST(t),
		}
	case *types.Array:
		// Generate composite literal: [N]Type{}
		return &ast.CompositeLit{
			Type: ti.typeToAST(t),
		}
	case *types.Named:
		// For named types, check the underlying type
		underlying := t.Underlying()
		switch underlying.(type) {
		case *types.Struct:
			// For named structs, use TypeName{}
			return &ast.CompositeLit{
				Type: &ast.Ident{Name: t.Obj().Name()},
			}
		case *types.Basic, *types.Pointer, *types.Slice, *types.Map, *types.Interface:
			// Use underlying type's zero value
			return ti.GenerateZeroValue(underlying)
		default:
			// Fallback: composite literal with type name
			return &ast.CompositeLit{
				Type: &ast.Ident{Name: t.Obj().Name()},
			}
		}
	default:
		// Fallback: use zero value initialization with new
		return &ast.StarExpr{
			X: &ast.CallExpr{
				Fun: &ast.Ident{Name: "new"},
				Args: []ast.Expr{
					ti.typeToAST(typ),
				},
			},
		}
	}
}

// basicZeroValue generates zero value for basic Go types
func (ti *TypeInference) basicZeroValue(basic *types.Basic) ast.Expr {
	switch basic.Kind() {
	case types.Bool:
		return &ast.Ident{Name: "false"}
	case types.Int, types.Int8, types.Int16, types.Int32, types.Int64:
		return &ast.BasicLit{Kind: token.INT, Value: "0"}
	case types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64, types.Uintptr:
		return &ast.BasicLit{Kind: token.INT, Value: "0"}
	case types.Float32, types.Float64:
		return &ast.BasicLit{Kind: token.FLOAT, Value: "0.0"}
	case types.Complex64, types.Complex128:
		return &ast.BasicLit{Kind: token.IMAG, Value: "0i"}
	case types.String:
		return &ast.BasicLit{Kind: token.STRING, Value: `""`}
	case types.UnsafePointer:
		return &ast.Ident{Name: "nil"}
	default:
		// UntypedNil, etc
		return &ast.Ident{Name: "nil"}
	}
}

// typeToAST converts a types.Type to an AST type expression
// This is needed for composite literals and type conversions
func (ti *TypeInference) typeToAST(typ types.Type) ast.Expr {
	switch t := typ.(type) {
	case *types.Basic:
		return &ast.Ident{Name: t.Name()}
	case *types.Pointer:
		return &ast.StarExpr{
			X: ti.typeToAST(t.Elem()),
		}
	case *types.Slice:
		return &ast.ArrayType{
			Elt: ti.typeToAST(t.Elem()),
		}
	case *types.Array:
		return &ast.ArrayType{
			Len: &ast.BasicLit{
				Kind:  token.INT,
				Value: fmt.Sprintf("%d", t.Len()),
			},
			Elt: ti.typeToAST(t.Elem()),
		}
	case *types.Map:
		return &ast.MapType{
			Key:   ti.typeToAST(t.Key()),
			Value: ti.typeToAST(t.Elem()),
		}
	case *types.Chan:
		return &ast.ChanType{
			Dir:   ti.chanDirToAST(t.Dir()),
			Value: ti.typeToAST(t.Elem()),
		}
	case *types.Struct:
		// For anonymous structs, we'd need to reconstruct the full type
		// For now, return placeholder
		return &ast.Ident{Name: "struct{}"}
	case *types.Interface:
		if t.Empty() {
			return &ast.InterfaceType{
				Methods: &ast.FieldList{},
			}
		}
		// For non-empty interfaces, use placeholder
		return &ast.Ident{Name: "interface{}"}
	case *types.Named:
		// Get the type name
		obj := t.Obj()
		if obj != nil {
			// Check if it's from the same package or imported
			if obj.Pkg() == nil || obj.Pkg() == ti.pkg {
				// Same package - use simple name
				return &ast.Ident{Name: obj.Name()}
			} else {
				// Different package - use selector
				return &ast.SelectorExpr{
					X:   &ast.Ident{Name: obj.Pkg().Name()},
					Sel: &ast.Ident{Name: obj.Name()},
				}
			}
		}
		return &ast.Ident{Name: "unknown"}
	default:
		return &ast.Ident{Name: "interface{}"}
	}
}

// chanDirToAST converts channel direction to AST representation
func (ti *TypeInference) chanDirToAST(dir types.ChanDir) ast.ChanDir {
	switch dir {
	case types.SendRecv:
		return ast.SEND | ast.RECV
	case types.SendOnly:
		return ast.SEND
	case types.RecvOnly:
		return ast.RECV
	default:
		return ast.SEND | ast.RECV
	}
}

// InferType determines the type of an expression with caching.
//
// CRITICAL FIX #3: Returns error if service is not healthy (closed or uninitialized).
func (ti *TypeInferenceService) InferType(expr ast.Expr) (types.Type, error) {
	// CRITICAL FIX #3: Prevent panics after Close()
	if !ti.IsHealthy() {
		return nil, fmt.Errorf("type inference service is not healthy (closed or uninitialized)")
	}

	// Check cache first
	if ti.cacheEnabled && ti.typeCache != nil {
		if cached, ok := ti.typeCache[expr]; ok {
			ti.cacheHits++
			return cached, nil
		}
	}

	ti.typeChecks++

	// Use existing InferExpressionType logic
	typ := ti.info.TypeOf(expr)
	if typ == nil {
		return nil, fmt.Errorf("could not determine expression type")
	}

	// Cache the result
	if ti.cacheEnabled && ti.typeCache != nil {
		ti.typeCache[expr] = typ
	}

	return typ, nil
}

// IsResultType detects Result<T, E> types and extracts T and E.
//
// CRITICAL FIX #3: Safe to call after Close() - returns false if not healthy.
func (ti *TypeInferenceService) IsResultType(typ types.Type) (T, E types.Type, ok bool) {
	// CRITICAL FIX #3: Safe degradation if closed
	if !ti.IsHealthy() {
		return nil, nil, false
	}

	// Check if type is a named type matching Result_* pattern
	named, isNamed := typ.(*types.Named)
	if !isNamed {
		return nil, nil, false
	}

	name := named.Obj().Name()

	// Check for Result_T_E naming pattern
	if !strings.HasPrefix(name, "Result_") {
		return nil, nil, false
	}

	// Check if it's in synthetic type registry (more reliable than name parsing)
	if info, found := ti.GetSyntheticType(name); found {
		// Extract T and E from underlying struct
		// Result<T, E> has fields: tag ResultTag, ok_0 T, err_0 E
		structType, ok := info.Underlying.Underlying().(*types.Struct)
		if !ok {
			return nil, nil, false
		}

		if structType.NumFields() != 3 {
			return nil, nil, false
		}

		// Fields: tag (0), ok_0 (1), err_0 (2)
		T = structType.Field(1).Type()
		E = structType.Field(2).Type()
		return T, E, true
	}

	// Fallback: Not in registry, cannot reliably determine T and E
	return nil, nil, false
}

// IsOptionType detects Option<T> types and extracts T.
//
// CRITICAL FIX #3: Safe to call after Close() - returns false if not healthy.
func (ti *TypeInferenceService) IsOptionType(typ types.Type) (T types.Type, ok bool) {
	// CRITICAL FIX #3: Safe degradation if closed
	if !ti.IsHealthy() {
		return nil, false
	}

	named, isNamed := typ.(*types.Named)
	if !isNamed {
		return nil, false
	}

	name := named.Obj().Name()
	if !strings.HasPrefix(name, "Option_") {
		return nil, false
	}

	// Check synthetic registry
	if info, found := ti.GetSyntheticType(name); found {
		// Option<T> has fields: tag OptionTag, some_0 T
		structType, ok := info.Underlying.Underlying().(*types.Struct)
		if !ok {
			return nil, false
		}

		if structType.NumFields() != 2 {
			return nil, false
		}

		T = structType.Field(1).Type()
		return T, true
	}

	return nil, false
}

// IsPointerType checks if a type is a pointer
func (ti *TypeInferenceService) IsPointerType(typ types.Type) bool {
	_, ok := typ.(*types.Pointer)
	return ok
}

// IsErrorType checks if a type is or implements the error interface
func (ti *TypeInferenceService) IsErrorType(typ types.Type) bool {
	// Check if type is built-in error interface
	if typ.String() == "error" {
		return true
	}

	// Check if implements error interface
	iface, ok := typ.Underlying().(*types.Interface)
	if !ok {
		return false
	}

	// error interface has one method: Error() string
	if iface.NumMethods() == 1 {
		method := iface.Method(0)
		if method.Name() == "Error" {
			sig, ok := method.Type().(*types.Signature)
			if ok && sig.Params().Len() == 0 && sig.Results().Len() == 1 {
				return sig.Results().At(0).Type().String() == "string"
			}
		}
	}

	return false
}

// IsGoErrorTuple detects Go functions returning (T, error)
func (ti *TypeInferenceService) IsGoErrorTuple(sig *types.Signature) (valueType types.Type, ok bool) {
	results := sig.Results()
	if results.Len() != 2 {
		return nil, false
	}

	// Second return must be error type
	secondType := results.At(1).Type()
	if !ti.IsErrorType(secondType) {
		return nil, false
	}

	// First return is the value type
	return results.At(0).Type(), true
}

// ShouldWrapAsResult determines if a call expression should be auto-wrapped
func (ti *TypeInferenceService) ShouldWrapAsResult(callExpr *ast.CallExpr) bool {
	// Get function type
	funcType, err := ti.InferType(callExpr.Fun)
	if err != nil {
		return false
	}

	sig, ok := funcType.(*types.Signature)
	if !ok {
		return false
	}

	_, isErrorTuple := ti.IsGoErrorTuple(sig)
	return isErrorTuple
}

// RegisterSyntheticType registers a generated type (Result, Option, etc)
func (ti *TypeInferenceService) RegisterSyntheticType(name string, info *SyntheticTypeInfo) {
	ti.syntheticTypes[name] = info
}

// GetSyntheticType retrieves information about a synthetic type
func (ti *TypeInferenceService) GetSyntheticType(name string) (*SyntheticTypeInfo, bool) {
	info, ok := ti.syntheticTypes[name]
	return info, ok
}

// IsSyntheticType checks if a type name is a registered synthetic type
func (ti *TypeInferenceService) IsSyntheticType(name string) bool {
	_, ok := ti.syntheticTypes[name]
	return ok
}

// Stats returns performance statistics
func (ti *TypeInferenceService) Stats() TypeInferenceStats {
	return TypeInferenceStats{
		TypeChecks: ti.typeChecks,
		CacheHits:  ti.cacheHits,
		CacheSize:  len(ti.typeCache),
	}
}

// Error Inspection Methods

// HasErrors returns true if any type checking errors were collected.
// This indicates the service may be in a degraded state with incomplete type information.
func (ti *TypeInferenceService) HasErrors() bool {
	return len(ti.errors) > 0
}

// GetErrors returns all collected type checking errors.
// Errors are collected during type checking via the config.Error callback.
// Common causes: incomplete imports, undefined types, syntax errors in parsed code.
func (ti *TypeInferenceService) GetErrors() []error {
	return ti.errors
}

// IsHealthy returns true if the service is in a usable state.
// A service is healthy if:
//  1. It has been properly initialized (info != nil)
//  2. It has not been closed (info != nil, typeCache != nil)
//  3. Optionally: It has no type checking errors (if strictMode would be enabled)
//
// Currently returns true if initialized and not closed, even with type errors.
// Type errors are common in editor environments with incomplete code.
func (ti *TypeInferenceService) IsHealthy() bool {
	// Check if closed
	if ti.info == nil || ti.typeCache == nil {
		return false
	}

	// Service is usable even with type errors (degraded mode)
	// Plugins should check HasErrors() if they need perfect type info
	return true
}

// ClearErrors clears collected errors.
// Rarely needed - errors are automatically cleared on Refresh().
func (ti *TypeInferenceService) ClearErrors() {
	ti.errors = make([]error, 0)
}
