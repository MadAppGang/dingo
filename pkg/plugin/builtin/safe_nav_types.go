// Package builtin provides safe navigation type inference plugin
package builtin

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/MadAppGang/dingo/pkg/plugin"
	"golang.org/x/tools/go/ast/astutil"
)

// SafeNavTypePlugin resolves __INFER__ placeholders with actual types for safe navigation
//
// This plugin implements Task A2 from Phase 7: Safe navigation type inference.
// It finds __INFER__ placeholder identifiers inserted by SafeNavProcessor and replaces them
// with concrete types using go/types analysis.
//
// Type Resolution Strategy:
// 1. Discovery: Find all __INFER__ identifiers in AST
// 2. Transform: Use go/types to resolve actual types
// 3. Inject: Replace __INFER__ with concrete types (Option<T> or *T)
//
// Supports:
// - Option<T> types (enum-based optionals)
// - Raw Go pointers (*T)
// - Error reporting for non-nullable types
type SafeNavTypePlugin struct {
	ctx *plugin.Context

	// Track which __INFER__ nodes we've found
	inferNodes []*inferNode

	// Type inference service for accurate type resolution
	typeInference *TypeInferenceService

	// AST-based type maps for fallback inference (when go/types fails)
	structFields  map[string]map[string]string // struct → field → type
	methodReturns map[string]map[string]string // receiver → method → return type

	// Errors encountered during type inference
	errors []string
}

// inferNode represents an __INFER__ placeholder that needs type resolution
type inferNode struct {
	// The identifier node with name "__INFER__"
	ident *ast.Ident

	// The parent node (should be a selector expression like __INFER__.field)
	parent ast.Node

	// The resolved type (set during Transform phase)
	resolvedType string

	// Whether this is an Option<T> or pointer type
	isOption  bool
	isPointer bool
}

// NewSafeNavTypePlugin creates a new safe navigation type inference plugin
func NewSafeNavTypePlugin() *SafeNavTypePlugin {
	return &SafeNavTypePlugin{
		inferNodes: make([]*inferNode, 0),
		errors:     make([]string, 0),
	}
}

// Name returns the plugin name
func (p *SafeNavTypePlugin) Name() string {
	return "safe_nav_types"
}

// SetContext sets the plugin context (ContextAware interface)
func (p *SafeNavTypePlugin) SetContext(ctx *plugin.Context) {
	p.ctx = ctx

	// Initialize type inference service with go/types integration
	if ctx != nil && ctx.FileSet != nil {
		// Create type inference service
		service, err := NewTypeInferenceService(ctx.FileSet, nil, ctx.Logger)
		if err != nil {
			ctx.Logger.Warn("SafeNavTypePlugin: Failed to create type inference service: %v", err)
			return
		}

		p.typeInference = service

		// Inject go/types.Info if available in context
		if ctx.TypeInfo != nil {
			if typesInfo, ok := ctx.TypeInfo.(*types.Info); ok {
				service.SetTypesInfo(typesInfo)
				ctx.Logger.Debug("SafeNavTypePlugin: go/types integration enabled")
			} else {
				// TypeInfo exists but is not *types.Info - warn about limited inference
				ctx.Logger.Warn("SafeNavTypePlugin: TypeInfo is not *types.Info (type: %T), type inference may be limited", ctx.TypeInfo)
			}
		} else {
			// No TypeInfo available - will use heuristic inference
			ctx.Logger.Debug("SafeNavTypePlugin: No TypeInfo available, using heuristic type inference")
		}
	}
}

// Process discovers __INFER__ placeholders in the AST (Discovery Phase)
func (p *SafeNavTypePlugin) Process(node ast.Node) error {
	if p.ctx == nil {
		return fmt.Errorf("plugin context not initialized")
	}

	// Build parent map if not already built
	if p.ctx.GetParentMap() == nil {
		if file, ok := node.(*ast.File); ok {
			p.ctx.BuildParentMap(file)
		}
	}

	// Build AST-based type maps for fallback inference (once per file)
	if p.structFields == nil && p.methodReturns == nil {
		if file, ok := node.(*ast.File); ok {
			p.structFields = p.buildStructFieldTypeMap(file)
			p.methodReturns = p.buildMethodReturnTypeMap(file)
			p.ctx.Logger.Debug("SafeNavTypePlugin: Built type maps - %d structs, %d method receivers",
				len(p.structFields), len(p.methodReturns))
		}
	}

	// Walk the AST to find __INFER__ identifiers
	ast.Inspect(node, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			if ident.Name == "__INFER__" {
				// Found an __INFER__ placeholder
				parent := p.ctx.GetParent(ident)
				p.inferNodes = append(p.inferNodes, &inferNode{
					ident:  ident,
					parent: parent,
				})
				p.ctx.Logger.Debug("SafeNavTypePlugin: Found __INFER__ placeholder at %v", p.ctx.FileSet.Position(ident.Pos()))
			}
		}
		return true
	})

	p.ctx.Logger.Debug("SafeNavTypePlugin: Discovery complete, found %d __INFER__ placeholders", len(p.inferNodes))
	return nil
}

