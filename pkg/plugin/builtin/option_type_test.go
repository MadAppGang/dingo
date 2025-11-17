package builtin

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/MadAppGang/dingo/pkg/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOptionTypePlugin_SomeTransformation tests Some() literal transformation
func TestOptionTypePlugin_SomeTransformation(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectType  string
		expectTag   string
		expectField string
	}{
		{
			name:        "Some with int literal",
			input:       `package main; func main() { x := Some(42) }`,
			expectType:  "Option_int",
			expectTag:   "OptionTag_Some",
			expectField: "some_0",
		},
		{
			name:        "Some with string literal",
			input:       `package main; func main() { x := Some("hello") }`,
			expectType:  "Option_string",
			expectTag:   "OptionTag_Some",
			expectField: "some_0",
		},
		{
			name:        "Some with variable",
			input:       `package main; func main() { user := User{}; x := Some(user) }`,
			expectType:  "Option_user",
			expectTag:   "OptionTag_Some",
			expectField: "some_0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse input
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.input, 0)
			require.NoError(t, err)

			// Create plugin and context
			p := NewOptionTypePlugin()
			ctx := &plugin.Context{
				Logger: &testLogger{},
			}

			// Transform
			result, err := p.Transform(ctx, file)
			require.NoError(t, err)
			require.NotNil(t, result)

			resultFile, ok := result.(*ast.File)
			require.True(t, ok)

			// Find the transformed Some() call
			var compositeLit *ast.CompositeLit
			ast.Inspect(resultFile, func(n ast.Node) bool {
				if lit, ok := n.(*ast.CompositeLit); ok {
					if ident, ok := lit.Type.(*ast.Ident); ok {
						if ident.Name == tt.expectType {
							compositeLit = lit
							return false
						}
					}
				}
				return true
			})

			require.NotNil(t, compositeLit, "Expected to find Option composite literal")
			assert.Len(t, compositeLit.Elts, 2, "Option should have 2 fields")

			// Verify tag field
			tagKV, ok := compositeLit.Elts[0].(*ast.KeyValueExpr)
			require.True(t, ok)
			assert.Equal(t, "tag", tagKV.Key.(*ast.Ident).Name)
			assert.Equal(t, tt.expectTag, tagKV.Value.(*ast.Ident).Name)

			// Verify some_0 field
			someKV, ok := compositeLit.Elts[1].(*ast.KeyValueExpr)
			require.True(t, ok)
			assert.Equal(t, tt.expectField, someKV.Key.(*ast.Ident).Name)
		})
	}
}

// TestOptionTypePlugin_TypeDeclarationEmission tests that Option types are emitted
func TestOptionTypePlugin_TypeDeclarationEmission(t *testing.T) {
	input := `package main
func main() {
	x := Some(42)
	y := Some("hello")
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", input, 0)
	require.NoError(t, err)

	p := NewOptionTypePlugin()
	ctx := &plugin.Context{
		Logger: &testLogger{},
	}

	result, err := p.Transform(ctx, file)
	require.NoError(t, err)

	resultFile, ok := result.(*ast.File)
	require.True(t, ok)

	// Count type declarations
	typeCount := 0
	constCount := 0
	for _, decl := range resultFile.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok {
			if genDecl.Tok == token.TYPE {
				typeCount++
			}
			if genDecl.Tok == token.CONST {
				constCount++
			}
		}
	}

	// Each Option type emits 2 type decls (tag type + struct) + 1 const decl
	// Option_int: 2 types + 1 const
	// Option_string: 2 types + 1 const
	assert.Equal(t, 4, typeCount, "Expected 4 type declarations")
	assert.Equal(t, 2, constCount, "Expected 2 const declarations")
}

// TestOptionTypePlugin_TypeNameSanitization tests that type names are properly sanitized
func TestOptionTypePlugin_TypeNameSanitization(t *testing.T) {
	tests := []struct {
		name     string
		typeName string
		expected string
	}{
		{
			name:     "pointer type",
			typeName: "*User",
			expected: "ptr_User",
		},
		{
			name:     "slice type",
			typeName: "[]byte",
			expected: "__byte", // [] becomes __
		},
		{
			name:     "map type",
			typeName: "map[string]int",
			expected: "map_string_int",
		},
		{
			name:     "package qualified",
			typeName: "pkg.Type",
			expected: "pkg_Type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitized := sanitizeTypeName(tt.typeName)
			assert.Equal(t, tt.expected, sanitized)
		})
	}
}

// TestOptionTypePlugin_DuplicateTypeHandling tests that duplicate types aren't emitted twice
func TestOptionTypePlugin_DuplicateTypeHandling(t *testing.T) {
	input := `package main
func main() {
	x := Some(42)
	y := Some(100)
	z := Some(200)
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", input, 0)
	require.NoError(t, err)

	p := NewOptionTypePlugin()
	ctx := &plugin.Context{
		Logger: &testLogger{},
	}

	result, err := p.Transform(ctx, file)
	require.NoError(t, err)

	resultFile, ok := result.(*ast.File)
	require.True(t, ok)

	// Count Option_int struct declarations
	count := 0
	for _, decl := range resultFile.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok {
			if genDecl.Tok == token.TYPE {
				for _, spec := range genDecl.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						if typeSpec.Name.Name == "Option_int" {
							count++
						}
					}
				}
			}
		}
	}

	assert.Equal(t, 1, count, "Option_int should only be declared once")
}

