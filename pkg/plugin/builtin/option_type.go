package builtin

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/MadAppGang/dingo/pkg/plugin"
)
// OptionTypePlugin generates Option<T> type declarations and transformations
//
// This plugin implements the Option type as a tagged union (sum type) with two variants:
// - Some(T): Contains a value of type T
// - None: Represents absence of value
//
// Generated structure:
//
//	type Option_T struct {
//	    tag     OptionTag
//	    some    *T        // Pointer for zero-value safety
//	}
//
// The plugin also generates:
// - OptionTag enum (Some, None)
// - Constructor functions (Option_T_Some, Option_T_None)
// - Helper methods (IsSome, IsNone, Unwrap, UnwrapOr, etc.)
type OptionTypePlugin struct {
	ctx *plugin.Context

	// Track which Option types we've already emitted to avoid duplicates
	emittedTypes map[string]bool

	// Declarations to inject at package level
	pendingDecls []ast.Decl

	// Type inference service for None validation
	typeInference *TypeInferenceService
}

// NewOptionTypePlugin creates a new Option type plugin
func NewOptionTypePlugin() *OptionTypePlugin {
	return &OptionTypePlugin{
		emittedTypes: make(map[string]bool),
		pendingDecls: make([]ast.Decl, 0),
	}
}

// Name returns the plugin name
func (p *OptionTypePlugin) Name() string {
	return "option_type"
}

// SetContext sets the plugin context (ContextAware interface)
func (p *OptionTypePlugin) SetContext(ctx *plugin.Context) {
	p.ctx = ctx

	if ctx != nil && ctx.FileSet != nil {
		// Create type inference service
		service, err := NewTypeInferenceService(ctx.FileSet, nil, ctx.Logger)
		if err != nil {
			ctx.Logger.Warn("Failed to create type inference service: %v", err)
		} else {
			p.typeInference = service

			// Inject go/types.Info if available in context
			if ctx.TypeInfo != nil {
				if typesInfo, ok := ctx.TypeInfo.(*types.Info); ok {
					service.SetTypesInfo(typesInfo)
					ctx.Logger.Debug("Option plugin: go/types integration enabled")
				}
			}

			// PRIORITY 4 FIX: Inject parent map for return statement inference
			if parentMap := ctx.GetParentMap(); parentMap != nil {
				service.SetParentMap(parentMap)
				ctx.Logger.Debug("Option plugin: parent map integration enabled")
			}
		}
	}
}

// SetTypeInference sets the type inference service
func (p *OptionTypePlugin) SetTypeInference(service *TypeInferenceService) {
	p.typeInference = service
}

// Process processes AST nodes to find and transform Option types
func (p *OptionTypePlugin) Process(node ast.Node) error {
	if p.ctx == nil {
		return fmt.Errorf("plugin context not initialized")
	}

	// Walk the AST to find Option type usage
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.IndexExpr:
			// Option<T>
			p.handleGenericOption(n)
		case *ast.Ident:
			// None singleton
			if n.Name == "None" {
				p.handleNoneExpression(n)
			}
		case *ast.CallExpr:
			// Some(value) constructor call
			if ident, ok := n.Fun.(*ast.Ident); ok && ident.Name == "Some" {
				p.handleSomeConstructor(n)
			}
		}
		return true
	})

	return nil
}

// handleGenericOption processes Option<T> syntax
func (p *OptionTypePlugin) handleGenericOption(expr *ast.IndexExpr) {
	// Check if the base type is "Option"
	if ident, ok := expr.X.(*ast.Ident); ok && ident.Name == "Option" {
		var typeName string
		// This is an Option<T> type
		telemType, ok := p.typeInference.InferType(expr.Index)
		if !ok || telemType == nil {
			p.ctx.Logger.Warn("OptionTypePlugin: Could not infer type for Option<T> element. Falling back to heuristic.")
			typeName = p.getTypeName(expr.Index)
		} else {
			typeName = p.typeInference.TypeToString(telemType)
		}
		optionType := fmt.Sprintf("Option%s", SanitizeTypeName(typeName))

		if !p.emittedTypes[optionType] {
			p.emitOptionDeclaration(typeName, optionType)
			p.emittedTypes[optionType] = true

			// Register with type inference service
			if p.typeInference != nil {
				valueType := p.typeInference.makeBasicType(typeName)
				// CRITICAL FIX #1: Pass original type string
				p.typeInference.RegisterOptionType(optionType, valueType, typeName)
			}
		}
	}
}