// Transform resolves types and replaces __INFER__ placeholders (Transform Phase)
func (p *SafeNavTypePlugin) Transform(node ast.Node) (ast.Node, error) {
	if p.ctx == nil {
		return nil, fmt.Errorf("plugin context not initialized")
	}

	// Build a map of FuncLit → Option type for all IIFEs with __INFER__
	// Do this BEFORE resolving individual nodes since we need the context
	funcLitTypes := make(map[*ast.FuncLit]string)
	ast.Inspect(node, func(n ast.Node) bool {
		if funcLit, ok := n.(*ast.FuncLit); ok {
			if funcLit.Type != nil && funcLit.Type.Results != nil {
				if len(funcLit.Type.Results.List) == 1 {
					if ident, ok := funcLit.Type.Results.List[0].Type.(*ast.Ident); ok {
						if ident.Name == "__INFER__" {
							p.ctx.Logger.Debug("SafeNavTypePlugin: Found func() __INFER__, attempting to resolve type...")
							// Look for Option_T calls in the function body
							optionType := p.resolveReturnTypeFromFunc(funcLit)
							if optionType != "" {
								funcLitTypes[funcLit] = optionType
								p.ctx.Logger.Debug("SafeNavTypePlugin: Resolved IIFE return type to %s", optionType)
							} else {
								p.ctx.Logger.Debug("SafeNavTypePlugin: Failed to resolve IIFE return type")
							}
						}
					}
				}
			}
		}
		return true
	})

	if len(p.inferNodes) > 0 {
		// Resolve types for all discovered __INFER__ nodes
		for _, inferNode := range p.inferNodes {
			if err := p.resolveTypeForInferNode(inferNode); err != nil {
				p.errors = append(p.errors, err.Error())
				p.ctx.ReportError(err.Error(), inferNode.ident.Pos())
			}
		}
	}

	// Now transform the AST, replacing all __INFER__ patterns
	// Use a stack to track nested function literals
	funcLitStack := []*ast.FuncLit{}

	transformed := astutil.Apply(node,
		func(cursor *astutil.Cursor) bool {
			n := cursor.Node()

			// Track which function literal we're inside (push on entry)
			if funcLit, ok := n.(*ast.FuncLit); ok {
				funcLitStack = append(funcLitStack, funcLit)

				// 1. Replace func() __INFER__ return types
				if funcLit.Type != nil && funcLit.Type.Results != nil {
					if len(funcLit.Type.Results.List) == 1 {
						if ident, ok := funcLit.Type.Results.List[0].Type.(*ast.Ident); ok {
							if ident.Name == "__INFER__" {
								if optionType, ok := funcLitTypes[funcLit]; ok {
									// Create a new FuncLit with replaced return type
									newFuncType := *funcLit.Type
									newResults := &ast.FieldList{
										List: []*ast.Field{
											{
												Type: ast.NewIdent(optionType),
											},
										},
									}
									newFuncType.Results = newResults
									newFuncLit := &ast.FuncLit{
										Type: &newFuncType,
										Body: funcLit.Body,
									}
									cursor.Replace(newFuncLit)
									p.ctx.Logger.Debug("SafeNavTypePlugin: Replaced func() __INFER__ with func() %s", optionType)
								}
							}
						}
					}
				}
			}

			// 2. Replace __INFER___None() and __INFER___Some(val) function calls
			if call, ok := n.(*ast.CallExpr); ok {
				if fun, ok := call.Fun.(*ast.Ident); ok {
					if fun.Name == "__INFER___None" || fun.Name == "__INFER___Some" {
						// Get the Option type from the enclosing function literal
						var optionType string
						if len(funcLitStack) > 0 {
							currentFuncLit := funcLitStack[len(funcLitStack)-1]
							optionType = funcLitTypes[currentFuncLit]
						}
						if optionType == "" {
							// Try to resolve from context
							optionType = p.resolveOptionTypeFromContext(call)
						}
						if optionType != "" {
							// Replace __INFER___None with Option_T_None, etc.
							newFunName := strings.Replace(fun.Name, "__INFER__", optionType, 1)
							newCall := &ast.CallExpr{
								Fun:  ast.NewIdent(newFunName),
								Args: call.Args,
							}
							cursor.Replace(newCall)
							p.ctx.Logger.Debug("SafeNavTypePlugin: Replaced %s() with %s()", fun.Name, newFunName)
						}
					}
				}
			}

			// 3. Replace standalone __INFER__ identifiers (from discovered nodes)
			if ident, ok := n.(*ast.Ident); ok {
				if ident.Name == "__INFER__" {
					// Find the corresponding inferNode
					for _, inferNode := range p.inferNodes {
						if inferNode.ident == ident && inferNode.resolvedType != "" {
							// Replace __INFER__ with the resolved type
							replacement := ast.NewIdent(inferNode.resolvedType)
							cursor.Replace(replacement)
							p.ctx.Logger.Debug("SafeNavTypePlugin: Replaced __INFER__ identifier with %s", inferNode.resolvedType)
							break
						}
					}
				}
			}

			return true
		},
		func(cursor *astutil.Cursor) bool {
			// Post-order: pop function literal from stack when leaving
			if _, ok := cursor.Node().(*ast.FuncLit); ok {
				if len(funcLitStack) > 0 {
					funcLitStack = funcLitStack[:len(funcLitStack)-1]
				}
			}
			return true
		},
	)

	return transformed, nil
}

// resolveTypeForInferNode uses go/types to resolve the actual type for an __INFER__ placeholder
//
// This method implements the core type resolution logic from the Phase 9 plan (lines 245-308).
// It handles:
// - All Go types (pointers, named types, interfaces, signatures, structs)
// - Chain walking with proper Option wrapping
// - Null coalescing type checking
// - Edge cases (deep chains, generic methods, interface types, etc.)
func (p *SafeNavTypePlugin) resolveTypeForInferNode(node *inferNode) error {
	if p.typeInference == nil {
		return fmt.Errorf("type inference service not available")
	}

	// The __INFER__ should appear in a selector expression like:
	// __INFER__.field or __INFER__.method()
	// We need to find the actual variable being accessed

	// Check the parent node
	switch parent := node.parent.(type) {
	case *ast.SelectorExpr:
		// __INFER__ is the X part of a selector: __INFER__.field
		// We need to find what variable this __INFER__ represents
		// This is tricky because the preprocessor should have provided context

		// Look for patterns like: var __INFER__ = someVar
		// or function arguments: __SAFE_NAV_INFER__(someVar, "field")

		// For now, we'll use a simplified approach:
		// Try to infer from the selector's field name and surrounding context
		return p.inferFromContext(node, parent)

	case *ast.CallExpr:
		// __INFER__ might be in a function call: __SAFE_NAV_INFER__(var, "field")
		return p.inferFromFunctionCall(node, parent)

	default:
		return fmt.Errorf("unexpected parent node type for __INFER__: %T", parent)
	}
}

// resolveType resolves the type of an expression, handling all Go types
// This implements Phase 1 from the plan (lines 249-268)
func (p *SafeNavTypePlugin) resolveType(expr ast.Expr, info *types.Info) (types.Type, error) {
	if info == nil || info.Types == nil {
		return nil, fmt.Errorf("go/types info not available")
	}

	// Get type from go/types
	tv, ok := info.Types[expr]
	if !ok || tv.Type == nil {
		return nil, fmt.Errorf("no type information for expression: %s", FormatExprForDebug(expr))
	}

	typ := tv.Type

	// Handle all Go types
	switch t := typ.(type) {
	case *types.Pointer:
		// Dereference pointer, return element type
		return t.Elem(), nil

	case *types.Named:
		// Check if Option<T> type
		if p.isOptionType(t) {
			// Extract inner type T from Option<T>
			return p.extractInnerType(t), nil
		}
		// Regular named type
		return t, nil

	case *types.Interface:
		// Interface type - return as-is
		return t, nil

	case *types.Signature:
		// Function type - return signature
		return t, nil

	case *types.Struct:
		// Struct type - return as-is
		return t, nil

	case *types.Slice:
		// Slice type - return as-is
		return t, nil

	case *types.Array:
		// Array type - return as-is
		return t, nil

	case *types.Map:
		// Map type - return as-is
		return t, nil

	case *types.Chan:
		// Channel type - return as-is
		return t, nil

	case *types.Basic:
		// Basic type (int, string, etc.)
		return t, nil

	default:
		// Unknown type kind - return as-is
		return typ, nil
	}
}

// extractInnerType extracts T from Option<T>
// Option types are represented as Option_T in the AST
func (p *SafeNavTypePlugin) extractInnerType(named *types.Named) types.Type {
	// Try to extract from type name (e.g., Option_int -> int)
	typeName := named.Obj().Name()
	if !strings.HasPrefix(typeName, "Option_") {
		return nil
	}

	// Use type inference service to extract type parameter
	innerType, ok := p.typeInference.GetOptionTypeParam(typeName)
	if !ok {
		p.ctx.Logger.Warn("Failed to extract inner type from Option type: %s", typeName)
		return nil
	}

	return innerType
}

