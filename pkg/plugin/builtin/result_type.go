// Package builtin provides Result<T, E> type generation plugin
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

// ResultTypePlugin generates Result<T, E> type declarations and transformations
//
// This plugin implements the Result type as a tagged union (sum type) with two variants:
// - Ok(T): Success case containing a value of type T
// - Err(E): Error case containing an error of type E
//
// Generated structure:
//   type Result_T_E struct {
//       tag    ResultTag
//       ok_0   *T        // Pointer for zero-value safety
//       err_0  *E        // Pointer for nil-ability
//   }
//
// The plugin also generates:
// - ResultTag enum (Ok, Err)
// - Constructor functions (Result_T_E_Ok, Result_T_E_Err)
// - Helper methods (IsOk, IsErr, Unwrap, UnwrapOr, etc.)
type ResultTypePlugin struct {
	ctx *plugin.Context

	// Track which Result types we've already emitted to avoid duplicates
	emittedTypes map[string]bool

	// Declarations to inject at package level
	pendingDecls []ast.Decl

	// Type information for proper type inference
	typesInfo *types.Info
	typesPkg  *types.Package
}

// NewResultTypePlugin creates a new Result type plugin
func NewResultTypePlugin() *ResultTypePlugin {
	return &ResultTypePlugin{
		emittedTypes: make(map[string]bool),
		pendingDecls: make([]ast.Decl, 0),
	}
}

// Name returns the plugin name
func (p *ResultTypePlugin) Name() string {
	return "result_type"
}

// SetContext sets the plugin context (ContextAware interface)
func (p *ResultTypePlugin) SetContext(ctx *plugin.Context) {
	p.ctx = ctx
}

// Process processes AST nodes to find and transform Result types
func (p *ResultTypePlugin) Process(node ast.Node) error {
	if p.ctx == nil {
		return fmt.Errorf("plugin context not initialized")
	}

	// Walk the AST to find Result type usage
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.IndexExpr:
			// Result<T> or Result<T, E>
			p.handleGenericResult(n)
		case *ast.IndexListExpr:
			// Go 1.18+ generic syntax: Result[T, E]
			p.handleGenericResultList(n)
		case *ast.CallExpr:
			// Ok(value) or Err(error) constructor calls
			p.handleConstructorCall(n)
		}
		return true
	})

	return nil
}

// handleGenericResult processes Result<T> or Result<T, E> syntax (IndexExpr)
func (p *ResultTypePlugin) handleGenericResult(expr *ast.IndexExpr) {
	// Check if the base type is "Result"
	if ident, ok := expr.X.(*ast.Ident); ok && ident.Name == "Result" {
		// This is a Result<T> (single type parameter)
		// Default error type to "error"
		typeName := p.getTypeName(expr.Index)
		resultType := fmt.Sprintf("Result_%s_error", p.sanitizeTypeName(typeName))

		if !p.emittedTypes[resultType] {
			p.emitResultDeclaration(typeName, "error", resultType)
			p.emittedTypes[resultType] = true
		}
	}
}

// handleGenericResultList processes Result[T, E] syntax (IndexListExpr for Go 1.18+)
func (p *ResultTypePlugin) handleGenericResultList(expr *ast.IndexListExpr) {
	// Check if the base type is "Result"
	if ident, ok := expr.X.(*ast.Ident); ok && ident.Name == "Result" {
		if len(expr.Indices) == 2 {
			// Result<T, E> with explicit error type
			okType := p.getTypeName(expr.Indices[0])
			errType := p.getTypeName(expr.Indices[1])
			resultType := fmt.Sprintf("Result_%s_%s",
				p.sanitizeTypeName(okType),
				p.sanitizeTypeName(errType))

			if !p.emittedTypes[resultType] {
				p.emitResultDeclaration(okType, errType, resultType)
				p.emittedTypes[resultType] = true
			}
		} else if len(expr.Indices) == 1 {
			// Result<T> with default error type
			okType := p.getTypeName(expr.Indices[0])
			resultType := fmt.Sprintf("Result_%s_error", p.sanitizeTypeName(okType))

			if !p.emittedTypes[resultType] {
				p.emitResultDeclaration(okType, "error", resultType)
				p.emittedTypes[resultType] = true
			}
		}
	}
}