// handleNoneExpression processes None singleton
//
// Type-Context-Aware None Constant (Phase 3 - Complex Feature)
//
// This method implements intelligent None constant handling that infers the target
// Option<T> type from the surrounding context (assignment, return, function argument).
//
// Supported contexts:
// 1. Assignment: var x Option_int = None
// 2. Return: return None (in function returning Option_T)
// 3. Function argument: foo(None) where parameter type is Option_T
//
// If type cannot be inferred, generates a clear error message.
func (p *OptionTypePlugin) handleNoneExpression(ident *ast.Ident) {
	if p.ctx == nil {
		return
	}

	// Try to infer target Option type from context
	targetType, inferred := p.inferNoneTypeFromContext(ident)

	if !inferred {
		// Cannot infer type from context
		pos := p.ctx.FileSet.Position(ident.Pos())
		errorMsg := fmt.Sprintf(
			"Cannot infer Option type for None constant at %s\n"+
				"Hint: Use explicit type annotation or Option_T_None() constructor\n"+
				"Example: var x Option_int = Option_int_None() or var x Option_int = None with type declaration",
			pos,
		)
		p.ctx.Logger.Error(errorMsg)
		p.ctx.ReportError(errorMsg, ident.Pos())
		return
	}

	// Successfully inferred type
	p.ctx.Logger.Debug("None constant: inferred Option type %s from context", targetType)

	// Ensure the Option type is declared
	optionTypeName := fmt.Sprintf("Option%s", SanitizeTypeName(targetType))
	if !p.emittedTypes[optionTypeName] {
		p.emitOptionDeclaration(targetType, optionTypeName)
		p.emittedTypes[optionTypeName] = true
	}

	// Create the replacement CompositeLit
	// None → Option_T{tag: OptionTagNone}
	replacement := &ast.CompositeLit{
		Type: ast.NewIdent(optionTypeName),
		Elts: []ast.Expr{
			&ast.KeyValueExpr{
				Key:   ast.NewIdent("tag"),
				Value: ast.NewIdent("OptionTagNone"),
			},
			// No some field for None variant
		},
	}

	p.ctx.Logger.Debug("Transforming None → %s{tag: OptionTagNone}", optionTypeName)
	p.ctx.Logger.Debug("Generated replacement AST: %v", replacement)

	// Note: Actual AST replacement happens in the Transform phase
}