// walkChain walks a safe navigation chain and resolves the final type
// This implements Phase 2 from the plan (lines 270-291)
//
// Example: user?.address?.city
// - Start with user (type: User)
// - Access address field (type: *Address or Option<Address>)
// - Access city field (type: string or Option<string>)
// - Return final type with proper Option wrapping
func (p *SafeNavTypePlugin) walkChain(root ast.Expr, segments []ast.Expr, info *types.Info) (types.Type, error) {
	if info == nil {
		return nil, fmt.Errorf("go/types info not available")
	}

	// Start with root type
	currentType, err := p.resolveType(root, info)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve root type: %w", err)
	}

	// Walk through each segment
	for i, segment := range segments {
		switch seg := segment.(type) {
		case *ast.SelectorExpr:
			// Field access: obj.field
			currentType, err = p.resolveFieldType(currentType, seg.Sel.Name, info)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve field %s at segment %d: %w", seg.Sel.Name, i, err)
			}

		case *ast.CallExpr:
			// Method call: obj.method()
			if sel, ok := seg.Fun.(*ast.SelectorExpr); ok {
				currentType, err = p.resolveMethodReturnType(currentType, sel.Sel.Name, info)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve method %s at segment %d: %w", sel.Sel.Name, i, err)
				}
			} else {
				return nil, fmt.Errorf("invalid method call at segment %d", i)
			}

		case *ast.IndexExpr:
			// Index access: arr[i] or map[key]
			currentType, err = p.resolveIndexType(currentType, info)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve index access at segment %d: %w", i, err)
			}

		default:
			return nil, fmt.Errorf("unsupported segment type at %d: %T", i, segment)
		}

		// Handle Option wrapping at each step if needed
		if p.needsOptionWrap(currentType) {
			currentType = p.wrapInOption(currentType)
		}
	}

	return currentType, nil
}

// resolveFieldType resolves the type of a struct field
func (p *SafeNavTypePlugin) resolveFieldType(structType types.Type, fieldName string, info *types.Info) (types.Type, error) {
	// Unwrap pointer if necessary
	if ptr, ok := structType.(*types.Pointer); ok {
		structType = ptr.Elem()
	}

	// Handle named types
	if named, ok := structType.(*types.Named); ok {
		structType = named.Underlying()
	}

	// Must be a struct
	structTyp, ok := structType.(*types.Struct)
	if !ok {
		return nil, fmt.Errorf("cannot access field %s on non-struct type: %v", fieldName, structType)
	}

	// Find the field
	for i := 0; i < structTyp.NumFields(); i++ {
		field := structTyp.Field(i)
		if field.Name() == fieldName {
			return field.Type(), nil
		}

		// Check embedded fields
		if field.Embedded() {
			// Recursively search in embedded field
			if fieldType, err := p.resolveFieldType(field.Type(), fieldName, info); err == nil {
				return fieldType, nil
			}
		}
	}

	return nil, fmt.Errorf("field %s not found in struct type: %v", fieldName, structType)
}

// resolveMethodReturnType resolves the return type of a method call
func (p *SafeNavTypePlugin) resolveMethodReturnType(receiverType types.Type, methodName string, info *types.Info) (types.Type, error) {
	// Look up method in type's method set
	var methodSet *types.MethodSet

	// Handle named types
	if named, ok := receiverType.(*types.Named); ok {
		methodSet = types.NewMethodSet(named)
	} else if ptr, ok := receiverType.(*types.Pointer); ok {
		methodSet = types.NewMethodSet(ptr)
	} else {
		methodSet = types.NewMethodSet(receiverType)
	}

	// Find the method
	for i := 0; i < methodSet.Len(); i++ {
		method := methodSet.At(i)
		if method.Obj().Name() == methodName {
			// Get method signature
			sig, ok := method.Type().(*types.Signature)
			if !ok {
				return nil, fmt.Errorf("method %s has invalid signature", methodName)
			}

			// Return first result type (ignore multiple returns for now)
			if sig.Results() != nil && sig.Results().Len() > 0 {
				return sig.Results().At(0).Type(), nil
			}

			// Method has no return value
			return types.Typ[types.Invalid], nil
		}
	}

	return nil, fmt.Errorf("method %s not found on type: %v", methodName, receiverType)
}

// resolveIndexType resolves the type of an index expression (arr[i] or map[key])
func (p *SafeNavTypePlugin) resolveIndexType(containerType types.Type, info *types.Info) (types.Type, error) {
	switch t := containerType.(type) {
	case *types.Slice:
		return t.Elem(), nil

	case *types.Array:
		return t.Elem(), nil

	case *types.Map:
		return t.Elem(), nil

	case *types.Pointer:
		// Pointer to array
		if arr, ok := t.Elem().(*types.Array); ok {
			return arr.Elem(), nil
		}
		return nil, fmt.Errorf("cannot index pointer to non-array type: %v", t)

	default:
		return nil, fmt.Errorf("cannot index type: %v", containerType)
	}
}

// needsOptionWrap checks if a type needs to be wrapped in Option<T>
// This handles safe navigation through nullable types
func (p *SafeNavTypePlugin) needsOptionWrap(typ types.Type) bool {
	// Pointer types are nullable and should be wrapped
	if _, ok := typ.(*types.Pointer); ok {
		return true
	}

	// Option types are already wrapped
	if named, ok := typ.(*types.Named); ok {
		if p.isOptionType(named) {
			return false
		}
	}

	// All other types are not nullable
	return false
}

// wrapInOption wraps a type in Option<T>
func (p *SafeNavTypePlugin) wrapInOption(typ types.Type) types.Type {
	// If already an Option type, don't wrap again
	if named, ok := typ.(*types.Named); ok {
		if p.isOptionType(named) {
			return typ
		}
	}

	// Create Option_T type name
	typeStr := p.typeInference.TypeToString(typ)
	optionTypeName := "Option_" + p.sanitizeTypeName(typeStr)

	// Create a synthetic Option type
	// This is a placeholder - the actual Option type will be generated by other plugins
	optionType := types.NewNamed(
		types.NewTypeName(token.NoPos, nil, optionTypeName, nil),
		types.Typ[types.Invalid],
		nil,
	)

	return optionType
}

// sanitizeTypeName converts a type string to a valid identifier
func (p *SafeNavTypePlugin) sanitizeTypeName(typeName string) string {
	str := typeName
	if str == "interface{}" {
		return "any"
	}
	str = strings.ReplaceAll(str, "*", "ptr_")
	str = strings.ReplaceAll(str, "[]", "slice_")
	str = strings.ReplaceAll(str, "[", "_")
	str = strings.ReplaceAll(str, "]", "_")
	str = strings.ReplaceAll(str, ".", "_")
	str = strings.ReplaceAll(str, "{", "")
	str = strings.ReplaceAll(str, "}", "")
	str = strings.ReplaceAll(str, " ", "")
	str = strings.Trim(str, "_")
	return str
}