// handleConstructorCall processes Ok(value) and Err(error) calls
//
// Task 1.2: Transform constructor calls to struct literals
//
// This method detects Ok() and Err() calls and transforms them into
// Result struct literals with the appropriate tag and field values.
//
// Type inference strategy:
// 1. Check for explicit type annotation (e.g., let x: Result<int, error> = Ok(42))
// 2. Infer from argument type for T, default error for E
// 3. Use context from surrounding expression (assignment, return, etc.)
func (p *ResultTypePlugin) handleConstructorCall(call *ast.CallExpr) {
	if ident, ok := call.Fun.(*ast.Ident); ok {
		switch ident.Name {
		case "Ok":
			p.transformOkConstructor(call)
		case "Err":
			p.transformErrConstructor(call)
		}
	}
}

// transformOkConstructor transforms Ok(value) → Result_T_E{tag: ResultTag_Ok, ok_0: &value}
// Returns the replacement node, or the original call if transformation fails
func (p *ResultTypePlugin) transformOkConstructor(call *ast.CallExpr) ast.Expr {
	if len(call.Args) != 1 {
		p.ctx.Logger.Warn("Ok() expects exactly one argument, found %d", len(call.Args))
		return call // Return unchanged
	}

	// Type inference: For now, use simple heuristics
	// In a full implementation, this would use TypeInferenceService
	valueArg := call.Args[0]
	okType := p.inferTypeFromExpr(valueArg)
	errType := "error" // Default error type

	// Generate unique Result type name
	resultTypeName := fmt.Sprintf("Result_%s_%s",
		p.sanitizeTypeName(okType),
		p.sanitizeTypeName(errType))

	// Ensure the Result type is declared
	if !p.emittedTypes[resultTypeName] {
		p.emitResultDeclaration(okType, errType, resultTypeName)
		p.emittedTypes[resultTypeName] = true
	}

	// Log transformation
	p.ctx.Logger.Debug("Transforming Ok(%s) → %s{tag: ResultTag_Ok, ok_0: &value}", okType, resultTypeName)

	// Create the replacement CompositeLit
	// Ok(value) → Result_T_E{tag: ResultTag_Ok, ok_0: &value}
	replacement := &ast.CompositeLit{
		Type: ast.NewIdent(resultTypeName),
		Elts: []ast.Expr{
			&ast.KeyValueExpr{
				Key:   ast.NewIdent("tag"),
				Value: ast.NewIdent("ResultTag_Ok"),
			},
			&ast.KeyValueExpr{
				Key: ast.NewIdent("ok_0"),
				Value: &ast.UnaryExpr{
					Op: token.AND,
					X:  valueArg, // Use original argument expression
				},
			},
		},
	}

	return replacement
}

// transformErrConstructor transforms Err(error) → Result_T_E{tag: ResultTag_Err, err_0: &error}
// Returns the replacement node, or the original call if transformation fails
func (p *ResultTypePlugin) transformErrConstructor(call *ast.CallExpr) ast.Expr {
	if len(call.Args) != 1 {
		p.ctx.Logger.Warn("Err() expects exactly one argument, found %d", len(call.Args))
		return call // Return unchanged
	}

	// Type inference: For Err, we need context to determine T
	// For now, we'll use placeholder type that can be refined later
	errorArg := call.Args[0]
	errType := p.inferTypeFromExpr(errorArg)

	// For Err(), the Ok type must be inferred from context
	// This is a limitation without full type inference
	// For now, we'll use "interface{}" as a placeholder
	okType := "interface{}" // Will be refined with type inference

	// Generate unique Result type name
	resultTypeName := fmt.Sprintf("Result_%s_%s",
		p.sanitizeTypeName(okType),
		p.sanitizeTypeName(errType))

	// Ensure the Result type is declared
	if !p.emittedTypes[resultTypeName] {
		p.emitResultDeclaration(okType, errType, resultTypeName)
		p.emittedTypes[resultTypeName] = true
	}

	// Log transformation
	p.ctx.Logger.Debug("Transforming Err(%s) → %s{tag: ResultTag_Err, err_0: &value}", errType, resultTypeName)

	// Create the replacement CompositeLit
	// Err(error) → Result_T_E{tag: ResultTag_Err, err_0: &error}
	replacement := &ast.CompositeLit{
		Type: ast.NewIdent(resultTypeName),
		Elts: []ast.Expr{
			&ast.KeyValueExpr{
				Key:   ast.NewIdent("tag"),
				Value: ast.NewIdent("ResultTag_Err"),
			},
			&ast.KeyValueExpr{
				Key: ast.NewIdent("err_0"),
				Value: &ast.UnaryExpr{
					Op: token.AND,
					X:  errorArg, // Use original argument expression
				},
			},
		},
	}

	return replacement
}

