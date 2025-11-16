// Package builtin provides built-in Dingo transformation plugins
package builtin

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/plugin"
	"golang.org/x/tools/go/ast/astutil"
)

// SumTypesPlugin transforms enum declarations and match expressions into Go code
//
// Features:
// - Enum declarations → Tagged union structs
// - Constructor functions for each variant
// - Is* helper methods for variant checking
// - Match expressions → Switch statements with destructuring
// - Exhaustiveness checking (coming in Phase 4)
//
// Transforms:
//   enum Shape { Circle{r: float64}, Point } → type Shape struct { tag ShapeTag; ... }
//   match x { Circle{r} => r, Point => 0.0 } → switch x.tag { case ShapeTag_Circle: ... }
type SumTypesPlugin struct {
	plugin.BasePlugin

	// State for current transformation
	currentFile     *dingoast.File
	currentContext  *plugin.Context
	enumRegistry    map[string]*dingoast.EnumDecl // Track all enums in file

	// Generated code tracking
	generatedDecls  []ast.Decl
	emittedDebugVar bool // Track if dingoDebug variable has been emitted
}

// NewSumTypesPlugin creates a new sum types transformation plugin
func NewSumTypesPlugin() *SumTypesPlugin {
	return &SumTypesPlugin{
		BasePlugin:     *plugin.NewBasePlugin("sum_types", "Sum types (enums) with pattern matching", nil),
		enumRegistry:   make(map[string]*dingoast.EnumDecl),
		generatedDecls: nil,
	}
}

// Name returns the plugin name
func (p *SumTypesPlugin) Name() string {
	return "sum_types"
}

// Transform transforms an AST node (file-level entry point)
func (p *SumTypesPlugin) Transform(ctx *plugin.Context, node ast.Node) (ast.Node, error) {
	file, ok := node.(*ast.File)
	if !ok {
		return node, nil
	}

	// Store context
	p.currentContext = ctx

	// Get the Dingo file wrapper to access DingoNodes map
	if ctx.CurrentFile != nil {
		if dingoFile, ok := ctx.CurrentFile.(*dingoast.File); ok {
			p.currentFile = dingoFile
		}
	}

	// If no Dingo nodes, nothing to transform
	if p.currentFile == nil || !p.currentFile.HasDingoNodes() {
		return node, nil
	}

	// Reset state
	p.enumRegistry = make(map[string]*dingoast.EnumDecl)
	p.generatedDecls = make([]ast.Decl, 0)
	p.emittedDebugVar = false

	// First pass: Collect all enum declarations and register them
	if err := p.collectEnums(file); err != nil {
		return nil, fmt.Errorf("collecting enums: %w", err)
	}

	// Second pass: Transform the AST
	result := astutil.Apply(file, p.preVisit, p.postVisit)

	// Validate result
	if result == nil {
		return nil, fmt.Errorf("AST transformation failed")
	}

	// Add generated declarations (tag enums, constructors, helpers) at the end
	if resultFile, ok := result.(*ast.File); ok {
		resultFile.Decls = append(resultFile.Decls, p.generatedDecls...)
		return resultFile, nil
	}

	return result, nil
}

// collectEnums builds a registry of all enum declarations in the file with validation
func (p *SumTypesPlugin) collectEnums(file *ast.File) error {
	for _, decl := range file.Decls {
		// Check if this declaration is a placeholder for an enum
		if dingoNode, hasDingo := p.currentFile.GetDingoNode(decl); hasDingo {
			if enumDecl, isEnum := dingoNode.(*dingoast.EnumDecl); isEnum {
				// Check for duplicate enum names
				if existing, exists := p.enumRegistry[enumDecl.Name.Name]; exists {
					return fmt.Errorf("duplicate enum %s (previous at %v)",
						enumDecl.Name.Name, existing.Name.Pos())
				}

				// Check for duplicate variant names within enum
				variantNames := make(map[string]bool)
				for _, v := range enumDecl.Variants {
					if variantNames[v.Name.Name] {
						return fmt.Errorf("duplicate variant %s in enum %s",
							v.Name.Name, enumDecl.Name.Name)
					}
					variantNames[v.Name.Name] = true
				}

				p.enumRegistry[enumDecl.Name.Name] = enumDecl
			}
		}
	}
	return nil
}

