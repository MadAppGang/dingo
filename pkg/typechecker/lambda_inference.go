// Package typechecker provides lambda type inference from call context.
// The LambdaTypeInferrer rewrites function literal parameter and return types
// from 'any' placeholders to actual types based on the expected function type.
package typechecker

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/MadAppGang/dingo/pkg/config"
)

// untypedToTypedName converts untyped basic type kinds to their typed equivalents.
// Returns empty string for already-typed basic types (caller should use typ.Name()).
func untypedToTypedName(kind types.BasicKind) string {
	switch kind {
	case types.UntypedBool:
		return "bool"
	case types.UntypedInt:
		return "int"
	case types.UntypedRune:
		return "rune"
	case types.UntypedFloat:
		return "float64"
	case types.UntypedComplex:
		return "complex128"
	case types.UntypedString:
		return "string"
	case types.UntypedNil:
		// nil is not a type - return empty to skip inference
		return ""
	default:
		return "" // not an untyped kind, use typ.Name()
	}
}

// sourceImportCache caches packages loaded via source importer.
// This prevents repeated expensive source parsing when the GC importer
// fails to fully populate package scopes.
var sourceImportCache = make(map[string]*types.Package)

// LambdaTypeInferrer rewrites function literal parameter and return types
// based on the expected function type from call context.
//
// It walks the AST looking for CallExpr nodes with FuncLit arguments,
// then uses go/types to look up the expected function signature and
// rewrites the FuncLit types accordingly.
//
// Uses a four-layer inference approach:
// Layer 1: Local inference via go/types (existing)
// Layer 2: dgo signature registry (hardcoded dgo.* functions)
// Layer 3: Generic unification (third-party generic functions)
// Layer 4: gopls fallback (optional, configurable via dingo.toml)
type LambdaTypeInferrer struct {
	fset        *token.FileSet
	info        *types.Info
	file        *ast.File
	changed     bool
	config      *config.TypeInferenceConfig
	goplsClient *GoplsClient // Lazy-initialized when needed
}

// NewLambdaTypeInferrer creates a new inferrer.
func NewLambdaTypeInferrer(fset *token.FileSet, file *ast.File, info *types.Info) *LambdaTypeInferrer {
	return NewLambdaTypeInferrerWithConfig(fset, file, info, nil)
}

// NewLambdaTypeInferrerWithConfig creates a new inferrer with custom configuration.
// If cfg is nil, uses default configuration (gopls disabled).
func NewLambdaTypeInferrerWithConfig(fset *token.FileSet, file *ast.File, info *types.Info, cfg *config.TypeInferenceConfig) *LambdaTypeInferrer {
	if cfg == nil {
		cfg = &config.TypeInferenceConfig{
			GoplsEnabled: false,
			GoplsTimeout: "5s",
			GoplsPath:    "",
		}
	}
	return &LambdaTypeInferrer{
		fset:   fset,
		info:   info,
		file:   file,
		config: cfg,
	}
}

// Infer walks the AST and rewrites lambda types from call context.
// Returns true if any changes were made.
func (inf *LambdaTypeInferrer) Infer() bool {
	inf.changed = false

	// Fast path: check if there are any function literals in the file
	// If not, skip all the expensive type resolution work
	if !inf.hasAnyFuncLiterals() {
		return false
	}

	// Layer 1-4: Infer types from call context
	ast.Inspect(inf.file, inf.visit)
	// Layer 5: Infer return types for standalone lambdas with typed parameters
	inf.inferStandaloneLambdas()
	return inf.changed
}

// hasAnyFuncLiterals does a fast check to see if the file has any function literals.
// This is an O(n) scan that avoids expensive type resolution for files without lambdas.
func (inf *LambdaTypeInferrer) hasAnyFuncLiterals() bool {
	found := false
	ast.Inspect(inf.file, func(n ast.Node) bool {
		if found {
			return false // Already found one, stop
		}
		if _, ok := n.(*ast.FuncLit); ok {
			found = true
			return false
		}
		return true
	})
	return found
}

// visit is the AST visitor that looks for call expressions with function literal arguments.
func (inf *LambdaTypeInferrer) visit(n ast.Node) bool {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return true
	}

	// Get the function signature - try instantiated version first (for generics)
	// This uses Info.Instances which has already-resolved generic type arguments
	funcType := inf.getInstantiatedSignature(call)

	// Fallback to getting type from the function definition
	if funcType == nil {
		funcType = inf.getFunctionType(call.Fun)
	}

	if funcType == nil {
		return true
	}

	// Match each argument to expected parameter type
	params := funcType.Params()
	for i, arg := range call.Args {
		funcLit, ok := arg.(*ast.FuncLit)
		if !ok {
			continue
		}

		// Get expected function type for this parameter position
		if i >= params.Len() {
			// Variadic parameter - skip for now
			// TODO: Handle variadic parameters in future
			continue
		}

		expectedType := params.At(i).Type()
		if expectedType == nil {
			continue
		}

		underlying := expectedType.Underlying()
		if underlying == nil {
			continue
		}

		expectedSig, ok := underlying.(*types.Signature)
		if !ok {
			// Parameter is not a function type
			continue
		}

		// Rewrite the function literal's types
		if inf.rewriteFuncLit(funcLit, expectedSig) {
			inf.changed = true
		}
	}

	return true
}

// getFunctionType extracts the function signature from a call expression's Fun field.
// Handles direct function calls, method calls, and selector expressions.
func (inf *LambdaTypeInferrer) getFunctionType(fun ast.Expr) *types.Signature {
	// First, try to get the instantiated type from Types map
	// This handles both explicit instantiation Map[T, R](...) and inferred instantiation Map(...)
	if tv, ok := inf.info.Types[fun]; ok {
		if sig, ok := tv.Type.(*types.Signature); ok {
			return sig
		}
	}

	switch fn := fun.(type) {
	case *ast.Ident:
		// Direct function call: Func1(...)
		if obj := inf.info.Uses[fn]; obj != nil {
			if sig, ok := obj.Type().(*types.Signature); ok {
				return sig
			}
		}

	case *ast.SelectorExpr:
		// Method call or package-qualified function: obj.Method(...) or pkg.Func(...)
		// First try as method selection
		if sel := inf.info.Selections[fn]; sel != nil {
			if sig, ok := sel.Type().(*types.Signature); ok {
				return sig
			}
		}
		// Then try as package-qualified function
		if obj := inf.info.Uses[fn.Sel]; obj != nil {
			if sig, ok := obj.Type().(*types.Signature); ok {
				return sig
			}
		}

	case *ast.IndexExpr, *ast.IndexListExpr:
		// Generic function instantiation with explicit type params already handled above
		// This case is kept for clarity but will be caught by Types map check
		break
	}

	return nil
}

// getInstantiatedSignature extracts the instantiated signature for a generic function call.
// Uses a four-layer approach:
// Layer 1: Local inference via go/types (existing behavior)
// Layer 2: dgo signature registry (hardcoded signatures for dgo.* functions)
// Layer 3: Generic unification (structural type matching for third-party generics)
// Layer 4: gopls fallback (optional, only if configured)
//
// Returns error if all layers fail (strict mode - no fallback to func(any) any).
func (inf *LambdaTypeInferrer) getInstantiatedSignature(call *ast.CallExpr) *types.Signature {
	// Layer 1: Try go/types local inference
	if sig := inf.tryLayer1GoTypes(call); sig != nil {
		return sig
	}

	// Layer 2: Try dgo signature registry
	if sig := inf.tryLayer2DgoRegistry(call); sig != nil {
		return sig
	}

	// Layer 3: Try generic unification
	if sig := inf.tryLayer3GenericUnification(call); sig != nil {
		return sig
	}

	// Layer 4: Try gopls fallback (only if enabled)
	if inf.config.GoplsEnabled {
		if sig := inf.tryLayer4GoplsFallback(call); sig != nil {
			return sig
		}
	}

	// All layers failed - return nil to indicate failure
	// The caller should handle this by reporting an error
	return nil
}

