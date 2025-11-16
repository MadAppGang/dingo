package builtin

import (
	"go/ast"
	"go/token"
	"testing"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CRITICAL FIX #1: IIFE Type Inference Tests
// ============================================================================

func TestInferMatchType_IntLiteral(t *testing.T) {
	p := NewSumTypesPlugin()

	matchExpr := &dingoast.MatchExpr{
		Arms: []*dingoast.MatchArm{
			{
				Body: &ast.BasicLit{
					Kind:  token.INT,
					Value: "42",
				},
			},
		},
	}

	resultType := p.inferMatchType(matchExpr)
	require.NotNil(t, resultType)

	ident, ok := resultType.(*ast.Ident)
	require.True(t, ok, "Should return identifier type")
	assert.Equal(t, "int", ident.Name, "INT literal should infer to int type")
}

func TestInferMatchType_FloatLiteral(t *testing.T) {
	p := NewSumTypesPlugin()

	matchExpr := &dingoast.MatchExpr{
		Arms: []*dingoast.MatchArm{
			{
				Body: &ast.BasicLit{
					Kind:  token.FLOAT,
					Value: "3.14",
				},
			},
		},
	}

	resultType := p.inferMatchType(matchExpr)
	ident := resultType.(*ast.Ident)
	assert.Equal(t, "float64", ident.Name, "FLOAT literal should infer to float64 type")
}

func TestInferMatchType_StringLiteral(t *testing.T) {
	p := NewSumTypesPlugin()

	matchExpr := &dingoast.MatchExpr{
		Arms: []*dingoast.MatchArm{
			{
				Body: &ast.BasicLit{
					Kind:  token.STRING,
					Value: `"hello"`,
				},
			},
		},
	}

	resultType := p.inferMatchType(matchExpr)
	ident := resultType.(*ast.Ident)
	assert.Equal(t, "string", ident.Name, "STRING literal should infer to string type")
}

func TestInferMatchType_CharLiteral(t *testing.T) {
	p := NewSumTypesPlugin()

	matchExpr := &dingoast.MatchExpr{
		Arms: []*dingoast.MatchArm{
			{
				Body: &ast.BasicLit{
					Kind:  token.CHAR,
					Value: "'a'",
				},
			},
		},
	}

	resultType := p.inferMatchType(matchExpr)
	ident := resultType.(*ast.Ident)
	assert.Equal(t, "rune", ident.Name, "CHAR literal should infer to rune type")
}

func TestInferMatchType_BinaryArithmetic(t *testing.T) {
	p := NewSumTypesPlugin()

	tests := []struct {
		name     string
		op       token.Token
		expected string
	}{
		{"addition", token.ADD, "float64"},
		{"subtraction", token.SUB, "float64"},
		{"multiplication", token.MUL, "float64"},
		{"division", token.QUO, "float64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matchExpr := &dingoast.MatchExpr{
				Arms: []*dingoast.MatchArm{
					{
						Body: &ast.BinaryExpr{
							X:  &ast.Ident{Name: "x"},
							Op: tt.op,
							Y:  &ast.Ident{Name: "y"},
						},
					},
				},
			}

			resultType := p.inferMatchType(matchExpr)
			ident := resultType.(*ast.Ident)
			assert.Equal(t, tt.expected, ident.Name, "Arithmetic operators should infer to float64")
		})
	}
}

func TestInferMatchType_BinaryComparison(t *testing.T) {
	p := NewSumTypesPlugin()

	tests := []struct {
		name string
		op   token.Token
	}{
		{"equal", token.EQL},
		{"not equal", token.NEQ},
		{"less than", token.LSS},
		{"greater than", token.GTR},
		{"less or equal", token.LEQ},
		{"greater or equal", token.GEQ},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matchExpr := &dingoast.MatchExpr{
				Arms: []*dingoast.MatchArm{
					{
						Body: &ast.BinaryExpr{
							X:  &ast.Ident{Name: "x"},
							Op: tt.op,
							Y:  &ast.Ident{Name: "y"},
						},
					},
				},
			}

			resultType := p.inferMatchType(matchExpr)
			ident := resultType.(*ast.Ident)
			assert.Equal(t, "bool", ident.Name, "Comparison operators should infer to bool")
		})
	}
}