// inferTypeFromExpr infers the type of an expression
//
// This is a hybrid implementation that:
// 1. Uses go/types if TypesInfo is available in the context
// 2. Falls back to structural heuristics for common cases
// 3. Returns "interface{}" for complex cases that need full type checking
//
// TODO(enhancement): Integrate full go/types type checking pipeline
func (p *ResultTypePlugin) inferTypeFromExpr(expr ast.Expr) string {
	// Try go/types first if available
	if p.typesInfo != nil && p.typesInfo.Types != nil {
		if tv, ok := p.typesInfo.Types[expr]; ok && tv.Type != nil {
			return tv.Type.String()
		}
	}

	// Fallback to structural heuristics
	switch e := expr.(type) {
	case *ast.BasicLit:
		// Infer from literal kind
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
		// Special built-in types
		switch e.Name {
		case "nil":
			return "interface{}"
		case "true", "false":
			return "bool"
		}

		// For variables, we'd need type information
		// Without go/types, we can't reliably infer the type
		// TODO: This is the key limitation - returns variable name instead of type
		// Full fix requires running go/types type checker
		if p.ctx != nil && p.ctx.Logger != nil {
			p.ctx.Logger.Debug("Type inference limitation: cannot determine type of identifier '%s' without go/types", e.Name)
		}
		return "interface{}"

	case *ast.CompositeLit:
		// Struct/array/map literals with explicit type
		if e.Type != nil {
			return p.exprToTypeString(e.Type)
		}
		return "interface{}"

	case *ast.UnaryExpr:
		// &x → pointer to x's type
		if e.Op == token.AND {
			innerType := p.inferTypeFromExpr(e.X)
			if innerType != "interface{}" {
				return "*" + innerType
			}
		}
		return "interface{}"

	case *ast.CallExpr:
		// Function calls would need return type analysis from go/types
		// Without it, we can't know the return type
		return "interface{}"

	case *ast.StarExpr:
		// *x → dereference, need type info
		return "interface{}"

	case *ast.SelectorExpr:
		// x.field → need type info for x
		return "interface{}"

	case *ast.IndexExpr:
		// arr[i] or map[key] → need type info
		return "interface{}"

	case *ast.ArrayType:
		return p.exprToTypeString(e)

	case *ast.StructType:
		return p.exprToTypeString(e)

	case *ast.FuncType:
		return p.exprToTypeString(e)

	case *ast.InterfaceType:
		return p.exprToTypeString(e)

	case *ast.MapType:
		return p.exprToTypeString(e)

	case *ast.ChanType:
		return p.exprToTypeString(e)
	}

	return "interface{}"
}