// tryLayer1GoTypes attempts local inference via go/types.
// This is the existing behavior - checks Info.Instances and manual type param resolution.
func (inf *LambdaTypeInferrer) tryLayer1GoTypes(call *ast.CallExpr) *types.Signature {
	// Extract the function identifier from various call forms
	var id *ast.Ident
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		// Direct call: Filter(...)
		id = fn
	case *ast.SelectorExpr:
		// Package-qualified or method call: pkg.Filter(...) or obj.Method(...)
		id = fn.Sel
	case *ast.IndexExpr:
		// Explicit single type arg: Filter[User](...)
		switch x := fn.X.(type) {
		case *ast.Ident:
			id = x
		case *ast.SelectorExpr:
			id = x.Sel
		}
	case *ast.IndexListExpr:
		// Explicit multiple type args: Map[User, string](...)
		switch x := fn.X.(type) {
		case *ast.Ident:
			id = x
		case *ast.SelectorExpr:
			id = x.Sel
		}
	}

	if id == nil {
		return nil
	}

	// Check if any lambda arguments have typed parameters (not 'any')
	// If so, prefer manual resolution to leverage body inference
	hasTypedLambda := false
	for _, arg := range call.Args {
		if funcLit, ok := arg.(*ast.FuncLit); ok {
			if funcLit.Type.Params != nil {
				for _, field := range funcLit.Type.Params.List {
					if !inf.isAnyType(field.Type) {
						hasTypedLambda = true
						break
					}
				}
			}
			if hasTypedLambda {
				break
			}
		}
	}

	// If we have typed lambdas, try manual resolution first (for body inference)
	if hasTypedLambda {
		genericSig := inf.getGenericSignature(call.Fun)
		if genericSig != nil && genericSig.TypeParams() != nil && genericSig.TypeParams().Len() > 0 {
			typeArgs := inf.resolveTypeParamsFromArgs(call, genericSig)
			if len(typeArgs) > 0 {
				instantiated := inf.instantiateSignature(genericSig, typeArgs)
				if instantiated != nil {
					return instantiated
				}
			}
		}
	}

	// Fallback to go/types' inference from Info.Instances
	if instance, ok := inf.info.Instances[id]; ok {
		if sig, ok := instance.Type.(*types.Signature); ok {
			return sig
		}
	}

	// Fallback: try Info.Types[call.Fun]
	if tv, ok := inf.info.Types[call.Fun]; ok {
		if sig, ok := tv.Type.(*types.Signature); ok {
			// Only return if it's already instantiated (no type parameters)
			if sig.TypeParams() == nil || sig.TypeParams().Len() == 0 {
				return sig
			}
			// Signature has type parameters - fall through to manual resolution
		}
	}

	// Manual type parameter resolution (for cases without typed lambdas)
	genericSig := inf.getGenericSignature(call.Fun)
	if genericSig != nil && genericSig.TypeParams() != nil && genericSig.TypeParams().Len() > 0 {
		typeArgs := inf.resolveTypeParamsFromArgs(call, genericSig)
		if len(typeArgs) > 0 {
			instantiated := inf.instantiateSignature(genericSig, typeArgs)
			return instantiated
		}
	}

	return nil
}

// tryLayer2DgoRegistry attempts inference using hardcoded dgo function signatures.
// This handles the common case of dgo.Map/Filter/etc. without external dependencies.
func (inf *LambdaTypeInferrer) tryLayer2DgoRegistry(call *ast.CallExpr) *types.Signature {
	// Check if this is a dgo.* call
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok || pkgIdent.Name != "dgo" {
		return nil
	}

	// Look up synthetic signature from registry
	genericSig := GetDgoSignature(sel.Sel.Name)
	if genericSig == nil {
		return nil
	}

	// Reuse the multi-pass resolver here instead of the standalone unifier.
	// In addition to binding input types from non-lambda arguments, it can bind
	// result type parameters from a lambda body after an earlier pass has typed
	// the lambda parameters. This is required for dependent calls such as:
	//
	//   groups := dgo.GroupBy(items, |item| item.Category)
	//   for _, group := range groups {
	//       names := dgo.Map(group, |item| item.Name)
	//   }
	//
	// On the first pass unresolved result parameters intentionally become any,
	// which still gives go/types enough structure to type the range variable.
	// A later pass then replaces them with the concrete lambda result type.
	typeArgs := inf.resolveTypeParamsFromArgs(call, genericSig)
	if len(typeArgs) == 0 {
		return nil
	}

	return inf.instantiateSignature(genericSig, typeArgs)
}

// tryLayer3GenericUnification attempts inference using structural type matching.
// This handles third-party generic functions (not just dgo.*).
func (inf *LambdaTypeInferrer) tryLayer3GenericUnification(call *ast.CallExpr) *types.Signature {
	// Get the generic signature from the function definition
	genericSig := inf.getGenericSignature(call.Fun)
	if genericSig == nil || genericSig.TypeParams() == nil {
		return nil
	}

	// Use unifier to extract type bindings from non-lambda arguments
	unifier := NewTypeUnifier(inf.fset, inf.info)
	bindings := unifier.InferTypeParams(call, genericSig)
	if len(bindings) == 0 {
		return nil
	}

	// Instantiate signature with resolved types
	return unifier.InstantiateSignature(genericSig, bindings)
}

// tryLayer4GoplsFallback attempts inference using gopls subprocess.
// Only called if config.GoplsEnabled is true.
func (inf *LambdaTypeInferrer) tryLayer4GoplsFallback(call *ast.CallExpr) *types.Signature {
	// Lazy-initialize gopls client
	if inf.goplsClient == nil {
		client, err := NewGoplsClient(inf.config)
		if err != nil {
			// gopls initialization failed - log and skip this layer
			// (In production, we might want to log this properly)
			return nil
		}
		inf.goplsClient = client
	}

	// Query gopls for the type at this position
	pos := inf.fset.Position(call.Pos())
	typeInfo, err := inf.goplsClient.QueryType(pos.Filename, pos.Line, pos.Column)
	if err != nil {
		return nil
	}

	// Parse the type info string into a types.Signature
	// This is a simplified implementation - production version would need
	// more robust parsing of gopls output
	_ = typeInfo // TODO: Parse gopls response into types.Signature

	return nil
}

// LambdaInferenceError represents a failure to infer lambda types after trying all layers.
type LambdaInferenceError struct {
	Pos     token.Position
	Call    string
	Message string
	Hint    string
}

func (e *LambdaInferenceError) Error() string {
	return fmt.Sprintf("%s: %s in call %s\n  hint: %s",
		e.Pos, e.Message, e.Call, e.Hint)
}

// formatCall converts a call expression to a string for error messages.
func (inf *LambdaTypeInferrer) formatCall(call *ast.CallExpr) string {
	// Simple formatting - just extract the function name
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name + "(...)"
	case *ast.SelectorExpr:
		if sel, ok := fn.X.(*ast.Ident); ok {
			return sel.Name + "." + fn.Sel.Name + "(...)"
		}
		return fn.Sel.Name + "(...)"
	default:
		return "function(...)"
	}
}

// rewriteFuncLit updates a function literal's parameter and return types
// to match the expected signature.
// Returns true if any changes were made.
func (inf *LambdaTypeInferrer) rewriteFuncLit(fn *ast.FuncLit, expected *types.Signature) bool {
	changed := false

	// Rewrite parameters
	if inf.rewriteParams(fn, expected) {
		changed = true
	}

	// Rewrite return type
	if inf.rewriteResults(fn, expected) {
		changed = true
	}

	return changed
}