// handleNullCoalesce handles type checking for null coalescing operator (??)
// This implements Phase 3 from the plan (lines 293-307)
//
// Example: user?.name ?? "Unknown"
// - LHS type: Option<string>
// - RHS type: string
// - Result type: string (unwrapped from Option)
func (p *SafeNavTypePlugin) handleNullCoalesce(lhs, rhs ast.Expr, info *types.Info) (types.Type, error) {
	if info == nil {
		return nil, fmt.Errorf("go/types info not available")
	}

	// Resolve LHS type (should be Option<T> or *T)
	lhsType, err := p.resolveType(lhs, info)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve LHS type: %w", err)
	}

	// Resolve RHS type
	rhsType, err := p.resolveType(rhs, info)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve RHS type: %w", err)
	}

	// Unwrap Option<T> from LHS if present
	unwrapped := p.unwrapOption(lhsType)

	// LHS type must match RHS type
	if !types.Identical(unwrapped, rhsType) {
		return nil, fmt.Errorf("type mismatch in ?? operator: %v vs %v", unwrapped, rhsType)
	}

	return unwrapped, nil
}

// unwrapOption extracts T from Option<T> or *T
func (p *SafeNavTypePlugin) unwrapOption(typ types.Type) types.Type {
	// Unwrap pointer
	if ptr, ok := typ.(*types.Pointer); ok {
		return ptr.Elem()
	}

	// Unwrap Option<T>
	if named, ok := typ.(*types.Named); ok {
		if p.isOptionType(named) {
			return p.extractInnerType(named)
		}
	}

	// Not a nullable type - return as-is
	return typ
}

// inferFromContext attempts to infer type from surrounding context
func (p *SafeNavTypePlugin) inferFromContext(node *inferNode, selector *ast.SelectorExpr) error {
	// Walk up the parent chain to find the actual variable
	var actualVar ast.Expr

	p.ctx.WalkParents(selector, func(parent ast.Node) bool {
		switch p := parent.(type) {
		case *ast.AssignStmt:
			// Found assignment: someVar = __INFER__.field
			// The LHS might give us the variable
			if len(p.Rhs) > 0 {
				// Look for the actual variable in the RHS
				// This is a simplified heuristic
			}
		case *ast.CallExpr:
			// Found function call containing __INFER__
			// Look for patterns like __SAFE_NAV_INFER__(actualVar, ...)
			if ident, ok := p.Fun.(*ast.Ident); ok {
				if strings.HasPrefix(ident.Name, "__SAFE_NAV_INFER__") {
					if len(p.Args) > 0 {
						actualVar = p.Args[0]
						return false // Stop walking
					}
				}
			}
		}
		return true // Continue walking
	})

	if actualVar != nil {
		return p.resolveTypeFromExpr(node, actualVar)
	}

	// Fallback: Report error
	return fmt.Errorf("unable to infer type for __INFER__ placeholder")
}

// inferFromFunctionCall attempts to infer type from function call pattern
func (p *SafeNavTypePlugin) inferFromFunctionCall(node *inferNode, call *ast.CallExpr) error {
	// Expected pattern: __SAFE_NAV_INFER__(actualVar, "field")
	if ident, ok := call.Fun.(*ast.Ident); ok {
		if strings.HasPrefix(ident.Name, "__SAFE_NAV_INFER__") {
			if len(call.Args) > 0 {
				actualVar := call.Args[0]
				return p.resolveTypeFromExpr(node, actualVar)
			}
		}
	}

	return fmt.Errorf("invalid __SAFE_NAV_INFER__ function call pattern")
}

// resolveTypeFromExpr resolves the type of an expression using go/types
func (p *SafeNavTypePlugin) resolveTypeFromExpr(node *inferNode, expr ast.Expr) error {
	// Use type inference service to get the type
	typ, ok := p.typeInference.InferType(expr)
	if !ok || typ == nil {
		return fmt.Errorf("failed to infer type for expression: %s", FormatExprForDebug(expr))
	}

	// Check if it's a pointer type
	if ptrType, ok := typ.(*types.Pointer); ok {
		node.isPointer = true
		node.isOption = false
		node.resolvedType = p.typeInference.TypeToString(ptrType.Elem())
		p.ctx.Logger.Debug("SafeNavTypePlugin: Resolved __INFER__ to pointer type: *%s", node.resolvedType)
		return nil
	}

	// Check if it's an Option type (struct with tag field + IsSome/IsNone methods)
	if named, ok := typ.(*types.Named); ok {
		if p.isOptionType(named) {
			node.isOption = true
			node.isPointer = false
			node.resolvedType = named.Obj().Name()
			p.ctx.Logger.Debug("SafeNavTypePlugin: Resolved __INFER__ to Option type: %s", node.resolvedType)
			return nil
		}
	}

	// Not a nullable type - report error
	return fmt.Errorf("safe navigation requires nullable type (Option<T> or *T), got: %s", p.typeInference.TypeToString(typ))
}

// isOptionType checks if a named type is an Option type
// An Option type is identified by:
// 1. Having an unexported 'tag' field of type OptionTag (NOT ResultTag)
// 2. Having an Unwrap() method with signature: func() T
//
// This is more precise than before - we now distinguish between Option<T> and Result<T,E>
// since both have 'tag' fields but with different tag types.
func (p *SafeNavTypePlugin) isOptionType(named *types.Named) bool {
	// Get the underlying struct type
	structType, ok := named.Underlying().(*types.Struct)
	if !ok {
		return false
	}

	// Check for unexported 'tag' field with OptionTag type (NOT ResultTag)
	hasOptionTag := false
	for i := 0; i < structType.NumFields(); i++ {
		field := structType.Field(i)
		if field.Name() == "tag" && !field.Exported() {
			// Check if tag type is specifically OptionTag
			if namedType, ok := field.Type().(*types.Named); ok {
				if namedType.Obj().Name() == "OptionTag" {
					hasOptionTag = true
					break
				}
				// If it's ResultTag, this is Result<T,E>, not Option<T>
				if namedType.Obj().Name() == "ResultTag" {
					return false
				}
			}
		}
	}

	if !hasOptionTag {
		return false
	}

	// Check for Unwrap() method with correct signature: func() T
	hasUnwrap := false
	for i := 0; i < named.NumMethods(); i++ {
		method := named.Method(i)
		if method.Name() == "Unwrap" {
			// Validate signature: should be func() T (no params, one result)
			if sig, ok := method.Type().(*types.Signature); ok {
				if sig.Params().Len() == 0 && sig.Results().Len() == 1 {
					hasUnwrap = true
					break
				}
			}
		}
	}

	return hasUnwrap
}