// handleSomeConstructor processes Some(value) constructor
func (p *OptionTypePlugin) handleSomeConstructor(call *ast.CallExpr) {
	if len(call.Args) != 1 {
		p.ctx.Logger.Warn("Some() expects exactly one argument, found %d", len(call.Args))
		return
	}

	// Type inference: Infer from argument type
	valueArg := call.Args[0]

	// CRITICAL FIX #3: Check error from inferTypeFromExpr
	valueType, err := p.inferTypeFromExpr(valueArg)
	if err != nil {
		// Type inference failed - use interface{} as last resort
		p.ctx.Logger.Warn("Type inference failed for Some(%s): %v, using interface{}", FormatExprForDebug(valueArg), err)
		valueType = "interface{}"
	}

	// CRITICAL FIX #3: Validate valueType is not empty
	if valueType == "" {
		p.ctx.Logger.Warn("Type inference returned empty string for Some(%s), using interface{}", FormatExprForDebug(valueArg))
		valueType = "interface{}"
	}

	// Generate unique Option type name
	optionTypeName := fmt.Sprintf("Option%s", SanitizeTypeName(valueType))

	// Ensure the Option type is declared
	if !p.emittedTypes[optionTypeName] {
		p.emitOptionDeclaration(valueType, optionTypeName)
		p.emittedTypes[optionTypeName] = true

		// Register with type inference service
		if p.typeInference != nil {
			vType := p.typeInference.makeBasicType(valueType)
			// CRITICAL FIX #1: Pass original type string
			p.typeInference.RegisterOptionType(optionTypeName, vType, valueType)
		}
	}

	// Fix A4: Handle addressability for literal values
	// Check if the argument is addressable
	var valueExpr ast.Expr
	if isAddressable(valueArg) {
		// Direct address-of operator
		valueExpr = &ast.UnaryExpr{
			Op: token.AND,
			X:  valueArg,
		}
		p.ctx.Logger.Debug("Some(%s): value is addressable, using &value", valueType)
	} else {
		// Wrap in IIFE to create addressable temporary variable
		valueExpr = wrapInIIFE(valueArg, valueType, p.ctx)
		p.ctx.Logger.Debug("Some(%s): value is non-addressable (literal), wrapping in IIFE", valueType)
	}

	// Transform the call to a struct literal
	p.ctx.Logger.Debug("Transforming Some(%s) → %s{tag: OptionTagSome, some: <addressable-value>}", valueType, optionTypeName)

	// Create the replacement CompositeLit
	// Some(value) → Option_T{tag: OptionTagSome, some: &value or IIFE}
	replacement := &ast.CompositeLit{
		Type: ast.NewIdent(optionTypeName),
		Elts: []ast.Expr{
			&ast.KeyValueExpr{
				Key:   ast.NewIdent("tag"),
				Value: ast.NewIdent("OptionTagSome"),
			},
			&ast.KeyValueExpr{
				Key:   ast.NewIdent("some"),
				Value: valueExpr,
			},
		},
	}

	// Replace the CallExpr with the CompositeLit in the parent node
	// This is done via AST transformation in the Transform phase
	// For now, we just log the transformation
	p.ctx.Logger.Debug("Generated replacement AST: %v", replacement)
}

// emitOptionDeclaration generates the Option type declaration and helper methods
func (p *OptionTypePlugin) emitOptionDeclaration(valueType, optionTypeName string) {
	if p.ctx == nil {
		return
	}
	// FileSet is only needed for position information (token.NoPos), not for type generation

	// Generate OptionTag enum (only once)
	if !p.emittedTypes["OptionTag"] {
		p.emitOptionTagEnum()
		p.emittedTypes["OptionTag"] = true
	}

	// Generate Option struct
	optionStruct := &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{
			&ast.TypeSpec{
				Name: &ast.Ident{
					NamePos: token.NoPos, // Prevent comment grabbing
					Name:    optionTypeName,
				},
				Type: &ast.StructType{
					Struct: token.NoPos, // Prevent comment grabbing
					Fields: &ast.FieldList{
						Opening: token.NoPos, // Prevent comment grabbing
						Closing: token.NoPos, // Prevent comment grabbing
						List: []*ast.Field{
							{
								Names: []*ast.Ident{
									{
										NamePos: token.NoPos, // Prevent comment grabbing
										Name:    "tag",
									},
								},
								Type: &ast.Ident{
									NamePos: token.NoPos, // Prevent comment grabbing
									Name:    "OptionTag",
								},
							},
							{
								Names: []*ast.Ident{
									{
										NamePos: token.NoPos, // Prevent comment grabbing
										Name:    "some",
									},
								},
								Type: p.typeToAST(valueType, true), // Pointer
							},
						},
					},
				},
			},
		},
	}

	p.pendingDecls = append(p.pendingDecls, optionStruct)

	// Generate constructor functions
	p.emitSomeConstructor(optionTypeName, valueType)
	p.emitNoneConstructor(optionTypeName, valueType)

	// Generate helper methods
	p.emitOptionHelperMethods(optionTypeName, valueType)
}