// rewriteParams updates function literal parameter types.
// It rebuilds the FieldList with fresh position info to avoid go/printer
// adding trailing commas from stale position data.
func (inf *LambdaTypeInferrer) rewriteParams(fn *ast.FuncLit, expected *types.Signature) bool {
	if fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
		return false
	}

	changed := false
	expectedParams := expected.Params()
	if expectedParams == nil {
		return false
	}

	paramIdx := 0
	newFields := make([]*ast.Field, len(fn.Type.Params.List))

	for i, field := range fn.Type.Params.List {
		// Handle multiple names in single field: (a, b int)
		numNames := len(field.Names)
		if numNames == 0 {
			numNames = 1
		}

		// Only rewrite if current type is 'any' (placeholder)
		if inf.isAnyType(field.Type) {
			// All names in this field share the same type
			if paramIdx < expectedParams.Len() {
				expectedType := expectedParams.At(paramIdx).Type()
				newTypeExpr := inf.typeToExpr(expectedType)
				if newTypeExpr != nil {
					// An unresolved synthetic type parameter is represented as any.
					// Replacing the existing placeholder with another any node is not
					// progress and would keep the multi-pass loop running forever.
					if inf.isAnyType(newTypeExpr) {
						newFields[i] = field
						paramIdx += numNames
						continue
					}

					// Create fresh Names with no position info to avoid
					// go/printer trailing comma issues from stale position data
					freshNames := make([]*ast.Ident, len(field.Names))
					for j, name := range field.Names {
						freshNames[j] = &ast.Ident{
							NamePos: token.NoPos,
							Name:    name.Name,
						}
					}

					// Create a fresh Field with no position info
					newFields[i] = &ast.Field{
						Names: freshNames,
						Type:  newTypeExpr,
						// Explicitly zero all position fields
						Doc:     nil,
						Tag:     nil,
						Comment: nil,
					}
					changed = true
					paramIdx += numNames
					continue
				}
			}
		}

		// Keep original field (unchanged)
		newFields[i] = field
		paramIdx += numNames
	}

	if changed {
		// Replace the FieldList with a fresh one to clear stale position info
		// Explicitly set Opening and Closing to token.NoPos to prevent go/printer
		// from adding trailing commas
		fn.Type.Params = &ast.FieldList{
			Opening: token.NoPos,
			List:    newFields,
			Closing: token.NoPos,
		}
	}

	return changed
}

// rewriteResults updates function literal return types.
func (inf *LambdaTypeInferrer) rewriteResults(fn *ast.FuncLit, expected *types.Signature) bool {
	expectedResults := expected.Results()
	if expectedResults == nil || expectedResults.Len() == 0 {
		// Expected function has no return value (void function)
		changed := false

		// Remove return type from signature if present
		if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
			fn.Type.Results = nil
			changed = true
		}

		// Fix body: remove return statements for single-expression bodies
		// Pattern: { return expr } → { expr }
		if fn.Body != nil && len(fn.Body.List) == 1 {
			if retStmt, ok := fn.Body.List[0].(*ast.ReturnStmt); ok && len(retStmt.Results) == 1 {
				// Replace return statement with expression statement
				fn.Body.List[0] = &ast.ExprStmt{X: retStmt.Results[0]}
				changed = true
			}
		}

		return changed
	}

	changed := false

	if fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
		// Lambda has no return type annotation, but expected type does
		// Add return type from expected signature
		// Note: Only add if we can successfully convert the type
		typeExprs := make([]ast.Expr, 0, expectedResults.Len())
		for i := 0; i < expectedResults.Len(); i++ {
			expectedType := expectedResults.At(i).Type()
			expr := inf.typeToExpr(expectedType)
			if expr != nil {
				typeExprs = append(typeExprs, expr)
			}
		}

		// Only add results if we successfully converted all types
		if len(typeExprs) == expectedResults.Len() {
			results := &ast.FieldList{
				List: make([]*ast.Field, len(typeExprs)),
			}
			for i, expr := range typeExprs {
				results.List[i] = &ast.Field{
					Type: expr,
				}
			}
			fn.Type.Results = results
			return true
		}
		return false
	}

	// Rewrite existing return types if they're 'any'
	for i, field := range fn.Type.Results.List {
		if i >= expectedResults.Len() {
			break
		}
		if inf.isAnyType(field.Type) {
			expectedType := expectedResults.At(i).Type()
			newTypeExpr := inf.typeToExpr(expectedType)
			if newTypeExpr != nil {
				// Do not count any -> any as a change. Synthetic dgo signatures
				// intentionally use any for output type parameters that need a
				// later inference pass.
				if inf.isAnyType(newTypeExpr) {
					continue
				}
				field.Type = newTypeExpr
				changed = true
			}
		}
	}

	return changed
}

// isAnyType checks if an expression is 'any' (interface{}).
// Used for type inference to detect placeholder types that need resolution.
func (inf *LambdaTypeInferrer) isAnyType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name == "any"
	case *ast.InterfaceType:
		return t.Methods == nil || len(t.Methods.List) == 0
	}
	return false
}

// isDingoAnyPlaceholder checks if an expression is specifically Dingo's 'any' placeholder.
// Unlike isAnyType, this only returns true for the 'any' identifier,
// NOT for Go's native 'interface{}' type literal.
//
// This distinction is important for FindUnresolvedLambdas:
// - Dingo lambdas (|x| expr) generate 'any' as a placeholder needing inference
// - Go-native func literals can legitimately use 'interface{}' and should not be flagged
func (inf *LambdaTypeInferrer) isDingoAnyPlaceholder(expr ast.Expr) bool {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name == "any"
	}
	return false
}

