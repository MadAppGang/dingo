package builtin

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/plugin"
)

// TestTuplePlugin_Discovery tests the Discovery phase (Process method)
func TestTuplePlugin_Discovery(t *testing.T) {
	tests := []struct {
		name          string
		source        string
		expectedTypes []string // Expected tuple type names generated
	}{
		{
			name: "Simple tuple literal - int and string",
			source: `package main
func main() {
	x := __TUPLE_2__LITERAL__abc123(42, "hello")
}`,
			expectedTypes: []string{"Tuple2IntString"},
		},
		{
			name: "Tuple with three elements",
			source: `package main
func main() {
	x := __TUPLE_3__LITERAL__def456(10, 20.5, true)
}`,
			expectedTypes: []string{"Tuple3IntFloat64Bool"},
		},
		{
			name: "Multiple tuples with same type",
			source: `package main
func main() {
	x := __TUPLE_2__LITERAL__abc(1, 2)
	y := __TUPLE_2__LITERAL__def(3, 4)
}`,
			expectedTypes: []string{"Tuple2IntInt"}, // Deduplication
		},
		{
			name: "Multiple tuples with different types",
			source: `package main
func main() {
	x := __TUPLE_2__LITERAL__abc(1, "hello")
	y := __TUPLE_2__LITERAL__def(2.5, true)
}`,
			expectedTypes: []string{"Tuple2IntString", "Tuple2Float64Bool"},
		},
		{
			name: "Nested tuple literals",
			source: `package main
func main() {
	x := __TUPLE_2__LITERAL__outer(__TUPLE_2__LITERAL__inner(1, 2), "test")
}`,
			expectedTypes: []string{"Tuple2IntInt", "Tuple2Tuple2IntIntString"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse source
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.source, parser.AllErrors)
			if err != nil {
				t.Fatalf("Failed to parse source: %v", err)
			}

			// Create plugin with context
			p := NewTuplePlugin()
			ctx := &plugin.Context{
				FileSet: fset,
				Logger:  plugin.NewNoOpLogger(),
			}

			// Initialize type inference first
			typesInfo := &types.Info{
				Types: make(map[ast.Expr]types.TypeAndValue),
			}
			ctx.TypeInfo = typesInfo

			// Now set context (will pick up TypeInfo)
			p.SetContext(ctx)

			// Process file (Discovery phase)
			err = p.Process(file)
			if err != nil {
				t.Fatalf("Process failed: %v", err)
			}

			// Verify emitted types
			if len(p.emittedTypes) != len(tt.expectedTypes) {
				t.Errorf("Expected %d types, got %d", len(tt.expectedTypes), len(p.emittedTypes))
			}

			for _, expectedType := range tt.expectedTypes {
				if !p.emittedTypes[expectedType] {
					t.Errorf("Expected type %s not emitted", expectedType)
				}
			}

			// Verify pending declarations
			if len(p.pendingDecls) != len(tt.expectedTypes) {
				t.Errorf("Expected %d pending declarations, got %d", len(tt.expectedTypes), len(p.pendingDecls))
			}
		})
	}
}