// emitOptionTagEnum generates the OptionTag enum
func (p *OptionTypePlugin) emitOptionTagEnum() {
	// type OptionTag uint8
	tagTypeDecl := &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{
			&ast.TypeSpec{
				Name: &ast.Ident{
					NamePos: token.NoPos, // Prevent comment grabbing
					Name:    "OptionTag",
				},
				Type: &ast.Ident{
					NamePos: token.NoPos, // Prevent comment grabbing
					Name:    "uint8",
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, tagTypeDecl)

	// const ( OptionTagSome OptionTag = iota; OptionTagNone )
	tagConstDecl := &ast.GenDecl{
		Tok: token.CONST,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names: []*ast.Ident{
					{
						NamePos: token.NoPos, // Prevent comment grabbing
						Name:    "OptionTagSome",
					},
				},
				Type: &ast.Ident{
					NamePos: token.NoPos, // Prevent comment grabbing
					Name:    "OptionTag",
				},
				Values: []ast.Expr{
					&ast.Ident{
						NamePos: token.NoPos, // Prevent comment grabbing
						Name:    "iota",
					},
				},
			},
			&ast.ValueSpec{
				Names: []*ast.Ident{
					{
						NamePos: token.NoPos, // Prevent comment grabbing
						Name:    "OptionTagNone",
					},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, tagConstDecl)
}

// emitSomeConstructor generates Some constructor
func (p *OptionTypePlugin) emitSomeConstructor(optionTypeName, valueType string) {
	funcName := fmt.Sprintf("%s_Some", optionTypeName)
	valueTypeAST := p.typeToAST(valueType, false)

	// func Option_T_Some(arg0 T) Option_T {
	//     return Option_T{tag: OptionTagSome, some: &arg0}
	// }
	constructorFunc := &ast.FuncDecl{
		Name: &ast.Ident{
			NamePos: token.NoPos, // Prevent comment grabbing
			Name:    funcName,
		},
		Type: &ast.FuncType{
			Func: token.NoPos, // Prevent comment grabbing
			Params: &ast.FieldList{
				Opening: token.NoPos, // Prevent comment grabbing
				Closing: token.NoPos, // Prevent comment grabbing
				List: []*ast.Field{
					{
						Names: []*ast.Ident{
							{
								NamePos: token.NoPos, // Prevent comment grabbing
								Name:    "arg0",
							},
						},
						Type: valueTypeAST,
					},
				},
			},
			Results: &ast.FieldList{
				Opening: token.NoPos, // Prevent comment grabbing
				Closing: token.NoPos, // Prevent comment grabbing
				List: []*ast.Field{
					{
						Type: &ast.Ident{
							NamePos: token.NoPos, // Prevent comment grabbing
							Name:    optionTypeName,
						},
					},
				},
			},
		},
		Body: &ast.BlockStmt{
			Lbrace: token.NoPos, // Prevent comment grabbing
			Rbrace: token.NoPos, // Prevent comment grabbing
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Return: token.NoPos, // Prevent comment grabbing
					Results: []ast.Expr{
						&ast.CompositeLit{
							Lbrace: token.NoPos, // Prevent comment grabbing
							Rbrace: token.NoPos, // Prevent comment grabbing
							Type: &ast.Ident{
								NamePos: token.NoPos, // Prevent comment grabbing
								Name:    optionTypeName,
							},
							Elts: []ast.Expr{
								&ast.KeyValueExpr{
									Colon: token.NoPos, // Prevent comment grabbing
									Key: &ast.Ident{
										NamePos: token.NoPos, // Prevent comment grabbing
										Name:    "tag",
									},
									Value: &ast.Ident{
										NamePos: token.NoPos, // Prevent comment grabbing
										Name:    "OptionTagSome",
									},
								},
								&ast.KeyValueExpr{
									Colon: token.NoPos, // Prevent comment grabbing
									Key: &ast.Ident{
										NamePos: token.NoPos, // Prevent comment grabbing
										Name:    "some",
									},
									Value: &ast.UnaryExpr{
										OpPos: token.NoPos, // Prevent comment grabbing
										Op:    token.AND,
										X: &ast.Ident{
											NamePos: token.NoPos, // Prevent comment grabbing
											Name:    "arg0",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	p.pendingDecls = append(p.pendingDecls, constructorFunc)
}

// emitNoneConstructor generates None constructor
func (p *OptionTypePlugin) emitNoneConstructor(optionTypeName, valueType string) {
	funcName := fmt.Sprintf("%s_None", optionTypeName)

	// func Option_T_None() Option_T {
	//     return Option_T{tag: OptionTagNone}
	// }
	constructorFunc := &ast.FuncDecl{
		Name: &ast.Ident{
			NamePos: token.NoPos, // Prevent comment grabbing
			Name:    funcName,
		},
		Type: &ast.FuncType{
			Func: token.NoPos, // Prevent comment grabbing
			Params: &ast.FieldList{
				Opening: token.NoPos, // Prevent comment grabbing
				Closing: token.NoPos, // Prevent comment grabbing
			},
			Results: &ast.FieldList{
				Opening: token.NoPos, // Prevent comment grabbing
				Closing: token.NoPos, // Prevent comment grabbing
				List: []*ast.Field{
					{
						Type: &ast.Ident{
							NamePos: token.NoPos, // Prevent comment grabbing
							Name:    optionTypeName,
						},
					},
				},
			},
		},
		Body: &ast.BlockStmt{
			Lbrace: token.NoPos, // Prevent comment grabbing
			Rbrace: token.NoPos, // Prevent comment grabbing
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Return: token.NoPos, // Prevent comment grabbing
					Results: []ast.Expr{
						&ast.CompositeLit{
							Lbrace: token.NoPos, // Prevent comment grabbing
							Rbrace: token.NoPos, // Prevent comment grabbing
							Type: &ast.Ident{
								NamePos: token.NoPos, // Prevent comment grabbing
								Name:    optionTypeName,
							},
							Elts: []ast.Expr{
								&ast.KeyValueExpr{
									Colon: token.NoPos, // Prevent comment grabbing
									Key: &ast.Ident{
										NamePos: token.NoPos, // Prevent comment grabbing
										Name:    "tag",
									},
									Value: &ast.Ident{
										NamePos: token.NoPos, // Prevent comment grabbing
										Name:    "OptionTagNone",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	p.pendingDecls = append(p.pendingDecls, constructorFunc)
}

// emitOptionHelperMethods generates IsSome, IsNone, Unwrap, UnwrapOr, etc.
func (p *OptionTypePlugin) emitOptionHelperMethods(optionTypeName, valueType string) {
	// IsSome() bool
	isSomeMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("o")},
					Type:  ast.NewIdent(optionTypeName),
				},
			},
		},
		Name: ast.NewIdent("IsSome"),
		Type: &ast.FuncType{
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent("bool")},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.BinaryExpr{
							X:  &ast.SelectorExpr{X: ast.NewIdent("o"), Sel: ast.NewIdent("tag")},
							Op: token.EQL,
							Y:  ast.NewIdent("OptionTagSome"),
						},
					},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, isSomeMethod)

	// IsNone() bool
	isNoneMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("o")},
					Type:  ast.NewIdent(optionTypeName),
				},
			},
		},
		Name: ast.NewIdent("IsNone"),
		Type: &ast.FuncType{
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent("bool")},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.BinaryExpr{
							X:  &ast.SelectorExpr{X: ast.NewIdent("o"), Sel: ast.NewIdent("tag")},
							Op: token.EQL,
							Y:  ast.NewIdent("OptionTagNone"),
						},
					},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, isNoneMethod)

	// Unwrap() T - panics if None
	unwrapMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("o")},
					Type:  ast.NewIdent(optionTypeName),
				},
			},
		},
		Name: ast.NewIdent("Unwrap"),
		Type: &ast.FuncType{
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: p.typeToAST(valueType, false)},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.SelectorExpr{X: ast.NewIdent("o"), Sel: ast.NewIdent("tag")},
						Op: token.NEQ,
						Y:  ast.NewIdent("OptionTagSome"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: ast.NewIdent("panic"),
									Args: []ast.Expr{
										&ast.BasicLit{
											Kind:  token.STRING,
											Value: `"called Unwrap on None"`,
										},
									},
								},
							},
						},
					},
				},
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.StarExpr{
							X: &ast.SelectorExpr{X: ast.NewIdent("o"), Sel: ast.NewIdent("some")},
						},
					},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, unwrapMethod)

	// UnwrapOr(defaultValue T) T
	unwrapOrMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("o")},
					Type:  ast.NewIdent(optionTypeName),
				},
			},
		},
		Name: ast.NewIdent("UnwrapOr"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("defaultValue")},
						Type:  p.typeToAST(valueType, false),
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: p.typeToAST(valueType, false)},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.SelectorExpr{X: ast.NewIdent("o"), Sel: ast.NewIdent("tag")},
						Op: token.EQL,
						Y:  ast.NewIdent("OptionTagSome"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{
									&ast.StarExpr{
										X: &ast.SelectorExpr{X: ast.NewIdent("o"), Sel: ast.NewIdent("some")},
									},
								},
							},
						},
					},
				},
				&ast.ReturnStmt{
					Results: []ast.Expr{ast.NewIdent("defaultValue")},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, unwrapOrMethod)

	// UnwrapOrElse(fn func() T) T
	unwrapOrElseMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("o")},
					Type:  ast.NewIdent(optionTypeName),
				},
			},
		},
		Name: ast.NewIdent("UnwrapOrElse"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("fn")},
						Type: &ast.FuncType{
							Params: &ast.FieldList{},
							Results: &ast.FieldList{
								List: []*ast.Field{
									{Type: p.typeToAST(valueType, false)},
								},
							},
						},
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: p.typeToAST(valueType, false)},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.SelectorExpr{X: ast.NewIdent("o"), Sel: ast.NewIdent("tag")},
						Op: token.EQL,
						Y:  ast.NewIdent("OptionTagSome"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{
									&ast.StarExpr{
										X: &ast.SelectorExpr{X: ast.NewIdent("o"), Sel: ast.NewIdent("some")},
									},
								},
							},
						},
					},
				},
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.CallExpr{
							Fun: ast.NewIdent("fn"),
						},
					},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, unwrapOrElseMethod)

	// Map(fn func(T) U) Option_U - Transform Some value, propagate None
	// Note: For simplicity in Phase 3, we'll use interface{} for U and let go/types infer later
	// A complete implementation would require generic type parameter tracking
	mapMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("o")},
					Type:  ast.NewIdent(optionTypeName),
				},
			},
		},
		Name: ast.NewIdent("Map"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("fn")},
						Type: &ast.FuncType{
							Params: &ast.FieldList{
								List: []*ast.Field{
									{Type: p.typeToAST(valueType, false)},
								},
							},
							Results: &ast.FieldList{
								List: []*ast.Field{
									{Type: ast.NewIdent("interface{}")},
								},
							},
						},
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent(optionTypeName)},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.SelectorExpr{X: ast.NewIdent("o"), Sel: ast.NewIdent("tag")},
						Op: token.EQL,
						Y:  ast.NewIdent("OptionTagNone"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{ast.NewIdent("o")},
							},
						},
					},
				},
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent("mapped")},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{
						&ast.CallExpr{
							Fun: ast.NewIdent("fn"),
							Args: []ast.Expr{
								&ast.StarExpr{
									X: &ast.SelectorExpr{X: ast.NewIdent("o"), Sel: ast.NewIdent("some")},
								},
							},
						},
					},
				},
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent("result")},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{
						&ast.TypeAssertExpr{
							X:    ast.NewIdent("mapped"),
							Type: p.typeToAST(valueType, false),
						},
					},
				},
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.CompositeLit{
							Type: ast.NewIdent(optionTypeName),
							Elts: []ast.Expr{
								&ast.KeyValueExpr{
									Key:   ast.NewIdent("tag"),
									Value: ast.NewIdent("OptionTagSome"),
								},
								&ast.KeyValueExpr{
									Key: ast.NewIdent("some"),
									Value: &ast.UnaryExpr{
										Op: token.AND,
										X:  ast.NewIdent("result"),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, mapMethod)

	// AndThen(fn func(T) Option_T) Option_T - Chain operations (flatMap)
	andThenMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("o")},
					Type:  ast.NewIdent(optionTypeName),
				},
			},
		},
		Name: ast.NewIdent("AndThen"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("fn")},
						Type: &ast.FuncType{
							Params: &ast.FieldList{
								List: []*ast.Field{
									{Type: p.typeToAST(valueType, false)},
								},
							},
							Results: &ast.FieldList{
								List: []*ast.Field{
									{Type: ast.NewIdent(optionTypeName)},
								},
							},
						},
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent(optionTypeName)},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.SelectorExpr{X: ast.NewIdent("o"), Sel: ast.NewIdent("tag")},
						Op: token.EQL,
						Y:  ast.NewIdent("OptionTagNone"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{ast.NewIdent("o")},
							},
						},
					},
				},
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.CallExpr{
							Fun: ast.NewIdent("fn"),
							Args: []ast.Expr{
								&ast.StarExpr{
									X: &ast.SelectorExpr{X: ast.NewIdent("o"), Sel: ast.NewIdent("some")},
								},
							},
						},
					},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, andThenMethod)

	// Filter(predicate func(T) bool) Option_T - Filter Some values, return None if false
	filterMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("o")},
					Type:  ast.NewIdent(optionTypeName),
				},
			},
		},
		Name: ast.NewIdent("Filter"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("predicate")},
						Type: &ast.FuncType{
							Params: &ast.FieldList{
								List: []*ast.Field{
									{Type: p.typeToAST(valueType, false)},
								},
							},
							Results: &ast.FieldList{
								List: []*ast.Field{
									{Type: ast.NewIdent("bool")},
								},
							},
						},
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent(optionTypeName)},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.SelectorExpr{X: ast.NewIdent("o"), Sel: ast.NewIdent("tag")},
						Op: token.EQL,
						Y:  ast.NewIdent("OptionTagNone"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{ast.NewIdent("o")},
							},
						},
					},
				},
				&ast.IfStmt{
					Cond: &ast.CallExpr{
						Fun: ast.NewIdent("predicate"),
						Args: []ast.Expr{
							&ast.StarExpr{
								X: &ast.SelectorExpr{X: ast.NewIdent("o"), Sel: ast.NewIdent("some")},
							},
						},
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{ast.NewIdent("o")},
							},
						},
					},
				},
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.CompositeLit{
							Type: ast.NewIdent(optionTypeName),
							Elts: []ast.Expr{
								&ast.KeyValueExpr{
									Key:   ast.NewIdent("tag"),
									Value: ast.NewIdent("OptionTagNone"),
								},
							},
						},
					},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, filterMethod)
}