// typeToExpr converts a types.Type to an ast.Expr.
// Reuses logic from existing TypeRewriter.
func (inf *LambdaTypeInferrer) typeToExpr(t types.Type) ast.Expr {
	if t == nil {
		return nil
	}

	switch typ := t.(type) {
	case *types.Basic:
		// UntypedNil has no valid type representation - skip it
		if typ.Kind() == types.UntypedNil {
			return nil
		}
		// Handle untyped constants by converting to their typed equivalents
		// using go/types Kind constants (no string manipulation)
		name := untypedToTypedName(typ.Kind())
		if name == "" {
			name = typ.Name() // fallback for already-typed basics
		}
		return &ast.Ident{Name: name}

	case *types.Pointer:
		elem := inf.typeToExpr(typ.Elem())
		if elem != nil {
			return &ast.StarExpr{X: elem}
		}

	case *types.Alias:
		// For type aliases (Go 1.22+), use the alias name
		// type Point2D = struct { _0 float64; _1 float64 }
		// When seen in a function signature, we want "Point2D" not the underlying struct
		obj := typ.Obj()
		if obj != nil {
			name := obj.Name()
			pkg := obj.Pkg()
			alias := inf.getPackageAlias(pkg)
			if alias != "" && alias != "main" {
				return &ast.SelectorExpr{
					X:   &ast.Ident{Name: alias},
					Sel: &ast.Ident{Name: name},
				}
			}
			return &ast.Ident{Name: name}
		}
		// Fallback to underlying type
		return inf.typeToExpr(typ.Underlying())

	case *types.Named:
		// For named types, use the type name
		obj := typ.Obj()
		if obj != nil {
			name := obj.Name()
			pkg := obj.Pkg()

			// Handle generic instantiation
			if typ.TypeArgs() != nil && typ.TypeArgs().Len() > 0 {
				// Generic type instantiation: Type[T1, T2, ...]
				typeArgs := make([]ast.Expr, typ.TypeArgs().Len())
				for i := 0; i < typ.TypeArgs().Len(); i++ {
					arg := inf.typeToExpr(typ.TypeArgs().At(i))
					if arg == nil {
						// Can't convert type arg - fall back to uninstantiated name
						goto simpleNamed
					}
					typeArgs[i] = arg
				}

				var baseExpr ast.Expr
				alias := inf.getPackageAlias(pkg)
				if alias != "" && alias != "main" {
					baseExpr = &ast.SelectorExpr{
						X:   &ast.Ident{Name: alias},
						Sel: &ast.Ident{Name: name},
					}
				} else {
					baseExpr = &ast.Ident{Name: name}
				}

				// Use IndexListExpr for multiple type args, IndexExpr for single
				if len(typeArgs) == 1 {
					return &ast.IndexExpr{
						X:     baseExpr,
						Index: typeArgs[0],
					}
				} else {
					return &ast.IndexListExpr{
						X:       baseExpr,
						Indices: typeArgs,
					}
				}
			}

		simpleNamed:
			alias := inf.getPackageAlias(pkg)
			if alias != "" && alias != "main" {
				// Qualified type: pkg.Name
				return &ast.SelectorExpr{
					X:   &ast.Ident{Name: alias},
					Sel: &ast.Ident{Name: name},
				}
			}
			return &ast.Ident{Name: name}
		}

	case *types.Slice:
		elem := inf.typeToExpr(typ.Elem())
		if elem != nil {
			return &ast.ArrayType{Elt: elem}
		}

	case *types.Array:
		elem := inf.typeToExpr(typ.Elem())
		if elem != nil {
			return &ast.ArrayType{
				Len: &ast.BasicLit{
					Kind:  token.INT,
					Value: strconv.FormatInt(typ.Len(), 10),
				},
				Elt: elem,
			}
		}

	case *types.Map:
		key := inf.typeToExpr(typ.Key())
		val := inf.typeToExpr(typ.Elem())
		if key != nil && val != nil {
			return &ast.MapType{Key: key, Value: val}
		}

	case *types.Chan:
		elem := inf.typeToExpr(typ.Elem())
		if elem != nil {
			dir := ast.SEND | ast.RECV
			switch typ.Dir() {
			case types.SendOnly:
				dir = ast.SEND
			case types.RecvOnly:
				dir = ast.RECV
			}
			return &ast.ChanType{Dir: dir, Value: elem}
		}

	case *types.Signature:
		// Function type
		return inf.signatureToFuncType(typ)

	case *types.Interface:
		// Build interface with method set
		if typ.NumMethods() == 0 {
			// Empty interface{} or any
			return &ast.InterfaceType{Methods: &ast.FieldList{}}
		}

		methods := &ast.FieldList{
			List: make([]*ast.Field, 0, typ.NumMethods()),
		}

		for i := 0; i < typ.NumMethods(); i++ {
			method := typ.Method(i)
			sig, ok := method.Type().(*types.Signature)
			if !ok {
				continue
			}

			methods.List = append(methods.List, &ast.Field{
				Names: []*ast.Ident{{Name: method.Name()}},
				Type:  inf.signatureToFuncType(sig),
			})
		}

		return &ast.InterfaceType{Methods: methods}

	case *types.Struct:
		// Anonymous struct - convert to struct type
		return inf.structToStructType(typ)

	case *types.TypeParam:
		// Generic type parameter (e.g., T in func[T any])
		// If we reach this case, it means the type parameter wasn't resolved
		// Fall back to 'any' for unresolved type parameters
		return &ast.Ident{Name: "any"}
	}

	return nil
}

// signatureToFuncType converts a function signature to a func type AST node.
func (inf *LambdaTypeInferrer) signatureToFuncType(sig *types.Signature) *ast.FuncType {
	funcType := &ast.FuncType{}

	// Parameters
	if sig.Params() != nil && sig.Params().Len() > 0 {
		params := &ast.FieldList{
			List: make([]*ast.Field, sig.Params().Len()),
		}
		for i := 0; i < sig.Params().Len(); i++ {
			param := sig.Params().At(i)
			params.List[i] = &ast.Field{
				Type: inf.typeToExpr(param.Type()),
			}
		}
		funcType.Params = params
	} else {
		funcType.Params = &ast.FieldList{}
	}

	// Results
	if sig.Results() != nil && sig.Results().Len() > 0 {
		results := &ast.FieldList{
			List: make([]*ast.Field, sig.Results().Len()),
		}
		for i := 0; i < sig.Results().Len(); i++ {
			result := sig.Results().At(i)
			results.List[i] = &ast.Field{
				Type: inf.typeToExpr(result.Type()),
			}
		}
		funcType.Results = results
	}

	return funcType
}

// structToStructType converts an anonymous struct type to an AST struct type.
func (inf *LambdaTypeInferrer) structToStructType(st *types.Struct) *ast.StructType {
	fields := &ast.FieldList{
		List: make([]*ast.Field, st.NumFields()),
	}

	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		fields.List[i] = &ast.Field{
			Names: []*ast.Ident{{Name: field.Name()}},
			Type:  inf.typeToExpr(field.Type()),
		}
	}

	return &ast.StructType{Fields: fields}
}

// getPackageAlias resolves the correct package identifier for a package,
// respecting import aliases in the file.
func (inf *LambdaTypeInferrer) getPackageAlias(pkg *types.Package) string {
	if pkg == nil {
		return ""
	}

	// Search file imports for this package
	pkgPath := pkg.Path()
	for _, imp := range inf.file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if path == pkgPath {
			if imp.Name != nil {
				// Explicit alias (including "." for dot imports)
				return imp.Name.Name
			}
			// Default name
			return pkg.Name()
		}
	}

	// Not imported (shouldn't happen if go/types succeeded)
	// Fall back to package name
	return pkg.Name()
}

// resolveExternalFunction resolves a function from an external package.
// For dgo.Map, it finds the package object for "dgo" and looks up "Map" in its scope.
//
// When the gc importer fails to fully load package scopes (which happens when there
// are type errors in the code being checked), we fall back to using the source importer
// which can parse and load packages from source files.
func (inf *LambdaTypeInferrer) resolveExternalFunction(sel *ast.SelectorExpr) types.Object {
	// Get the package identifier (e.g., "dgo" in dgo.Map)
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil
	}

	// Resolve the package from info.Uses
	pkgObj := inf.info.Uses[pkgIdent]
	if pkgObj == nil {
		return nil
	}

	// Check if it's a package name
	pkgName, ok := pkgObj.(*types.PkgName)
	if !ok {
		return nil
	}

	// Get the imported package
	pkg := pkgName.Imported()
	if pkg == nil {
		return nil
	}

	// Look up the function in the package's exported scope
	funcName := sel.Sel.Name
	result := pkg.Scope().Lookup(funcName)

	// If the scope lookup failed, the package may not have been fully loaded
	// (this happens when the gc importer encounters type errors).
	// Fall back to using the source importer which can load packages from source.
	if result == nil {
		result = inf.resolveExternalFunctionViaSourceImporter(pkg.Path(), funcName)
	}

	return result
}

// resolveExternalFunctionViaSourceImporter uses the source importer as a fallback
// when the gc importer fails to populate package scopes due to type errors.
// Uses a package-level cache to avoid repeated expensive source parsing.
func (inf *LambdaTypeInferrer) resolveExternalFunctionViaSourceImporter(pkgPath, funcName string) types.Object {
	// Check cache first
	if pkg, ok := sourceImportCache[pkgPath]; ok {
		if pkg == nil {
			return nil // Previous import failed
		}
		return pkg.Scope().Lookup(funcName)
	}

	// Use the source importer which parses Go source files
	imp := importer.ForCompiler(inf.fset, "source", nil)

	pkg, err := imp.Import(pkgPath)
	if err != nil {
		sourceImportCache[pkgPath] = nil // Cache the failure
		return nil
	}

	// Cache the successfully imported package
	sourceImportCache[pkgPath] = pkg
	return pkg.Scope().Lookup(funcName)
}

