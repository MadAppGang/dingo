// Package builtin provides None constant type inference plugin
package builtin

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"github.com/MadAppGang/dingo/pkg/plugin"
	"golang.org/x/tools/go/ast/astutil"
)

// NoneContextPlugin infers types for None constants from surrounding context
//
// This plugin implements context-aware type inference for the None constant.
// None represents the absence of a value in Option<T>, but the type T must be
// inferred from context.
//
// Valid contexts (in precedence order):
// 1. Explicit type annotation: let x: Option<int> = None
// 2. Return statement: return None (from function signature)
// 3. Function call argument: processAge(None) (from parameter type)
// 4. Struct field: User{ age: None } (from field type)
// 5. Assignment target: x = None (from variable type)
// 6. Match arm: match { _ => None } (from other arms)
//
// If no valid context is found, the plugin emits a compile error requiring
// explicit type annotation.
type NoneContextPlugin struct {
	ctx *plugin.Context

	// Track None identifiers found during discovery
	noneNodes []*ast.Ident

	// Type inference service for accurate type resolution
	typeInference *TypeInferenceService
}

// NewNoneContextPlugin creates a new None context inference plugin
func NewNoneContextPlugin() *NoneContextPlugin {
	return &NoneContextPlugin{
		noneNodes: make([]*ast.Ident, 0),
	}
}

// Name returns the plugin name
func (p *NoneContextPlugin) Name() string {
	return "none_context"
}

// SetContext sets the plugin context (ContextAware interface)
func (p *NoneContextPlugin) SetContext(ctx *plugin.Context) {
	p.ctx = ctx

	// Initialize type inference service with go/types integration
	if ctx != nil && ctx.FileSet != nil {
		service, err := NewTypeInferenceService(ctx.FileSet, nil, ctx.Logger)
		if err != nil {
			ctx.Logger.Warn("Failed to create type inference service: %v", err)
		} else {
			p.typeInference = service

			// Inject go/types.Info if available in context
			if ctx.TypeInfo != nil {
				if typesInfo, ok := ctx.TypeInfo.(*types.Info); ok {
					service.SetTypesInfo(typesInfo)
					ctx.Logger.Debug("None context plugin: go/types integration enabled")
				}
			}
		}
	}
}

// Process processes AST nodes to find None identifiers (Discovery phase)
func (p *NoneContextPlugin) Process(node ast.Node) error {
	if p.ctx == nil {
		return fmt.Errorf("plugin context not initialized")
	}

	// Walk the AST to find None identifiers
	ast.Inspect(node, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && ident.Name == "None" {
			// Skip if this is a None tag enum constant (already handled by OptionTypePlugin)
			// Check if parent is a SelectorExpr (e.g., OptionTag_None)
			if p.ctx != nil {
				parent := p.ctx.GetParent(ident)
				if _, ok := parent.(*ast.SelectorExpr); ok {
					return true // Skip selector expressions
				}
			}

			p.noneNodes = append(p.noneNodes, ident)
		}
		return true
	})

	return nil
}

// Transform transforms None identifiers based on inferred context (Transform phase)
func (p *NoneContextPlugin) Transform(node ast.Node) (ast.Node, error) {
	if p.ctx == nil {
		return node, fmt.Errorf("plugin context not initialized")
	}

	// Apply transformations using astutil.Apply
	result := astutil.Apply(node, func(cursor *astutil.Cursor) bool {
		n := cursor.Node()

		// Check if this is a None identifier we need to transform
		if ident, ok := n.(*ast.Ident); ok && ident.Name == "None" {
			// Skip if already transformed or not in our list
			if !p.isTrackedNone(ident) {
				return true
			}

			// Infer type from context
			optionType, err := p.inferNoneType(ident)
			if err != nil {
				// Emit compile error for ambiguous None
				p.ctx.ReportError(
					fmt.Sprintf("cannot infer type for None constant: %v. Add explicit type annotation: let x: Option<T> = None", err),
					ident.Pos(),
				)
				return true
			}

			// Replace None with typed Option zero value
			// Option_T{tag: OptionTag_None, some_0: nil}
			replacement := p.createNoneValue(optionType)
			cursor.Replace(replacement)
		}

		return true
	}, nil)

	return result, nil
}

// isTrackedNone checks if this None identifier is in our tracked list
func (p *NoneContextPlugin) isTrackedNone(ident *ast.Ident) bool {
	for _, tracked := range p.noneNodes {
		if tracked == ident {
			return true
		}
	}
	return false
}

