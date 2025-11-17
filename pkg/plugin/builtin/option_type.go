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

// OptionTypePlugin provides the built-in Option<T> type
//
// Option is a generic sum type that represents a value that may or may not be present.
// It eliminates nil pointer bugs by making nullability explicit in the type system.
//
// Dingo syntax:
//   Some(value) - Creates Option<T> with a value
//   None        - Creates Option<T> with no value
//
// Transpiles to:
//   Option_T{tag: OptionTag_Some, some_0: value}
//   Option_T{tag: OptionTag_None}
//
// This plugin automatically registers Option as a built-in enum type.
type OptionTypePlugin struct {
	plugin.BasePlugin
	currentContext  *plugin.Context
	emittedTypes    map[string]bool // Track emitted type declarations to avoid duplicates
	generatedDecls  []ast.Decl      // Collect generated type declarations
}

// NewOptionTypePlugin creates a new Option type plugin
func NewOptionTypePlugin() *OptionTypePlugin {
	return &OptionTypePlugin{
		BasePlugin: *plugin.NewBasePlugin(
			"option_type",
			"Built-in Option<T> generic type for null safety",
			nil, // No dependencies
		),
		emittedTypes:   make(map[string]bool),
		generatedDecls: make([]ast.Decl, 0),
	}
}

// Name returns the plugin name
func (p *OptionTypePlugin) Name() string {
	return "option_type"
}