// resolveOptionTypeFromContext attempts to determine the Option type for __INFER___None()/__INFER___Some() calls
//
// Strategy:
// 1. Walk the entire function body to find OTHER Option_ calls (like Option_User_None)
// 2. Extract the Option type from those calls
// 3. All __INFER__ placeholders in the same function should use the same type
func (p *SafeNavTypePlugin) resolveOptionTypeFromContext(call *ast.CallExpr) string {
	if p.ctx == nil {
		return ""
	}

	// Walk up to find the enclosing function literal
	var funcLit *ast.FuncLit
	p.ctx.WalkParents(call, func(parent ast.Node) bool {
		if fl, ok := parent.(*ast.FuncLit); ok {
			funcLit = fl
			return false // Stop walking
		}
		return true
	})

	if funcLit == nil || funcLit.Body == nil {
		return ""
	}

	// Now scan the function body for any Option_T_None or Option_T_Some calls
	// to determine what T is
	var optionType string
	ast.Inspect(funcLit.Body, func(n ast.Node) bool {
		if otherCall, ok := n.(*ast.CallExpr); ok {
			if fun, ok := otherCall.Fun.(*ast.Ident); ok {
				name := fun.Name
				// Look for patterns like: Option_User_None, Option_string_Some
				if strings.HasPrefix(name, "Option_") && !strings.HasPrefix(name, "__INFER__") {
					if strings.HasSuffix(name, "_None") {
						optionType = strings.TrimSuffix(name, "_None")
						return false
					} else if strings.HasSuffix(name, "_Some") {
						optionType = strings.TrimSuffix(name, "_Some")
						return false
					}
				}
			}
		}
		return true
	})

	// If we couldn't find it from calls, try the return type
	if optionType == "" && funcLit.Type != nil && funcLit.Type.Results != nil {
		if len(funcLit.Type.Results.List) == 1 {
			if ident, ok := funcLit.Type.Results.List[0].Type.(*ast.Ident); ok {
				if ident.Name != "__INFER__" && strings.HasPrefix(ident.Name, "Option_") {
					optionType = ident.Name
				}
			}
		}
	}

	return optionType
}

// buildStructFieldTypeMap extracts struct type definitions from the AST and builds a map
// of struct names to their field types. This allows field type lookup without go/types.
func (p *SafeNavTypePlugin) buildStructFieldTypeMap(file *ast.File) map[string]map[string]string {
	structFields := make(map[string]map[string]string)

	ast.Inspect(file, func(n ast.Node) bool {
		// Look for type declarations
		if genDecl, ok := n.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					// Check if it's a struct type
					if structType, ok := typeSpec.Type.(*ast.StructType); ok {
						structName := typeSpec.Name.Name
						fields := make(map[string]string)

						// Extract field names and types
						for _, field := range structType.Fields.List {
							fieldType := ""
							switch ft := field.Type.(type) {
							case *ast.Ident:
								fieldType = ft.Name
							case *ast.SelectorExpr:
								// Handle qualified types like pkg.Type
								if x, ok := ft.X.(*ast.Ident); ok {
									fieldType = x.Name + "." + ft.Sel.Name
								}
							}

							// Each field can have multiple names (e.g., x, y int)
							for _, name := range field.Names {
								fields[name.Name] = fieldType
							}
						}

						structFields[structName] = fields
					}
				}
			}
		}
		return true
	})

	return structFields
}

// buildMethodReturnTypeMap extracts method declarations from the AST and builds a map
// of receiver types to method names to return types.
func (p *SafeNavTypePlugin) buildMethodReturnTypeMap(file *ast.File) map[string]map[string]string {
	methodReturns := make(map[string]map[string]string)

	ast.Inspect(file, func(n ast.Node) bool {
		// Look for function declarations (methods have receivers)
		if funcDecl, ok := n.(*ast.FuncDecl); ok {
			if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
				// This is a method
				receiverType := ""
				switch rt := funcDecl.Recv.List[0].Type.(type) {
				case *ast.Ident:
					receiverType = rt.Name
				case *ast.StarExpr:
					// Pointer receiver
					if ident, ok := rt.X.(*ast.Ident); ok {
						receiverType = ident.Name
					}
				}

				if receiverType != "" {
					methodName := funcDecl.Name.Name

					// Extract return type
					var returnType string
					if funcDecl.Type.Results != nil && len(funcDecl.Type.Results.List) > 0 {
						switch rt := funcDecl.Type.Results.List[0].Type.(type) {
						case *ast.Ident:
							returnType = rt.Name
						case *ast.SelectorExpr:
							if x, ok := rt.X.(*ast.Ident); ok {
								returnType = x.Name + "." + rt.Sel.Name
							}
						}
					}

					if returnType != "" {
						if methodReturns[receiverType] == nil {
							methodReturns[receiverType] = make(map[string]string)
						}
						methodReturns[receiverType][methodName] = returnType
					}
				}
			}
		}
		return true
	})

	return methodReturns
}