// preVisit is called before visiting a node's children
func (p *SumTypesPlugin) preVisit(cursor *astutil.Cursor) bool {
	return true // Continue traversal
}

// postVisit is called after visiting a node's children
func (p *SumTypesPlugin) postVisit(cursor *astutil.Cursor) bool {
	node := cursor.Node()

	// Check if this is a placeholder for a Dingo sum types node
	if decl, ok := node.(ast.Decl); ok {
		if dingoNode, hasDingo := p.currentFile.GetDingoNode(decl); hasDingo {
			if enumDecl, isEnum := dingoNode.(*dingoast.EnumDecl); isEnum {
				// Transform enum declaration to Go tagged union
				p.transformEnumDecl(cursor, enumDecl)
				return true
			}
		}
	}

	// Check for match expressions
	if expr, ok := node.(ast.Expr); ok {
		if dingoNode, hasDingo := p.currentFile.GetDingoNode(expr); hasDingo {
			if matchExpr, isMatch := dingoNode.(*dingoast.MatchExpr); isMatch {
				// Transform match expression to switch statement
				p.transformMatchExpr(cursor, matchExpr)
				return true
			}
		}
	}

	return true
}

// transformEnumDecl transforms an enum declaration into a tagged union struct
func (p *SumTypesPlugin) transformEnumDecl(cursor *astutil.Cursor, enumDecl *dingoast.EnumDecl) {
	enumName := enumDecl.Name.Name

	// Generate tag enum type and constants (returns 2 declarations)
	tagDecls := p.generateTagEnum(enumDecl)
	p.generatedDecls = append(p.generatedDecls, tagDecls...)

	// Generate tagged union struct
	unionDecl := p.generateUnionStruct(enumDecl)
	p.generatedDecls = append(p.generatedDecls, unionDecl)

	// Generate constructor functions for each variant
	for _, variant := range enumDecl.Variants {
		constructor := p.generateConstructor(enumDecl, variant)
		p.generatedDecls = append(p.generatedDecls, constructor)
	}

	// Generate Is* helper methods for each variant
	for _, variant := range enumDecl.Variants {
		helper := p.generateHelperMethod(enumDecl, variant)
		p.generatedDecls = append(p.generatedDecls, helper)
	}

	// Get the placeholder node before deleting
	placeholder := cursor.Node()

	// Remove the placeholder declaration
	cursor.Delete()

	// Clean up placeholder from DingoNodes map
	if p.currentFile != nil {
		p.currentFile.RemoveDingoNode(placeholder)
	}

	p.currentContext.Logger.Info("Generated sum type: %s with %d variants", enumName, len(enumDecl.Variants))
}

// generateTagEnum creates the tag enum type and constants
// Example: type ShapeTag uint8; const ( ShapeTag_Circle ShapeTag = iota; ... )
// Returns TWO declarations: [typeDecl, constDecl]
func (p *SumTypesPlugin) generateTagEnum(enumDecl *dingoast.EnumDecl) []ast.Decl {
	enumName := enumDecl.Name.Name
	tagName := enumName + "Tag"

	// Use enum declaration position for generated code
	pos := enumDecl.Name.Pos()

	// Create type declaration: type ShapeTag uint8
	typeSpec := &ast.TypeSpec{
		Name: &ast.Ident{Name: tagName},
		Type: &ast.Ident{Name: "uint8"},
	}

	typeDecl := &ast.GenDecl{
		TokPos: pos, // FIX: Add position information
		Tok:    token.TYPE,
		Specs:  []ast.Spec{typeSpec},
	}

	// Create const block with iota
	constSpecs := make([]ast.Spec, len(enumDecl.Variants))
	for i, variant := range enumDecl.Variants {
		constName := tagName + "_" + variant.Name.Name
		var value ast.Expr
		if i == 0 {
			// First constant uses iota
			value = &ast.Ident{Name: "iota"}
		}
		constSpecs[i] = &ast.ValueSpec{
			Names: []*ast.Ident{{Name: constName}},
			Type:  &ast.Ident{Name: tagName},
			Values: func() []ast.Expr {
				if value != nil {
					return []ast.Expr{value}
				}
				return nil
			}(),
		}
	}

	constDecl := &ast.GenDecl{
		TokPos: pos,       // FIX: Add position information
		Tok:    token.CONST,
		Lparen: 1, // Grouped const
		Specs:  constSpecs,
	}

	return []ast.Decl{typeDecl, constDecl}
}