// inferNoneTypeFromContext attempts to infer the Option<T> type from surrounding context
//
// LIMITATION (Phase 3): Full context-based None inference requires AST parent tracking
// and complete go/types integration, which is planned for Phase 4.
//
// Current Behavior:
// - Relies on TypeInferenceService.InferTypeFromContext() (currently a stub)
// - WITHOUT go/types context: Always fails, user MUST use explicit type annotations
// - WITH go/types context (Phase 4+): Will infer from assignment/return/parameter types
//
// Workarounds for Phase 3:
// 1. Explicit type annotation: let x: Option<int> = None
// 2. Explicit constructor: Option_int_None()
// 3. Assignment to typed variable: var x Option_int = None (requires go/types)
//
// Strategy (when fully implemented in Phase 4):
// 1. Walk up the AST to find parent nodes
// 2. Analyze parent context (assignment, return, call)
// 3. Extract type information from context
// 4. Return the inferred type parameter T
//
// Returns: (typeParam string, success bool)
func (p *OptionTypePlugin) inferNoneTypeFromContext(noneIdent *ast.Ident) (string, bool) {
	// We need to walk the AST to find the parent node of None
	// This requires access to the full file AST with parent tracking

	// For now, use TypeInferenceService if available (it has go/types context)
	if p.typeInference != nil && p.typeInference.typesInfo != nil {
		// Try to use go/types to infer expected type
		if typ, ok := p.typeInference.InferTypeFromContext(noneIdent); ok {
			// Check if it's an Option type
			typeStr := p.typeInference.TypeToString(typ)
			if strings.HasPrefix(typeStr, "Option_") {
				// Extract T from Option_T
				tParam := strings.TrimPrefix(typeStr, "Option_")
				// Reverse sanitization to get original type name
				tParam = p.desanitizeTypeName(tParam)
				p.ctx.Logger.Debug("Inferred None type from go/types: %s", tParam)
				return tParam, true
			}
		}
	}

	// Fallback: Manual AST walking (limited without parent tracking)
	// This is a simplified implementation - full implementation requires
	// AST visitor pattern with parent tracking

	// PHASE 3 LIMITATION: InferTypeFromContext() is a stub that always returns false
	// Users must use explicit type annotations until Phase 4
	p.ctx.Logger.Debug("None type inference: go/types not available or context not found (Phase 3 limitation)")
	return "", false
}