// exprToTypeString converts an AST type expression to a string representation
func (p *ResultTypePlugin) exprToTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name

	case *ast.StarExpr:
		return "*" + p.exprToTypeString(t.X)

	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + p.exprToTypeString(t.Elt)
		}
		// For sized arrays, would need to evaluate length expression
		return "[]" + p.exprToTypeString(t.Elt)

	case *ast.SelectorExpr:
		pkg := p.exprToTypeString(t.X)
		return pkg + "." + t.Sel.Name

	case *ast.MapType:
		key := p.exprToTypeString(t.Key)
		value := p.exprToTypeString(t.Value)
		return fmt.Sprintf("map[%s]%s", key, value)

	case *ast.ChanType:
		elem := p.exprToTypeString(t.Value)
		switch t.Dir {
		case ast.SEND:
			return "chan<- " + elem
		case ast.RECV:
			return "<-chan " + elem
		default:
			return "chan " + elem
		}

	case *ast.InterfaceType:
		return "interface{}"

	case *ast.StructType:
		return "struct{}"

	case *ast.FuncType:
		return "func()"
	}

	return "interface{}"
}

// emitResultDeclaration generates the Result type declaration and helper methods
func (p *ResultTypePlugin) emitResultDeclaration(okType, errType, resultTypeName string) {
	if p.ctx == nil || p.ctx.FileSet == nil {
		return
	}

	// Generate ResultTag enum (only once)
	if !p.emittedTypes["ResultTag"] {
		p.emitResultTagEnum()
		p.emittedTypes["ResultTag"] = true
	}

	// Generate Result struct
	resultStruct := &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{
			&ast.TypeSpec{
				Name: ast.NewIdent(resultTypeName),
				Type: &ast.StructType{
					Fields: &ast.FieldList{
						List: []*ast.Field{
							{
								Names: []*ast.Ident{ast.NewIdent("tag")},
								Type:  ast.NewIdent("ResultTag"),
							},
							{
								Names: []*ast.Ident{ast.NewIdent("ok_0")},
								Type:  p.typeToAST(okType, true), // Pointer for zero-value safety
							},
							{
								Names: []*ast.Ident{ast.NewIdent("err_0")},
								Type:  p.typeToAST(errType, true), // Pointer
							},
						},
					},
				},
			},
		},
	}

	p.pendingDecls = append(p.pendingDecls, resultStruct)

	// Generate constructor functions
	p.emitConstructorFunction(resultTypeName, okType, true, "Ok")
	p.emitConstructorFunction(resultTypeName, errType, false, "Err")

	// Generate helper methods
	p.emitHelperMethods(resultTypeName, okType, errType)
}

// emitResultTagEnum generates the ResultTag enum
func (p *ResultTypePlugin) emitResultTagEnum() {
	// type ResultTag uint8
	tagTypeDecl := &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{
			&ast.TypeSpec{
				Name: ast.NewIdent("ResultTag"),
				Type: ast.NewIdent("uint8"),
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, tagTypeDecl)

	// const ( ResultTag_Ok ResultTag = iota; ResultTag_Err )
	tagConstDecl := &ast.GenDecl{
		Tok:    token.CONST,
		Lparen: 1, // Required for const block
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names: []*ast.Ident{ast.NewIdent("ResultTag_Ok")},
				Type:  ast.NewIdent("ResultTag"),
				Values: []ast.Expr{
					ast.NewIdent("iota"),
				},
			},
			&ast.ValueSpec{
				Names: []*ast.Ident{ast.NewIdent("ResultTag_Err")},
			},
		},
		Rparen: 2, // Required for const block
	}
	p.pendingDecls = append(p.pendingDecls, tagConstDecl)
}

