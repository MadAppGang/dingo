package builtin

import (
	"go/ast"
	"go/printer"
	"go/token"
	"strings"
	"testing"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a test enum declaration
func makeTestEnum(name string, variants ...*dingoast.VariantDecl) *dingoast.EnumDecl {
	return &dingoast.EnumDecl{
		Name:     &ast.Ident{Name: name},
		Variants: variants,
	}
}

// Helper function to create a unit variant
func makeUnitVariant(name string) *dingoast.VariantDecl {
	return &dingoast.VariantDecl{
		Name: &ast.Ident{Name: name},
		Kind: dingoast.VariantUnit,
	}
}

// Helper function to create a struct variant
func makeStructVariant(name string, fields ...*ast.Field) *dingoast.VariantDecl {
	return &dingoast.VariantDecl{
		Name:   &ast.Ident{Name: name},
		Kind:   dingoast.VariantStruct,
		Fields: &ast.FieldList{List: fields},
	}
}

// Helper function to create a field
func makeField(name string, typ ast.Expr) *ast.Field {
	return &ast.Field{
		Names: []*ast.Ident{{Name: name}},
		Type:  typ,
	}
}

// Helper to print AST node for debugging
func printAST(t *testing.T, node ast.Node) string {
	var buf strings.Builder
	fset := token.NewFileSet()
	err := printer.Fprint(&buf, fset, node)
	require.NoError(t, err, "Failed to print AST")
	return buf.String()
}

// ============================================================================
// Test Plugin Creation
// ============================================================================

func TestNewSumTypesPlugin(t *testing.T) {
	p := NewSumTypesPlugin()

	require.NotNil(t, p, "Expected plugin to be non-nil")
	assert.Equal(t, "sum_types", p.Name())
	assert.NotNil(t, p.enumRegistry)
	assert.Empty(t, p.enumRegistry)
}

// ============================================================================
// Test Enum Registry
// ============================================================================

func TestCollectEnums_Success(t *testing.T) {
	p := NewSumTypesPlugin()

	// Create test enums
	enum1 := makeTestEnum("Status", makeUnitVariant("Pending"), makeUnitVariant("Active"))
	enum2 := makeTestEnum("Priority", makeUnitVariant("Low"), makeUnitVariant("High"))

	// Create placeholder nodes in Go AST
	placeholder1 := &ast.BadDecl{}
	placeholder2 := &ast.BadDecl{}

	file := &ast.File{
		Name:  &ast.Ident{Name: "main"},
		Decls: []ast.Decl{placeholder1, placeholder2},
	}

	// Create Dingo file wrapper with enum mappings
	dingoFile := dingoast.NewFile(&ast.File{Name: &ast.Ident{Name: "main"}})
	dingoFile.DingoNodes[placeholder1] = enum1
	dingoFile.DingoNodes[placeholder2] = enum2

	p.currentFile = dingoFile

	// Collect enums
	err := p.collectEnums(file)
	require.NoError(t, err)

	// Verify registry
	assert.Len(t, p.enumRegistry, 2)
	assert.Equal(t, enum1, p.enumRegistry["Status"])
	assert.Equal(t, enum2, p.enumRegistry["Priority"])
}

func TestCollectEnums_DuplicateEnumName(t *testing.T) {
	p := NewSumTypesPlugin()

	// Create two enums with same name
	enum1 := makeTestEnum("Status", makeUnitVariant("Pending"))
	enum2 := makeTestEnum("Status", makeUnitVariant("Active"))

	placeholder1 := &ast.BadDecl{}
	placeholder2 := &ast.BadDecl{}

	file := &ast.File{
		Name:  &ast.Ident{Name: "main"},
		Decls: []ast.Decl{placeholder1, placeholder2},
	}

	dingoFile := dingoast.NewFile(&ast.File{Name: &ast.Ident{Name: "main"}})
	dingoFile.DingoNodes[placeholder1] = enum1
	dingoFile.DingoNodes[placeholder2] = enum2

	p.currentFile = dingoFile

	// Should error on duplicate
	err := p.collectEnums(file)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate enum Status")
}

func TestCollectEnums_DuplicateVariantName(t *testing.T) {
	p := NewSumTypesPlugin()

	// Create enum with duplicate variant names
	enum := makeTestEnum("Shape",
		makeUnitVariant("Circle"),
		makeUnitVariant("Circle"), // Duplicate!
	)

	placeholder := &ast.BadDecl{}

	file := &ast.File{
		Name:  &ast.Ident{Name: "main"},
		Decls: []ast.Decl{placeholder},
	}

	dingoFile := dingoast.NewFile(&ast.File{Name: &ast.Ident{Name: "main"}})
	dingoFile.DingoNodes[placeholder] = enum

	p.currentFile = dingoFile

	// Should error on duplicate variant
	err := p.collectEnums(file)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate variant Circle")
	assert.Contains(t, err.Error(), "enum Shape")
}

// ============================================================================
// Test Tag Enum Generation
// ============================================================================

func TestGenerateTagEnum_Simple(t *testing.T) {
	p := NewSumTypesPlugin()

	enum := makeTestEnum("Status",
		makeUnitVariant("Pending"),
		makeUnitVariant("Approved"),
		makeUnitVariant("Rejected"),
	)

	decls := p.generateTagEnum(enum)
	require.Len(t, decls, 2, "Should generate type decl and const decl")

	// Check type declaration
	typeDecl, ok := decls[0].(*ast.GenDecl)
	require.True(t, ok, "First decl should be GenDecl")
	assert.Equal(t, token.TYPE, typeDecl.Tok)
	require.Len(t, typeDecl.Specs, 1)

	typeSpec := typeDecl.Specs[0].(*ast.TypeSpec)
	assert.Equal(t, "StatusTag", typeSpec.Name.Name)

	// Check const declaration
	constDecl, ok := decls[1].(*ast.GenDecl)
	require.True(t, ok, "Second decl should be GenDecl")
	assert.Equal(t, token.CONST, constDecl.Tok)
	require.Len(t, constDecl.Specs, 3, "Should have 3 const specs")

	// Check first constant uses iota
	firstSpec := constDecl.Specs[0].(*ast.ValueSpec)
	assert.Equal(t, "StatusTag_Pending", firstSpec.Names[0].Name)
	// Should have iota in first value
	require.NotNil(t, firstSpec.Values)
	require.Len(t, firstSpec.Values, 1)

	// Check other constants exist
	secondSpec := constDecl.Specs[1].(*ast.ValueSpec)
	assert.Equal(t, "StatusTag_Approved", secondSpec.Names[0].Name)

	thirdSpec := constDecl.Specs[2].(*ast.ValueSpec)
	assert.Equal(t, "StatusTag_Rejected", thirdSpec.Names[0].Name)
}

func TestGenerateTagEnum_WithGenerics(t *testing.T) {
	p := NewSumTypesPlugin()

	enum := makeTestEnum("Result", makeUnitVariant("Ok"), makeUnitVariant("Err"))
	enum.TypeParams = &ast.FieldList{
		List: []*ast.Field{
			{Names: []*ast.Ident{{Name: "T"}}, Type: &ast.Ident{Name: "any"}},
			{Names: []*ast.Ident{{Name: "E"}}, Type: &ast.Ident{Name: "any"}},
		},
	}

	decls := p.generateTagEnum(enum)
	require.Len(t, decls, 2)

	// Tag enum should NOT be generic (tag is discriminator only)
	typeDecl := decls[0].(*ast.GenDecl)
	typeSpec := typeDecl.Specs[0].(*ast.TypeSpec)
	assert.Equal(t, "ResultTag", typeSpec.Name.Name)
	assert.Nil(t, typeSpec.TypeParams, "Tag enum should not have type parameters")
}

// ============================================================================
// Test Union Struct Generation
// ============================================================================

func TestGenerateUnionStruct_AllVariantKinds(t *testing.T) {
	p := NewSumTypesPlugin()

	enum := makeTestEnum("Shape",
		makeUnitVariant("Point"),
		makeStructVariant("Circle",
			makeField("radius", &ast.Ident{Name: "float64"}),
		),
		makeStructVariant("Rectangle",
			makeField("width", &ast.Ident{Name: "float64"}),
			makeField("height", &ast.Ident{Name: "float64"}),
		),
	)

	decl := p.generateUnionStruct(enum)
	require.NotNil(t, decl)

	genDecl, ok := decl.(*ast.GenDecl)
	require.True(t, ok)
	assert.Equal(t, token.TYPE, genDecl.Tok)

	typeSpec := genDecl.Specs[0].(*ast.TypeSpec)
	assert.Equal(t, "Shape", typeSpec.Name.Name)

	structType, ok := typeSpec.Type.(*ast.StructType)
	require.True(t, ok)

	fields := structType.Fields.List
	require.Len(t, fields, 4, "Should have tag + 3 variant fields")

	// First field should be tag
	assert.Equal(t, "tag", fields[0].Names[0].Name)
	tagType, ok := fields[0].Type.(*ast.Ident)
	require.True(t, ok)
	assert.Equal(t, "ShapeTag", tagType.Name)

	// Other fields should be pointers
	assert.Equal(t, "circle_radius", fields[1].Names[0].Name)
	_, ok = fields[1].Type.(*ast.StarExpr)
	assert.True(t, ok, "Variant fields should be pointers")

	assert.Equal(t, "rectangle_width", fields[2].Names[0].Name)
	assert.Equal(t, "rectangle_height", fields[3].Names[0].Name)
}

func TestGenerateUnionStruct_Generic(t *testing.T) {
	p := NewSumTypesPlugin()

	enum := makeTestEnum("Option", makeUnitVariant("Some"), makeUnitVariant("None"))
	enum.TypeParams = &ast.FieldList{
		List: []*ast.Field{
			{Names: []*ast.Ident{{Name: "T"}}, Type: &ast.Ident{Name: "any"}},
		},
	}

	// Add a Some variant with T type
	enum.Variants[0] = makeStructVariant("Some",
		makeField("value", &ast.Ident{Name: "T"}),
	)

	decl := p.generateUnionStruct(enum)
	require.NotNil(t, decl)

	genDecl := decl.(*ast.GenDecl)
	typeSpec := genDecl.Specs[0].(*ast.TypeSpec)

	// Check that struct has type parameters
	require.NotNil(t, typeSpec.TypeParams, "Generic enum should have type params")
	assert.Len(t, typeSpec.TypeParams.List, 1)
	assert.Equal(t, "T", typeSpec.TypeParams.List[0].Names[0].Name)
}

func TestGenerateUnionStruct_NilFields(t *testing.T) {
	p := NewSumTypesPlugin()

	// Create variant with nil fields (unit variant edge case)
	enum := makeTestEnum("Status", &dingoast.VariantDecl{
		Name:   &ast.Ident{Name: "Pending"},
		Kind:   dingoast.VariantUnit,
		Fields: nil, // Explicitly nil
	})

	// Should not panic
	decl := p.generateUnionStruct(enum)
	require.NotNil(t, decl)

	genDecl := decl.(*ast.GenDecl)
	typeSpec := genDecl.Specs[0].(*ast.TypeSpec)
	structType := typeSpec.Type.(*ast.StructType)

	// Should only have tag field
	assert.Len(t, structType.Fields.List, 1)
	assert.Equal(t, "tag", structType.Fields.List[0].Names[0].Name)
}

// ============================================================================
// Test Constructor Generation
// ============================================================================

func TestGenerateConstructor_UnitVariant(t *testing.T) {
	p := NewSumTypesPlugin()

	enum := makeTestEnum("Status", makeUnitVariant("Pending"))
	variant := enum.Variants[0]

	decl := p.generateConstructor(enum, variant)
	require.NotNil(t, decl)

	funcDecl, ok := decl.(*ast.FuncDecl)
	require.True(t, ok)

	// Check function name
	assert.Equal(t, "Status_Pending", funcDecl.Name.Name)

	// Check no parameters for unit variant
	assert.Empty(t, funcDecl.Type.Params.List)

	// Check return type
	returnType := funcDecl.Type.Results.List[0].Type
	ident, ok := returnType.(*ast.Ident)
	require.True(t, ok)
	assert.Equal(t, "Status", ident.Name)

	// Check body sets tag
	require.NotNil(t, funcDecl.Body)
	require.Len(t, funcDecl.Body.List, 1)
}

func TestGenerateConstructor_StructVariant(t *testing.T) {
	p := NewSumTypesPlugin()

	enum := makeTestEnum("Shape",
		makeStructVariant("Circle",
			makeField("radius", &ast.Ident{Name: "float64"}),
		),
	)
	variant := enum.Variants[0]

	decl := p.generateConstructor(enum, variant)
	require.NotNil(t, decl)

	funcDecl := decl.(*ast.FuncDecl)

	// Check function name
	assert.Equal(t, "Shape_Circle", funcDecl.Name.Name)

	// Check has parameter
	require.Len(t, funcDecl.Type.Params.List, 1)
	param := funcDecl.Type.Params.List[0]
	assert.Equal(t, "radius", param.Names[0].Name)

	// Check return type
	returnType := funcDecl.Type.Results.List[0].Type
	ident, ok := returnType.(*ast.Ident)
	require.True(t, ok)
	assert.Equal(t, "Shape", ident.Name)
}

func TestGenerateConstructor_GenericEnum(t *testing.T) {
	p := NewSumTypesPlugin()

	enum := makeTestEnum("Result", makeUnitVariant("Ok"))
	enum.TypeParams = &ast.FieldList{
		List: []*ast.Field{
			{Names: []*ast.Ident{{Name: "T"}}, Type: &ast.Ident{Name: "any"}},
			{Names: []*ast.Ident{{Name: "E"}}, Type: &ast.Ident{Name: "any"}},
		},
	}

	// Add value field to Ok variant
	enum.Variants[0] = makeStructVariant("Ok",
		makeField("value", &ast.Ident{Name: "T"}),
	)

	decl := p.generateConstructor(enum, enum.Variants[0])
	require.NotNil(t, decl)

	funcDecl := decl.(*ast.FuncDecl)

	// Check function has type parameters
	require.NotNil(t, funcDecl.Type.TypeParams)
	assert.Len(t, funcDecl.Type.TypeParams.List, 2)

	// Check return type uses type parameters
	returnType := funcDecl.Type.Results.List[0].Type
	_, ok := returnType.(*ast.IndexListExpr)
	assert.True(t, ok, "Generic return type should be IndexListExpr for 2+ params")
}

func TestGenerateConstructor_GenericEnum_SingleParam(t *testing.T) {
	p := NewSumTypesPlugin()

	enum := makeTestEnum("Option", makeUnitVariant("Some"))
	enum.TypeParams = &ast.FieldList{
		List: []*ast.Field{
			{Names: []*ast.Ident{{Name: "T"}}, Type: &ast.Ident{Name: "any"}},
		},
	}

	enum.Variants[0] = makeStructVariant("Some",
		makeField("value", &ast.Ident{Name: "T"}),
	)

	decl := p.generateConstructor(enum, enum.Variants[0])
	require.NotNil(t, decl)

	funcDecl := decl.(*ast.FuncDecl)

	// Check return type uses IndexExpr for single param
	returnType := funcDecl.Type.Results.List[0].Type
	_, ok := returnType.(*ast.IndexExpr)
	assert.True(t, ok, "Single generic param should use IndexExpr")
}

// ============================================================================
// Test Helper Method Generation
// ============================================================================

func TestGenerateHelperMethod_IsMethod(t *testing.T) {
	p := NewSumTypesPlugin()

	enum := makeTestEnum("Status", makeUnitVariant("Pending"))
	variant := enum.Variants[0]

	decl := p.generateHelperMethod(enum, variant)
	require.NotNil(t, decl)

	funcDecl, ok := decl.(*ast.FuncDecl)
	require.True(t, ok)

	// Check method name
	assert.Equal(t, "IsPending", funcDecl.Name.Name)

	// Check receiver
	require.NotNil(t, funcDecl.Recv)
	require.Len(t, funcDecl.Recv.List, 1)
	receiver := funcDecl.Recv.List[0]
	assert.Equal(t, "e", receiver.Names[0].Name) // Implementation uses "e"

	// Check return type is bool
	require.Len(t, funcDecl.Type.Results.List, 1)
	returnType := funcDecl.Type.Results.List[0].Type
	ident, ok := returnType.(*ast.Ident)
	require.True(t, ok)
	assert.Equal(t, "bool", ident.Name)

	// Check body compares tag
	require.NotNil(t, funcDecl.Body)
	require.Len(t, funcDecl.Body.List, 1)

	returnStmt, ok := funcDecl.Body.List[0].(*ast.ReturnStmt)
	require.True(t, ok)
	require.Len(t, returnStmt.Results, 1)

	// Should be binary expression: s.tag == StatusTag_Pending
	binaryExpr, ok := returnStmt.Results[0].(*ast.BinaryExpr)
	require.True(t, ok)
	assert.Equal(t, token.EQL, binaryExpr.Op)
}

func TestGenerateHelperMethod_GenericEnum(t *testing.T) {
	p := NewSumTypesPlugin()

	enum := makeTestEnum("Option", makeUnitVariant("Some"))
	enum.TypeParams = &ast.FieldList{
		List: []*ast.Field{
			{Names: []*ast.Ident{{Name: "T"}}, Type: &ast.Ident{Name: "any"}},
		},
	}

	decl := p.generateHelperMethod(enum, enum.Variants[0])
	require.NotNil(t, decl)

	funcDecl := decl.(*ast.FuncDecl)

	// Check receiver has type parameters
	receiver := funcDecl.Recv.List[0]
	indexExpr, ok := receiver.Type.(*ast.IndexExpr)
	require.True(t, ok, "Generic receiver should use IndexExpr")
	assert.Equal(t, "Option", indexExpr.X.(*ast.Ident).Name)
}

func TestGenerateHelperMethod_AllVariants(t *testing.T) {
	p := NewSumTypesPlugin()

	enum := makeTestEnum("Status",
		makeUnitVariant("Pending"),
		makeUnitVariant("Approved"),
		makeUnitVariant("Rejected"),
	)

	// Generate method for each variant
	methods := []ast.Decl{}
	for _, variant := range enum.Variants {
		methods = append(methods, p.generateHelperMethod(enum, variant))
	}

	require.Len(t, methods, 3, "Should generate one method per variant")

	// Check all method names
	names := []string{}
	for _, m := range methods {
		funcDecl := m.(*ast.FuncDecl)
		names = append(names, funcDecl.Name.Name)
	}

	assert.Contains(t, names, "IsPending")
	assert.Contains(t, names, "IsApproved")
	assert.Contains(t, names, "IsRejected")
}

// ============================================================================
// Test Full Transform
// ============================================================================

func TestTransform_NoEnums(t *testing.T) {
	p := NewSumTypesPlugin()
	ctx := &plugin.Context{
		FileSet: token.NewFileSet(),
	}

	file := &ast.File{
		Name:  &ast.Ident{Name: "main"},
		Decls: []ast.Decl{},
	}

	result, err := p.Transform(ctx, file)
	require.NoError(t, err)
	assert.Equal(t, file, result, "File without enums should be unchanged")
}

func TestTransform_WithEnum(t *testing.T) {
	p := NewSumTypesPlugin()
	ctx := &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  &plugin.NoOpLogger{}, // Add logger to prevent nil pointer
	}

	// Create enum
	enum := makeTestEnum("Status",
		makeUnitVariant("Pending"),
		makeUnitVariant("Active"),
	)

	placeholder := &ast.BadDecl{}

	file := &ast.File{
		Name:  &ast.Ident{Name: "main"},
		Decls: []ast.Decl{placeholder},
	}

	dingoFile := dingoast.NewFile(file)
	dingoFile.DingoNodes[placeholder] = enum

	ctx.CurrentFile = dingoFile

	result, err := p.Transform(ctx, file)
	require.NoError(t, err)
	require.NotNil(t, result)

	resultFile, ok := result.(*ast.File)
	require.True(t, ok)

	// Should have generated declarations
	// Original placeholder is removed, replaced with:
	// - Tag enum type decl
	// - Tag enum const decl
	// - Union struct decl
	// - 2 constructor functions
	// - 2 helper methods
	// Total: 7 declarations
	assert.GreaterOrEqual(t, len(resultFile.Decls), 7,
		"Should have tag enum, struct, constructors, and helpers")

	// Verify we can print it (validates AST structure)
	output := printAST(t, resultFile)
	assert.Contains(t, output, "StatusTag")
	assert.Contains(t, output, "type Status struct")
	assert.Contains(t, output, "Status_Pending")
	assert.Contains(t, output, "IsPending")
}