// generateUnionStruct creates the tagged union struct
// Example: type Shape struct { tag ShapeTag; circle_r *float64; ... }
//
// MEMORY LAYOUT:
// Tagged unions use a discriminated union pattern with pointer fields.
// Memory overhead: 1 byte (tag) + 8 bytes per variant field (pointer)
// Only the active variant's fields are non-nil, others are nil.
// This design enables safe pattern matching and variant checking.
func (p *SumTypesPlugin) generateUnionStruct(enumDecl *dingoast.EnumDecl) ast.Decl {
	enumName := enumDecl.Name.Name
	tagName := enumName + "Tag"

	// Use enum declaration position for generated code
	pos := enumDecl.Name.Pos()

	// Start with tag field
	fields := []*ast.Field{
		{
			Names: []*ast.Ident{{Name: "tag"}},
			Type:  &ast.Ident{Name: tagName},
		},
	}

	// Add fields for each variant's data
	for _, variant := range enumDecl.Variants {
		fields = append(fields, p.generateVariantFields(variant)...)
	}

	// Create struct type
	structType := &ast.StructType{
		Fields: &ast.FieldList{
			List: fields,
		},
	}

	// Create type spec
	typeSpec := &ast.TypeSpec{
		Name: &ast.Ident{Name: enumName},
		Type: structType,
	}

	// Handle generics if present
	if enumDecl.TypeParams != nil {
		typeSpec.TypeParams = enumDecl.TypeParams
	}

	return &ast.GenDecl{
		TokPos: pos, // FIX: Add position information
		Tok:    token.TYPE,
		Specs:  []ast.Spec{typeSpec},
	}
}

// generateVariantFields creates struct fields for a variant's data
// Unit variants have no fields
// Tuple/struct variants get pointer fields to store their data
func (p *SumTypesPlugin) generateVariantFields(variant *dingoast.VariantDecl) []*ast.Field {
	if variant.Kind == dingoast.VariantUnit || variant.Fields == nil {
		return nil // No data fields for unit variants or nil fields
	}

	variantName := strings.ToLower(variant.Name.Name)

	fields := make([]*ast.Field, 0)
	fieldNames := make(map[string]bool) // Track field names for collision detection

	// Track tuple field index for synthetic naming
	fieldIndex := 0

	// For each field in the variant, create a pointer field in the union
	for _, f := range variant.Fields.List {
		if f.Names == nil || len(f.Names) == 0 {
			// CRITICAL FIX #2: Tuple field - generate synthetic name
			fieldName := fmt.Sprintf("%s_%d", variantName, fieldIndex)
			fieldIndex++

			// Check for field name collisions
			if fieldNames[fieldName] {
				p.currentContext.Logger.Error("field name collision: %s (variant: %s)", fieldName, variant.Name.Name)
				continue
			}
			fieldNames[fieldName] = true

			fields = append(fields, &ast.Field{
				Names: []*ast.Ident{{Name: fieldName}},
				Type:  &ast.StarExpr{X: f.Type}, // Pointer type
			})
		} else {
			// Named field - use provided names
			for _, name := range f.Names {
				fieldName := variantName + "_" + name.Name

				// Check for field name collisions
				if fieldNames[fieldName] {
					p.currentContext.Logger.Error("field name collision: %s (variant: %s)", fieldName, variant.Name.Name)
					continue
				}
				fieldNames[fieldName] = true

				fields = append(fields, &ast.Field{
					Names: []*ast.Ident{{Name: fieldName}},
					Type:  &ast.StarExpr{X: f.Type}, // Pointer type
				})
			}
		}
	}

	return fields
}

