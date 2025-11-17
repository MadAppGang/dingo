// Package builtin provides built-in Dingo transformation plugins
package builtin

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/plugin"
	"golang.org/x/tools/go/ast/astutil"
)

// ResultTypePlugin provides the built-in Result<T, E> type
//
// Result is a generic sum type that represents either success (Ok) or failure (Err).
// It integrates with the error propagation operator (?) and pattern matching.
//
// Dingo syntax:
//   Ok(value)   - Creates Result<T, error> with success value
//   Err(error)  - Creates Result<T, E> with error value
//
// Transpiles to:
//   Result_T_E{tag: ResultTag_Ok, ok_0: value}
//   Result_T_E{tag: ResultTag_Err, err_0: error}
//
// This plugin automatically registers Result as a built-in enum type.
type ResultTypePlugin struct {
	plugin.BasePlugin
	currentContext  *plugin.Context
	emittedTypes    map[string]bool // Track emitted type declarations to avoid duplicates
	generatedDecls  []ast.Decl      // Collect generated type declarations
}

// NewResultTypePlugin creates a new Result type plugin
func NewResultTypePlugin() *ResultTypePlugin {
	return &ResultTypePlugin{
		BasePlugin: *plugin.NewBasePlugin(
			"result_type",
			"Built-in Result<T, E> generic type for error handling",
			nil, // No dependencies
		),
		emittedTypes:   make(map[string]bool),
		generatedDecls: make([]ast.Decl, 0),
	}
}

// Name returns the plugin name
func (p *ResultTypePlugin) Name() string {
	return "result_type"
}

// Transform handles Result type transformations
func (p *ResultTypePlugin) Transform(ctx *plugin.Context, node ast.Node) (ast.Node, error) {
	file, ok := node.(*ast.File)
	if !ok {
		return node, nil
	}

	p.currentContext = ctx

	// Reset state for new file
	p.emittedTypes = make(map[string]bool)
	p.generatedDecls = make([]ast.Decl, 0)

	// Use astutil.Apply for proper node replacement
	result := astutil.Apply(file, func(cursor *astutil.Cursor) bool {
		n := cursor.Node()

		// Look for call expressions
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check if it's an Ok() or Err() call
		ident, ok := callExpr.Fun.(*ast.Ident)
		if !ok {
			return true
		}

		if ident.Name == "Ok" {
			if replacement := p.transformOkLiteral(callExpr, ctx); replacement != nil {
				cursor.Replace(replacement)
			}
		} else if ident.Name == "Err" {
			if replacement := p.transformErrLiteral(callExpr, ctx); replacement != nil {
				cursor.Replace(replacement)
			}
		}

		return true
	}, nil)

	// Add generated type declarations to the beginning of the file
	if resultFile, ok := result.(*ast.File); ok && len(p.generatedDecls) > 0 {
		// Insert type declarations after imports, before other declarations
		importCount := 0
		for _, decl := range resultFile.Decls {
			if _, isImport := decl.(*ast.GenDecl); isImport {
				importCount++
			} else {
				break
			}
		}

		// Insert generated declarations
		newDecls := make([]ast.Decl, 0, len(resultFile.Decls)+len(p.generatedDecls))
		newDecls = append(newDecls, resultFile.Decls[:importCount]...)
		newDecls = append(newDecls, p.generatedDecls...)
		newDecls = append(newDecls, resultFile.Decls[importCount:]...)
		resultFile.Decls = newDecls

		return resultFile, nil
	}

	return result, nil
}