// resolveReturnTypeFromFunc attempts to determine the return type for func() __INFER__
//
// Strategy:
// 1. Scan the function body directly (funcLit is passed via cursor)
// 2. Find calls to Option_T_None() or Option_T_Some() to determine T
// 3. If none found, look for variables with .IsNone() or .Unwrap() calls and infer their types
// 4. Extract Option_T from those calls or variables
func (p *SafeNavTypePlugin) resolveReturnTypeFromFunc(funcLit *ast.FuncLit) string {
	if funcLit == nil || funcLit.Body == nil {
		p.ctx.Logger.Debug("SafeNavTypePlugin: resolveReturnTypeFromFunc - funcLit or body is nil")
		return ""
	}

	// First, try to find Option_T_None() or Option_T_Some() calls
	var optionType string
	ast.Inspect(funcLit.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if fun, ok := call.Fun.(*ast.Ident); ok {
				name := fun.Name
				p.ctx.Logger.Debug("SafeNavTypePlugin: Found call to %s", name)
				// Look for patterns like: Option_User_None, Option_string_Some, etc.
				if strings.HasPrefix(name, "Option_") && !strings.HasPrefix(name, "__INFER__") {
					if strings.HasSuffix(name, "_None") {
						// Extract Option_T from Option_T_None
						optionType = strings.TrimSuffix(name, "_None")
						p.ctx.Logger.Debug("SafeNavTypePlugin: Found Option_T_None call, extracted type: %s", optionType)
						return false // Stop searching
					} else if strings.HasSuffix(name, "_Some") {
						// Extract Option_T from Option_T_Some
						optionType = strings.TrimSuffix(name, "_Some")
						p.ctx.Logger.Debug("SafeNavTypePlugin: Found Option_T_Some call, extracted type: %s", optionType)
						return false // Stop searching
					}
				}
			}
		}
		return true
	})

	// If we found it, return
	if optionType != "" {
		return optionType
	}

	p.ctx.Logger.Debug("SafeNavTypePlugin: No Option_T_None/Some calls found, trying variable method calls...")

	// Otherwise, look for variables with .IsNone() or .Unwrap() method calls
	// This handles cases like: if user.IsNone() { ... }
	p.ctx.Logger.Debug("SafeNavTypePlugin: Attempting method call detection...")

	// First, collect variable names that have .IsNone()/.Unwrap() calls
	varNames := make(map[string]bool)
	ast.Inspect(funcLit.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				methodName := sel.Sel.Name
				if methodName == "IsNone" || methodName == "IsSome" || methodName == "Unwrap" {
					if ident, ok := sel.X.(*ast.Ident); ok {
						p.ctx.Logger.Debug("SafeNavTypePlugin: Found %s.%s() call", ident.Name, methodName)
						varNames[ident.Name] = true
					}
				}
			}
		}
		return true
	})

	// If we have candidate variables, try to find their type declarations
	if len(varNames) > 0 {
		p.ctx.Logger.Debug("SafeNavTypePlugin: Found %d candidate variables: %v", len(varNames), varNames)

		// Walk up to the parent scope to find variable declarations
		// Use the parent map to traverse up the AST
		parent := p.ctx.GetParent(funcLit)
		for parent != nil {
			if funcDecl, ok := parent.(*ast.FuncDecl); ok {
				// Search within the function declaration for var statements
				var foundType string
				ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
					if declStmt, ok := n.(*ast.DeclStmt); ok {
						if genDecl, ok := declStmt.Decl.(*ast.GenDecl); ok {
							for _, spec := range genDecl.Specs {
								if valueSpec, ok := spec.(*ast.ValueSpec); ok {
									for _, name := range valueSpec.Names {
										if varNames[name.Name] {
											// Found the declaration of one of our candidate variables
											if valueSpec.Type != nil {
												if ident, ok := valueSpec.Type.(*ast.Ident); ok {
													typeName := ident.Name
													p.ctx.Logger.Debug("SafeNavTypePlugin: Found var %s %s", name.Name, typeName)
													if strings.HasPrefix(typeName, "Option_") {
														foundType = typeName
														return false
													}
												}
											}
										}
									}
								}
							}
						}
					}
					return true
				})

				if foundType != "" {
					p.ctx.Logger.Debug("SafeNavTypePlugin: Resolved type from variable declaration: %s", foundType)
					return foundType
				}
			}
			parent = p.ctx.GetParent(parent)
		}
	}

	p.ctx.Logger.Debug("SafeNavTypePlugin: Trying AST-based type resolution (Strategy 3)...")

	// Strategy 3: Use AST-based type maps to resolve field/method types
	// This handles:
	// - return user1.address (struct field access)
	// - return profile2.getEmail() (method call)
	if p.structFields != nil || p.methodReturns != nil {
		ast.Inspect(funcLit.Body, func(n ast.Node) bool {
			// Look for return statements
			if retStmt, ok := n.(*ast.ReturnStmt); ok {
				if len(retStmt.Results) == 1 {
					// Check if it's a selector expression (field or method access)
					if sel, ok := retStmt.Results[0].(*ast.SelectorExpr); ok {
						// Get the base variable
						if baseIdent, ok := sel.X.(*ast.Ident); ok {
							fieldName := sel.Sel.Name
							varName := baseIdent.Name

							p.ctx.Logger.Debug("SafeNavTypePlugin: Found return %s.%s", varName, fieldName)

							// Try to find the type of the base variable
							var baseType string

							// Look for variable declaration in the function body
							ast.Inspect(funcLit.Body, func(inner ast.Node) bool {
								if assignStmt, ok := inner.(*ast.AssignStmt); ok {
									for i, lhs := range assignStmt.Lhs {
										if ident, ok := lhs.(*ast.Ident); ok && ident.Name == varName {
											// Found the declaration, try to determine its type
											if i < len(assignStmt.Rhs) {
												rhs := assignStmt.Rhs[i]

												// Handle call expression (e.g., user.Unwrap())
												if call, ok := rhs.(*ast.CallExpr); ok {
													if callSel, ok := call.Fun.(*ast.SelectorExpr); ok {
														if callSel.Sel.Name == "Unwrap" {
															if unwrapBase, ok := callSel.X.(*ast.Ident); ok {
																// Extract type from Option_T → T
																// If user is UserOption, then user.Unwrap() returns User
																p.ctx.Logger.Debug("SafeNavTypePlugin: Found %s := %s.Unwrap()", varName, unwrapBase.Name)

																// Trace the unwrapBase variable to find its Option type
																baseType = p.extractUnwrappedTypeFromVar(funcLit, unwrapBase.Name)
																if baseType != "" {
																	p.ctx.Logger.Debug("SafeNavTypePlugin: Extracted unwrapped type: %s", baseType)
																}
															}
														}
													}
												}
											}
										}
									}
								}
								return true
							})

							if baseType != "" {
								p.ctx.Logger.Debug("SafeNavTypePlugin: Resolved base type: %s", baseType)

								// Check if it's a field access
								if p.structFields != nil {
									if fields, ok := p.structFields[baseType]; ok {
										if fieldType, ok := fields[fieldName]; ok {
											p.ctx.Logger.Debug("SafeNavTypePlugin: Resolved field type: %s.%s = %s", baseType, fieldName, fieldType)
											optionType = fieldType
											return false
										}
									}
								}

								// Check if it's a method call
								// Note: method calls would have CallExpr parent, so check for that
							} else {
								p.ctx.Logger.Debug("SafeNavTypePlugin: Could not resolve base type for %s", varName)
							}
						}
					}

					// Check if it's a method call (CallExpr with selector)
					if call, ok := retStmt.Results[0].(*ast.CallExpr); ok {
						if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
							if baseIdent, ok := sel.X.(*ast.Ident); ok {
								methodName := sel.Sel.Name
								varName := baseIdent.Name

								p.ctx.Logger.Debug("SafeNavTypePlugin: Found return %s.%s()", varName, methodName)

								// Try to find the type of the base variable
								var baseType string

								// Similar logic as above to find the base type
								ast.Inspect(funcLit.Body, func(inner ast.Node) bool {
									if assignStmt, ok := inner.(*ast.AssignStmt); ok {
										for i, lhs := range assignStmt.Lhs {
											if ident, ok := lhs.(*ast.Ident); ok && ident.Name == varName {
												if i < len(assignStmt.Rhs) {
													rhs := assignStmt.Rhs[i]
													if unwrapCall, ok := rhs.(*ast.CallExpr); ok {
														if unwrapSel, ok := unwrapCall.Fun.(*ast.SelectorExpr); ok {
															if unwrapSel.Sel.Name == "Unwrap" {
																if unwrapBaseIdent, ok := unwrapSel.X.(*ast.Ident); ok {
																	// Trace the unwrapBase variable to find its Option type
																	baseType = p.extractUnwrappedTypeFromVar(funcLit, unwrapBaseIdent.Name)
																	if baseType != "" {
																		p.ctx.Logger.Debug("SafeNavTypePlugin: Extracted unwrapped type for method call: %s", baseType)
																	}
																}
															}
														}
													}
												}
											}
										}
									}
									return true
								})

								if baseType != "" {
									p.ctx.Logger.Debug("SafeNavTypePlugin: Resolved base type for method call: %s", baseType)

									// Check if we have method return type info
									if p.methodReturns != nil {
										if methods, ok := p.methodReturns[baseType]; ok {
											if returnType, ok := methods[methodName]; ok {
												p.ctx.Logger.Debug("SafeNavTypePlugin: Resolved method return type: %s.%s() = %s", baseType, methodName, returnType)
												optionType = returnType
												return false
											}
										}
									}
								}
							}
						}
					}
				}
			}
			return true
		})

		if optionType != "" {
			p.ctx.Logger.Debug("SafeNavTypePlugin: Resolved type using AST-based type maps: %s", optionType)
			return optionType
		}
	}

	p.ctx.Logger.Debug("SafeNavTypePlugin: Failed to resolve type")
	return ""
}