// generateConstructor creates a constructor function for a variant
// Example: func Shape_Circle(r float64) Shape { return Shape{tag: ShapeTag_Circle, circle_r: &r} }
func (p *SumTypesPlugin) generateConstructor(enumDecl *dingoast.EnumDecl, variant *dingoast.VariantDecl) ast.Decl {
	enumName := enumDecl.Name.Name
	variantName := variant.Name.Name
	funcName := enumName + "_" + variantName

	// Build parameter list (deep copy to avoid aliasing)
	var params *ast.FieldList
	if variant.Fields != nil && variant.Fields.List != nil {
		paramsCopy := make([]*ast.Field, len(variant.Fields.List))
		fieldIndex := 0
		for i, f := range variant.Fields.List {
			if f.Names == nil || len(f.Names) == 0 {
				// CRITICAL FIX #2 (constructor params): Tuple field - generate synthetic parameter name
				syntheticName := fmt.Sprintf("arg%d", fieldIndex)
				paramsCopy[i] = &ast.Field{
					Names: []*ast.Ident{{Name: syntheticName}},
					Type:  f.Type, // Types are immutable, OK to share
				}
				fieldIndex++
			} else {
				// Named field - deep copy
				namesCopy := make([]*ast.Ident, len(f.Names))
				for j, name := range f.Names {
					namesCopy[j] = &ast.Ident{
						Name:    name.Name,
						NamePos: name.NamePos,
					}
				}
				paramsCopy[i] = &ast.Field{
					Names: namesCopy,
					Type:  f.Type, // Types are immutable, OK to share
				}
			}
		}
		params = &ast.FieldList{List: paramsCopy}
	} else {
		params = &ast.FieldList{}
	}

	// Build return type (same as enum)
	var returnType ast.Expr = &ast.Ident{Name: enumName}

	// Handle generic return type if needed
	if enumDecl.TypeParams != nil {
		// For generic enums, we need to specify type parameters in return type
		typeArgs := make([]ast.Expr, len(enumDecl.TypeParams.List))
		for i, param := range enumDecl.TypeParams.List {
			typeArgs[i] = param.Names[0]
		}
		// Use IndexExpr for single type param, IndexListExpr for multiple
		if len(typeArgs) == 1 {
			returnType = &ast.IndexExpr{
				X:     &ast.Ident{Name: enumName},
				Index: typeArgs[0],
			}
		} else {
			returnType = &ast.IndexListExpr{
				X:       &ast.Ident{Name: enumName},
				Indices: typeArgs,
			}
		}
	}

	// Build composite literal for return statement
	compositeLit := &ast.CompositeLit{
		Type: returnType,
		Elts: p.generateConstructorFields(enumDecl, variant),
	}

	// Build function
	funcDecl := &ast.FuncDecl{
		Name: &ast.Ident{Name: funcName},
		Type: &ast.FuncType{
			Params: params,
			Results: &ast.FieldList{
				List: []*ast.Field{{Type: returnType}},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{compositeLit},
				},
			},
		},
	}

	// Add type parameters if generic
	if enumDecl.TypeParams != nil {
		funcDecl.Type.TypeParams = enumDecl.TypeParams
	}

	return funcDecl
}

// generateConstructorFields creates the fields for the composite literal in constructor
func (p *SumTypesPlugin) generateConstructorFields(enumDecl *dingoast.EnumDecl, variant *dingoast.VariantDecl) []ast.Expr {
	enumName := enumDecl.Name.Name
	variantName := variant.Name.Name
	tagName := enumName + "Tag_" + variantName
	variantNameLower := strings.ToLower(variantName)

	fields := []ast.Expr{
		// tag: ShapeTag_Circle
		&ast.KeyValueExpr{
			Key:   &ast.Ident{Name: "tag"},
			Value: &ast.Ident{Name: tagName},
		},
	}

	// Add data fields (pointer to param values)
	if variant.Fields != nil {
		fieldIndex := 0
		for _, f := range variant.Fields.List {
			if f.Names == nil || len(f.Names) == 0 {
				// CRITICAL FIX #2 (constructor): Tuple field - use synthetic name and index
				fieldName := fmt.Sprintf("%s_%d", variantNameLower, fieldIndex)
				// For tuple constructor params, we need to use the parameter name
				// Since we don't have explicit names, we'll use the same index-based approach
				paramName := fmt.Sprintf("arg%d", fieldIndex)
				fields = append(fields, &ast.KeyValueExpr{
					Key: &ast.Ident{Name: fieldName},
					Value: &ast.UnaryExpr{
						Op: token.AND,
						X:  &ast.Ident{Name: paramName},
					},
				})
				fieldIndex++
			} else {
				// Named fields
				for _, name := range f.Names {
					fieldName := variantNameLower + "_" + name.Name
					fields = append(fields, &ast.KeyValueExpr{
						Key: &ast.Ident{Name: fieldName},
						Value: &ast.UnaryExpr{
							Op: token.AND,
							X:  &ast.Ident{Name: name.Name},
						},
					})
				}
			}
		}
	}

	return fields
}