// getGenericSignature extracts the generic (uninstantiated) signature from a function expression.
// Returns nil if the function is not generic or if it cannot be resolved.
func (inf *LambdaTypeInferrer) getGenericSignature(fun ast.Expr) *types.Signature {
	// Unwrap indexing expressions to get to the base function
	var baseExpr ast.Expr = fun
	switch fn := fun.(type) {
	case *ast.IndexExpr:
		baseExpr = fn.X
	case *ast.IndexListExpr:
		baseExpr = fn.X
	}

	// Look up the function object
	var obj types.Object
	switch base := baseExpr.(type) {
	case *ast.Ident:
		obj = inf.info.Uses[base]
	case *ast.SelectorExpr:
		// Try as method selection first
		if sel := inf.info.Selections[base]; sel != nil {
			obj = sel.Obj()
		} else {
			// Try as package-qualified identifier (works for local package aliases)
			obj = inf.info.Uses[base.Sel]

			// If that fails, resolve via package scope lookup for external packages
			if obj == nil {
				obj = inf.resolveExternalFunction(base)
			}
		}
	}

	if obj == nil {
		return nil
	}

	// Extract signature
	sig, ok := obj.Type().(*types.Signature)
	if !ok {
		return nil
	}

	// Check if it has type parameters
	if sig.TypeParams() == nil || sig.TypeParams().Len() == 0 {
		return nil
	}

	return sig
}

// resolveTypeParamsFromArgs resolves type parameters by analyzing concrete argument types.
// For Filter[T](items []T, predicate func(T) bool), if we see Filter(users, ...) where
// users is []User, we resolve T=User.
//
// Also attempts to infer return type parameters from lambda body expressions.
func (inf *LambdaTypeInferrer) resolveTypeParamsFromArgs(call *ast.CallExpr, genericSig *types.Signature) []types.Type {
	if genericSig.TypeParams() == nil || genericSig.TypeParams().Len() == 0 {
		return nil
	}

	// Build substitution map: TypeParam -> ConcreteType
	subst := make(map[*types.TypeParam]types.Type)

	params := genericSig.Params()
	for i, arg := range call.Args {
		// Get parameter index (handle variadic later)
		if i >= params.Len() {
			continue
		}

		// Get expected parameter type (with type parameters)
		paramType := params.At(i).Type()

		// Special handling for lambda arguments - infer from body
		if funcLit, ok := arg.(*ast.FuncLit); ok {
			// Extract type parameters from lambda body return expressions
			inf.inferTypeParamsFromLambdaBody(funcLit, paramType, subst)
			continue
		}

		// Get concrete argument type
		// For identifiers (especially range loop variables), also check info.Defs and info.Uses
		// Go's type checker stores loop iteration variable types in info.Defs, NOT info.Types
		var argType types.Type
		if ident, ok := arg.(*ast.Ident); ok {
			// For identifiers, check Defs (defining uses like range vars) or Uses (referring uses)
			if obj := inf.info.Defs[ident]; obj != nil {
				argType = obj.Type()
			} else if obj := inf.info.Uses[ident]; obj != nil {
				argType = obj.Type()
			}
		}
		if argType == nil {
			// Fall back to Types map for expressions (function calls, etc.)
			if tv, ok := inf.info.Types[arg]; ok {
				argType = tv.Type
			}
		}
		if !isUsableInferenceType(argType) {
			// importer.Default cannot resolve module-local packages when the
			// transpiler is used as a library. That leaves variables derived
			// from dgo calls (especially range variables) with invalid types.
			// Recover their type from the dgo signature registry and the AST.
			switch arg := arg.(type) {
			case *ast.Ident:
				argType = inf.inferRangeVariableType(arg)
				if !isUsableInferenceType(argType) {
					argType = inf.inferAssignedDgoResultType(arg)
				}
			case *ast.CallExpr:
				argType = inf.inferDgoCallResultType(arg)
			}
		}
		if !isUsableInferenceType(argType) {
			continue
		}

		// Match argument type to parameter type to extract type parameter bindings
		inf.matchTypeToParam(argType, paramType, subst)
	}

	// Convert substitution map to ordered type arguments
	// The order must match genericSig.TypeParams()
	typeParams := genericSig.TypeParams()
	typeArgs := make([]types.Type, typeParams.Len())

	for i := 0; i < typeParams.Len(); i++ {
		tp := typeParams.At(i)
		if concreteType, ok := subst[tp]; ok {
			typeArgs[i] = concreteType
		} else {
			// Use 'any' for unresolved type parameters
			// This is a safe fallback when inference fails
			typeArgs[i] = types.Universe.Lookup("any").Type()
		}
	}

	// Return even if some type args are 'any' - partial resolution is better than nothing
	return typeArgs
}

// isUsableInferenceType reports whether a go/types result carries concrete
// information. With partial type checking, unresolved identifiers are recorded
// as types.Invalid rather than nil.
func isUsableInferenceType(t types.Type) bool {
	if t == nil {
		return false
	}
	if basic, ok := t.(*types.Basic); ok && basic.Kind() == types.Invalid {
		return false
	}
	return true
}

// inferRangeVariableType recovers the type of an identifier introduced by a
// range statement when go/types could not resolve the range expression because
// it came from a module-local dgo call.
func (inf *LambdaTypeInferrer) inferRangeVariableType(ident *ast.Ident) types.Type {
	target := inf.info.Uses[ident]
	if target == nil {
		target = inf.info.Defs[ident]
	}
	if target == nil {
		return nil
	}

	var inferred types.Type
	ast.Inspect(inf.file, func(n ast.Node) bool {
		if inferred != nil {
			return false
		}
		rangeStmt, ok := n.(*ast.RangeStmt)
		if !ok {
			return true
		}

		isKey := rangeIdentDefinesObject(inf.info, rangeStmt.Key, target)
		isValue := rangeIdentDefinesObject(inf.info, rangeStmt.Value, target)
		if !isKey && !isValue {
			return true
		}

		containerType := inf.inferAssignedDgoResultType(rangeStmt.X)
		if !isUsableInferenceType(containerType) {
			return true
		}

		switch container := containerType.Underlying().(type) {
		case *types.Map:
			if isValue {
				inferred = container.Elem()
			} else {
				inferred = container.Key()
			}
		case *types.Slice:
			if isValue {
				inferred = container.Elem()
			} else {
				inferred = types.Typ[types.Int]
			}
		case *types.Array:
			if isValue {
				inferred = container.Elem()
			} else {
				inferred = types.Typ[types.Int]
			}
		case *types.Chan:
			if isValue || rangeStmt.Value == nil {
				inferred = container.Elem()
			}
		case *types.Basic:
			if container.Kind() == types.String {
				if isValue {
					inferred = types.Typ[types.Rune]
				} else {
					inferred = types.Typ[types.Int]
				}
			}
		}

		return inferred == nil
	})

	return inferred
}

func rangeIdentDefinesObject(info *types.Info, expr ast.Expr, target types.Object) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && info.Defs[ident] == target
}