func TestInferMatchType_BinaryLogical(t *testing.T) {
	p := NewSumTypesPlugin()

	tests := []struct {
		name string
		op   token.Token
	}{
		{"logical and", token.LAND},
		{"logical or", token.LOR},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matchExpr := &dingoast.MatchExpr{
				Arms: []*dingoast.MatchArm{
					{
						Body: &ast.BinaryExpr{
							X:  &ast.Ident{Name: "x"},
							Op: tt.op,
							Y:  &ast.Ident{Name: "y"},
						},
					},
				},
			}

			resultType := p.inferMatchType(matchExpr)
			ident := resultType.(*ast.Ident)
			assert.Equal(t, "bool", ident.Name, "Logical operators should infer to bool")
		})
	}
}

func TestInferMatchType_EmptyArms(t *testing.T) {
	p := NewSumTypesPlugin()

	matchExpr := &dingoast.MatchExpr{
		Arms: []*dingoast.MatchArm{},
	}

	resultType := p.inferMatchType(matchExpr)
	ident := resultType.(*ast.Ident)
	assert.Equal(t, "interface{}", ident.Name, "Empty match should default to interface{}")
}

func TestInferMatchType_ComplexExpression(t *testing.T) {
	p := NewSumTypesPlugin()

	// Complex expression that's not a literal or simple binary expr
	matchExpr := &dingoast.MatchExpr{
		Arms: []*dingoast.MatchArm{
			{
				Body: &ast.CallExpr{
					Fun: &ast.Ident{Name: "someFunc"},
				},
			},
		},
	}

	resultType := p.inferMatchType(matchExpr)
	ident := resultType.(*ast.Ident)
	assert.Equal(t, "interface{}", ident.Name, "Complex expressions should default to interface{}")
}

// ============================================================================
// CRITICAL FIX #2: Tuple Variant Backing Fields Tests
// ============================================================================

func TestGenerateVariantFields_TupleSingleField(t *testing.T) {
	p := NewSumTypesPlugin()
	p.currentContext = &plugin.Context{Logger: &plugin.NoOpLogger{}}

	// Create tuple variant with single unnamed field
	variant := &dingoast.VariantDecl{
		Name: &ast.Ident{Name: "Circle"},
		Kind: dingoast.VariantTuple,
		Fields: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: nil, // Unnamed tuple field
					Type:  &ast.Ident{Name: "float64"},
				},
			},
		},
	}

	fields := p.generateVariantFields(variant)

	require.Len(t, fields, 1, "Single tuple field should generate one struct field")
	assert.Equal(t, "circle_0", fields[0].Names[0].Name, "Should generate synthetic name circle_0")

	// Check it's a pointer type
	starExpr, ok := fields[0].Type.(*ast.StarExpr)
	require.True(t, ok, "Tuple field should be pointer type")
	assert.Equal(t, "float64", starExpr.X.(*ast.Ident).Name)
}

func TestGenerateVariantFields_TupleMultipleFields(t *testing.T) {
	p := NewSumTypesPlugin()
	p.currentContext = &plugin.Context{Logger: &plugin.NoOpLogger{}}

	// Create tuple variant with multiple unnamed fields
	variant := &dingoast.VariantDecl{
		Name: &ast.Ident{Name: "Point2D"},
		Kind: dingoast.VariantTuple,
		Fields: &ast.FieldList{
			List: []*ast.Field{
				{Names: nil, Type: &ast.Ident{Name: "float64"}},
				{Names: nil, Type: &ast.Ident{Name: "float64"}},
			},
		},
	}

	fields := p.generateVariantFields(variant)

	require.Len(t, fields, 2, "Should generate two struct fields")
	assert.Equal(t, "point2d_0", fields[0].Names[0].Name, "First field should be point2d_0")
	assert.Equal(t, "point2d_1", fields[1].Names[0].Name, "Second field should be point2d_1")
}

