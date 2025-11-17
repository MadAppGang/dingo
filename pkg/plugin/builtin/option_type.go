// Package builtin provides Option<T> type generation plugin
package builtin

import (
	"fmt"
	"go/ast"
	"go/token"
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
//   type Option_T struct {
//       tag     OptionTag
//       some_0  *T        // Pointer for zero-value safety
//   }
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
		// This is an Option<T> type
		typeName := p.getTypeName(expr.Index)
		optionType := fmt.Sprintf("Option_%s", p.sanitizeTypeName(typeName))

		if !p.emittedTypes[optionType] {
			p.emitOptionDeclaration(typeName, optionType)
			p.emittedTypes[optionType] = true

			// Register with type inference service
			if p.typeInference != nil {
				valueType := p.typeInference.makeBasicType(typeName)
				p.typeInference.RegisterOptionType(optionType, valueType)
			}
		}
	}
}

// handleNoneExpression processes None singleton
//
// Task 1.5: Add None type inference validation
//
// This method validates that None can be type-inferred from context.
// If not, it generates a compilation error with helpful suggestions.
func (p *OptionTypePlugin) handleNoneExpression(ident *ast.Ident) {
	// Task 1.5: Validate None type inference
	if p.typeInference == nil {
		p.ctx.Logger.Warn("Type inference not available for None validation at %v", p.ctx.FileSet.Position(ident.Pos()))
		return
	}

	// Check if None can be inferred from context
	ok, suggestion := p.typeInference.ValidateNoneInference(ident)

	if !ok {
		// Generate compilation error
		pos := p.ctx.FileSet.Position(ident.Pos())
		errorMsg := fmt.Sprintf(
			"Error: Cannot infer type for None at line %d, column %d\n%s",
			pos.Line,
			pos.Column,
			suggestion,
		)

		// Log the error (in a real implementation, this would be added to error list)
		p.ctx.Logger.Error(errorMsg)

		// TODO: Add to compilation error list
		// For now, we just log it
		p.ctx.Logger.Debug("None type inference failed: %s", errorMsg)
	} else {
		nonePos := p.ctx.FileSet.Position(ident.Pos())
		p.ctx.Logger.Debug("None type inference succeeded at %v", nonePos)
	}
}

// handleSomeConstructor processes Some(value) constructor
func (p *OptionTypePlugin) handleSomeConstructor(call *ast.CallExpr) {
	if len(call.Args) != 1 {
		p.ctx.Logger.Warn("Some() expects exactly one argument, found %d", len(call.Args))
		return
	}

	// Type inference: Infer from argument type
	valueArg := call.Args[0]
	valueType := p.inferTypeFromExpr(valueArg)

	// Generate unique Option type name
	optionTypeName := fmt.Sprintf("Option_%s", p.sanitizeTypeName(valueType))

	// Ensure the Option type is declared
	if !p.emittedTypes[optionTypeName] {
		p.emitOptionDeclaration(valueType, optionTypeName)
		p.emittedTypes[optionTypeName] = true

		// Register with type inference service
		if p.typeInference != nil {
			vType := p.typeInference.makeBasicType(valueType)
			p.typeInference.RegisterOptionType(optionTypeName, vType)
		}
	}

	// Transform the call to a struct literal
	p.ctx.Logger.Debug("Transforming Some(%s) → %s{tag: OptionTag_Some, some_0: &value}", valueType, optionTypeName)

	// Note: Actual AST transformation would happen here
}