// transformOkLiteral transforms Ok(value) into Result_T_error{tag: ResultTag_Ok, ok_0: value}
func (p *ResultTypePlugin) transformOkLiteral(callExpr *ast.CallExpr, ctx *plugin.Context) ast.Node {
	// CRITICAL FIX #6: Add nil checks
	if callExpr == nil || ctx == nil {
		return nil
	}

	if len(callExpr.Args) != 1 {
		if ctx.Logger != nil {
			ctx.Logger.Error("Ok() expects exactly 1 argument, got %d", len(callExpr.Args))
		}
		return nil
	}

	valueExpr := callExpr.Args[0]
	if valueExpr == nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("Ok() argument is nil")
		}
		return nil
	}

	// Infer T from the argument
	var valueTypeName string
	if ctx.TypeInference != nil {
		if service, ok := ctx.TypeInference.(*TypeInferenceService); ok {
			if typ, err := service.InferType(valueExpr); err == nil && typ != nil {
				valueTypeName = p.typeToString(typ)
			}
		}
	}

	// Fallback: Try to infer from the expression itself
	if valueTypeName == "" {
		valueTypeName = p.inferTypeFromExpr(valueExpr)
	}

	// E defaults to "error"
	errorTypeName := "error"

	// Generate Result_T_E type name (sanitized for Go identifiers)
	resultTypeName := fmt.Sprintf("Result_%s_%s", p.sanitizeTypeName(valueTypeName), p.sanitizeTypeName(errorTypeName))

	// Ensure Result type declaration is emitted
	p.emitResultDeclaration(resultTypeName, valueTypeName, errorTypeName)

	// Create composite literal: Result_T_E{tag: ResultTag_Ok, ok_0: value}
	return &ast.CompositeLit{
		Type: &ast.Ident{Name: resultTypeName},
		Elts: []ast.Expr{
			&ast.KeyValueExpr{
				Key:   &ast.Ident{Name: "tag"},
				Value: &ast.Ident{Name: "ResultTag_Ok"},
			},
			&ast.KeyValueExpr{
				Key:   &ast.Ident{Name: "ok_0"},
				Value: valueExpr,
			},
		},
	}
}

// transformErrLiteral transforms Err(error) into Result_T_error{tag: ResultTag_Err, err_0: error}
func (p *ResultTypePlugin) transformErrLiteral(callExpr *ast.CallExpr, ctx *plugin.Context) ast.Node {
	// CRITICAL FIX #6: Add nil checks
	if callExpr == nil || ctx == nil {
		return nil
	}

	if len(callExpr.Args) != 1 {
		if ctx.Logger != nil {
			ctx.Logger.Error("Err() expects exactly 1 argument, got %d", len(callExpr.Args))
		}
		return nil
	}

	errorExpr := callExpr.Args[0]
	if errorExpr == nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("Err() argument is nil")
		}
		return nil
	}

	// Infer E from the argument
	var errorTypeName string
	if ctx.TypeInference != nil {
		if service, ok := ctx.TypeInference.(*TypeInferenceService); ok {
			if typ, err := service.InferType(errorExpr); err == nil && typ != nil {
				errorTypeName = p.typeToString(typ)
			}
		}
	}

	// Fallback
	if errorTypeName == "" {
		errorTypeName = "error"
	}

	// CRITICAL FIX #3: T requires context inference - fail with clear error for now
	// TODO: Implement parent function return type analysis
	valueTypeName := ""
	if ctx.TypeInference != nil {
		// Try to infer from enclosing function context (future enhancement)
		// For now, fail-fast with clear error
	}

	if valueTypeName == "" {
		// Generate invalid code that clearly shows the problem
		ctx.Logger.Error("Err() requires function return type annotation - cannot infer success type")
		valueTypeName = "ERROR_CANNOT_INFER_TYPE" // Will fail compilation with clear message
	}

	// Generate Result_T_E type name
	resultTypeName := fmt.Sprintf("Result_%s_%s", p.sanitizeTypeName(valueTypeName), p.sanitizeTypeName(errorTypeName))

	// Ensure Result type declaration is emitted (even with error placeholder)
	p.emitResultDeclaration(resultTypeName, valueTypeName, errorTypeName)

	// Create composite literal: Result_T_E{tag: ResultTag_Err, err_0: error}
	return &ast.CompositeLit{
		Type: &ast.Ident{Name: resultTypeName},
		Elts: []ast.Expr{
			&ast.KeyValueExpr{
				Key:   &ast.Ident{Name: "tag"},
				Value: &ast.Ident{Name: "ResultTag_Err"},
			},
			&ast.KeyValueExpr{
				Key:   &ast.Ident{Name: "err_0"},
				Value: errorExpr,
			},
		},
	}
}

// typeToString converts a types.Type to a string representation for naming
func (p *ResultTypePlugin) typeToString(typ types.Type) string {
	if typ == nil {
		return "unknown"
	}

	// Handle basic types
	switch t := typ.(type) {
	case *types.Basic:
		return t.Name()
	case *types.Named:
		obj := t.Obj()
		if obj != nil {
			return obj.Name()
		}
	case *types.Pointer:
		elem := p.typeToString(t.Elem())
		return "ptr_" + elem
	case *types.Slice:
		elem := p.typeToString(t.Elem())
		return "slice_" + elem
	case *types.Array:
		elem := p.typeToString(t.Elem())
		return fmt.Sprintf("array_%s", elem)
	case *types.Map:
		key := p.typeToString(t.Key())
		val := p.typeToString(t.Elem())
		return fmt.Sprintf("map_%s_%s", key, val)
	}

	// Fallback to String() method
	str := typ.String()
	// Remove package paths for cleaner names
	if idx := strings.LastIndex(str, "."); idx >= 0 {
		str = str[idx+1:]
	}
	return str
}