func TestGenerateVariantFields_TupleThreeFields(t *testing.T) {
	p := NewSumTypesPlugin()
	p.currentContext = &plugin.Context{Logger: &plugin.NoOpLogger{}}

	variant := &dingoast.VariantDecl{
		Name: &ast.Ident{Name: "Point3D"},
		Kind: dingoast.VariantTuple,
		Fields: &ast.FieldList{
			List: []*ast.Field{
				{Names: nil, Type: &ast.Ident{Name: "float64"}},
				{Names: nil, Type: &ast.Ident{Name: "float64"}},
				{Names: nil, Type: &ast.Ident{Name: "float64"}},
			},
		},
	}

	fields := p.generateVariantFields(variant)

	require.Len(t, fields, 3)
	assert.Equal(t, "point3d_0", fields[0].Names[0].Name)
	assert.Equal(t, "point3d_1", fields[1].Names[0].Name)
	assert.Equal(t, "point3d_2", fields[2].Names[0].Name)
}

func TestGenerateConstructor_TupleParameters(t *testing.T) {
	p := NewSumTypesPlugin()

	enum := makeTestEnum("Shape")
	variant := &dingoast.VariantDecl{
		Name: &ast.Ident{Name: "Circle"},
		Kind: dingoast.VariantTuple,
		Fields: &ast.FieldList{
			List: []*ast.Field{
				{Names: nil, Type: &ast.Ident{Name: "float64"}},
			},
		},
	}

	decl := p.generateConstructor(enum, variant)
	funcDecl := decl.(*ast.FuncDecl)

	// Check parameter has synthetic name
	require.Len(t, funcDecl.Type.Params.List, 1)
	param := funcDecl.Type.Params.List[0]
	assert.Equal(t, "arg0", param.Names[0].Name, "Tuple parameter should be named arg0")
	assert.Equal(t, "float64", param.Type.(*ast.Ident).Name)
}

func TestGenerateConstructor_TupleMultipleParameters(t *testing.T) {
	p := NewSumTypesPlugin()

	enum := makeTestEnum("Point")
	variant := &dingoast.VariantDecl{
		Name: &ast.Ident{Name: "Point2D"},
		Kind: dingoast.VariantTuple,
		Fields: &ast.FieldList{
			List: []*ast.Field{
				{Names: nil, Type: &ast.Ident{Name: "float64"}},
				{Names: nil, Type: &ast.Ident{Name: "float64"}},
			},
		},
	}

	decl := p.generateConstructor(enum, variant)
	funcDecl := decl.(*ast.FuncDecl)

	// Check parameters have synthetic names
	require.Len(t, funcDecl.Type.Params.List, 2)
	assert.Equal(t, "arg0", funcDecl.Type.Params.List[0].Names[0].Name)
	assert.Equal(t, "arg1", funcDecl.Type.Params.List[1].Names[0].Name)
}

func TestGenerateConstructorFields_TupleMapping(t *testing.T) {
	p := NewSumTypesPlugin()

	enum := makeTestEnum("Shape")
	variant := &dingoast.VariantDecl{
		Name: &ast.Ident{Name: "Circle"},
		Kind: dingoast.VariantTuple,
		Fields: &ast.FieldList{
			List: []*ast.Field{
				{Names: nil, Type: &ast.Ident{Name: "float64"}},
			},
		},
	}

	fields := p.generateConstructorFields(enum, variant)

	// Should have tag + circle_0 field
	require.Len(t, fields, 2)

	// Check tag field
	tagField := fields[0].(*ast.KeyValueExpr)
	assert.Equal(t, "tag", tagField.Key.(*ast.Ident).Name)

	// Check circle_0 field mapping
	dataField := fields[1].(*ast.KeyValueExpr)
	assert.Equal(t, "circle_0", dataField.Key.(*ast.Ident).Name, "Should map to circle_0 field")

	// Check value is &arg0
	unaryExpr := dataField.Value.(*ast.UnaryExpr)
	assert.Equal(t, token.AND, unaryExpr.Op)
	assert.Equal(t, "arg0", unaryExpr.X.(*ast.Ident).Name, "Should reference arg0 parameter")
}