// TestTuplePlugin_Transform tests the Transform phase
func TestTuplePlugin_Transform(t *testing.T) {
	tests := []struct {
		name           string
		source         string
		expectedOutput string // Expected transformed code pattern
	}{
		{
			name: "Simple tuple literal transformation",
			source: `package main
func main() {
	x := __TUPLE_2__LITERAL__abc(42, "hello")
}`,
			expectedOutput: `Tuple2IntString{_0: 42, _1: "hello"}`,
		},
		{
			name: "Tuple in return statement",
			source: `package main
func test() {
	return __TUPLE_2__LITERAL__abc(1, 2)
}`,
			expectedOutput: `Tuple2IntInt{_0: 1, _1: 2}`,
		},
		{
			name: "Tuple in function call",
			source: `package main
func main() {
	foo(__TUPLE_2__LITERAL__abc(1, 2))
}`,
			expectedOutput: `Tuple2IntInt{_0: 1, _1: 2}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse source
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.source, parser.AllErrors)
			if err != nil {
				t.Fatalf("Failed to parse source: %v", err)
			}

			// Create plugin with context
			p := NewTuplePlugin()
			ctx := &plugin.Context{
				FileSet: fset,
				Logger:  plugin.NewNoOpLogger(),
			}
			p.SetContext(ctx)

			// Initialize type inference
			typesInfo := &types.Info{
				Types: make(map[ast.Expr]types.TypeAndValue),
			}
			ctx.TypeInfo = typesInfo

			// Process first (Discovery phase)
			err = p.Process(file)
			if err != nil {
				t.Fatalf("Process failed: %v", err)
			}

			// Transform (Transform phase)
			transformed, err := p.Transform(file)
			if err != nil {
				t.Fatalf("Transform failed: %v", err)
			}

			// Verify transformation (simple pattern check)
			transformedFile := transformed.(*ast.File)
			found := false
			ast.Inspect(transformedFile, func(n ast.Node) bool {
				if comp, ok := n.(*ast.CompositeLit); ok {
					if ident, ok := comp.Type.(*ast.Ident); ok {
						if strings.Contains(ident.Name, "Tuple") {
							found = true
						}
					}
				}
				return true
			})

			if !found {
				t.Errorf("Expected tuple struct literal in transformed AST")
			}
		})
	}
}

// TestTuplePlugin_TypeNameGeneration tests canonical type name generation
func TestTuplePlugin_TypeNameGeneration(t *testing.T) {
	tests := []struct {
		name         string
		arity        int
		elementTypes []string
		expectedName string
	}{
		{
			name:         "Simple int and string",
			arity:        2,
			elementTypes: []string{"int", "string"},
			expectedName: "Tuple2IntString",
		},
		{
			name:         "Three basic types",
			arity:        3,
			elementTypes: []string{"int", "float64", "bool"},
			expectedName: "Tuple3IntFloat64Bool",
		},
		{
			name:         "User-defined types",
			arity:        2,
			elementTypes: []string{"User", "Error"},
			expectedName: "Tuple2UserError",
		},
		{
			name:         "Pointer types",
			arity:        2,
			elementTypes: []string{"*int", "string"},
			expectedName: "Tuple2PtrIntString",
		},
		{
			name:         "Slice types",
			arity:        2,
			elementTypes: []string{"[]string", "int"},
			expectedName: "Tuple2SliceStringInt",
		},
		{
			name:         "Map types",
			arity:        2,
			elementTypes: []string{"map[string]int", "bool"},
			expectedName: "Tuple2MapStringIntBool",
		},
		{
			name:         "Mixed complex types",
			arity:        3,
			elementTypes: []string{"*int", "[]string", "map[string]bool"},
			expectedName: "Tuple3PtrIntSliceStringMapStringBool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewTuplePlugin()
			typeName := p.generateTypeName(tt.arity, tt.elementTypes)

			if typeName != tt.expectedName {
				t.Errorf("Expected type name %s, got %s", tt.expectedName, typeName)
			}
		})
	}
}

// TestTuplePlugin_SanitizeTypeName tests type name sanitization
func TestTuplePlugin_SanitizeTypeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"int", "Int"},
		{"string", "String"},
		{"bool", "Bool"},
		{"error", "Error"},
		{"*int", "PtrInt"},
		{"[]string", "SliceString"},
		{"map[string]int", "MapStringInt"},
		{"chan int", "ChanInt"},
		{"interface{}", "Any"},
		{"User", "User"},                       // User types unchanged
		{"pkg.User", "User"},                   // Package prefix removed
		{"*[]map[string]int", "PtrSliceMapStringInt"}, // Nested complex
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeTupleTypeName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeTupleTypeName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestTuplePlugin_Deduplication tests type deduplication
func TestTuplePlugin_Deduplication(t *testing.T) {
	source := `package main
func main() {
	x := __TUPLE_2__LITERAL__abc(1, "hello")
	y := __TUPLE_2__LITERAL__def(2, "world")
	z := __TUPLE_2__LITERAL__ghi(3, "test")
}`

	// Parse source
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", source, parser.AllErrors)
	if err != nil {
		t.Fatalf("Failed to parse source: %v", err)
	}

	// Create plugin with context
	p := NewTuplePlugin()
	ctx := &plugin.Context{
		FileSet: fset,
		Logger:  plugin.NewNoOpLogger(),
	}
	p.SetContext(ctx)

	// Initialize type inference
	typesInfo := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
	}
	ctx.TypeInfo = typesInfo

	// Process file
	err = p.Process(file)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Should only emit ONE Tuple2IntString type
	if len(p.emittedTypes) != 1 {
		t.Errorf("Expected 1 unique type, got %d", len(p.emittedTypes))
	}

	if !p.emittedTypes["Tuple2IntString"] {
		t.Errorf("Expected Tuple2IntString to be emitted")
	}

	// Should only have ONE pending declaration
	if len(p.pendingDecls) != 1 {
		t.Errorf("Expected 1 pending declaration, got %d", len(p.pendingDecls))
	}
}

// TestTuplePlugin_DeclarationProvider tests DeclarationProvider interface
func TestTuplePlugin_DeclarationProvider(t *testing.T) {
	p := NewTuplePlugin()
	ctx := &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}
	p.SetContext(ctx)

	// Emit some declarations
	p.emitTupleDeclaration("Tuple2IntString", []string{"int", "string"})
	p.emitTupleDeclaration("Tuple3IntFloatBool", []string{"int", "float64", "bool"})

	// Get pending declarations
	decls := p.GetPendingDeclarations()
	if len(decls) != 2 {
		t.Errorf("Expected 2 pending declarations, got %d", len(decls))
	}

	// Clear declarations
	p.ClearPendingDeclarations()
	decls = p.GetPendingDeclarations()
	if len(decls) != 0 {
		t.Errorf("Expected 0 pending declarations after clear, got %d", len(decls))
	}
}

// TestTuplePlugin_ParseTypeExpr tests type expression parsing
func TestTuplePlugin_ParseTypeExpr(t *testing.T) {
	tests := []struct {
		name     string
		typeStr  string
		expected string // AST node type
	}{
		{"Basic type", "int", "*ast.Ident"},
		{"Pointer type", "*int", "*ast.StarExpr"},
		{"Slice type", "[]string", "*ast.ArrayType"},
		{"Map type", "map[string]int", "*ast.MapType"},
		{"Channel type", "chan int", "*ast.ChanType"},
		{"Interface type", "interface{}", "*ast.InterfaceType"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewTuplePlugin()
			expr := p.parseTypeExpr(tt.typeStr)

			exprType := getNodeType(expr)
			if exprType != tt.expected {
				t.Errorf("parseTypeExpr(%q) returned %s, expected %s", tt.typeStr, exprType, tt.expected)
			}
		})
	}
}

// Helper function to get node type as string
func getNodeType(n ast.Node) string {
	switch n.(type) {
	case *ast.Ident:
		return "*ast.Ident"
	case *ast.StarExpr:
		return "*ast.StarExpr"
	case *ast.ArrayType:
		return "*ast.ArrayType"
	case *ast.MapType:
		return "*ast.MapType"
	case *ast.ChanType:
		return "*ast.ChanType"
	case *ast.InterfaceType:
		return "*ast.InterfaceType"
	default:
		return "unknown"
	}
}

// TestTuplePlugin_EdgeCases tests edge cases and error handling
func TestTuplePlugin_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		source string
		expectError bool
	}{
		{
			name: "Invalid arity - 1 element",
			source: `package main
func main() {
	x := __TUPLE_1__LITERAL__abc(42)
}`,
			expectError: false, // Plugin logs warning but doesn't error
		},
		{
			name: "Invalid arity - 13 elements",
			source: `package main
func main() {
	x := __TUPLE_13__LITERAL__abc(1,2,3,4,5,6,7,8,9,10,11,12,13)
}`,
			expectError: false, // Plugin logs warning but doesn't error
		},
		{
			name: "Non-marker function call",
			source: `package main
func main() {
	x := normalFunction(1, 2)
}`,
			expectError: false, // Should be ignored
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.source, parser.AllErrors)
			if err != nil {
				t.Fatalf("Failed to parse source: %v", err)
			}

			p := NewTuplePlugin()
			ctx := &plugin.Context{
				FileSet: fset,
				Logger:  plugin.NewNoOpLogger(),
			}
			p.SetContext(ctx)

			err = p.Process(file)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