// inferNoneType infers the Option<T> type from surrounding context
func (p *NoneContextPlugin) inferNoneType(noneIdent *ast.Ident) (string, error) {
	if p.ctx == nil {
		return "", fmt.Errorf("no context available")
	}

	// Walk parent chain to find type context (in precedence order)
	var inferredType string
	var foundContext bool

	p.ctx.WalkParents(noneIdent, func(parent ast.Node) bool {
		switch parentNode := parent.(type) {
		case *ast.ReturnStmt:
			// Context: return None
			// Infer from function return type
			if typ, err := p.findReturnType(noneIdent); err == nil && typ != "" {
				inferredType = typ
				foundContext = true
				return false // Stop walking
			}

		case *ast.AssignStmt:
			// Context: x = None
			// Infer from assignment target type
			if typ, err := p.findAssignmentType(noneIdent, parentNode); err == nil && typ != "" {
				inferredType = typ
				foundContext = true
				return false
			}

		case *ast.CallExpr:
			// Context: foo(None)
			// Infer from function parameter type
			if typ, err := p.findParameterType(noneIdent, parentNode); err == nil && typ != "" {
				inferredType = typ
				foundContext = true
				return false
			}

		case *ast.CompositeLit:
			// Context: User{ age: None }
			// Infer from struct field type
			if typ, err := p.findFieldType(noneIdent, parentNode); err == nil && typ != "" {
				inferredType = typ
				foundContext = true
				return false
			}

		case *ast.ValueSpec:
			// Context: var x Option<int> = None
			// Explicit type annotation
			if parentNode.Type != nil {
				if typ := p.extractOptionType(parentNode.Type); typ != "" {
					inferredType = typ
					foundContext = true
					return false
				}
			}
		}

		return true // Continue walking up
	})

	if !foundContext || inferredType == "" {
		return "", fmt.Errorf("no valid type context found")
	}

	// Validate that the inferred type is an Option type
	if !strings.HasPrefix(inferredType, "Option_") {
		return "", fmt.Errorf("expected Option<T> type, got %s", inferredType)
	}

	return inferredType, nil
}

// findReturnType infers type from function return signature
func (p *NoneContextPlugin) findReturnType(noneIdent *ast.Ident) (string, error) {
	// Walk up to find enclosing function
	var funcDecl *ast.FuncDecl
	p.ctx.WalkParents(noneIdent, func(parent ast.Node) bool {
		if fn, ok := parent.(*ast.FuncDecl); ok {
			funcDecl = fn
			return false
		}
		return true
	})

	if funcDecl == nil || funcDecl.Type.Results == nil || len(funcDecl.Type.Results.List) == 0 {
		return "", fmt.Errorf("no function return type")
	}

	// Get return type from function signature
	returnType := funcDecl.Type.Results.List[0].Type

	// Extract Option<T> type name
	optionTypeName := p.extractOptionType(returnType)
	if optionTypeName == "" {
		return "", fmt.Errorf("return type is not Option<T>")
	}

	return optionTypeName, nil
}

// findAssignmentType infers type from assignment target
func (p *NoneContextPlugin) findAssignmentType(noneIdent *ast.Ident, assignStmt *ast.AssignStmt) (string, error) {
	// Find which RHS position the None is in
	rhsIndex := -1
	for i, rhs := range assignStmt.Rhs {
		if p.containsNode(rhs, noneIdent) {
			rhsIndex = i
			break
		}
	}

	if rhsIndex < 0 || rhsIndex >= len(assignStmt.Lhs) {
		return "", fmt.Errorf("cannot find assignment target")
	}

	// Get the LHS identifier
	lhs := assignStmt.Lhs[rhsIndex]
	lhsIdent, ok := lhs.(*ast.Ident)
	if !ok {
		return "", fmt.Errorf("assignment target is not an identifier")
	}

	// Use go/types to get the type of the LHS variable (C7 FIX: Use Defs, not Uses)
	if p.typeInference != nil && p.typeInference.typesInfo != nil {
		// First try Defs (for variable definitions: x := None)
		if obj := p.typeInference.typesInfo.Defs[lhsIdent]; obj != nil {
			if named, ok := obj.Type().(*types.Named); ok {
				typeName := named.Obj().Name()
				if strings.HasPrefix(typeName, "Option_") {
					return typeName, nil
				}
			}
		}
		// Fall back to Uses (for reassignment: x = None where x was already declared)
		if obj := p.typeInference.typesInfo.Uses[lhsIdent]; obj != nil {
			if named, ok := obj.Type().(*types.Named); ok {
				typeName := named.Obj().Name()
				if strings.HasPrefix(typeName, "Option_") {
					return typeName, nil
				}
			}
		}
	}

	return "", fmt.Errorf("cannot infer type from assignment target")
}