// emitConstructorFunction generates Ok or Err constructor
func (p *ResultTypePlugin) emitConstructorFunction(resultTypeName, argType string, isOk bool, funcSuffix string) {
	variantTag := "ResultTag_Ok"
	fieldName := "ok_0"
	if !isOk {
		variantTag = "ResultTag_Err"
		fieldName = "err_0"
	}

	funcName := fmt.Sprintf("%s_%s", resultTypeName, funcSuffix)
	argTypeAST := p.typeToAST(argType, false) // Non-pointer parameter

	// func Result_T_E_Ok(arg0 T) Result_T_E {
	//     return Result_T_E{tag: ResultTag_Ok, ok_0: &arg0}
	// }
	constructorFunc := &ast.FuncDecl{
		Name: ast.NewIdent(funcName),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("arg0")},
						Type:  argTypeAST,
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{
						Type: ast.NewIdent(resultTypeName),
					},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.CompositeLit{
							Type: ast.NewIdent(resultTypeName),
							Elts: []ast.Expr{
								&ast.KeyValueExpr{
									Key:   ast.NewIdent("tag"),
									Value: ast.NewIdent(variantTag),
								},
								&ast.KeyValueExpr{
									Key: ast.NewIdent(fieldName),
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

// emitHelperMethods generates IsOk, IsErr, Unwrap, UnwrapOr, etc.
func (p *ResultTypePlugin) emitHelperMethods(resultTypeName, okType, errType string) {
	// IsOk() bool
	isOkMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("IsOk"),
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
							X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
							Op: token.EQL,
							Y:  ast.NewIdent("ResultTag_Ok"),
						},
					},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, isOkMethod)

	// IsErr() bool
	isErrMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("IsErr"),
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
							X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
							Op: token.EQL,
							Y:  ast.NewIdent("ResultTag_Err"),
						},
					},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, isErrMethod)

	// Unwrap() T - panics if Err
	// Note: Returns *T (dereferenced), so we need to handle pointer unwrapping
	unwrapMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("Unwrap"),
		Type: &ast.FuncType{
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: p.typeToAST(okType, false)}, // Non-pointer return
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// if r.tag != ResultTag_Ok { panic("called Unwrap on Err") }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
						Op: token.NEQ,
						Y:  ast.NewIdent("ResultTag_Ok"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: ast.NewIdent("panic"),
									Args: []ast.Expr{
										&ast.BasicLit{
											Kind:  token.STRING,
											Value: `"called Unwrap on Err"`,
										},
									},
								},
							},
						},
					},
				},
				// if r.ok_0 == nil { panic("Result contains nil Ok value") }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("ok_0")},
						Op: token.EQL,
						Y:  ast.NewIdent("nil"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: ast.NewIdent("panic"),
									Args: []ast.Expr{
										&ast.BasicLit{
											Kind:  token.STRING,
											Value: `"Result contains nil Ok value"`,
										},
									},
								},
							},
						},
					},
				},
				// return *r.ok_0
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.StarExpr{
							X: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("ok_0")},
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
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("UnwrapOr"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("defaultValue")},
						Type:  p.typeToAST(okType, false),
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: p.typeToAST(okType, false)},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// if r.tag == ResultTag_Ok { return *r.ok_0 }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
						Op: token.EQL,
						Y:  ast.NewIdent("ResultTag_Ok"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{
									&ast.StarExpr{
										X: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("ok_0")},
									},
								},
							},
						},
					},
				},
				// return defaultValue
				&ast.ReturnStmt{
					Results: []ast.Expr{ast.NewIdent("defaultValue")},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, unwrapOrMethod)

	// UnwrapErr() E - panics if Ok
	unwrapErrMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("UnwrapErr"),
		Type: &ast.FuncType{
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: p.typeToAST(errType, false)},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// if r.tag != ResultTag_Err { panic("called UnwrapErr on Ok") }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
						Op: token.NEQ,
						Y:  ast.NewIdent("ResultTag_Err"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: ast.NewIdent("panic"),
									Args: []ast.Expr{
										&ast.BasicLit{
											Kind:  token.STRING,
											Value: `"called UnwrapErr on Ok"`,
										},
									},
								},
							},
						},
					},
				},
				// if r.err_0 == nil { panic("Result contains nil Err value") }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("err_0")},
						Op: token.EQL,
						Y:  ast.NewIdent("nil"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: ast.NewIdent("panic"),
									Args: []ast.Expr{
										&ast.BasicLit{
											Kind:  token.STRING,
											Value: `"Result contains nil Err value"`,
										},
									},
								},
							},
						},
					},
				},
				// return *r.err_0
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.StarExpr{
							X: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("err_0")},
						},
					},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, unwrapErrMethod)

	// Task 1.3: Add complete helper method set
	// TODO(Stage 3): Implement advanced helper methods (Map, MapErr, Filter, AndThen, OrElse)
	// Currently disabled to prevent nil panics - these methods require generic type handling
	// p.emitAdvancedHelperMethods(resultTypeName, okType, errType)
}