// inferAssignedDgoResultType resolves an expression such as byCategory back to
// its defining dgo.GroupBy call, then instantiates the registry signature using
// the types available in the current inference pass.
func (inf *LambdaTypeInferrer) inferAssignedDgoResultType(expr ast.Expr) types.Type {
	if tv, ok := inf.info.Types[expr]; ok && isUsableInferenceType(tv.Type) {
		return tv.Type
	}

	if call, ok := expr.(*ast.CallExpr); ok {
		return inf.inferDgoCallResultType(call)
	}

	ident, ok := expr.(*ast.Ident)
	if !ok {
		return nil
	}
	target := inf.info.Uses[ident]
	if target == nil {
		target = inf.info.Defs[ident]
	}
	if target == nil {
		return nil
	}

	var inferred types.Type
	ast.Inspect(inf.file, func(n ast.Node) bool {
		if inferred != nil {
			return false
		}

		switch decl := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range decl.Lhs {
				lhsIdent, ok := lhs.(*ast.Ident)
				if !ok || inf.info.Defs[lhsIdent] != target {
					continue
				}
				rhsIndex := i
				if len(decl.Rhs) == 1 {
					rhsIndex = 0
				}
				if rhsIndex < len(decl.Rhs) {
					if call, ok := decl.Rhs[rhsIndex].(*ast.CallExpr); ok {
						inferred = inf.inferDgoCallResultType(call)
					}
				}
				return inferred == nil
			}

		case *ast.ValueSpec:
			for i, name := range decl.Names {
				if inf.info.Defs[name] != target || len(decl.Values) == 0 {
					continue
				}
				valueIndex := i
				if len(decl.Values) == 1 {
					valueIndex = 0
				}
				if valueIndex < len(decl.Values) {
					if call, ok := decl.Values[valueIndex].(*ast.CallExpr); ok {
						inferred = inf.inferDgoCallResultType(call)
					}
				}
				return inferred == nil
			}
		}

		return true
	})

	return inferred
}

func (inf *LambdaTypeInferrer) inferDgoCallResultType(call *ast.CallExpr) types.Type {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok || pkgIdent.Name != "dgo" {
		return nil
	}

	genericSig := GetDgoSignature(sel.Sel.Name)
	if genericSig == nil {
		return nil
	}
	typeArgs := inf.resolveTypeParamsFromArgs(call, genericSig)
	instantiated := inf.instantiateSignature(genericSig, typeArgs)
	if instantiated == nil || instantiated.Results() == nil || instantiated.Results().Len() == 0 {
		return nil
	}
	return instantiated.Results().At(0).Type()
}

// matchTypeToParam matches a concrete type to a (possibly generic) parameter type,
// extracting type parameter bindings.
// For example: matchTypeToParam([]User, []T, subst) sets subst[T] = User
func (inf *LambdaTypeInferrer) matchTypeToParam(concreteType, paramType types.Type, subst map[*types.TypeParam]types.Type) {
	// Handle type parameter directly
	if tp, ok := paramType.(*types.TypeParam); ok {
		if _, exists := subst[tp]; !exists {
			subst[tp] = concreteType
		}
		return
	}

	// Handle slice: []T
	if concreteSlice, ok := concreteType.(*types.Slice); ok {
		if paramSlice, ok := paramType.(*types.Slice); ok {
			inf.matchTypeToParam(concreteSlice.Elem(), paramSlice.Elem(), subst)
			return
		}
	}

	// Handle array: [N]T
	if concreteArray, ok := concreteType.(*types.Array); ok {
		if paramArray, ok := paramType.(*types.Array); ok {
			inf.matchTypeToParam(concreteArray.Elem(), paramArray.Elem(), subst)
			return
		}
	}

	// Handle map: map[K]V
	if concreteMap, ok := concreteType.(*types.Map); ok {
		if paramMap, ok := paramType.(*types.Map); ok {
			inf.matchTypeToParam(concreteMap.Key(), paramMap.Key(), subst)
			inf.matchTypeToParam(concreteMap.Elem(), paramMap.Elem(), subst)
			return
		}
	}

	// Handle pointer: *T
	if concretePtr, ok := concreteType.(*types.Pointer); ok {
		if paramPtr, ok := paramType.(*types.Pointer); ok {
			inf.matchTypeToParam(concretePtr.Elem(), paramPtr.Elem(), subst)
			return
		}
	}

	// Handle channel: chan T, <-chan T, chan<- T
	if concreteChan, ok := concreteType.(*types.Chan); ok {
		if paramChan, ok := paramType.(*types.Chan); ok {
			inf.matchTypeToParam(concreteChan.Elem(), paramChan.Elem(), subst)
			return
		}
	}

	// Handle named types with type arguments: Type[T1, T2]
	if concreteNamed, ok := concreteType.(*types.Named); ok {
		if paramNamed, ok := paramType.(*types.Named); ok {
			// Check if both are instantiations of the same generic type
			if concreteNamed.Obj() == paramNamed.Obj() {
				// Match type arguments
				concreteArgs := concreteNamed.TypeArgs()
				paramArgs := paramNamed.TypeArgs()
				if concreteArgs != nil && paramArgs != nil {
					for i := 0; i < concreteArgs.Len() && i < paramArgs.Len(); i++ {
						inf.matchTypeToParam(concreteArgs.At(i), paramArgs.At(i), subst)
					}
				}
			}
			return
		}
	}

	// Handle function types: func(T) R
	if concreteSig, ok := concreteType.(*types.Signature); ok {
		if paramSig, ok := paramType.(*types.Signature); ok {
			// Match parameters
			if concreteSig.Params() != nil && paramSig.Params() != nil {
				for i := 0; i < concreteSig.Params().Len() && i < paramSig.Params().Len(); i++ {
					inf.matchTypeToParam(
						concreteSig.Params().At(i).Type(),
						paramSig.Params().At(i).Type(),
						subst,
					)
				}
			}
			// Match results
			if concreteSig.Results() != nil && paramSig.Results() != nil {
				for i := 0; i < concreteSig.Results().Len() && i < paramSig.Results().Len(); i++ {
					inf.matchTypeToParam(
						concreteSig.Results().At(i).Type(),
						paramSig.Results().At(i).Type(),
						subst,
					)
				}
			}
			return
		}
	}

	// No structural match - types don't align or no type parameters present
}

// instantiateSignature creates a concrete signature by substituting type arguments
// into a generic signature's type parameters.
func (inf *LambdaTypeInferrer) instantiateSignature(genericSig *types.Signature, typeArgs []types.Type) *types.Signature {
	if len(typeArgs) == 0 {
		return nil
	}

	typeParams := genericSig.TypeParams()
	if typeParams == nil || typeParams.Len() != len(typeArgs) {
		return nil
	}

	// Use types.Instantiate to perform the substitution
	// This handles all the complex cases (nested types, constraints, etc.)
	// NOTE: Create a fresh context for each instantiation
	ctx := types.NewContext()
	instantiated, err := types.Instantiate(ctx, genericSig, typeArgs, false)
	if err != nil {
		return nil
	}

	sig, ok := instantiated.(*types.Signature)
	if !ok {
		return nil
	}

	return sig
}

// inferTypeParamsFromLambdaBody attempts to infer type parameters from a lambda's body.
// For example, given Map[T, R](items []T, transform func(T) R):
// - If lambda body returns `user.Name` where Name is string, infer R=string
// - If expected param type is func(T) R, match return type R to body's return expression
//
// NOTE: This only works if lambda parameters have already been typed (not 'any').
// The multi-pass type inference in pure_pipeline.go handles this:
// Pass 1: Infer T from non-lambda args, rewrite lambda params
// Pass 2: Re-type-check, now lambda body is typed, infer R from return expr
func (inf *LambdaTypeInferrer) inferTypeParamsFromLambdaBody(
	funcLit *ast.FuncLit,
	expectedParamType types.Type,
	subst map[*types.TypeParam]types.Type,
) {
	// Extract function signature from expected parameter type
	expectedSig, ok := expectedParamType.(*types.Signature)
	if !ok {
		return
	}

	// Only process if the function has a return type
	if expectedSig.Results() == nil || expectedSig.Results().Len() == 0 {
		return
	}

	// Check if lambda parameters are still 'any' - if so, skip body inference
	// Body inference only works after parameters have been typed
	if funcLit.Type.Params != nil {
		for _, field := range funcLit.Type.Params.List {
			if inf.isAnyType(field.Type) {
				// Parameters not yet typed - skip body inference for this pass
				// Will retry on next pass after params are rewritten
				return
			}
		}
	}

	// Get the return type from expected signature (may contain type parameters)
	expectedReturnType := expectedSig.Results().At(0).Type()

	// Find return expression in lambda body
	returnExpr := inf.extractReturnExpression(funcLit.Body)
	if returnExpr == nil {
		return
	}

	// Get the type of the return expression using go/types
	tv, ok := inf.info.Types[returnExpr]
	if !ok {
		return
	}
	actualReturnType := tv.Type

	// Match actual return type to expected return type (which may have type params)
	// Example: actualReturnType=string, expectedReturnType=R → sets subst[R]=string
	inf.matchTypeToParam(actualReturnType, expectedReturnType, subst)
}