// desanitizeTypeName attempts to reverse the sanitization process
// This is a best-effort approach - not always accurate
func (p *OptionTypePlugin) desanitizeTypeName(sanitized string) string {
	s := sanitized
	// Reverse common sanitization patterns
	s = strings.ReplaceAll(s, "ptr_", "*")
	s = strings.ReplaceAll(s, "slice_", "[]")
	// Note: This is incomplete - map types, array types are more complex
	return s
}

// Helper methods (same as Result plugin)

func (p *OptionTypePlugin) getTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + p.getTypeName(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + p.getTypeName(t.Elt)
		}
		return "[N]" + p.getTypeName(t.Elt)
	case *ast.SelectorExpr:
		return p.getTypeName(t.X) + "." + t.Sel.Name
	default:
		return "unknown"
	}
}


func (p *OptionTypePlugin) typeToAST(typeName string, asPointer bool) ast.Expr {
	var baseType ast.Expr

	if strings.HasPrefix(typeName, "*") {
		baseType = &ast.StarExpr{
			X: ast.NewIdent(strings.TrimPrefix(typeName, "*")),
		}
	} else if strings.HasPrefix(typeName, "[]") {
		baseType = &ast.ArrayType{
			Elt: ast.NewIdent(strings.TrimPrefix(typeName, "[]")),
		}
	} else {
		baseType = ast.NewIdent(typeName)
	}

	if asPointer {
		return &ast.StarExpr{X: baseType}
	}

	return baseType
}

