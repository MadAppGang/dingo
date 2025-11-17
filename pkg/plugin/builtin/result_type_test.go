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

// TestResultTypePlugin_OkTransformation tests Ok() literal transformation
func TestResultTypePlugin_OkTransformation(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectType    string
		expectTag     string
		expectField   string
		expectValue   string
	}{
		{
			name:        "Ok with int literal",
			input:       `package main; func main() { x := Ok(42) }`,
			expectType:  "Result_int_error",
			expectTag:   "ResultTag_Ok",
			expectField: "ok_0",
			expectValue: "42",
		},
		{
			name:        "Ok with string literal",
			input:       `package main; func main() { x := Ok("hello") }`,
			expectType:  "Result_string_error",
			expectTag:   "ResultTag_Ok",
			expectField: "ok_0",
			expectValue: `"hello"`,
		},
		{
			name:        "Ok with variable",
			input:       `package main; func main() { user := User{}; x := Ok(user) }`,
			expectType:  "Result_user_error",
			expectTag:   "ResultTag_Ok",
			expectField: "ok_0",
			expectValue: "user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse input
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.input, 0)
			require.NoError(t, err)

			// Create plugin and context
			p := NewResultTypePlugin()
			ctx := &plugin.Context{
				Logger: &testLogger{},
			}

			// Transform
			result, err := p.Transform(ctx, file)
			require.NoError(t, err)
			require.NotNil(t, result)

			resultFile, ok := result.(*ast.File)
			require.True(t, ok)

			// Find the transformed Ok() call
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

			require.NotNil(t, compositeLit, "Expected to find Result composite literal")
			assert.Len(t, compositeLit.Elts, 2, "Result should have 2 fields")

			// Verify tag field
			tagKV, ok := compositeLit.Elts[0].(*ast.KeyValueExpr)
			require.True(t, ok)
			assert.Equal(t, "tag", tagKV.Key.(*ast.Ident).Name)
			assert.Equal(t, tt.expectTag, tagKV.Value.(*ast.Ident).Name)

			// Verify ok_0 field
			okKV, ok := compositeLit.Elts[1].(*ast.KeyValueExpr)
			require.True(t, ok)
			assert.Equal(t, tt.expectField, okKV.Key.(*ast.Ident).Name)
		})
	}
}

// TestResultTypePlugin_TypeDeclarationEmission tests that Result types are emitted
func TestResultTypePlugin_TypeDeclarationEmission(t *testing.T) {
	input := `package main
func main() {
	x := Ok(42)
	y := Ok("hello")
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", input, 0)
	require.NoError(t, err)

	p := NewResultTypePlugin()
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

	// Each Result type emits 2 type decls (tag type + struct) + 1 const decl
	// Result_int_error: 2 types + 1 const
	// Result_string_error: 2 types + 1 const
	assert.Equal(t, 4, typeCount, "Expected 4 type declarations")
	assert.Equal(t, 2, constCount, "Expected 2 const declarations")
}

// TestResultTypePlugin_TypeNameSanitization tests that type names are properly sanitized
func TestResultTypePlugin_TypeNameSanitization(t *testing.T) {
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

	p := NewResultTypePlugin()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitized := p.sanitizeTypeName(tt.typeName)
			assert.Equal(t, tt.expected, sanitized)
		})
	}
}

// TestResultTypePlugin_DuplicateTypeHandling tests that duplicate types aren't emitted twice
func TestResultTypePlugin_DuplicateTypeHandling(t *testing.T) {
	input := `package main
func main() {
	x := Ok(42)
	y := Ok(100)
	z := Ok(200)
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", input, 0)
	require.NoError(t, err)

	p := NewResultTypePlugin()
	ctx := &plugin.Context{
		Logger: &testLogger{},
	}

	result, err := p.Transform(ctx, file)
	require.NoError(t, err)

	resultFile, ok := result.(*ast.File)
	require.True(t, ok)

	// Count Result_int_error struct declarations
	count := 0
	for _, decl := range resultFile.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok {
			if genDecl.Tok == token.TYPE {
				for _, spec := range genDecl.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						if typeSpec.Name.Name == "Result_int_error" {
							count++
						}
					}
				}
			}
		}
	}

	assert.Equal(t, 1, count, "Result_int_error should only be declared once")
}