// emitOptionDeclaration generates the Option type declaration and helper methods
func (p *OptionTypePlugin) emitOptionDeclaration(valueType, optionTypeName string) {
	if p.ctx == nil || p.ctx.FileSet == nil {
		return
	}

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
				Name: ast.NewIdent(optionTypeName),
				Type: &ast.StructType{
					Fields: &ast.FieldList{
						List: []*ast.Field{
							{
								Names: []*ast.Ident{ast.NewIdent("tag")},
								Type:  ast.NewIdent("OptionTag"),
							},
							{
								Names: []*ast.Ident{ast.NewIdent("some_0")},
								Type:  p.typeToAST(valueType, true), // Pointer
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
				Name: ast.NewIdent("OptionTag"),
				Type: ast.NewIdent("uint8"),
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, tagTypeDecl)

	// const ( OptionTag_Some OptionTag = iota; OptionTag_None )
	tagConstDecl := &ast.GenDecl{
		Tok: token.CONST,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names: []*ast.Ident{ast.NewIdent("OptionTag_Some")},
				Type:  ast.NewIdent("OptionTag"),
				Values: []ast.Expr{
					ast.NewIdent("iota"),
				},
			},
			&ast.ValueSpec{
				Names: []*ast.Ident{ast.NewIdent("OptionTag_None")},
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
	//     return Option_T{tag: OptionTag_Some, some_0: &arg0}
	// }
	constructorFunc := &ast.FuncDecl{
		Name: ast.NewIdent(funcName),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("arg0")},
						Type:  valueTypeAST,
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{
						Type: ast.NewIdent(optionTypeName),
					},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.CompositeLit{
							Type: ast.NewIdent(optionTypeName),
							Elts: []ast.Expr{
								&ast.KeyValueExpr{
									Key:   ast.NewIdent("tag"),
									Value: ast.NewIdent("OptionTag_Some"),
								},
								&ast.KeyValueExpr{
									Key: ast.NewIdent("some_0"),
									Value: &ast.UnaryExpr{
										Op: token.AND,
										X:  ast.NewIdent("arg0"),
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
	//     return Option_T{tag: OptionTag_None}
	// }
	constructorFunc := &ast.FuncDecl{
		Name: ast.NewIdent(funcName),
		Type: &ast.FuncType{
			Params: &ast.FieldList{},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{
						Type: ast.NewIdent(optionTypeName),
					},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.CompositeLit{
							Type: ast.NewIdent(optionTypeName),
							Elts: []ast.Expr{
								&ast.KeyValueExpr{
									Key:   ast.NewIdent("tag"),
									Value: ast.NewIdent("OptionTag_None"),
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
							Y:  ast.NewIdent("OptionTag_Some"),
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
							Y:  ast.NewIdent("OptionTag_None"),
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
						Y:  ast.NewIdent("OptionTag_Some"),
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
							X: &ast.SelectorExpr{X: ast.NewIdent("o"), Sel: ast.NewIdent("some_0")},
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
						Y:  ast.NewIdent("OptionTag_Some"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{
									&ast.StarExpr{
										X: &ast.SelectorExpr{X: ast.NewIdent("o"), Sel: ast.NewIdent("some_0")},
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

func (p *OptionTypePlugin) sanitizeTypeName(typeName string) string {
	s := typeName
	s = strings.ReplaceAll(s, "*", "ptr_")
	s = strings.ReplaceAll(s, "[]", "slice_")
	s = strings.ReplaceAll(s, "[", "_")
	s = strings.ReplaceAll(s, "]", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.Trim(s, "_")
	return s
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

func (p *OptionTypePlugin) inferTypeFromExpr(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		switch e.Kind {
		case token.INT:
			return "int"
		case token.FLOAT:
			return "float64"
		case token.STRING:
			return "string"
		case token.CHAR:
			return "rune"
		}
	case *ast.Ident:
		return e.Name
	case *ast.CallExpr:
		return "interface{}"
	}
	return "interface{}"
}

// GetPendingDeclarations returns declarations to be injected at package level
func (p *OptionTypePlugin) GetPendingDeclarations() []ast.Decl {
	return p.pendingDecls
}

// ClearPendingDeclarations clears the pending declarations list
func (p *OptionTypePlugin) ClearPendingDeclarations() {
	p.pendingDecls = make([]ast.Decl, 0)
}