// CRITICAL FIX #3: Now returns (string, error) instead of just string
func (p *OptionTypePlugin) inferTypeFromExpr(expr ast.Expr) (string, error) {
	if expr == nil {
		return "", fmt.Errorf("cannot infer type from nil expression")
	}

	// Fix A5: Use TypeInferenceService for accurate type inference
	if p.typeInference != nil {
		if typ, ok := p.typeInference.InferType(expr); ok && typ != nil {
			typeStr := p.typeInference.TypeToString(typ)
			p.ctx.Logger.Debug("Type inference (go/types): %T → %s", expr, typeStr)
			return typeStr, nil
		}
		p.ctx.Logger.Debug("Type inference (go/types) failed for %T, falling back to heuristics", expr)
	}

	// Fallback: Structural heuristics (when go/types unavailable)
	switch e := expr.(type) {
	case *ast.BasicLit:
		switch e.Kind {
		case token.INT:
			return "int", nil
		case token.FLOAT:
			return "float64", nil
		case token.STRING:
			return "string", nil
		case token.CHAR:
			return "rune", nil
		}
	case *ast.Ident:
		// Special built-in types
		switch e.Name {
		case "nil":
			return "interface{}", nil
		case "true", "false":
			return "bool", nil
		}
		// CRITICAL FIX #3: Return error for identifiers without go/types
		return "", fmt.Errorf("cannot determine type of identifier '%s' without go/types", e.Name)
	case *ast.CallExpr:
		// CRITICAL FIX #3: Return error for function calls
		return "", fmt.Errorf("function call requires go/types for return type inference")
	}

	// CRITICAL FIX #3: Return error instead of "interface{}" fallback
	return "", fmt.Errorf("type inference failed for expression type %T", expr)
}

// GetPendingDeclarations returns declarations to be injected at package level
func (p *OptionTypePlugin) GetPendingDeclarations() []ast.Decl {
	return p.pendingDecls
}

// ClearPendingDeclarations clears the pending declarations list
func (p *OptionTypePlugin) ClearPendingDeclarations() {
	p.pendingDecls = make([]ast.Decl, 0)
}