// extractReturnExpression finds the return expression in a lambda body.
// Handles both expression lambdas ({ return expr }) and simple expression statements ({ expr }).
func (inf *LambdaTypeInferrer) extractReturnExpression(body *ast.BlockStmt) ast.Expr {
	if body == nil || len(body.List) == 0 {
		return nil
	}

	// Check for single-statement body
	if len(body.List) == 1 {
		switch stmt := body.List[0].(type) {
		case *ast.ReturnStmt:
			// { return expr }
			if len(stmt.Results) > 0 {
				return stmt.Results[0]
			}
		case *ast.ExprStmt:
			// { expr } - treated as implicit return
			return stmt.X
		}
	}

	// For multi-statement bodies, find the last return statement
	// Walk backwards through statements to find return
	for i := len(body.List) - 1; i >= 0; i-- {
		if retStmt, ok := body.List[i].(*ast.ReturnStmt); ok {
			if len(retStmt.Results) > 0 {
				return retStmt.Results[0]
			}
		}
	}

	return nil
}

// inferStandaloneLambdas is Layer 5 of type inference.
// It handles standalone lambdas (assigned to variables) that have typed parameters
// but unresolved 'any' return types.
//
// For example: add := |x: int, y: int| x + y
// Generated:   add := func(x int, y int) any { return x + y }
//
// This method analyzes the return expression's type using go/types and rewrites
// the 'any' return type to the inferred concrete type.
func (inf *LambdaTypeInferrer) inferStandaloneLambdas() {
	// Find all function literals that:
	// 1. Have all parameters typed (not 'any')
	// 2. Have 'any' as return type
	// 3. Have a return expression we can analyze
	ast.Inspect(inf.file, func(n ast.Node) bool {
		funcLit, ok := n.(*ast.FuncLit)
		if !ok {
			return true
		}

		// Skip if no return type or return type is not 'any'
		if funcLit.Type.Results == nil || len(funcLit.Type.Results.List) == 0 {
			return true
		}
		if len(funcLit.Type.Results.List) != 1 {
			// Multiple return values - skip for now
			return true
		}
		if !inf.isAnyType(funcLit.Type.Results.List[0].Type) {
			// Return type is already concrete
			return true
		}

		// Check if all parameters are typed (not 'any')
		if !inf.allParamsTyped(funcLit) {
			return true
		}

		// Try to infer return type from body expression
		returnType := inf.inferReturnTypeFromBody(funcLit)
		if returnType != nil {
			// Skip UntypedNil - interface{} is the correct return type for nil-returning functions
			// This catches UntypedNil from any code path (info.Types or analyzeExpressionType)
			if basic, ok := returnType.(*types.Basic); ok && basic.Kind() == types.UntypedNil {
				return true
			}
			// Rewrite the return type
			newTypeExpr := inf.typeToExpr(returnType)
			if newTypeExpr != nil {
				funcLit.Type.Results.List[0].Type = newTypeExpr
				inf.changed = true
			}
		}

		return true
	})
}

// allParamsTyped checks if all parameters in a function literal have concrete types.
func (inf *LambdaTypeInferrer) allParamsTyped(funcLit *ast.FuncLit) bool {
	if funcLit.Type.Params == nil || len(funcLit.Type.Params.List) == 0 {
		// No parameters - consider typed
		return true
	}

	for _, field := range funcLit.Type.Params.List {
		if inf.isAnyType(field.Type) {
			return false
		}
	}
	return true
}

// inferReturnTypeFromBody analyzes a function literal's body to determine the return type.
// It uses go/types to type-check the return expression given the parameter types.
func (inf *LambdaTypeInferrer) inferReturnTypeFromBody(funcLit *ast.FuncLit) types.Type {
	// Extract the return expression
	returnExpr := inf.extractReturnExpression(funcLit.Body)
	if returnExpr == nil {
		return nil
	}

	// Try to get the type from info.Types (already computed)
	if tv, ok := inf.info.Types[returnExpr]; ok && tv.Type != nil {
		// Handle untyped constants
		if basic, ok := tv.Type.(*types.Basic); ok {
			// UntypedNil has no typed equivalent - skip inference
			// interface{} is the correct return type for functions that return nil
			if basic.Kind() == types.UntypedNil {
				return nil
			}
			if name := untypedToTypedName(basic.Kind()); name != "" {
				return types.Universe.Lookup(name).Type()
			}
		}
		return tv.Type
	}

	// If the expression type isn't in info.Types, try analyzing the expression
	// This handles cases where the initial type check failed due to 'any' return type
	return inf.analyzeExpressionType(funcLit, returnExpr)
}

// analyzeExpressionType performs targeted type analysis on an expression
// within a function literal context.
func (inf *LambdaTypeInferrer) analyzeExpressionType(funcLit *ast.FuncLit, expr ast.Expr) types.Type {
	// Build a map of parameter names to their types
	paramTypes := make(map[string]types.Type)
	if funcLit.Type.Params != nil {
		for _, field := range funcLit.Type.Params.List {
			// Get the parameter type
			fieldType := inf.exprToType(field.Type)
			if fieldType == nil {
				continue
			}
			for _, name := range field.Names {
				paramTypes[name.Name] = fieldType
			}
		}
	}

	// Analyze the expression based on its structure
	return inf.evaluateExprType(expr, paramTypes)
}

// evaluateExprType recursively evaluates the type of an expression
// given a context of known variable types.
func (inf *LambdaTypeInferrer) evaluateExprType(expr ast.Expr, ctx map[string]types.Type) types.Type {
	switch e := expr.(type) {
	case *ast.Ident:
		// Variable reference - look up in context or info.Uses
		if t, ok := ctx[e.Name]; ok {
			return t
		}
		if obj := inf.info.Uses[e]; obj != nil {
			return obj.Type()
		}

	case *ast.BasicLit:
		// Literal value - infer type from literal kind
		switch e.Kind {
		case token.INT:
			return types.Typ[types.Int]
		case token.FLOAT:
			return types.Typ[types.Float64]
		case token.STRING:
			return types.Typ[types.String]
		case token.CHAR:
			return types.Typ[types.Rune]
		}

	case *ast.BinaryExpr:
		// Binary operation - type depends on operands and operator
		leftType := inf.evaluateExprType(e.X, ctx)
		rightType := inf.evaluateExprType(e.Y, ctx)

		// For arithmetic/comparison operators, result type follows Go rules
		switch e.Op {
		case token.ADD, token.SUB, token.MUL, token.QUO, token.REM:
			// Arithmetic - result is same as operand type
			if leftType != nil {
				return leftType
			}
			return rightType
		case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
			// Comparison - result is bool
			return types.Typ[types.Bool]
		case token.LAND, token.LOR:
			// Logical - result is bool
			return types.Typ[types.Bool]
		case token.AND, token.OR, token.XOR, token.SHL, token.SHR, token.AND_NOT:
			// Bitwise - result is same as operand type
			if leftType != nil {
				return leftType
			}
			return rightType
		}

	case *ast.UnaryExpr:
		// Unary operation
		operandType := inf.evaluateExprType(e.X, ctx)
		switch e.Op {
		case token.NOT:
			return types.Typ[types.Bool]
		case token.SUB, token.ADD, token.XOR:
			return operandType
		}

	case *ast.ParenExpr:
		return inf.evaluateExprType(e.X, ctx)

	case *ast.SelectorExpr:
		// Field access - look up in info.Selections or info.Types
		if sel, ok := inf.info.Selections[e]; ok {
			return sel.Type()
		}
		if tv, ok := inf.info.Types[e]; ok {
			return tv.Type
		}

	case *ast.CallExpr:
		// Function call - look up return type
		if tv, ok := inf.info.Types[e]; ok {
			return tv.Type
		}
		// Try to get the function's return type
		funType := inf.evaluateExprType(e.Fun, ctx)
		if sig, ok := funType.(*types.Signature); ok {
			if sig.Results() != nil && sig.Results().Len() > 0 {
				return sig.Results().At(0).Type()
			}
		}

	case *ast.IndexExpr:
		// Index expression - element type of slice/array/map
		containerType := inf.evaluateExprType(e.X, ctx)
		if containerType != nil {
			switch ct := containerType.Underlying().(type) {
			case *types.Slice:
				return ct.Elem()
			case *types.Array:
				return ct.Elem()
			case *types.Map:
				return ct.Elem()
			}
		}
	}

	// Fallback: check info.Types
	if tv, ok := inf.info.Types[expr]; ok {
		return tv.Type
	}

	return nil
}