// generateHelperMethod creates an Is* helper method for a variant
// Example: func (s Shape) IsCircle() bool { return s.tag == ShapeTag_Circle }
func (p *SumTypesPlugin) generateHelperMethod(enumDecl *dingoast.EnumDecl, variant *dingoast.VariantDecl) ast.Decl {
	enumName := enumDecl.Name.Name
	variantName := variant.Name.Name
	methodName := "Is" + variantName

	// Build receiver type
	var receiverType ast.Expr = &ast.Ident{Name: enumName}
	if enumDecl.TypeParams != nil {
		// For generic enums, add type parameters
		typeArgs := make([]ast.Expr, len(enumDecl.TypeParams.List))
		for i, param := range enumDecl.TypeParams.List {
			typeArgs[i] = param.Names[0]
		}
		// Use IndexExpr for single type param, IndexListExpr for multiple
		if len(typeArgs) == 1 {
			receiverType = &ast.IndexExpr{
				X:     &ast.Ident{Name: enumName},
				Index: typeArgs[0],
			}
		} else {
			receiverType = &ast.IndexListExpr{
				X:       &ast.Ident{Name: enumName},
				Indices: typeArgs,
			}
		}
	}

	// Tag constant name
	tagConstName := enumName + "Tag_" + variantName

	// Build method
	funcDecl := &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{{
				Names: []*ast.Ident{{Name: "e"}},
				Type:  receiverType,
			}},
		},
		Name: &ast.Ident{Name: methodName},
		Type: &ast.FuncType{
			Params: &ast.FieldList{},
			Results: &ast.FieldList{
				List: []*ast.Field{{
					Type: &ast.Ident{Name: "bool"},
				}},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.BinaryExpr{
							X: &ast.SelectorExpr{
								X:   &ast.Ident{Name: "e"},
								Sel: &ast.Ident{Name: "tag"},
							},
							Op: token.EQL,
							Y:  &ast.Ident{Name: tagConstName},
						},
					},
				},
			},
		},
	}

	return funcDecl
}

// transformMatchExpr transforms a match expression into a switch statement or IIFE
func (p *SumTypesPlugin) transformMatchExpr(cursor *astutil.Cursor, matchExpr *dingoast.MatchExpr) {
	// Try to infer enum type from match subject for tag constant naming
	enumType := p.inferEnumType(matchExpr)
	if enumType == "" {
		p.currentContext.Logger.Error("cannot infer enum type from match expression")
		return
	}

	// Check if we're in expression context (match should return a value)
	isExprContext := p.isExpressionContext(cursor)

	// Create switch statement
	switchStmt := p.buildSwitchStatement(matchExpr, enumType, isExprContext)

	if isExprContext {
		// Wrap in IIFE for expression context
		iife := p.wrapInIIFE(switchStmt, matchExpr)
		cursor.Replace(iife)
	} else {
		// Use as statement
		cursor.Replace(switchStmt)
	}
}

// isExpressionContext detects if a match is used in an expression position
func (p *SumTypesPlugin) isExpressionContext(cursor *astutil.Cursor) bool {
	parent := cursor.Parent()

	switch parent.(type) {
	case *ast.AssignStmt: // x := match ...
		return true
	case *ast.ReturnStmt: // return match ...
		return true
	case *ast.BinaryExpr: // if match ... == 5
		return true
	case *ast.CallExpr: // fmt.Println(match ...)
		return true
	case *ast.CompositeLit: // {..., match ...}
		return true
	case *ast.ExprStmt: // match ... (statement context)
		return false
	default:
		// Conservative: assume expression context
		return true
	}
}

// buildSwitchStatement creates a switch statement from match expression
func (p *SumTypesPlugin) buildSwitchStatement(matchExpr *dingoast.MatchExpr, enumType string, isExprContext bool) *ast.SwitchStmt {
	switchStmt := &ast.SwitchStmt{
		Tag: &ast.SelectorExpr{
			X:   matchExpr.Expr,
			Sel: &ast.Ident{Name: "tag"},
		},
		Body: &ast.BlockStmt{
			List: make([]ast.Stmt, 0, len(matchExpr.Arms)),
		},
	}

	// Convert each match arm to a case clause
	for _, arm := range matchExpr.Arms {
		caseClause, err := p.transformMatchArm(enumType, matchExpr.Expr, arm, isExprContext)
		if err != nil {
			p.currentContext.Logger.Error("match arm transformation failed: %v", err)
			continue
		}
		switchStmt.Body.List = append(switchStmt.Body.List, caseClause)
	}

	return switchStmt
}