// Transform handles Option type transformations
func (p *OptionTypePlugin) Transform(ctx *plugin.Context, node ast.Node) (ast.Node, error) {
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

		// Look for Some() call expressions
		if callExpr, ok := n.(*ast.CallExpr); ok {
			if ident, ok := callExpr.Fun.(*ast.Ident); ok && ident.Name == "Some" {
				if replacement := p.transformSomeLiteral(callExpr, ctx); replacement != nil {
					cursor.Replace(replacement)
				}
			}
		}

		// Look for None identifier
		if ident, ok := n.(*ast.Ident); ok && ident.Name == "None" {
			// None is trickier - we need type context
			// For now, we'll leave None as-is and require explicit type annotation
			// TODO: Infer Option_T from assignment or return context
			// if replacement := p.transformNoneLiteral(ident, ctx); replacement != nil {
			//     cursor.Replace(replacement)
			// }
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

// transformSomeLiteral transforms Some(value) into Option_T{tag: OptionTag_Some, some_0: value}
func (p *OptionTypePlugin) transformSomeLiteral(callExpr *ast.CallExpr, ctx *plugin.Context) ast.Node {
	// CRITICAL FIX #6: Add nil checks
	if callExpr == nil || ctx == nil {
		return nil
	}

	if len(callExpr.Args) != 1 {
		if ctx.Logger != nil {
			ctx.Logger.Error("Some() expects exactly 1 argument, got %d", len(callExpr.Args))
		}
		return nil
	}

	valueExpr := callExpr.Args[0]
	if valueExpr == nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("Some() argument is nil")
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

	// Generate Option_T type name (sanitized for Go identifiers)
	optionTypeName := fmt.Sprintf("Option_%s", p.sanitizeTypeName(valueTypeName))

	// Ensure Option type declaration is emitted
	p.emitOptionDeclaration(optionTypeName, valueTypeName)

	// Create composite literal: Option_T{tag: OptionTag_Some, some_0: value}
	return &ast.CompositeLit{
		Type: &ast.Ident{Name: optionTypeName},
		Elts: []ast.Expr{
			&ast.KeyValueExpr{
				Key:   &ast.Ident{Name: "tag"},
				Value: &ast.Ident{Name: "OptionTag_Some"},
			},
			&ast.KeyValueExpr{
				Key:   &ast.Ident{Name: "some_0"},
				Value: valueExpr,
			},
		},
	}
}

// transformNoneLiteral transforms None into Option_T{tag: OptionTag_None}
// This requires type context which is currently not implemented
func (p *OptionTypePlugin) transformNoneLiteral(ident *ast.Ident, ctx *plugin.Context) ast.Node {
	// TODO: Implement type context inference
	// For now, None requires explicit type annotation or will be handled by the sum_types plugin
	return nil
}

// typeToString converts a types.Type to a string representation for naming
func (p *OptionTypePlugin) typeToString(typ types.Type) string {
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
func (p *OptionTypePlugin) sanitizeTypeName(name string) string {
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

// CreateOptionEnum creates a synthetic Option<T> enum declaration
// This can be used by other plugins to understand Option's structure
func (p *OptionTypePlugin) CreateOptionEnum() *dingoast.EnumDecl {
	return &dingoast.EnumDecl{
		Name: &ast.Ident{Name: "Option"},
		TypeParams: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{{Name: "T"}},
					Type:  &ast.Ident{Name: "any"},
				},
			},
		},
		Variants: []*dingoast.VariantDecl{
			{
				Name: &ast.Ident{Name: "Some"},
				Kind: dingoast.VariantTuple,
				Fields: &ast.FieldList{
					List: []*ast.Field{
						{Type: &ast.Ident{Name: "T"}},
					},
				},
			},
			{
				Name: &ast.Ident{Name: "None"},
				Kind: dingoast.VariantUnit,
			},
		},
	}
}

// GenerateHelperMethods generates helper methods for Option<T>
// These are injected into files that use Option
func (p *OptionTypePlugin) GenerateHelperMethods(typeArgs []ast.Expr) []ast.Decl {
	decls := make([]ast.Decl, 0)

	// Type name for this specific Option instance (e.g., Option[T])
	// Always use IndexExpr for generic types with single type parameter in Go 1.18+
	optionType := &ast.IndexExpr{
		X:     &ast.Ident{Name: "Option"},
		Index: typeArgs[0],
	}

	// Generate IsSome() method
	decls = append(decls, &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{{Name: "o"}},
					Type:  optionType,
				},
			},
		},
		Name: &ast.Ident{Name: "IsSome"},
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
								X:   &ast.Ident{Name: "o"},
								Sel: &ast.Ident{Name: "tag"},
							},
							Op: token.EQL,
							Y:  &ast.Ident{Name: "OptionTag_Some"},
						},
					},
				},
			},
		},
	})

	// Generate IsNone() method
	decls = append(decls, &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{{Name: "o"}},
					Type:  optionType,
				},
			},
		},
		Name: &ast.Ident{Name: "IsNone"},
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
								X:   &ast.Ident{Name: "o"},
								Sel: &ast.Ident{Name: "tag"},
							},
							Op: token.EQL,
							Y:  &ast.Ident{Name: "OptionTag_None"},
						},
					},
				},
			},
		},
	})

	// Generate Unwrap() method
	// func (o Option[T]) Unwrap() T {
	//     if o.IsNone() { panic("called Unwrap on None") }
	//     return *o.some_0
	// }
	decls = append(decls, &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{{Name: "o"}},
					Type:  optionType,
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
							X:   &ast.Ident{Name: "o"},
							Sel: &ast.Ident{Name: "IsNone"},
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
											Value: `"dingo: called Option.Unwrap() on None value"`,
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
								X:   &ast.Ident{Name: "o"},
								Sel: &ast.Ident{Name: "some_0"},
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
					Names: []*ast.Ident{{Name: "o"}},
					Type:  optionType,
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
							X:   &ast.Ident{Name: "o"},
							Sel: &ast.Ident{Name: "IsSome"},
						},
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{
									&ast.StarExpr{
										X: &ast.SelectorExpr{
											X:   &ast.Ident{Name: "o"},
											Sel: &ast.Ident{Name: "some_0"},
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

// emitOptionDeclaration generates the Option type declaration (struct, tag enum, constants)
// if it hasn't been emitted yet for this T combination
func (p *OptionTypePlugin) emitOptionDeclaration(optionTypeName, valueTypeName string) {
	// Check if already emitted
	if p.emittedTypes[optionTypeName] {
		return
	}

	// Mark as emitted
	p.emittedTypes[optionTypeName] = true

	// Generate tag enum name
	tagName := optionTypeName + "Tag"

	// 1. Generate tag type declaration: type Option_T_Tag uint8
	typeSpec := &ast.TypeSpec{
		Name: &ast.Ident{Name: tagName},
		Type: &ast.Ident{Name: "uint8"},
	}
	typeDecl := &ast.GenDecl{
		Tok:   token.TYPE,
		Specs: []ast.Spec{typeSpec},
	}
	p.generatedDecls = append(p.generatedDecls, typeDecl)

	// 2. Generate tag constants: const ( Option_T_Tag_Some Option_T_Tag = iota; Option_T_Tag_None )
	constSpecs := []ast.Spec{
		&ast.ValueSpec{
			Names:  []*ast.Ident{{Name: tagName + "_Some"}},
			Type:   &ast.Ident{Name: tagName},
			Values: []ast.Expr{&ast.Ident{Name: "iota"}},
		},
		&ast.ValueSpec{
			Names: []*ast.Ident{{Name: tagName + "_None"}},
		},
	}
	constDecl := &ast.GenDecl{
		Tok:    token.CONST,
		Lparen: 1, // Grouped const
		Specs:  constSpecs,
	}
	p.generatedDecls = append(p.generatedDecls, constDecl)

	// 3. Generate struct type: type Option_T struct { tag Option_T_Tag; some_0 *T }
	structFields := []*ast.Field{
		{
			Names: []*ast.Ident{{Name: "tag"}},
			Type:  &ast.Ident{Name: tagName},
		},
		{
			Names: []*ast.Ident{{Name: "some_0"}},
			Type:  &ast.StarExpr{X: &ast.Ident{Name: valueTypeName}},
		},
	}

	structTypeSpec := &ast.TypeSpec{
		Name: &ast.Ident{Name: optionTypeName},
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
		p.currentContext.Logger.Debug("Generated Option type: %s", optionTypeName)
	}
}