func TestGenerateConstructorFields_TupleMultipleMapping(t *testing.T) {
	p := NewSumTypesPlugin()

	enum := makeTestEnum("Point")
	variant := &dingoast.VariantDecl{
		Name: &ast.Ident{Name: "Point2D"},
		Kind: dingoast.VariantTuple,
		Fields: &ast.FieldList{
			List: []*ast.Field{
				{Names: nil, Type: &ast.Ident{Name: "float64"}},
				{Names: nil, Type: &ast.Ident{Name: "float64"}},
			},
		},
	}

	fields := p.generateConstructorFields(enum, variant)

	// tag + point2d_0 + point2d_1
	require.Len(t, fields, 3)

	field0 := fields[1].(*ast.KeyValueExpr)
	assert.Equal(t, "point2d_0", field0.Key.(*ast.Ident).Name)
	unary0 := field0.Value.(*ast.UnaryExpr)
	assert.Equal(t, "arg0", unary0.X.(*ast.Ident).Name)

	field1 := fields[2].(*ast.KeyValueExpr)
	assert.Equal(t, "point2d_1", field1.Key.(*ast.Ident).Name)
	unary1 := field1.Value.(*ast.UnaryExpr)
	assert.Equal(t, "arg1", unary1.X.(*ast.Ident).Name)
}

// ============================================================================
// CRITICAL FIX #3: Debug Mode Variable Tests
// ============================================================================

func TestEmitDebugVariable(t *testing.T) {
	p := NewSumTypesPlugin()
	p.generatedDecls = []ast.Decl{}

	// Initially, no debug var emitted
	assert.False(t, p.emittedDebugVar)
	assert.Empty(t, p.generatedDecls)

	// Emit debug variable
	p.emitDebugVariable()

	// Should be emitted now
	assert.True(t, p.emittedDebugVar)
	require.Len(t, p.generatedDecls, 1, "Should add one declaration")

	// Check the declaration
	genDecl, ok := p.generatedDecls[0].(*ast.GenDecl)
	require.True(t, ok, "Should be GenDecl")
	assert.Equal(t, token.VAR, genDecl.Tok)

	// Check the variable spec
	require.Len(t, genDecl.Specs, 1)
	valueSpec := genDecl.Specs[0].(*ast.ValueSpec)
	assert.Equal(t, "dingoDebug", valueSpec.Names[0].Name)

	// Check the value is os.Getenv("DINGO_DEBUG") != ""
	require.Len(t, valueSpec.Values, 1)
	binaryExpr, ok := valueSpec.Values[0].(*ast.BinaryExpr)
	require.True(t, ok, "Value should be binary expression")
	assert.Equal(t, token.NEQ, binaryExpr.Op)

	// Check left side is os.Getenv call
	callExpr, ok := binaryExpr.X.(*ast.CallExpr)
	require.True(t, ok)
	selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
	require.True(t, ok)
	assert.Equal(t, "os", selExpr.X.(*ast.Ident).Name)
	assert.Equal(t, "Getenv", selExpr.Sel.Name)

	// Check argument is "DINGO_DEBUG"
	require.Len(t, callExpr.Args, 1)
	argLit := callExpr.Args[0].(*ast.BasicLit)
	assert.Equal(t, token.STRING, argLit.Kind)
	assert.Equal(t, `"DINGO_DEBUG"`, argLit.Value)

	// Check right side is ""
	rightLit := binaryExpr.Y.(*ast.BasicLit)
	assert.Equal(t, token.STRING, rightLit.Kind)
	assert.Equal(t, `""`, rightLit.Value)
}