// wrapInIIFE wraps a switch statement in an immediately-invoked function expression
func (p *SumTypesPlugin) wrapInIIFE(switchStmt *ast.SwitchStmt, matchExpr *dingoast.MatchExpr) *ast.CallExpr {
	// Infer return type (simple heuristic - default to interface{})
	resultType := p.inferMatchType(matchExpr)

	// Create IIFE
	iife := &ast.CallExpr{
		Fun: &ast.FuncLit{
			Type: &ast.FuncType{
				Params: &ast.FieldList{},
				Results: &ast.FieldList{
					List: []*ast.Field{{Type: resultType}},
				},
			},
			Body: &ast.BlockStmt{
				List: []ast.Stmt{
					switchStmt,
					// Add panic for exhaustiveness safety
					&ast.ExprStmt{
						X: &ast.CallExpr{
							Fun: &ast.Ident{Name: "panic"},
							Args: []ast.Expr{
								&ast.BasicLit{
									Kind:  token.STRING,
									Value: `"unreachable: match should be exhaustive"`,
								},
							},
						},
					},
				},
			},
		},
	}

	return iife
}

// inferMatchType infers the return type of a match expression
func (p *SumTypesPlugin) inferMatchType(matchExpr *dingoast.MatchExpr) ast.Expr {
	// CRITICAL FIX #1: Implement basic type inference from first arm
	if len(matchExpr.Arms) == 0 {
		return &ast.Ident{Name: "interface{}"}
	}

	firstArm := matchExpr.Arms[0]

	// Try to infer from first arm's body expression
	switch expr := firstArm.Body.(type) {
	case *ast.BasicLit:
		return p.inferFromLiteral(expr)
	case *ast.BinaryExpr:
		return p.inferFromBinaryExpr(expr)
	default:
		// Fallback to interface{} for safety
		return &ast.Ident{Name: "interface{}"}
	}
}

// inferFromLiteral infers type from a literal expression
func (p *SumTypesPlugin) inferFromLiteral(lit *ast.BasicLit) ast.Expr {
	switch lit.Kind {
	case token.INT:
		return &ast.Ident{Name: "int"}
	case token.FLOAT:
		return &ast.Ident{Name: "float64"}
	case token.STRING:
		return &ast.Ident{Name: "string"}
	case token.CHAR:
		return &ast.Ident{Name: "rune"}
	default:
		return &ast.Ident{Name: "interface{}"}
	}
}

// inferFromBinaryExpr infers type from a binary expression
func (p *SumTypesPlugin) inferFromBinaryExpr(expr *ast.BinaryExpr) ast.Expr {
	// Arithmetic operators typically produce float64 for mixed int/float
	switch expr.Op {
	case token.ADD, token.SUB, token.MUL, token.QUO:
		// For arithmetic, default to float64 (safe for most cases)
		return &ast.Ident{Name: "float64"}
	case token.EQL, token.NEQ, token.LSS, token.GTR, token.LEQ, token.GEQ:
		// Comparison operators return bool
		return &ast.Ident{Name: "bool"}
	case token.LAND, token.LOR:
		// Logical operators return bool
		return &ast.Ident{Name: "bool"}
	default:
		return &ast.Ident{Name: "interface{}"}
	}
}

// inferEnumType attempts to infer the enum type from a match expression
// Phase 1: Simple heuristic - check first variant pattern against registry
// Phase 3: Use proper type inference from type checker
func (p *SumTypesPlugin) inferEnumType(matchExpr *dingoast.MatchExpr) string {
	// Look through match arms to find a variant pattern
	for _, arm := range matchExpr.Arms {
		if arm.Pattern.Variant != nil {
			variantName := arm.Pattern.Variant.Name
			// Search registry for enum containing this variant
			for enumName, enumDecl := range p.enumRegistry {
				for _, variant := range enumDecl.Variants {
					if variant.Name.Name == variantName {
						return enumName
					}
				}
			}
		}
	}
	return "" // Cannot infer
}