// ============================================================================
// Test Nil Safety
// ============================================================================

func TestGenerateVariantFields_NilFields(t *testing.T) {
	p := NewSumTypesPlugin()

	variant := &dingoast.VariantDecl{
		Name:   &ast.Ident{Name: "Pending"},
		Kind:   dingoast.VariantUnit,
		Fields: nil, // Explicitly nil
	}

	// Should not panic
	fields := p.generateVariantFields(variant)
	assert.Nil(t, fields, "Nil fields should return nil")
}

func TestGenerateVariantFields_EmptyFieldList(t *testing.T) {
	p := NewSumTypesPlugin()

	variant := &dingoast.VariantDecl{
		Name:   &ast.Ident{Name: "Pending"},
		Kind:   dingoast.VariantUnit,
		Fields: &ast.FieldList{List: nil}, // Empty list
	}

	// Should not panic
	fields := p.generateVariantFields(variant)
	assert.Empty(t, fields)
}

func TestGenerateVariantFields_FieldsWithNilNames(t *testing.T) {
	p := NewSumTypesPlugin()

	variant := makeStructVariant("Circle")
	variant.Fields = &ast.FieldList{
		List: []*ast.Field{
			{Names: nil, Type: &ast.Ident{Name: "float64"}}, // Nil names
		},
	}

	// Should skip malformed fields
	fields := p.generateVariantFields(variant)
	assert.Empty(t, fields, "Should skip fields with nil names")
}

// ============================================================================
// Test Error Cases
// ============================================================================

func TestTransform_NilResult(t *testing.T) {
	p := NewSumTypesPlugin()
	ctx := &plugin.Context{
		FileSet: token.NewFileSet(),
	}

	// Pass non-file node
	result, err := p.Transform(ctx, &ast.Ident{Name: "test"})
	require.NoError(t, err)
	assert.NotNil(t, result, "Non-file nodes should be returned unchanged")
}