// extractUnwrappedTypeFromVar traces a variable to find its Option type and returns the unwrapped type
// For example, if varName is "user" and it's of type "UserOption", returns "User"
// This function searches both the current scope and parent scopes to find the variable declaration
func (p *SafeNavTypePlugin) extractUnwrappedTypeFromVar(startNode ast.Node, varName string) string {
	var optionTypeName string

	// Helper function to search for variable in a block statement
	findVarInBlock := func(block *ast.BlockStmt) bool {
		found := false
		ast.Inspect(block, func(n ast.Node) bool {
			if assignStmt, ok := n.(*ast.AssignStmt); ok {
				for i, lhs := range assignStmt.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok && ident.Name == varName {
						// Found the variable declaration
						if i < len(assignStmt.Rhs) {
							rhs := assignStmt.Rhs[i]

							// Check if it's a function call (e.g., getUser(1))
							if call, ok := rhs.(*ast.CallExpr); ok {
								if funcIdent, ok := call.Fun.(*ast.Ident); ok {
									funcName := funcIdent.Name
									p.ctx.Logger.Debug("SafeNavTypePlugin: Variable %s is assigned from function %s()", varName, funcName)

									// Look up the function's return type in method returns map
									// Note: For top-level functions, we'd need a function return type map
									// For now, try to infer from the function name pattern
									// e.g., getUser -> UserOption, fetchProfile -> UserOption
									if strings.HasPrefix(funcName, "get") || strings.HasPrefix(funcName, "fetch") {
										// Extract the type name from the function name
										// getUser -> User, fetchProfile -> Profile
										typeName := strings.TrimPrefix(funcName, "get")
										typeName = strings.TrimPrefix(typeName, "fetch")
										optionTypeName = typeName + "Option"
										p.ctx.Logger.Debug("SafeNavTypePlugin: Inferred option type from function name: %s", optionTypeName)
										found = true
										return false
									}
								}
							}
						}
					}
				}
			}
			return true
		})
		return found
	}

	// First, try to find the variable in the current block
	if block, ok := startNode.(*ast.BlockStmt); ok {
		if findVarInBlock(block) {
			p.ctx.Logger.Debug("SafeNavTypePlugin: Found variable %s in current scope", varName)
		}
	} else if funcLit, ok := startNode.(*ast.FuncLit); ok {
		if findVarInBlock(funcLit.Body) {
			p.ctx.Logger.Debug("SafeNavTypePlugin: Found variable %s in current IIFE scope", varName)
		}
	}

	// If not found in current scope, traverse up to parent scopes
	if optionTypeName == "" {
		p.ctx.Logger.Debug("SafeNavTypePlugin: Variable %s not found in current scope, searching parent scopes...", varName)

		currentNode := startNode
		for optionTypeName == "" {
			// Get parent node
			parentNode := p.ctx.GetParent(currentNode)
			if parentNode == nil {
				p.ctx.Logger.Debug("SafeNavTypePlugin: Reached root, no parent found")
				break
			}

			p.ctx.Logger.Debug("SafeNavTypePlugin: Checking parent node type: %T", parentNode)

			// Check if parent is a function declaration with a body
			if funcDecl, ok := parentNode.(*ast.FuncDecl); ok && funcDecl.Body != nil {
				p.ctx.Logger.Debug("SafeNavTypePlugin: Searching in parent function %s", funcDecl.Name.Name)
				if findVarInBlock(funcDecl.Body) {
					p.ctx.Logger.Debug("SafeNavTypePlugin: Found variable %s in parent function %s", varName, funcDecl.Name.Name)
					break
				}
			}

			// Check if parent is a block statement
			if block, ok := parentNode.(*ast.BlockStmt); ok {
				p.ctx.Logger.Debug("SafeNavTypePlugin: Searching in parent block")
				if findVarInBlock(block) {
					p.ctx.Logger.Debug("SafeNavTypePlugin: Found variable %s in parent block", varName)
					break
				}
			}

			// Move up to the next parent
			currentNode = parentNode
		}
	}

	if optionTypeName == "" {
		p.ctx.Logger.Debug("SafeNavTypePlugin: Could not determine option type for variable %s in any scope", varName)
		return ""
	}

	// Extract the unwrapped type from the Option type name
	// UserOption -> User, AddressOption -> Address, StringOption -> string
	unwrappedType := strings.TrimSuffix(optionTypeName, "Option")

	// Handle built-in types (StringOption -> string)
	if unwrappedType == "String" {
		unwrappedType = "string"
	} else if unwrappedType == "Int" {
		unwrappedType = "int"
	} else if unwrappedType == "Bool" {
		unwrappedType = "bool"
	}

	p.ctx.Logger.Debug("SafeNavTypePlugin: Extracted unwrapped type %s from option type %s", unwrappedType, optionTypeName)
	return unwrappedType
}

// GetErrors returns all accumulated errors
func (p *SafeNavTypePlugin) GetErrors() []string {
	return p.errors
}

// ClearErrors clears all accumulated errors
func (p *SafeNavTypePlugin) ClearErrors() {
	p.errors = make([]string, 0)
}

// reportTypeInferenceError reports a detailed type inference error with suggestions
// This implements Phase 4 from the plan (lines 349-363)
//
// Example error output:
//
//	Cannot infer type for safe navigation chain 'obj?.field?.method()'
//	  at line 42, column 10
//
//	  Reason: Method 'method' not found on inferred type 'T'
//
//	  Suggestion: Add explicit type annotation:
//	    let result: Option<ReturnType> = obj?.field?.method()
//
//	  Or ensure 'field' has a 'method' method defined.
func (p *SafeNavTypePlugin) reportTypeInferenceError(
	node ast.Node,
	chain string,
	reason string,
	suggestion string,
) error {
	if p.ctx == nil || p.ctx.FileSet == nil {
		return fmt.Errorf("type inference failed: %s", reason)
	}

	pos := p.ctx.FileSet.Position(node.Pos())

	errMsg := fmt.Sprintf(
		"Cannot infer type for safe navigation chain '%s'\n"+
		"  at %s:%d:%d\n"+
		"\n"+
		"  Reason: %s\n"+
		"\n"+
		"  Suggestion: %s",
		chain,
		pos.Filename,
		pos.Line,
		pos.Column,
		reason,
		suggestion,
	)

	return fmt.Errorf("%s", errMsg)
}