func TestEmitDebugVariable_OnlyOnce(t *testing.T) {
	p := NewSumTypesPlugin()
	p.generatedDecls = []ast.Decl{}

	// Emit once
	p.emitDebugVariable()
	assert.Len(t, p.generatedDecls, 1)

	// Emit again - should not add another
	p.emitDebugVariable()
	assert.Len(t, p.generatedDecls, 1, "Should only emit debug variable once")
}

func TestEmitDebugVariable_ResetState(t *testing.T) {
	p := NewSumTypesPlugin()

	// Emit
	p.emitDebugVariable()
	assert.True(t, p.emittedDebugVar)

	// Reset
	p.Reset()
	assert.False(t, p.emittedDebugVar, "Reset should clear emittedDebugVar flag")
	assert.Empty(t, p.generatedDecls)
}

// ============================================================================
// Pattern Destructuring Tests
// ============================================================================

func TestGenerateDestructuring_StructPattern(t *testing.T) {
	p := NewSumTypesPlugin()
	p.currentContext = &plugin.Context{
		Logger:      &plugin.NoOpLogger{},
		DingoConfig: &config.Config{Features: config.FeatureConfig{NilSafetyChecks: "on"}},
	}

	// Register enum
	enum := makeTestEnum("Shape",
		makeStructVariant("Circle",
			makeField("radius", &ast.Ident{Name: "float64"}),
		),
	)
	p.enumRegistry = map[string]*dingoast.EnumDecl{
		"Shape": enum,
	}

	// Create struct pattern: Circle { radius }
	pattern := &dingoast.Pattern{
		Kind:    dingoast.PatternStruct,
		Variant: &ast.Ident{Name: "Circle"},
		Fields: []*dingoast.FieldPattern{
			{
				FieldName: &ast.Ident{Name: "radius"},
				Binding:   &ast.Ident{Name: "radius"},
			},
		},
	}

	matchedExpr := &ast.Ident{Name: "shape"}

	stmts := p.generateDestructuring("Shape", matchedExpr, pattern)

	// Should generate: nil check + assignment
	require.Len(t, stmts, 2)

	// First statement is nil check
	ifStmt, ok := stmts[0].(*ast.IfStmt)
	require.True(t, ok, "Should generate nil check")

	// Check condition: shape.circle_radius == nil
	binaryExpr := ifStmt.Cond.(*ast.BinaryExpr)
	selExpr := binaryExpr.X.(*ast.SelectorExpr)
	assert.Equal(t, "shape", selExpr.X.(*ast.Ident).Name)
	assert.Equal(t, "circle_radius", selExpr.Sel.Name)

	// Second statement is assignment: radius := *shape.circle_radius
	assignStmt, ok := stmts[1].(*ast.AssignStmt)
	require.True(t, ok, "Should generate assignment")
	assert.Equal(t, token.DEFINE, assignStmt.Tok)
	assert.Equal(t, "radius", assignStmt.Lhs[0].(*ast.Ident).Name)

	// RHS should be *shape.circle_radius
	starExpr := assignStmt.Rhs[0].(*ast.StarExpr)
	rhsSelExpr := starExpr.X.(*ast.SelectorExpr)
	assert.Equal(t, "shape", rhsSelExpr.X.(*ast.Ident).Name)
	assert.Equal(t, "circle_radius", rhsSelExpr.Sel.Name)
}