// transformMatchArm converts a match arm to a switch case clause
// enumType is the inferred enum name for generating correct tag constants
func (p *SumTypesPlugin) transformMatchArm(enumType string, matchedExpr ast.Expr, arm *dingoast.MatchArm, isExprContext bool) (*ast.CaseClause, error) {
	pattern := arm.Pattern

	var caseExpr ast.Expr
	var body []ast.Stmt

	// Check for match guards (not supported yet)
	if arm.Guard != nil {
		return nil, fmt.Errorf("match guards are not yet supported")
	}

	if pattern.Wildcard {
		// Wildcard becomes default case
		caseExpr = nil
	} else if pattern.Variant == nil {
		// Literal patterns not supported
		return nil, fmt.Errorf("literal patterns are not yet supported (only variant patterns allowed)")
	} else {
		// Variant pattern becomes case for that tag
		// Generate correct tag constant name: EnumTag_Variant
		variantName := pattern.Variant.Name
		tagConstName := enumType + "Tag_" + variantName
		caseExpr = &ast.Ident{Name: tagConstName}
	}

	// Add destructuring statements if needed
	if pattern.Variant != nil && len(pattern.Fields) > 0 {
		// Generate field extraction code
		// Example: r := *matchedExpr.circle_r
		body = p.generateDestructuring(enumType, matchedExpr, pattern)
	}

	// Add the arm body
	if isExprContext {
		// Wrap in return statement for expression context
		body = append(body, &ast.ReturnStmt{
			Results: []ast.Expr{arm.Body},
		})
	} else {
		// Use as expression statement
		body = append(body, &ast.ExprStmt{X: arm.Body})
	}

	return &ast.CaseClause{
		List: func() []ast.Expr {
			if caseExpr != nil {
				return []ast.Expr{caseExpr}
			}
			return nil // default case
		}(),
		Body: body,
	}, nil
}

// generateDestructuring creates statements to extract fields from a variant
// enumType is the enum name, used to locate the variant definition
func (p *SumTypesPlugin) generateDestructuring(enumType string, matchedExpr ast.Expr, pattern *dingoast.Pattern) []ast.Stmt {
	stmts := make([]ast.Stmt, 0)

	// Get enum from registry to access variant field information
	enumDecl, exists := p.enumRegistry[enumType]
	if !exists {
		p.currentContext.Logger.Error("enum type not found in registry: %s", enumType)
		return stmts
	}

	// Find the variant in the enum
	var variantDecl *dingoast.VariantDecl
	for _, v := range enumDecl.Variants {
		if v.Name.Name == pattern.Variant.Name {
			variantDecl = v
			break
		}
	}

	if variantDecl == nil {
		p.currentContext.Logger.Error("variant not found: %s", pattern.Variant.Name)
		return stmts
	}

	// Handle unit patterns (no destructuring needed)
	if pattern.Kind == dingoast.PatternUnit {
		return stmts
	}

	variantName := strings.ToLower(pattern.Variant.Name)

	// Get nil safety mode from config
	var nilSafetyMode config.NilSafetyMode = config.NilSafetyOn // Default
	if p.currentContext.DingoConfig != nil {
		if cfg, ok := p.currentContext.DingoConfig.(*config.Config); ok {
			nilSafetyMode = cfg.GetNilSafetyMode()
		}
	}

	// Generate destructuring based on pattern type
	switch pattern.Kind {
	case dingoast.PatternStruct:
		// Struct pattern: Circle { radius, height }
		for _, fieldPat := range pattern.Fields {
			// Get the binding name
			bindingName := fieldPat.Binding.Name

			// Generate field name: circle_radius
			fieldName := variantName + "_" + bindingName

			// Create selector: shape.circle_radius
			fieldAccess := &ast.SelectorExpr{
				X:   matchedExpr,
				Sel: &ast.Ident{Name: fieldName},
			}

			// Add nil check if enabled
			nilCheck := p.generateNilCheck(fieldAccess, pattern.Variant.Name, bindingName, nilSafetyMode)
			if nilCheck != nil {
				stmts = append(stmts, nilCheck)
			}

			// Generate assignment: radius := *shape.circle_radius
			stmt := &ast.AssignStmt{
				Lhs: []ast.Expr{&ast.Ident{Name: bindingName}},
				Tok: token.DEFINE,
				Rhs: []ast.Expr{
					&ast.StarExpr{X: fieldAccess}, // Dereference pointer
				},
			}
			stmts = append(stmts, stmt)
		}

	case dingoast.PatternTuple:
		// Tuple pattern: Circle(radius)
		for i, fieldPat := range pattern.Fields {
			bindingName := fieldPat.Binding.Name

			// Generate field name: circle_0, circle_1, etc.
			fieldName := fmt.Sprintf("%s_%d", variantName, i)

			fieldAccess := &ast.SelectorExpr{
				X:   matchedExpr,
				Sel: &ast.Ident{Name: fieldName},
			}

			// Add nil check
			nilCheck := p.generateNilCheck(fieldAccess, pattern.Variant.Name, fmt.Sprintf("field_%d", i), nilSafetyMode)
			if nilCheck != nil {
				stmts = append(stmts, nilCheck)
			}

			// Generate assignment
			stmt := &ast.AssignStmt{
				Lhs: []ast.Expr{&ast.Ident{Name: bindingName}},
				Tok: token.DEFINE,
				Rhs: []ast.Expr{
					&ast.StarExpr{X: fieldAccess},
				},
			}
			stmts = append(stmts, stmt)
		}
	}

	return stmts
}