// sanitizeTypeName ensures type names are valid Go identifiers
func (p *ResultTypePlugin) sanitizeTypeName(name string) string {
	// Replace invalid characters with underscores
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "[", "_")
	name = strings.ReplaceAll(name, "]", "_")
	name = strings.ReplaceAll(name, "*", "ptr_")
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "(", "_")
	name = strings.ReplaceAll(name, ")", "_")
	name = strings.ReplaceAll(name, ",", "_")
	return name
}

// inferTypeFromExpr tries to infer type from the expression structure
func (p *ResultTypePlugin) inferTypeFromExpr(expr ast.Expr) string {
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
		// Return the identifier name as a hint
		return e.Name
	case *ast.CompositeLit:
		if e.Type != nil {
			if ident, ok := e.Type.(*ast.Ident); ok {
				return ident.Name
			}
		}
	}
	return "T" // Generic placeholder
}

// CreateResultEnum creates a synthetic Result<T, E> enum declaration
// This can be used by other plugins to understand Result's structure
func (p *ResultTypePlugin) CreateResultEnum() *dingoast.EnumDecl {
	return &dingoast.EnumDecl{
		Name: &ast.Ident{Name: "Result"},
		TypeParams: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{{Name: "T"}},
					Type:  &ast.Ident{Name: "any"},
				},
				{
					Names: []*ast.Ident{{Name: "E"}},
					Type:  &ast.Ident{Name: "any"},
				},
			},
		},
		Variants: []*dingoast.VariantDecl{
			{
				Name: &ast.Ident{Name: "Ok"},
				Kind: dingoast.VariantTuple,
				Fields: &ast.FieldList{
					List: []*ast.Field{
						{Type: &ast.Ident{Name: "T"}},
					},
				},
			},
			{
				Name: &ast.Ident{Name: "Err"},
				Kind: dingoast.VariantTuple,
				Fields: &ast.FieldList{
					List: []*ast.Field{
						{Type: &ast.Ident{Name: "E"}},
					},
				},
			},
		},
	}
}

// GenerateHelperMethods generates helper methods for Result<T, E>
// These are injected into files that use Result
func (p *ResultTypePlugin) GenerateHelperMethods(typeArgs []ast.Expr) []ast.Decl {
	decls := make([]ast.Decl, 0)

	// Type name for this specific Result instance (e.g., Result[T, E])
	// Always use IndexListExpr for generic types in Go 1.18+
	resultType := &ast.IndexListExpr{
		X:       &ast.Ident{Name: "Result"},
		Indices: typeArgs,
	}

	// Generate IsOk() method
	decls = append(decls, &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{{Name: "r"}},
					Type:  resultType,
				},
			},
		},
		Name: &ast.Ident{Name: "IsOk"},
		Type: &ast.FuncType{
			Params: &ast.FieldList{},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: &ast.Ident{Name: "bool"}},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.BinaryExpr{
							X: &ast.SelectorExpr{
								X:   &ast.Ident{Name: "r"},
								Sel: &ast.Ident{Name: "tag"},
							},
							Op: token.EQL,
							Y:  &ast.Ident{Name: "ResultTag_Ok"},
						},
					},
				},
			},
		},
	})

	// Generate IsErr() method
	decls = append(decls, &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{{Name: "r"}},
					Type:  resultType,
				},
			},
		},
		Name: &ast.Ident{Name: "IsErr"},
		Type: &ast.FuncType{
			Params: &ast.FieldList{},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: &ast.Ident{Name: "bool"}},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.BinaryExpr{
							X: &ast.SelectorExpr{
								X:   &ast.Ident{Name: "r"},
								Sel: &ast.Ident{Name: "tag"},
							},
							Op: token.EQL,
							Y:  &ast.Ident{Name: "ResultTag_Err"},
						},
					},
				},
			},
		},
	})

	// Generate Unwrap() method
	// func (r Result[T,E]) Unwrap() T {
	//     if r.IsErr() { panic("called Unwrap on Err") }
	//     return *r.ok_0
	// }
	decls = append(decls, &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{{Name: "r"}},
					Type:  resultType,
				},
			},
		},
		Name: &ast.Ident{Name: "Unwrap"},
		Type: &ast.FuncType{
			Params: &ast.FieldList{},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: typeArgs[0]}, // T
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.IfStmt{
					Cond: &ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   &ast.Ident{Name: "r"},
							Sel: &ast.Ident{Name: "IsErr"},
						},
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: &ast.Ident{Name: "panic"},
									Args: []ast.Expr{
										&ast.BasicLit{
											Kind:  token.STRING,
											Value: `"dingo: called Result.Unwrap() on Err value"`,
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
							X: &ast.SelectorExpr{
								X:   &ast.Ident{Name: "r"},
								Sel: &ast.Ident{Name: "ok_0"},
							},
						},
					},
				},
			},
		},
	})

	// Generate UnwrapOr(default T) method
	decls = append(decls, &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{{Name: "r"}},
					Type:  resultType,
				},
			},
		},
		Name: &ast.Ident{Name: "UnwrapOr"},
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{{Name: "defaultVal"}},
						Type:  typeArgs[0], // T
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: typeArgs[0]}, // T
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.IfStmt{
					Cond: &ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   &ast.Ident{Name: "r"},
							Sel: &ast.Ident{Name: "IsOk"},
						},
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{
									&ast.StarExpr{
										X: &ast.SelectorExpr{
											X:   &ast.Ident{Name: "r"},
											Sel: &ast.Ident{Name: "ok_0"},
										},
									},
								},
							},
						},
					},
				},
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.Ident{Name: "defaultVal"},
					},
				},
			},
		},
	})

	return decls
}