func TestGenerateDestructuring_TuplePattern(t *testing.T) {
	p := NewSumTypesPlugin()
	p.currentContext = &plugin.Context{
		Logger:      &plugin.NoOpLogger{},
		DingoConfig: &config.Config{Features: config.FeatureConfig{NilSafetyChecks: "on"}},
	}

	// Register enum with tuple variant
	enum := makeTestEnum("Shape")
	enum.Variants = []*dingoast.VariantDecl{
		{
			Name: &ast.Ident{Name: "Circle"},
			Kind: dingoast.VariantTuple,
			Fields: &ast.FieldList{
				List: []*ast.Field{
					{Names: nil, Type: &ast.Ident{Name: "float64"}},
				},
			},
		},
	}
	p.enumRegistry = map[string]*dingoast.EnumDecl{
		"Shape": enum,
	}

	// Create tuple pattern: Circle(r)
	pattern := &dingoast.Pattern{
		Kind:    dingoast.PatternTuple,
		Variant: &ast.Ident{Name: "Circle"},
		Fields: []*dingoast.FieldPattern{
			{
				Binding: &ast.Ident{Name: "r"},
			},
		},
	}

	matchedExpr := &ast.Ident{Name: "shape"}

	stmts := p.generateDestructuring("Shape", matchedExpr, pattern)

	// Should generate: nil check + assignment
	require.Len(t, stmts, 2)

	// Check assignment uses circle_0
	assignStmt := stmts[1].(*ast.AssignStmt)
	assert.Equal(t, "r", assignStmt.Lhs[0].(*ast.Ident).Name)

	starExpr := assignStmt.Rhs[0].(*ast.StarExpr)
	selExpr := starExpr.X.(*ast.SelectorExpr)
	assert.Equal(t, "circle_0", selExpr.Sel.Name, "Should access circle_0 field")
}

func TestGenerateDestructuring_TupleMultipleFields(t *testing.T) {
	p := NewSumTypesPlugin()
	p.currentContext = &plugin.Context{
		Logger:      &plugin.NoOpLogger{},
		DingoConfig: &config.Config{Features: config.FeatureConfig{NilSafetyChecks: "on"}},
	}

	enum := makeTestEnum("Point")
	enum.Variants = []*dingoast.VariantDecl{
		{
			Name: &ast.Ident{Name: "Point2D"},
			Kind: dingoast.VariantTuple,
			Fields: &ast.FieldList{
				List: []*ast.Field{
					{Names: nil, Type: &ast.Ident{Name: "float64"}},
					{Names: nil, Type: &ast.Ident{Name: "float64"}},
				},
			},
		},
	}
	p.enumRegistry = map[string]*dingoast.EnumDecl{
		"Point": enum,
	}

	pattern := &dingoast.Pattern{
		Kind:    dingoast.PatternTuple,
		Variant: &ast.Ident{Name: "Point2D"},
		Fields: []*dingoast.FieldPattern{
			{Binding: &ast.Ident{Name: "x"}},
			{Binding: &ast.Ident{Name: "y"}},
		},
	}

	matchedExpr := &ast.Ident{Name: "p"}

	stmts := p.generateDestructuring("Point", matchedExpr, pattern)

	// nil check + assign for x, nil check + assign for y = 4 statements
	require.Len(t, stmts, 4)

	// Check x assignment
	xAssign := stmts[1].(*ast.AssignStmt)
	assert.Equal(t, "x", xAssign.Lhs[0].(*ast.Ident).Name)
	xSel := xAssign.Rhs[0].(*ast.StarExpr).X.(*ast.SelectorExpr)
	assert.Equal(t, "point2d_0", xSel.Sel.Name)

	// Check y assignment
	yAssign := stmts[3].(*ast.AssignStmt)
	assert.Equal(t, "y", yAssign.Lhs[0].(*ast.Ident).Name)
	ySel := yAssign.Rhs[0].(*ast.StarExpr).X.(*ast.SelectorExpr)
	assert.Equal(t, "point2d_1", ySel.Sel.Name)
}