// generateNilCheck creates a nil safety check based on configuration
func (p *SumTypesPlugin) generateNilCheck(
	fieldAccess *ast.SelectorExpr,
	variantName string,
	fieldName string,
	nilSafetyMode config.NilSafetyMode,
) ast.Stmt {
	switch nilSafetyMode {
	case config.NilSafetyOff:
		return nil // No check

	case config.NilSafetyOn:
		// Always check with runtime panic
		return &ast.IfStmt{
			Cond: &ast.BinaryExpr{
				X:  fieldAccess,
				Op: token.EQL,
				Y:  &ast.Ident{Name: "nil"},
			},
			Body: &ast.BlockStmt{
				List: []ast.Stmt{
					&ast.ExprStmt{
						X: &ast.CallExpr{
							Fun: &ast.Ident{Name: "panic"},
							Args: []ast.Expr{
								&ast.BasicLit{
									Kind:  token.STRING,
									Value: fmt.Sprintf(`"dingo: invalid %s - nil %s field (union not created via constructor?)"`, variantName, fieldName),
								},
							},
						},
					},
				},
			},
		}

	case config.NilSafetyDebug:
		// CRITICAL FIX #3: Ensure dingoDebug variable is emitted
		p.emitDebugVariable()

		// Check only when DINGO_DEBUG env var is set
		return &ast.IfStmt{
			Cond: &ast.BinaryExpr{
				X: &ast.BinaryExpr{
					X:  &ast.Ident{Name: "dingoDebug"},
					Op: token.LAND,
					Y: &ast.BinaryExpr{
						X:  fieldAccess,
						Op: token.EQL,
						Y:  &ast.Ident{Name: "nil"},
					},
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
									Value: fmt.Sprintf(`"dingo: invalid %s - nil %s field (union not created via constructor?)"`, variantName, fieldName),
								},
							},
						},
					},
				},
			},
		}
	}

	return nil
}

// emitDebugVariable emits the dingoDebug package-level variable (once per file)
func (p *SumTypesPlugin) emitDebugVariable() {
	if p.emittedDebugVar {
		return // Already emitted
	}

	// Generate: var dingoDebug = os.Getenv("DINGO_DEBUG") != ""
	debugVar := &ast.GenDecl{
		Tok: token.VAR,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names: []*ast.Ident{{Name: "dingoDebug"}},
				Values: []ast.Expr{
					&ast.BinaryExpr{
						X: &ast.CallExpr{
							Fun: &ast.SelectorExpr{
								X:   &ast.Ident{Name: "os"},
								Sel: &ast.Ident{Name: "Getenv"},
							},
							Args: []ast.Expr{
								&ast.BasicLit{
									Kind:  token.STRING,
									Value: `"DINGO_DEBUG"`,
								},
							},
						},
						Op: token.NEQ,
						Y:  &ast.BasicLit{Kind: token.STRING, Value: `""`},
					},
				},
			},
		},
	}

	p.generatedDecls = append(p.generatedDecls, debugVar)
	p.emittedDebugVar = true
}

// Reset resets the plugin's internal state (useful for testing)
func (p *SumTypesPlugin) Reset() {
	p.enumRegistry = make(map[string]*dingoast.EnumDecl)
	p.generatedDecls = nil
	p.currentFile = nil
	p.emittedDebugVar = false
}