// exprToType converts a type expression AST node to a types.Type.
// Handles basic types, named types, and common composite types.
func (inf *LambdaTypeInferrer) exprToType(expr ast.Expr) types.Type {
	switch e := expr.(type) {
	case *ast.Ident:
		// Basic type or named type
		if basic := inf.lookupBasicType(e.Name); basic != nil {
			return basic
		}
		// Look up named type in Uses
		if obj := inf.info.Uses[e]; obj != nil {
			return obj.Type()
		}

	case *ast.SelectorExpr:
		// Qualified type: pkg.Type
		if tv, ok := inf.info.Types[e]; ok {
			return tv.Type
		}
		if obj := inf.info.Uses[e.Sel]; obj != nil {
			return obj.Type()
		}

	case *ast.StarExpr:
		// Pointer type
		elemType := inf.exprToType(e.X)
		if elemType != nil {
			return types.NewPointer(elemType)
		}

	case *ast.ArrayType:
		// Slice or array type
		elemType := inf.exprToType(e.Elt)
		if elemType != nil {
			if e.Len == nil {
				return types.NewSlice(elemType)
			}
			// Array - would need to evaluate length, skip for now
		}
	}

	return nil
}

// lookupBasicType returns the basic type for a type name, or nil if not a basic type.
func (inf *LambdaTypeInferrer) lookupBasicType(name string) types.Type {
	switch name {
	case "bool":
		return types.Typ[types.Bool]
	case "int":
		return types.Typ[types.Int]
	case "int8":
		return types.Typ[types.Int8]
	case "int16":
		return types.Typ[types.Int16]
	case "int32":
		return types.Typ[types.Int32]
	case "int64":
		return types.Typ[types.Int64]
	case "uint":
		return types.Typ[types.Uint]
	case "uint8":
		return types.Typ[types.Uint8]
	case "uint16":
		return types.Typ[types.Uint16]
	case "uint32":
		return types.Typ[types.Uint32]
	case "uint64":
		return types.Typ[types.Uint64]
	case "uintptr":
		return types.Typ[types.Uintptr]
	case "float32":
		return types.Typ[types.Float32]
	case "float64":
		return types.Typ[types.Float64]
	case "complex64":
		return types.Typ[types.Complex64]
	case "complex128":
		return types.Typ[types.Complex128]
	case "string":
		return types.Typ[types.String]
	case "byte":
		return types.Typ[types.Byte]
	case "rune":
		return types.Typ[types.Rune]
	case "any":
		return types.Universe.Lookup("any").Type()
	case "error":
		return types.Universe.Lookup("error").Type()
	}
	return nil
}

// UnresolvedLambda represents a lambda expression that still has unresolved 'any' types
// after type inference. This is used to generate helpful error messages.
type UnresolvedLambda struct {
	Line         int      // 1-indexed line number
	Column       int      // 1-indexed column number
	ParamNames   []string // Names of parameters with 'any' type
	HasAnyReturn bool     // True if return type is still 'any'
	FuncLit      *ast.FuncLit
}

// FindUnresolvedLambdas walks the AST and finds all function literals that still have
// 'any' types in their parameters or return values after type inference.
// These represent lambdas where type inference failed and the user needs to provide
// explicit type annotations.
//
// IIFEs (Immediately Invoked Function Expressions) are excluded because they are
// generated by the transpiler for null-coalesce, ternary, and match expressions.
// These generated IIFEs don't need user-provided type annotations.
func (inf *LambdaTypeInferrer) FindUnresolvedLambdas() []UnresolvedLambda {
	var unresolved []UnresolvedLambda

	// First pass: identify all IIFEs (FuncLit nodes that are the Fun of a CallExpr)
	iifeSet := make(map[*ast.FuncLit]bool)
	ast.Inspect(inf.file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		// If the function being called IS a FuncLit, this is an IIFE
		if funcLit, ok := call.Fun.(*ast.FuncLit); ok {
			iifeSet[funcLit] = true
		}
		return true
	})

	// Second pass: find unresolved lambdas, excluding IIFEs
	ast.Inspect(inf.file, func(n ast.Node) bool {
		funcLit, ok := n.(*ast.FuncLit)
		if !ok {
			return true
		}

		// Skip IIFEs - they're generated code (null-coalesce, ternary, match)
		if iifeSet[funcLit] {
			return true
		}

		// Check if this lambda has any unresolved 'any' types
		// We use isDingoAnyPlaceholder instead of isAnyType because:
		// - Dingo lambdas (|x| expr) generate 'any' identifier as placeholder
		// - Go-native func literals can legitimately use 'interface{}' type
		// Only Dingo placeholders should be flagged as needing type annotation
		//
		// IMPORTANT: We only flag lambdas with 'any' in PARAMETERS, not just returns.
		// This is because Go's parser normalizes 'interface{}' to 'any' (Go 1.18+),
		// so native Go code like `func(x int) (interface{}, error)` would have
		// 'any' in the return type after parsing. But Dingo-generated lambdas
		// always have 'any' in their parameters because they're generated with
		// placeholder types. Native Go func literals have typed parameters.
		var anyParams []string
		hasAnyReturn := false

		// Check parameters
		if funcLit.Type.Params != nil {
			for _, field := range funcLit.Type.Params.List {
				if inf.isDingoAnyPlaceholder(field.Type) {
					// Collect parameter names
					for _, name := range field.Names {
						anyParams = append(anyParams, name.Name)
					}
				}
			}
		}

		// Only check return type if we have untyped params
		// This avoids false positives from native Go code using interface{} returns
		if len(anyParams) > 0 && funcLit.Type.Results != nil {
			for _, field := range funcLit.Type.Results.List {
				if inf.isDingoAnyPlaceholder(field.Type) {
					hasAnyReturn = true
					break
				}
			}
		}

		// Only flag lambdas with unresolved parameter types
		// Unresolved return types alone could be from native Go interface{} usage
		if len(anyParams) > 0 {
			pos := inf.fset.Position(funcLit.Pos())
			unresolved = append(unresolved, UnresolvedLambda{
				Line:         pos.Line,
				Column:       pos.Column,
				ParamNames:   anyParams,
				HasAnyReturn: hasAnyReturn,
				FuncLit:      funcLit,
			})
		}

		return true
	})

	return unresolved
}

// Note: Error formatting is now handled by the LSP error_formatter.go
// The LSP server uses ErrorFormatter interface to create editor-specific messages
// from the structured UnresolvedLambdaErrorData in transpiler.TranspileError