func TestGenerateDestructuring_UnitPattern(t *testing.T) {
	p := NewSumTypesPlugin()
	p.currentContext = &plugin.Context{
		Logger:      &plugin.NoOpLogger{},
		DingoConfig: &config.Config{Features: config.FeatureConfig{NilSafetyChecks: "on"}},
	}

	enum := makeTestEnum("Status", makeUnitVariant("Pending"))
	p.enumRegistry = map[string]*dingoast.EnumDecl{
		"Status": enum,
	}

	pattern := &dingoast.Pattern{
		Kind:    dingoast.PatternUnit,
		Variant: &ast.Ident{Name: "Pending"},
		Fields:  nil,
	}

	matchedExpr := &ast.Ident{Name: "s"}

	stmts := p.generateDestructuring("Status", matchedExpr, pattern)

	// Unit patterns have no fields, should generate no statements
	assert.Empty(t, stmts, "Unit patterns should not generate destructuring code")
}

// ============================================================================
// Nil Safety Mode Tests
// ============================================================================

func TestGenerateNilCheck_OffMode(t *testing.T) {
	p := NewSumTypesPlugin()

	fieldAccess := &ast.SelectorExpr{
		X:   &ast.Ident{Name: "shape"},
		Sel: &ast.Ident{Name: "circle_radius"},
	}

	stmt := p.generateNilCheck(fieldAccess, "Circle", "radius", config.NilSafetyOff)

	assert.Nil(t, stmt, "Off mode should generate no nil check")
}

func TestGenerateNilCheck_OnMode(t *testing.T) {
	p := NewSumTypesPlugin()

	fieldAccess := &ast.SelectorExpr{
		X:   &ast.Ident{Name: "shape"},
		Sel: &ast.Ident{Name: "circle_radius"},
	}

	stmt := p.generateNilCheck(fieldAccess, "Circle", "radius", config.NilSafetyOn)

	require.NotNil(t, stmt, "On mode should generate nil check")

	ifStmt, ok := stmt.(*ast.IfStmt)
	require.True(t, ok, "Should be if statement")

	// Check condition: shape.circle_radius == nil
	binaryExpr := ifStmt.Cond.(*ast.BinaryExpr)
	assert.Equal(t, token.EQL, binaryExpr.Op)

	selExpr := binaryExpr.X.(*ast.SelectorExpr)
	assert.Equal(t, "shape", selExpr.X.(*ast.Ident).Name)
	assert.Equal(t, "circle_radius", selExpr.Sel.Name)

	nilIdent := binaryExpr.Y.(*ast.Ident)
	assert.Equal(t, "nil", nilIdent.Name)

	// Check body contains panic
	require.Len(t, ifStmt.Body.List, 1)
	exprStmt := ifStmt.Body.List[0].(*ast.ExprStmt)
	callExpr := exprStmt.X.(*ast.CallExpr)
	assert.Equal(t, "panic", callExpr.Fun.(*ast.Ident).Name)
}

func TestGenerateNilCheck_DebugMode(t *testing.T) {
	p := NewSumTypesPlugin()
	p.generatedDecls = []ast.Decl{}
	p.emittedDebugVar = false

	fieldAccess := &ast.SelectorExpr{
		X:   &ast.Ident{Name: "shape"},
		Sel: &ast.Ident{Name: "circle_radius"},
	}

	stmt := p.generateNilCheck(fieldAccess, "Circle", "radius", config.NilSafetyDebug)

	require.NotNil(t, stmt, "Debug mode should generate conditional nil check")

	// Should have emitted debug variable
	assert.True(t, p.emittedDebugVar, "Debug mode should emit dingoDebug variable")
	assert.Len(t, p.generatedDecls, 1)

	ifStmt := stmt.(*ast.IfStmt)

	// Check condition: dingoDebug && shape.circle_radius == nil
	outerBinary := ifStmt.Cond.(*ast.BinaryExpr)

	// Left side should be dingoDebug
	debugVar := outerBinary.X.(*ast.BinaryExpr).X.(*ast.Ident)
	assert.Equal(t, "dingoDebug", debugVar.Name)

	// Operator should be &&
	assert.Equal(t, token.LAND, outerBinary.X.(*ast.BinaryExpr).Op)
}