// TestResultTypePlugin_ErrTransformation tests Err() literal transformation
func TestResultTypePlugin_ErrTransformation(t *testing.T) {
	input := `package main
import "errors"
func main() {
	x := Err(errors.New("failed"))
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", input, 0)
	require.NoError(t, err)

	p := NewResultTypePlugin()
	ctx := &plugin.Context{
		Logger: &testLogger{},
	}

	result, err := p.Transform(ctx, file)
	require.NoError(t, err)

	resultFile, ok := result.(*ast.File)
	require.True(t, ok)

	// Find the transformed Err() call
	var compositeLit *ast.CompositeLit
	ast.Inspect(resultFile, func(n ast.Node) bool {
		if lit, ok := n.(*ast.CompositeLit); ok {
			compositeLit = lit
			return false
		}
		return true
	})

	require.NotNil(t, compositeLit, "Expected to find Result composite literal")

	// Verify it has tag field set to Err
	if len(compositeLit.Elts) >= 1 {
		tagKV, ok := compositeLit.Elts[0].(*ast.KeyValueExpr)
		require.True(t, ok)
		assert.Equal(t, "tag", tagKV.Key.(*ast.Ident).Name)
		assert.Equal(t, "ResultTag_Err", tagKV.Value.(*ast.Ident).Name)
	}
}

// TestResultTypePlugin_GracefulDegradation tests behavior when TypeInferenceService is unavailable
func TestResultTypePlugin_GracefulDegradation(t *testing.T) {
	input := `package main
func main() {
	x := Ok(42)
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", input, 0)
	require.NoError(t, err)

	p := NewResultTypePlugin()
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

	// Verify Result type was generated
	foundType := false
	for _, decl := range resultFile.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok {
			if genDecl.Tok == token.TYPE {
				for _, spec := range genDecl.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						if typeSpec.Name.Name == "Result_int_error" {
							foundType = true
						}
					}
				}
			}
		}
	}
	assert.True(t, foundType, "Should generate Result type even without TypeInferenceService")
}

// TestResultTypePlugin_NilChecks tests that nil inputs are handled gracefully
func TestResultTypePlugin_NilChecks(t *testing.T) {
	p := NewResultTypePlugin()
	ctx := &plugin.Context{
		Logger: &testLogger{},
	}

	t.Run("nil context", func(t *testing.T) {
		callExpr := &ast.CallExpr{
			Fun:  &ast.Ident{Name: "Ok"},
			Args: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "42"}},
		}
		result := p.transformOkLiteral(callExpr, nil)
		assert.Nil(t, result)
	})

	t.Run("nil call expression", func(t *testing.T) {
		result := p.transformOkLiteral(nil, ctx)
		assert.Nil(t, result)
	})

	t.Run("empty args", func(t *testing.T) {
		callExpr := &ast.CallExpr{
			Fun:  &ast.Ident{Name: "Ok"},
			Args: []ast.Expr{},
		}
		result := p.transformOkLiteral(callExpr, ctx)
		assert.Nil(t, result)
	})

	t.Run("too many args", func(t *testing.T) {
		callExpr := &ast.CallExpr{
			Fun: &ast.Ident{Name: "Ok"},
			Args: []ast.Expr{
				&ast.BasicLit{Kind: token.INT, Value: "42"},
				&ast.BasicLit{Kind: token.INT, Value: "100"},
			},
		}
		result := p.transformOkLiteral(callExpr, ctx)
		assert.Nil(t, result)
	})
}

// TestResultTypePlugin_TypeToString tests type to string conversion
func TestResultTypePlugin_TypeToString(t *testing.T) {
	p := NewResultTypePlugin()

	t.Run("nil type", func(t *testing.T) {
		result := p.typeToString(nil)
		assert.Equal(t, "unknown", result)
	})

	// Additional type conversion tests would require setting up types.Type instances
	// which is complex - defer to integration tests
}

// testLogger is defined in functional_utils_test.go