// findParameterType infers type from function call parameter
func (p *NoneContextPlugin) findParameterType(noneIdent *ast.Ident, callExpr *ast.CallExpr) (string, error) {
	// Find which argument position the None is in
	argIndex := -1
	for i, arg := range callExpr.Args {
		if p.containsNode(arg, noneIdent) {
			argIndex = i
			break
		}
	}

	if argIndex < 0 {
		return "", fmt.Errorf("None not found in call arguments")
	}

	// Use go/types to get function signature
	if p.typeInference != nil && p.typeInference.typesInfo != nil {
		if tv, ok := p.typeInference.typesInfo.Types[callExpr.Fun]; ok {
			if funcType, ok := tv.Type.(*types.Signature); ok {
				if argIndex < funcType.Params().Len() {
					paramType := funcType.Params().At(argIndex).Type()
					if named, ok := paramType.(*types.Named); ok {
						typeName := named.Obj().Name()
						if strings.HasPrefix(typeName, "Option_") {
							return typeName, nil
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("cannot infer type from function parameter")
}

// findFieldType infers type from struct field
func (p *NoneContextPlugin) findFieldType(noneIdent *ast.Ident, compLit *ast.CompositeLit) (string, error) {
	// Find which field the None is in
	var fieldName string
	for _, elt := range compLit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if p.containsNode(kv.Value, noneIdent) {
				if ident, ok := kv.Key.(*ast.Ident); ok {
					fieldName = ident.Name
					break
				}
			}
		}
	}

	if fieldName == "" {
		return "", fmt.Errorf("cannot find field name")
	}

	// Use go/types to get struct type and field type
	if p.typeInference != nil && p.typeInference.typesInfo != nil {
		if tv, ok := p.typeInference.typesInfo.Types[compLit.Type]; ok {
			if structType, ok := tv.Type.Underlying().(*types.Struct); ok {
				for i := 0; i < structType.NumFields(); i++ {
					field := structType.Field(i)
					if field.Name() == fieldName {
						if named, ok := field.Type().(*types.Named); ok {
							typeName := named.Obj().Name()
							if strings.HasPrefix(typeName, "Option_") {
								return typeName, nil
							}
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("cannot infer type from struct field")
}

// extractOptionType extracts the Option_T type name from an AST type expression
func (p *NoneContextPlugin) extractOptionType(typeExpr ast.Expr) string {
	switch t := typeExpr.(type) {
	case *ast.Ident:
		// Option_T (already transformed)
		if strings.HasPrefix(t.Name, "Option_") {
			return t.Name
		}

	case *ast.IndexExpr:
		// Option<T> (not yet transformed)
		if ident, ok := t.X.(*ast.Ident); ok && ident.Name == "Option" {
			// Extract T from Option<T>
			innerType := p.getTypeName(t.Index)
			return "Option_" + innerType
		}
	}

	return ""
}

// getTypeName converts an AST type expression to a string name
func (p *NoneContextPlugin) getTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		// Package-qualified type (e.g., pkg.Type)
		if x, ok := t.X.(*ast.Ident); ok {
			return x.Name + "_" + t.Sel.Name
		}
	case *ast.StarExpr:
		// Pointer type
		return "ptr_" + p.getTypeName(t.X)
	case *ast.ArrayType:
		// Slice or array type
		if t.Len == nil {
			return "slice_" + p.getTypeName(t.Elt)
		}
		return "array_" + p.getTypeName(t.Elt)
	}
	return "unknown"
}

// containsNode checks if a tree contains a specific node
func (p *NoneContextPlugin) containsNode(tree ast.Node, target ast.Node) bool {
	found := false
	ast.Inspect(tree, func(n ast.Node) bool {
		if n == target {
			found = true
			return false
		}
		return true
	})
	return found
}

// createNoneValue creates an Option_T zero value for None
func (p *NoneContextPlugin) createNoneValue(optionTypeName string) ast.Expr {
	// Generate: Option_T{tag: OptionTag_None, some_0: nil}
	return &ast.CompositeLit{
		Type: &ast.Ident{Name: optionTypeName},
		Elts: []ast.Expr{
			&ast.KeyValueExpr{
				Key:   &ast.Ident{Name: "tag"},
				Value: &ast.Ident{Name: "OptionTag_None"},
			},
			&ast.KeyValueExpr{
				Key:   &ast.Ident{Name: "some_0"},
				Value: &ast.Ident{Name: "nil"},
			},
		},
	}
}

// Inject adds any pending type declarations to the file (Inject phase)
func (p *NoneContextPlugin) Inject(file *ast.File) error {
	// None context plugin doesn't inject new types (handled by OptionTypePlugin)
	// This method is here for future extensions
	return nil
}
