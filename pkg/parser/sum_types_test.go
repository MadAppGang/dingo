//go:build ignore
// +build ignore

// NOTE: This test file is excluded from builds because sum types (enum/match)
// are not yet implemented. The required AST nodes (EnumDecl, MatchExpr, etc.)
// will be added in Phase 3+. To enable these tests, remove the build constraints
// above and implement the missing AST types in pkg/ast/.

package parser

import (
	"go/token"
	"testing"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Priority 1 Tests: Enum Parsing
// ============================================================================

func TestParseEnum_UnitVariants(t *testing.T) {
	t.Skip("Enum parsing not yet implemented - requires EnumDecl AST node (deferred to Phase 3+)")

	src := []byte(`package main

enum Status {
	Pending,
	Active,
	Complete,
}
`)
	fset := token.NewFileSet()
	file, err := ParseFile(fset, "test.dingo", src, 0)
	require.NoError(t, err)
	require.NotNil(t, file)

	// Find enum declaration
	var enumDecl *dingoast.EnumDecl
	for _, node := range file.DingoNodes {
		if enum, ok := node.(*dingoast.EnumDecl); ok {
			enumDecl = enum
			break
		}
	}

	require.NotNil(t, enumDecl, "Expected to find EnumDecl in DingoNodes")
	assert.Equal(t, "Status", enumDecl.Name.Name)
	assert.Len(t, enumDecl.Variants, 3)

	// Check variants
	variants := []string{"Pending", "Active", "Complete"}
	for i, expected := range variants {
		assert.Equal(t, expected, enumDecl.Variants[i].Name.Name, "Variant %d name", i)
		assert.Equal(t, dingoast.VariantUnit, enumDecl.Variants[i].Kind, "Variant %d kind", i)
		assert.Nil(t, enumDecl.Variants[i].Fields, "Unit variant should have nil fields")
	}
}

func TestParseEnum_TupleVariants(t *testing.T) {
	t.Skip("Enum parsing not yet implemented - requires EnumDecl AST node (deferred to Phase 3+)")

	src := []byte(`package main

enum Shape {
	Circle(radius: float64),
	Line(start: float64, end: float64),
}
`)
	fset := token.NewFileSet()
	file, err := ParseFile(fset, "test.dingo", src, 0)
	require.NoError(t, err)

	var enumDecl *dingoast.EnumDecl
	for _, node := range file.DingoNodes {
		if enum, ok := node.(*dingoast.EnumDecl); ok {
			enumDecl = enum
			break
		}
	}

	require.NotNil(t, enumDecl)
	assert.Equal(t, "Shape", enumDecl.Name.Name)
	assert.Len(t, enumDecl.Variants, 2)

	// Circle variant
	circle := enumDecl.Variants[0]
	assert.Equal(t, "Circle", circle.Name.Name)
	assert.Equal(t, dingoast.VariantTuple, circle.Kind)
	require.NotNil(t, circle.Fields)
	assert.Len(t, circle.Fields.List, 1)
	assert.Equal(t, "radius", circle.Fields.List[0].Names[0].Name)

	// Line variant
	line := enumDecl.Variants[1]
	assert.Equal(t, "Line", line.Name.Name)
	assert.Equal(t, dingoast.VariantTuple, line.Kind)
	require.NotNil(t, line.Fields)
	assert.Len(t, line.Fields.List, 2)
	assert.Equal(t, "start", line.Fields.List[0].Names[0].Name)
	assert.Equal(t, "end", line.Fields.List[1].Names[0].Name)
}

func TestParseEnum_StructVariants(t *testing.T) {
	t.Skip("Enum parsing not yet implemented - requires EnumDecl AST node (deferred to Phase 3+)")

	src := []byte(`package main

enum HttpResponse {
	Ok { body: string },
	Error { code: int, message: string },
}
`)
	fset := token.NewFileSet()
	file, err := ParseFile(fset, "test.dingo", src, 0)
	require.NoError(t, err)

	var enumDecl *dingoast.EnumDecl
	for _, node := range file.DingoNodes {
		if enum, ok := node.(*dingoast.EnumDecl); ok {
			enumDecl = enum
			break
		}
	}

	require.NotNil(t, enumDecl)
	assert.Equal(t, "HttpResponse", enumDecl.Name.Name)
	assert.Len(t, enumDecl.Variants, 2)

	// Ok variant
	ok := enumDecl.Variants[0]
	assert.Equal(t, "Ok", ok.Name.Name)
	assert.Equal(t, dingoast.VariantStruct, ok.Kind)
	require.NotNil(t, ok.Fields)
	assert.Len(t, ok.Fields.List, 1)
	assert.Equal(t, "body", ok.Fields.List[0].Names[0].Name)

	// Error variant
	errVariant := enumDecl.Variants[1]
	assert.Equal(t, "Error", errVariant.Name.Name)
	assert.Equal(t, dingoast.VariantStruct, errVariant.Kind)
	require.NotNil(t, errVariant.Fields)
	assert.Len(t, errVariant.Fields.List, 2)
	assert.Equal(t, "code", errVariant.Fields.List[0].Names[0].Name)
	assert.Equal(t, "message", errVariant.Fields.List[1].Names[0].Name)
}

func TestParseEnum_Generic(t *testing.T) {
	t.Skip("Enum parsing not yet implemented - requires EnumDecl AST node (deferred to Phase 3+)")

	src := []byte(`package main

enum Result<T, E> {
	Ok(T),
	Err(E),
}
`)
	fset := token.NewFileSet()
	file, err := ParseFile(fset, "test.dingo", src, 0)
	require.NoError(t, err)

	var enumDecl *dingoast.EnumDecl
	for _, node := range file.DingoNodes {
		if enum, ok := node.(*dingoast.EnumDecl); ok {
			enumDecl = enum
			break
		}
	}

	require.NotNil(t, enumDecl)
	assert.Equal(t, "Result", enumDecl.Name.Name)

	// Check type parameters
	require.NotNil(t, enumDecl.TypeParams, "Generic enum should have type parameters")
	assert.Len(t, enumDecl.TypeParams.List, 2)
	assert.Equal(t, "T", enumDecl.TypeParams.List[0].Names[0].Name)
	assert.Equal(t, "E", enumDecl.TypeParams.List[1].Names[0].Name)

	// Check variants
	assert.Len(t, enumDecl.Variants, 2)
	assert.Equal(t, "Ok", enumDecl.Variants[0].Name.Name)
	assert.Equal(t, "Err", enumDecl.Variants[1].Name.Name)
}

func TestParseEnum_MixedVariants(t *testing.T) {
	t.Skip("Enum parsing not yet implemented - requires EnumDecl AST node (deferred to Phase 3+)")

	src := []byte(`package main

enum Shape {
	Point,
	Circle { radius: float64 },
	Rectangle { width: float64, height: float64 },
}
`)
	fset := token.NewFileSet()
	file, err := ParseFile(fset, "test.dingo", src, 0)
	require.NoError(t, err)

	var enumDecl *dingoast.EnumDecl
	for _, node := range file.DingoNodes {
		if enum, ok := node.(*dingoast.EnumDecl); ok {
			enumDecl = enum
			break
		}
	}

	require.NotNil(t, enumDecl)
	assert.Len(t, enumDecl.Variants, 3)

	// Point - unit variant
	assert.Equal(t, dingoast.VariantUnit, enumDecl.Variants[0].Kind)
	assert.Nil(t, enumDecl.Variants[0].Fields)

	// Circle - struct variant with 1 field
	assert.Equal(t, dingoast.VariantStruct, enumDecl.Variants[1].Kind)
	assert.Len(t, enumDecl.Variants[1].Fields.List, 1)

	// Rectangle - struct variant with 2 fields
	assert.Equal(t, dingoast.VariantStruct, enumDecl.Variants[2].Kind)
	assert.Len(t, enumDecl.Variants[2].Fields.List, 2)
}

// ============================================================================
// Priority 1 Tests: Match Expression Parsing
// ============================================================================
// TODO(Phase 3+): Match expression parsing not yet implemented
// The match expression AST exists but parser support is deferred

func TestParseMatch_AllPatternTypes(t *testing.T) {
	t.Skip("Match expression parsing not yet implemented - deferred to Phase 3+")

	src := []byte(`package main

func test(r: Response) {
	match r {
		Ok { body } => println(body),
		Error { code, message } => println(code),
		_ => println("unknown"),
	}
}
`)
	fset := token.NewFileSet()
	file, err := ParseFile(fset, "test.dingo", src, 0)
	require.NoError(t, err)

	// Find match expression
	var matchExpr *dingoast.MatchExpr
	for _, node := range file.DingoNodes {
		if match, ok := node.(*dingoast.MatchExpr); ok {
			matchExpr = match
			break
		}
	}

	require.NotNil(t, matchExpr, "Expected to find MatchExpr in DingoNodes")
	assert.Len(t, matchExpr.Arms, 3)

	// First arm: Ok { body }
	arm0 := matchExpr.Arms[0]
	assert.Equal(t, dingoast.PatternStruct, arm0.Pattern.Kind)
	assert.Equal(t, "Ok", arm0.Pattern.Variant.Name)
	assert.Len(t, arm0.Pattern.Fields, 1)
	assert.Equal(t, "body", arm0.Pattern.Fields[0].Binding.Name)

	// Second arm: Error { code, message }
	arm1 := matchExpr.Arms[1]
	assert.Equal(t, dingoast.PatternStruct, arm1.Pattern.Kind)
	assert.Equal(t, "Error", arm1.Pattern.Variant.Name)
	assert.Len(t, arm1.Pattern.Fields, 2)
	assert.Equal(t, "code", arm1.Pattern.Fields[0].Binding.Name)
	assert.Equal(t, "message", arm1.Pattern.Fields[1].Binding.Name)

	// Third arm: _
	arm2 := matchExpr.Arms[2]
	assert.Equal(t, dingoast.PatternWildcard, arm2.Pattern.Kind)
	assert.True(t, arm2.Pattern.Wildcard)
}

func TestParseMatch_TuplePattern(t *testing.T) {
	t.Skip("Match expression parsing not yet implemented - deferred to Phase 3+")

	src := []byte(`package main

func test(s: Shape) {
	match s {
		Circle(r) => println(r),
		Point => println("point"),
	}
}
`)
	fset := token.NewFileSet()
	file, err := ParseFile(fset, "test.dingo", src, 0)
	require.NoError(t, err)

	var matchExpr *dingoast.MatchExpr
	for _, node := range file.DingoNodes {
		if match, ok := node.(*dingoast.MatchExpr); ok {
			matchExpr = match
			break
		}
	}

	require.NotNil(t, matchExpr)
	assert.Len(t, matchExpr.Arms, 2)

	// Circle(r) pattern
	arm0 := matchExpr.Arms[0]
	assert.Equal(t, dingoast.PatternTuple, arm0.Pattern.Kind)
	assert.Equal(t, "Circle", arm0.Pattern.Variant.Name)
	assert.Len(t, arm0.Pattern.Fields, 1)
	assert.Equal(t, "r", arm0.Pattern.Fields[0].Binding.Name)

	// Point pattern
	arm1 := matchExpr.Arms[1]
	assert.Equal(t, dingoast.PatternUnit, arm1.Pattern.Kind)
	assert.Equal(t, "Point", arm1.Pattern.Variant.Name)
	assert.Len(t, arm1.Pattern.Fields, 0)
}

func TestParseMatch_WildcardOnly(t *testing.T) {
	t.Skip("Match expression parsing not yet implemented - deferred to Phase 3+")

	src := []byte(`package main

func test(x: Status) {
	match x {
		_ => println("any"),
	}
}
`)
	fset := token.NewFileSet()
	file, err := ParseFile(fset, "test.dingo", src, 0)
	require.NoError(t, err)

	var matchExpr *dingoast.MatchExpr
	for _, node := range file.DingoNodes {
		if match, ok := node.(*dingoast.MatchExpr); ok {
			matchExpr = match
			break
		}
	}

	require.NotNil(t, matchExpr)
	assert.Len(t, matchExpr.Arms, 1)

	arm := matchExpr.Arms[0]
	assert.Equal(t, dingoast.PatternWildcard, arm.Pattern.Kind)
	assert.True(t, arm.Pattern.Wildcard)
	assert.Nil(t, arm.Pattern.Variant)
}

func TestParseMatch_MultiFieldDestructuring(t *testing.T) {
	t.Skip("Match expression parsing not yet implemented - deferred to Phase 3+")

	src := []byte(`package main

func test(s: Shape) {
	match s {
		Rectangle { width, height } => println(width),
		Circle { radius } => println(radius),
	}
}
`)
	fset := token.NewFileSet()
	file, err := ParseFile(fset, "test.dingo", src, 0)
	require.NoError(t, err)

	var matchExpr *dingoast.MatchExpr
	for _, node := range file.DingoNodes {
		if match, ok := node.(*dingoast.MatchExpr); ok {
			matchExpr = match
			break
		}
	}

	require.NotNil(t, matchExpr)
	assert.Len(t, matchExpr.Arms, 2)

	// Rectangle { width, height }
	rect := matchExpr.Arms[0]
	assert.Equal(t, dingoast.PatternStruct, rect.Pattern.Kind)
	assert.Len(t, rect.Pattern.Fields, 2)
	assert.Equal(t, "width", rect.Pattern.Fields[0].Binding.Name)
	assert.Equal(t, "height", rect.Pattern.Fields[1].Binding.Name)

	// Circle { radius }
	circle := matchExpr.Arms[1]
	assert.Equal(t, dingoast.PatternStruct, circle.Pattern.Kind)
	assert.Len(t, circle.Pattern.Fields, 1)
	assert.Equal(t, "radius", circle.Pattern.Fields[0].Binding.Name)
}