// emitAdvancedHelperMethods generates Map, MapErr, Filter, AndThen, OrElse, And, Or methods
// Task 1.3: Complete helper method implementation
func (p *ResultTypePlugin) emitAdvancedHelperMethods(resultTypeName, okType, errType string) {
	// Map(fn func(T) U) Result<U, E>
	// Transforms the Ok value if present
	mapMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
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
									{Type: p.typeToAST(okType, false)},
								},
							},
							Results: &ast.FieldList{
								List: []*ast.Field{
									{Type: ast.NewIdent("interface{}")}, // Generic U type
								},
							},
						},
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent("interface{}")}, // Returns Result<U, E>
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// if r.tag == ResultTag_Ok { return fn(*r.ok_0) wrapped as Ok }
				// This is a placeholder - full implementation needs generic handling
				&ast.ReturnStmt{
					Results: []ast.Expr{ast.NewIdent("nil")},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, mapMethod)

	// MapErr(fn func(E) F) Result<T, F>
	// Transforms the Err value if present
	mapErrMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("MapErr"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("fn")},
						Type: &ast.FuncType{
							Params: &ast.FieldList{
								List: []*ast.Field{
									{Type: p.typeToAST(errType, false)},
								},
							},
							Results: &ast.FieldList{
								List: []*ast.Field{
									{Type: ast.NewIdent("interface{}")}, // Generic F type
								},
							},
						},
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent("interface{}")}, // Returns Result<T, F>
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{ast.NewIdent("nil")},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, mapErrMethod)

	// Filter(predicate func(T) bool) Result<T, E>
	// Converts Ok to Err if predicate fails
	filterMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
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
									{Type: p.typeToAST(okType, false)},
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
					{Type: ast.NewIdent(resultTypeName)},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// if r.tag == ResultTag_Ok && predicate(*r.ok_0) { return r }
				// else { return Err variant }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X: &ast.BinaryExpr{
							X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
							Op: token.EQL,
							Y:  ast.NewIdent("ResultTag_Ok"),
						},
						Op: token.LAND,
						Y: &ast.CallExpr{
							Fun: ast.NewIdent("predicate"),
							Args: []ast.Expr{
								&ast.StarExpr{
									X: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("ok_0")},
								},
							},
						},
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{ast.NewIdent("r")},
							},
						},
					},
				},
				// Return error variant (would need proper error creation)
				&ast.ReturnStmt{
					Results: []ast.Expr{ast.NewIdent("r")},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, filterMethod)

	// AndThen(fn func(T) Result<U, E>) Result<U, E>
	// Monadic bind operation
	andThenMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
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
									{Type: p.typeToAST(okType, false)},
								},
							},
							Results: &ast.FieldList{
								List: []*ast.Field{
									{Type: ast.NewIdent("interface{}")}, // Result<U, E>
								},
							},
						},
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent("interface{}")},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{ast.NewIdent("nil")},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, andThenMethod)

	// OrElse(fn func(E) Result<T, F>) Result<T, F>
	// Handle Err case with fallback
	orElseMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("OrElse"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("fn")},
						Type: &ast.FuncType{
							Params: &ast.FieldList{
								List: []*ast.Field{
									{Type: p.typeToAST(errType, false)},
								},
							},
							Results: &ast.FieldList{
								List: []*ast.Field{
									{Type: ast.NewIdent("interface{}")}, // Result<T, F>
								},
							},
						},
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent("interface{}")},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{ast.NewIdent("nil")},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, orElseMethod)

	// And(other Result<U, E>) Result<U, E>
	// Returns other if Ok, returns Err if Err
	andMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("And"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("other")},
						Type:  ast.NewIdent("interface{}"), // Generic Result<U, E>
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent("interface{}")},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// if r.tag == ResultTag_Ok { return other }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
						Op: token.EQL,
						Y:  ast.NewIdent("ResultTag_Ok"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{ast.NewIdent("other")},
							},
						},
					},
				},
				// return r (as Err variant)
				&ast.ReturnStmt{
					Results: []ast.Expr{ast.NewIdent("r")},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, andMethod)

	// Or(other Result<T, E>) Result<T, E>
	// Returns r if Ok, returns other if Err
	orMethod := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("r")},
					Type:  ast.NewIdent(resultTypeName),
				},
			},
		},
		Name: ast.NewIdent("Or"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("other")},
						Type:  ast.NewIdent(resultTypeName),
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent(resultTypeName)},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// if r.tag == ResultTag_Ok { return r }
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
						Op: token.EQL,
						Y:  ast.NewIdent("ResultTag_Ok"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{ast.NewIdent("r")},
							},
						},
					},
				},
				// return other
				&ast.ReturnStmt{
					Results: []ast.Expr{ast.NewIdent("other")},
				},
			},
		},
	}
	p.pendingDecls = append(p.pendingDecls, orMethod)
}