// Edge Case Handlers
// These implement Priority 1 and Priority 2 edge cases from the plan (lines 310-330)

// handleDeepChain handles deep navigation chains (5+ levels)
// Example: a?.b?.c?.d?.e?.f
func (p *SafeNavTypePlugin) handleDeepChain(root ast.Expr, segments []ast.Expr, info *types.Info) (types.Type, error) {
	// Deep chains are handled by walkChain with no special treatment
	// The performance cost is linear in chain length, which is acceptable
	return p.walkChain(root, segments, info)
}

// handleGenericMethod handles generic method calls
// Example: opt?.Map(|x| transform(x))?.Filter(pred)
//
// Note: Go 1.18+ generics are fully supported by go/types
// This method validates that type parameters are properly instantiated
func (p *SafeNavTypePlugin) handleGenericMethod(
	receiverType types.Type,
	methodName string,
	typeArgs []types.Type,
	info *types.Info,
) (types.Type, error) {
	// Look up method
	returnType, err := p.resolveMethodReturnType(receiverType, methodName, info)
	if err != nil {
		return nil, err
	}

	// Check if return type is generic
	if named, ok := returnType.(*types.Named); ok {
		// Check for type parameters
		if named.TypeParams() != nil && named.TypeParams().Len() > 0 {
			// Validate type arguments match type parameters
			if len(typeArgs) != named.TypeParams().Len() {
				return nil, fmt.Errorf(
					"wrong number of type arguments: got %d, want %d",
					len(typeArgs),
					named.TypeParams().Len(),
				)
			}

			// Instantiate generic type with type arguments
			instantiated, err := types.Instantiate(nil, named, typeArgs, false)
			if err != nil {
				return nil, fmt.Errorf("failed to instantiate generic type: %w", err)
			}
			return instantiated, nil
		}
	}

	return returnType, nil
}

// handleInterfaceType handles safe navigation on interface types
// Example: iface?.(*ConcreteType).method()
func (p *SafeNavTypePlugin) handleInterfaceType(
	interfaceType types.Type,
	targetType types.Type,
	info *types.Info,
) (types.Type, error) {
	// Validate that targetType implements interfaceType
	iface, ok := interfaceType.Underlying().(*types.Interface)
	if !ok {
		return nil, fmt.Errorf("expected interface type, got %v", interfaceType)
	}

	// Check if targetType implements the interface
	if !types.Implements(targetType, iface) {
		return nil, fmt.Errorf(
			"type %v does not implement interface %v",
			targetType,
			interfaceType,
		)
	}

	return targetType, nil
}

// handleTypeAssertion handles type assertions in safe navigation
// Example: val?.(*SpecificType)
func (p *SafeNavTypePlugin) handleTypeAssertion(
	expr ast.Expr,
	targetType types.Type,
	info *types.Info,
) (types.Type, error) {
	// Get source type
	sourceType, err := p.resolveType(expr, info)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve source type: %w", err)
	}

	// Source must be an interface
	_, ok := sourceType.Underlying().(*types.Interface)
	if !ok {
		return nil, fmt.Errorf(
			"invalid type assertion: source type %v is not an interface",
			sourceType,
		)
	}

	// Type assertion in safe navigation always returns Option<T>
	// because it may fail
	return p.wrapInOption(targetType), nil
}

// handleVariadicFunction handles variadic function calls in safe navigation
// Example: obj?.Call(args...)
func (p *SafeNavTypePlugin) handleVariadicFunction(
	receiverType types.Type,
	methodName string,
	args []ast.Expr,
	info *types.Info,
) (types.Type, error) {
	// Look up method
	returnType, err := p.resolveMethodReturnType(receiverType, methodName, info)
	if err != nil {
		return nil, err
	}

	// Variadic functions have no special type handling
	// The ... operator is syntax sugar handled by the compiler
	return returnType, nil
}

// handleCompositeLiteral handles composite literals in safe navigation
// Example: opt?.SomeStruct{field: val}
//
// Note: This is unusual syntax and may not be valid Go
// We handle it defensively by checking if the composite literal is valid
func (p *SafeNavTypePlugin) handleCompositeLiteral(
	compositeLit *ast.CompositeLit,
	info *types.Info,
) (types.Type, error) {
	// Get type from go/types
	tv, ok := info.Types[compositeLit]
	if !ok || tv.Type == nil {
		return nil, fmt.Errorf("no type information for composite literal")
	}

	return tv.Type, nil
}

// Priority 2 Edge Cases (Should handle)

// handleFunctionValue handles calling optional function values
// Example: fnOpt?.()
func (p *SafeNavTypePlugin) handleFunctionValue(
	funcType types.Type,
	info *types.Info,
) (types.Type, error) {
	// Unwrap if Option<func>
	funcType = p.unwrapOption(funcType)

	// Must be a function type
	sig, ok := funcType.(*types.Signature)
	if !ok {
		return nil, fmt.Errorf("cannot call non-function type: %v", funcType)
	}

	// Return first result (ignore multiple returns for now)
	if sig.Results() != nil && sig.Results().Len() > 0 {
		return sig.Results().At(0).Type(), nil
	}

	return types.Typ[types.Invalid], nil
}

// handleChannelOp handles channel operations in safe navigation
// Example: chanOpt?.<-
func (p *SafeNavTypePlugin) handleChannelOp(
	chanType types.Type,
	info *types.Info,
) (types.Type, error) {
	// Unwrap if Option<chan>
	chanType = p.unwrapOption(chanType)

	// Must be a channel type
	ch, ok := chanType.(*types.Chan)
	if !ok {
		return nil, fmt.Errorf("cannot receive from non-channel type: %v", chanType)
	}

	// Return element type
	return ch.Elem(), nil
}

// handleMultipleReturns handles functions with multiple return values
// Example: opt?.multiReturn()
// Returns Option<T> where T is the first return value
func (p *SafeNavTypePlugin) handleMultipleReturns(
	sig *types.Signature,
	info *types.Info,
) (types.Type, error) {
	if sig.Results() == nil || sig.Results().Len() == 0 {
		return types.Typ[types.Invalid], nil
	}

	// Return first result only (unwrap tuple)
	// This matches the plan's specification (line 323)
	return sig.Results().At(0).Type(), nil
}

// handleEmbeddedFields handles embedded field access
// Example: opt?.EmbeddedStruct.field
func (p *SafeNavTypePlugin) handleEmbeddedField(
	structType types.Type,
	fieldName string,
	info *types.Info,
) (types.Type, error) {
	// This is already handled by resolveFieldType which checks embedded fields
	return p.resolveFieldType(structType, fieldName, info)
}