// emitResultDeclaration generates the Result type declaration (struct, tag enum, constants)
// if it hasn't been emitted yet for this T/E combination
func (p *ResultTypePlugin) emitResultDeclaration(resultTypeName, valueTypeName, errorTypeName string) {
	// Check if already emitted
	if p.emittedTypes[resultTypeName] {
		return
	}

	// Mark as emitted
	p.emittedTypes[resultTypeName] = true

	// Generate tag enum name
	tagName := resultTypeName + "Tag"

	// 1. Generate tag type declaration: type Result_T_E_Tag uint8
	typeSpec := &ast.TypeSpec{
		Name: &ast.Ident{Name: tagName},
		Type: &ast.Ident{Name: "uint8"},
	}
	typeDecl := &ast.GenDecl{
		Tok:   token.TYPE,
		Specs: []ast.Spec{typeSpec},
	}
	p.generatedDecls = append(p.generatedDecls, typeDecl)

	// 2. Generate tag constants: const ( Result_T_E_Tag_Ok Result_T_E_Tag = iota; Result_T_E_Tag_Err )
	constSpecs := []ast.Spec{
		&ast.ValueSpec{
			Names:  []*ast.Ident{{Name: tagName + "_Ok"}},
			Type:   &ast.Ident{Name: tagName},
			Values: []ast.Expr{&ast.Ident{Name: "iota"}},
		},
		&ast.ValueSpec{
			Names: []*ast.Ident{{Name: tagName + "_Err"}},
		},
	}
	constDecl := &ast.GenDecl{
		Tok:    token.CONST,
		Lparen: 1, // Grouped const
		Specs:  constSpecs,
	}
	p.generatedDecls = append(p.generatedDecls, constDecl)

	// 3. Generate struct type: type Result_T_E struct { tag Result_T_E_Tag; ok_0 *T; err_0 *E }
	structFields := []*ast.Field{
		{
			Names: []*ast.Ident{{Name: "tag"}},
			Type:  &ast.Ident{Name: tagName},
		},
		{
			Names: []*ast.Ident{{Name: "ok_0"}},
			Type:  &ast.StarExpr{X: &ast.Ident{Name: valueTypeName}},
		},
		{
			Names: []*ast.Ident{{Name: "err_0"}},
			Type:  &ast.StarExpr{X: &ast.Ident{Name: errorTypeName}},
		},
	}

	structTypeSpec := &ast.TypeSpec{
		Name: &ast.Ident{Name: resultTypeName},
		Type: &ast.StructType{
			Fields: &ast.FieldList{List: structFields},
		},
	}
	structDecl := &ast.GenDecl{
		Tok:   token.TYPE,
		Specs: []ast.Spec{structTypeSpec},
	}
	p.generatedDecls = append(p.generatedDecls, structDecl)

	if p.currentContext != nil && p.currentContext.Logger != nil {
		p.currentContext.Logger.Debug("Generated Result type: %s", resultTypeName)
	}
}