func TestGenerateNilCheck_DebugMode_MultipleCallsEmitOnce(t *testing.T) {
	p := NewSumTypesPlugin()
	p.generatedDecls = []ast.Decl{}

	fieldAccess := &ast.SelectorExpr{
		X:   &ast.Ident{Name: "shape"},
		Sel: &ast.Ident{Name: "field"},
	}

	// First call
	p.generateNilCheck(fieldAccess, "Variant", "field", config.NilSafetyDebug)
	assert.Len(t, p.generatedDecls, 1)

	// Second call
	p.generateNilCheck(fieldAccess, "Variant", "field", config.NilSafetyDebug)
	assert.Len(t, p.generatedDecls, 1, "Should only emit debug variable once")
}

// ============================================================================
// Configuration Integration Tests
// ============================================================================

func TestConfig_GetNilSafetyMode(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected config.NilSafetyMode
	}{
		{"off mode", "off", config.NilSafetyOff},
		{"on mode", "on", config.NilSafetyOn},
		{"debug mode", "debug", config.NilSafetyDebug},
		{"empty defaults to on", "", config.NilSafetyOn},
		{"invalid defaults to on", "invalid", config.NilSafetyOn},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Features: config.FeatureConfig{
					NilSafetyChecks: tt.value,
				},
			}

			result := cfg.GetNilSafetyMode()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfig_Validate_NilSafety(t *testing.T) {
	validValues := []string{"off", "on", "debug"}
	for _, v := range validValues {
		t.Run("valid_"+v, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Features.NilSafetyChecks = v

			err := cfg.Validate()
			assert.NoError(t, err, "Valid nil_safety_checks value should pass validation")
		})
	}

	invalidValues := []string{"invalid", "yes", "no", "true", "false"}
	for _, v := range invalidValues {
		t.Run("invalid_"+v, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Features.NilSafetyChecks = v

			err := cfg.Validate()
			require.Error(t, err, "Invalid nil_safety_checks value should fail validation")
			assert.Contains(t, err.Error(), "invalid nil_safety_checks")
		})
	}
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestIntegration_TupleVariantEndToEnd(t *testing.T) {
	p := NewSumTypesPlugin()
	p.currentContext = &plugin.Context{
		Logger:      &plugin.NoOpLogger{},
		DingoConfig: &config.Config{Features: config.FeatureConfig{NilSafetyChecks: "on"}},
	}

	// Create enum with tuple variant
	enum := makeTestEnum("Shape")
	enum.Variants = []*dingoast.VariantDecl{
		{
			Name: &ast.Ident{Name: "Circle"},
			Kind: dingoast.VariantTuple,
			Fields: &ast.FieldList{
				List: []*ast.Field{
					{Names: nil, Type: &ast.Ident{Name: "float64"}},
				},
			},
		},
	}

	// Generate all components
	tagDecls := p.generateTagEnum(enum)
	unionDecl := p.generateUnionStruct(enum)
	constructor := p.generateConstructor(enum, enum.Variants[0])
	helper := p.generateHelperMethod(enum, enum.Variants[0])

	// Verify tag enum
	require.Len(t, tagDecls, 2)

	// Verify union struct has circle_0 field
	genDecl := unionDecl.(*ast.GenDecl)
	typeSpec := genDecl.Specs[0].(*ast.TypeSpec)
	structType := typeSpec.Type.(*ast.StructType)
	require.Len(t, structType.Fields.List, 2) // tag + circle_0
	assert.Equal(t, "circle_0", structType.Fields.List[1].Names[0].Name)

	// Verify constructor has arg0 parameter
	funcDecl := constructor.(*ast.FuncDecl)
	require.Len(t, funcDecl.Type.Params.List, 1)
	assert.Equal(t, "arg0", funcDecl.Type.Params.List[0].Names[0].Name)

	// Verify helper method exists
	helperFunc := helper.(*ast.FuncDecl)
	assert.Equal(t, "IsCircle", helperFunc.Name.Name)
}