// TestOptionTypePlugin_GracefulDegradation tests behavior when TypeInferenceService is unavailable
func TestOptionTypePlugin_GracefulDegradation(t *testing.T) {
	input := `package main
func main() {
	x := Some(42)
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", input, 0)
	require.NoError(t, err)

	p := NewOptionTypePlugin()
	ctx := &plugin.Context{
		Logger:        &testLogger{},
		TypeInference: nil, // No type inference service
	}

	// Should still work using fallback type inference
	result, err := p.Transform(ctx, file)
	require.NoError(t, err)
	assert.NotNil(t, result)

	resultFile, ok := result.(*ast.File)
	require.True(t, ok)

	// Verify Option type was generated
	foundType := false
	for _, decl := range resultFile.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok {
			if genDecl.Tok == token.TYPE {
				for _, spec := range genDecl.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						if typeSpec.Name.Name == "Option_int" {
							foundType = true
						}
					}
				}
			}
		}
	}
	assert.True(t, foundType, "Should generate Option type even without TypeInferenceService")
}

// TestOptionTypePlugin_NilChecks tests that nil inputs are handled gracefully
func TestOptionTypePlugin_NilChecks(t *testing.T) {
	p := NewOptionTypePlugin()
	ctx := &plugin.Context{
		Logger: &testLogger{},
	}

	t.Run("nil context", func(t *testing.T) {
		callExpr := &ast.CallExpr{
			Fun:  &ast.Ident{Name: "Some"},
			Args: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "42"}},
		}
		result := p.transformSomeLiteral(callExpr, nil)
		assert.Nil(t, result)
	})

	t.Run("nil call expression", func(t *testing.T) {
		result := p.transformSomeLiteral(nil, ctx)
		assert.Nil(t, result)
	})

	t.Run("empty args", func(t *testing.T) {
		callExpr := &ast.CallExpr{
			Fun:  &ast.Ident{Name: "Some"},
			Args: []ast.Expr{},
		}
		result := p.transformSomeLiteral(callExpr, ctx)
		assert.Nil(t, result)
	})

	t.Run("too many args", func(t *testing.T) {
		callExpr := &ast.CallExpr{
			Fun: &ast.Ident{Name: "Some"},
			Args: []ast.Expr{
				&ast.BasicLit{Kind: token.INT, Value: "42"},
				&ast.BasicLit{Kind: token.INT, Value: "100"},
			},
		}
		result := p.transformSomeLiteral(callExpr, ctx)
		assert.Nil(t, result)
	})
}

// TestOptionTypePlugin_TypeToString tests type to string conversion
func TestOptionTypePlugin_TypeToString(t *testing.T) {
	t.Run("nil type", func(t *testing.T) {
		result := typeToString(nil)
		assert.Equal(t, "unknown", result)
	})

	// Additional type conversion tests are in type_utils_test.go
}