// getTypeName extracts type name from AST expression
func (p *ResultTypePlugin) getTypeName(expr ast.Expr) string {
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

// sanitizeTypeName converts type names to valid Go identifiers
// Examples:
//   *User → ptr_User
//   []byte → slice_byte
//   map[string]int → map_string_int
//   interface{} → any
func (p *ResultTypePlugin) sanitizeTypeName(typeName string) string {
	s := typeName

	// Special cases
	if s == "interface{}" {
		return "any"
	}

	s = strings.ReplaceAll(s, "*", "ptr_")
	s = strings.ReplaceAll(s, "[]", "slice_")
	s = strings.ReplaceAll(s, "[", "_")
	s = strings.ReplaceAll(s, "]", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "{", "")
	s = strings.ReplaceAll(s, "}", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.Trim(s, "_")
	return s
}

// typeToAST converts a type string to an AST type expression
func (p *ResultTypePlugin) typeToAST(typeName string, asPointer bool) ast.Expr {
	var baseType ast.Expr

	// Handle pointer types
	if strings.HasPrefix(typeName, "*") {
		baseType = &ast.StarExpr{
			X: ast.NewIdent(strings.TrimPrefix(typeName, "*")),
		}
	} else if strings.HasPrefix(typeName, "[]") {
		// Slice type
		baseType = &ast.ArrayType{
			Elt: ast.NewIdent(strings.TrimPrefix(typeName, "[]")),
		}
	} else {
		// Simple identifier
		baseType = ast.NewIdent(typeName)
	}

	// Wrap in pointer if requested
	if asPointer {
		return &ast.StarExpr{X: baseType}
	}

	return baseType
}

// GetPendingDeclarations returns declarations to be injected at package level
func (p *ResultTypePlugin) GetPendingDeclarations() []ast.Decl {
	return p.pendingDecls
}

// ClearPendingDeclarations clears the pending declarations list
func (p *ResultTypePlugin) ClearPendingDeclarations() {
	p.pendingDecls = make([]ast.Decl, 0)
}

// Transform performs AST transformations on the node
// This method replaces Ok() and Err() constructor calls with struct literals
func (p *ResultTypePlugin) Transform(node ast.Node) (ast.Node, error) {
	if p.ctx == nil {
		return nil, fmt.Errorf("plugin context not initialized")
	}

	// Use astutil.Apply to walk and transform the AST
	transformed := astutil.Apply(node,
		func(cursor *astutil.Cursor) bool {
			n := cursor.Node()

			// Check if this is a CallExpr we need to transform
			if call, ok := n.(*ast.CallExpr); ok {
				if ident, ok := call.Fun.(*ast.Ident); ok {
					var replacement ast.Expr
					switch ident.Name {
					case "Ok":
						replacement = p.transformOkConstructor(call)
					case "Err":
						replacement = p.transformErrConstructor(call)
					}

					// Replace the node if transformation occurred
					if replacement != nil && replacement != call {
						cursor.Replace(replacement)
					}
				}
			}
			return true
		},
		nil, // Post-order not needed
	)

	return transformed, nil
}
