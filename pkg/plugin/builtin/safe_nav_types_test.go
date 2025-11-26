package builtin

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/MadAppGang/dingo/pkg/plugin"
)

// testLogger implements plugin.Logger for test output
type testLogger struct {
	t *testing.T
}

func (l *testLogger) Debugf(format string, args ...interface{}) {
	l.t.Logf("[DEBUG] "+format, args...)
}

func (l *testLogger) Info(msg string) {
	l.t.Logf("[INFO] %s", msg)
}

func (l *testLogger) Warnf(format string, args ...interface{}) {
	l.t.Logf("[WARN] "+format, args...)
}

func (l *testLogger) Error(msg string) {
	l.t.Logf("[ERROR] %s", msg)
}

// TestSafeNavTypePlugin_Discovery tests the discovery phase of finding __INFER__ placeholders
func TestSafeNavTypePlugin_Discovery(t *testing.T) {
	tests := []struct {
		name           string
		source         string
		expectedCount  int
		description    string
	}{
		{
			name: "single __INFER__ placeholder",
			source: `package main
func main() {
	x := __INFER__.field
}`,
			expectedCount: 1,
			description:   "Should find one __INFER__ identifier",
		},
		{
			name: "multiple __INFER__ placeholders",
			source: `package main
func main() {
	x := __INFER__.field1
	y := __INFER__.field2
	z := __INFER__.method()
}`,
			expectedCount: 3,
			description:   "Should find three __INFER__ identifiers",
		},
		{
			name: "chained __INFER__ placeholders",
			source: `package main
func main() {
	x := __INFER__.field1.__INFER__.field2
}`,
			expectedCount: 2,
			description:   "Should find two __INFER__ identifiers in chain",
		},
		{
			name: "no __INFER__ placeholders",
			source: `package main
func main() {
	x := user.field
}`,
			expectedCount: 0,
			description:   "Should find zero __INFER__ identifiers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse source
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.source, parser.ParseComments)
			if err != nil {
				t.Fatalf("Failed to parse source: %v", err)
			}

			// Create plugin context
			ctx := &plugin.Context{
				FileSet: fset,
				Logger:  plugin.NewNoOpLogger(),
			}

			// Create plugin
			p := NewSafeNavTypePlugin()
			p.SetContext(ctx)

			// Run discovery phase
			err = p.Process(file)
			if err != nil {
				t.Fatalf("Process failed: %v", err)
			}

			// Verify count
			if len(p.inferNodes) != tt.expectedCount {
				t.Errorf("%s: expected %d __INFER__ nodes, got %d",
					tt.description, tt.expectedCount, len(p.inferNodes))
			}
		})
	}
}

// TestSafeNavTypePlugin_PointerTypeResolution tests type resolution for pointer types
func TestSafeNavTypePlugin_PointerTypeResolution(t *testing.T) {
	tests := []struct {
		name           string
		source         string
		expectedType   string
		expectedPtr    bool
		expectedOption bool
		description    string
	}{
		{
			name: "simple pointer type",
			source: `package main
type User struct {
	name string
}
func main() {
	var user *User
	x := __SAFE_NAV_INFER__(user, "name")
}`,
			expectedType:   "User",
			expectedPtr:    true,
			expectedOption: false,
			description:    "Should resolve *User to pointer type",
		},
		{
			name: "pointer to built-in type",
			source: `package main
func main() {
	var count *int
	x := __SAFE_NAV_INFER__(count, "value")
}`,
			expectedType:   "int",
			expectedPtr:    true,
			expectedOption: false,
			description:    "Should resolve *int to pointer type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse source
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.source, parser.ParseComments)
			if err != nil {
				t.Fatalf("Failed to parse source: %v", err)
			}

			// Type check the file
			conf := types.Config{}
			info := &types.Info{
				Types: make(map[ast.Expr]types.TypeAndValue),
				Defs:  make(map[*ast.Ident]types.Object),
				Uses:  make(map[*ast.Ident]types.Object),
			}
			_, err = conf.Check("test", fset, []*ast.File{file}, info)
			if err != nil {
				// Type errors are expected for __SAFE_NAV_INFER__ function
				// We'll ignore them for testing purposes
				t.Logf("Type check warnings (expected): %v", err)
			}

			// Create plugin context with type info
			ctx := &plugin.Context{
				FileSet:  fset,
				TypeInfo: info,
				Logger:   plugin.NewNoOpLogger(),
			}

			// Create plugin
			p := NewSafeNavTypePlugin()
			p.SetContext(ctx)

			// Run discovery phase
			err = p.Process(file)
			if err != nil {
				t.Fatalf("Process failed: %v", err)
			}

			// Run transform phase
			_, err = p.Transform(file)
			if err != nil {
				t.Fatalf("Transform failed: %v", err)
			}

			// Verify resolution (check inferNodes)
			if len(p.inferNodes) > 0 {
				node := p.inferNodes[0]
				if node.resolvedType != tt.expectedType {
					t.Errorf("%s: expected type %s, got %s",
						tt.description, tt.expectedType, node.resolvedType)
				}
				if node.isPointer != tt.expectedPtr {
					t.Errorf("%s: expected isPointer=%v, got %v",
						tt.description, tt.expectedPtr, node.isPointer)
				}
				if node.isOption != tt.expectedOption {
					t.Errorf("%s: expected isOption=%v, got %v",
						tt.description, tt.expectedOption, node.isOption)
				}
			}
		})
	}
}

// TestSafeNavTypePlugin_OptionTypeResolution tests type resolution for Option types
func TestSafeNavTypePlugin_OptionTypeResolution(t *testing.T) {
	tests := []struct {
		name           string
		source         string
		expectedType   string
		expectedPtr    bool
		expectedOption bool
		description    string
	}{
		{
			name: "Option type detection",
			source: `package main
type OptionTag uint8
const (
	OptionTagSome OptionTag = iota
	OptionTagNone
)
type Option_User struct {
	tag   OptionTag
	value *User
}
func (o Option_User) IsSome() bool {
	return o.tag == OptionTagSome
}
func (o Option_User) IsNone() bool {
	return o.tag == OptionTagNone
}
type User struct {
	name string
}
func main() {
	var user Option_User
	x := __SAFE_NAV_INFER__(user, "name")
}`,
			expectedType:   "Option_User",
			expectedPtr:    false,
			expectedOption: true,
			description:    "Should resolve Option_User to Option type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse source
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.source, parser.ParseComments)
			if err != nil {
				t.Fatalf("Failed to parse source: %v", err)
			}

			// Type check the file
			conf := types.Config{}
			info := &types.Info{
				Types: make(map[ast.Expr]types.TypeAndValue),
				Defs:  make(map[*ast.Ident]types.Object),
				Uses:  make(map[*ast.Ident]types.Object),
			}
			_, err = conf.Check("test", fset, []*ast.File{file}, info)
			if err != nil {
				t.Logf("Type check warnings (expected): %v", err)
			}

			// Create plugin context with type info
			ctx := &plugin.Context{
				FileSet:  fset,
				TypeInfo: info,
				Logger:   plugin.NewNoOpLogger(),
			}

			// Create plugin
			p := NewSafeNavTypePlugin()
			p.SetContext(ctx)

			// Run discovery phase
			err = p.Process(file)
			if err != nil {
				t.Fatalf("Process failed: %v", err)
			}

			// Run transform phase
			_, err = p.Transform(file)
			if err != nil {
				t.Fatalf("Transform failed: %v", err)
			}

			// Verify resolution
			if len(p.inferNodes) > 0 {
				node := p.inferNodes[0]
				if node.resolvedType != tt.expectedType {
					t.Errorf("%s: expected type %s, got %s",
						tt.description, tt.expectedType, node.resolvedType)
				}
				if node.isPointer != tt.expectedPtr {
					t.Errorf("%s: expected isPointer=%v, got %v",
						tt.description, tt.expectedPtr, node.isPointer)
				}
				if node.isOption != tt.expectedOption {
					t.Errorf("%s: expected isOption=%v, got %v",
						tt.description, tt.expectedOption, node.isOption)
				}
			}
		})
	}
}

// TestSafeNavTypePlugin_ErrorReporting tests error reporting for invalid types
func TestSafeNavTypePlugin_ErrorReporting(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		expectError bool
		errorContains string
		description string
	}{
		{
			name: "non-nullable type error",
			source: `package main
type User struct {
	name string
}
func main() {
	var user User
	x := __SAFE_NAV_INFER__(user, "name")
}`,
			expectError:   true,
			errorContains: "nullable type",
			description:   "Should report error for non-nullable type",
		},
		{
			name: "built-in type error",
			source: `package main
func main() {
	var count int
	x := __SAFE_NAV_INFER__(count, "value")
}`,
			expectError:   true,
			errorContains: "nullable type",
			description:   "Should report error for built-in type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse source
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.source, parser.ParseComments)
			if err != nil {
				t.Fatalf("Failed to parse source: %v", err)
			}

			// Type check the file
			conf := types.Config{}
			info := &types.Info{
				Types: make(map[ast.Expr]types.TypeAndValue),
				Defs:  make(map[*ast.Ident]types.Object),
				Uses:  make(map[*ast.Ident]types.Object),
			}
			_, err = conf.Check("test", fset, []*ast.File{file}, info)
			if err != nil {
				t.Logf("Type check warnings (expected): %v", err)
			}

			// Create plugin context with type info
			ctx := &plugin.Context{
				FileSet:  fset,
				TypeInfo: info,
				Logger:   plugin.NewNoOpLogger(),
			}

			// Create plugin
			p := NewSafeNavTypePlugin()
			p.SetContext(ctx)

			// Run discovery phase
			err = p.Process(file)
			if err != nil {
				t.Fatalf("Process failed: %v", err)
			}

			// Run transform phase
			_, err = p.Transform(file)

			// Check for errors
			errors := p.GetErrors()
			hasError := len(errors) > 0 || ctx.HasErrors()

			if tt.expectError && !hasError {
				t.Errorf("%s: expected error but got none", tt.description)
			}

			if tt.expectError && hasError {
				// Verify error message contains expected substring
				foundExpected := false
				// Check plugin errors ([]string)
				for _, e := range errors {
					if containsSubstring(e, tt.errorContains) {
						foundExpected = true
						break
					}
				}
				// Check context errors ([]error)
				if !foundExpected {
					for _, e := range ctx.GetErrors() {
						if e != nil && containsSubstring(e.Error(), tt.errorContains) {
							foundExpected = true
							break
						}
					}
				}
				if !foundExpected {
					t.Errorf("%s: error message should contain '%s', plugin errors: %v, context errors: %v",
						tt.description, tt.errorContains, errors, ctx.GetErrors())
				}
			}
		})
	}
}

// TestSafeNavTypePlugin_Transform tests AST transformation
func TestSafeNavTypePlugin_Transform(t *testing.T) {
	source := `package main
type User struct {
	name string
}
func main() {
	var user *User
	x := __INFER__.name
}`

	// Parse source
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse source: %v", err)
	}

	// Type check the file
	conf := types.Config{}
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	_, err = conf.Check("test", fset, []*ast.File{file}, info)
	if err != nil {
		t.Logf("Type check warnings (expected): %v", err)
	}

	// Create plugin context
	ctx := &plugin.Context{
		FileSet:  fset,
		TypeInfo: info,
		Logger:   plugin.NewNoOpLogger(),
	}

	// Create plugin
	p := NewSafeNavTypePlugin()
	p.SetContext(ctx)

	// Run discovery
	err = p.Process(file)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Verify we found __INFER__
	if len(p.inferNodes) == 0 {
		t.Fatal("Expected to find __INFER__ node")
	}

	// Run transform
	transformed, err := p.Transform(file)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	// Verify transformation occurred
	if transformed == nil {
		t.Fatal("Transform returned nil")
	}

	// The __INFER__ should have been replaced
	// Walk the transformed AST to verify no __INFER__ remains
	foundInfer := false
	ast.Inspect(transformed, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			if ident.Name == "__INFER__" {
				foundInfer = true
			}
		}
		return true
	})

	if foundInfer {
		t.Error("Transform should have replaced __INFER__ but it still exists")
	}
}

// TestIsOptionType_Comprehensive tests the enhanced isOptionType function
func TestIsOptionType_Comprehensive(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		typeName   string
		shouldPass bool
		description string
	}{
		{
			name: "Valid Option type with OptionTag",
			source: `package main
type OptionTag uint8
type Option_User struct {
	tag OptionTag
	value *User
}
func (o Option_User) Unwrap() User {
	return *o.value
}
type User struct{}`,
			typeName:   "Option_User",
			shouldPass: true,
			description: "Should detect valid Option<T> type",
		},
		{
			name: "Result type with ResultTag should fail",
			source: `package main
type ResultTag uint8
type Result_User_Error struct {
	tag ResultTag
	value *User
	err *Error
}
func (r Result_User_Error) Unwrap() User {
	return *r.value
}
type User struct{}
type Error struct{}`,
			typeName:   "Result_User_Error",
			shouldPass: false,
			description: "Should NOT detect Result<T,E> as Option<T>",
		},
		{
			name: "Struct with tag but no Unwrap",
			source: `package main
type OptionTag uint8
type Option_User struct {
	tag OptionTag
	value *User
}
type User struct{}`,
			typeName:   "Option_User",
			shouldPass: false,
			description: "Should require Unwrap() method",
		},
		{
			name: "Struct with Unwrap but no tag",
			source: `package main
type Option_User struct {
	value *User
}
func (o Option_User) Unwrap() User {
	return *o.value
}
type User struct{}`,
			typeName:   "Option_User",
			shouldPass: false,
			description: "Should require tag field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse and type-check source
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.source, parser.ParseComments)
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			conf := types.Config{}
			info := &types.Info{
				Types: make(map[ast.Expr]types.TypeAndValue),
				Defs:  make(map[*ast.Ident]types.Object),
				Uses:  make(map[*ast.Ident]types.Object),
			}
			pkg, err := conf.Check("test", fset, []*ast.File{file}, info)
			if err != nil {
				t.Logf("Type check warnings: %v", err)
			}

			// Find the type in the package
			obj := pkg.Scope().Lookup(tt.typeName)
			if obj == nil {
				t.Fatalf("Type %s not found in package", tt.typeName)
			}

			named, ok := obj.Type().(*types.Named)
			if !ok {
				t.Fatalf("Type %s is not a named type", tt.typeName)
			}

			// Create plugin and test isOptionType
			p := NewSafeNavTypePlugin()
			result := p.isOptionType(named)

			if result != tt.shouldPass {
				t.Errorf("%s: expected %v, got %v", tt.description, tt.shouldPass, result)
			}
		})
	}
}

// TestSafeNavTypePlugin_PlaceholderReplacement tests complete placeholder replacement
func TestSafeNavTypePlugin_PlaceholderReplacement(t *testing.T) {
	source := `package main
type OptionTag uint8
const (
	OptionTagSome OptionTag = iota
	OptionTagNone
)
type Option_User struct {
	tag OptionTag
	value *User
}
func (o Option_User) IsSome() bool { return o.tag == OptionTagSome }
func (o Option_User) IsNone() bool { return o.tag == OptionTagNone }
func (o Option_User) Unwrap() User { return *o.value }
func Option_User_None() Option_User { return Option_User{tag: OptionTagNone} }
func Option_User_Some(v User) Option_User { return Option_User{tag: OptionTagSome, value: &v} }

type User struct { name string }

func main() {
	var user Option_User
	result := func() __INFER__ {
		if user.IsNone() {
			return __INFER___None()
		}
		return __INFER___Some(user.Unwrap())
	}()
	_ = result
}`

	// Parse source
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Type check
	conf := types.Config{}
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	_, err = conf.Check("test", fset, []*ast.File{file}, info)
	if err != nil {
		t.Logf("Type check warnings: %v", err)
	}

	// Create plugin with test logger that prints to test output
	testLogger := &testLogger{t: t}
	ctx := &plugin.Context{
		FileSet:  fset,
		TypeInfo: info,
		Logger:   testLogger,
	}
	// Build parent map for context traversal
	ctx.BuildParentMap(file)

	p := NewSafeNavTypePlugin()
	p.SetContext(ctx)

	// Process and transform
	err = p.Process(file)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	transformed, err := p.Transform(file)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	// Verify no __INFER__ placeholders remain
	foundInfer := false
	foundInferNone := false
	foundInferSome := false
	foundOptionType := false

	ast.Inspect(transformed, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			if ident.Name == "__INFER__" {
				foundInfer = true
			}
			if ident.Name == "Option_User" {
				foundOptionType = true
			}
		}
		if call, ok := n.(*ast.CallExpr); ok {
			if fun, ok := call.Fun.(*ast.Ident); ok {
				if fun.Name == "__INFER___None" {
					foundInferNone = true
				}
				if fun.Name == "__INFER___Some" {
					foundInferSome = true
				}
			}
		}
		return true
	})

	if foundInfer {
		t.Error("Found unreplaced __INFER__ identifier")
	}
	if foundInferNone {
		t.Error("Found unreplaced __INFER___None() call")
	}
	if foundInferSome {
		t.Error("Found unreplaced __INFER___Some() call")
	}
	if !foundOptionType {
		t.Error("Expected to find Option_User type in transformed AST")
	}
}

// TestSafeNavTypePlugin_OptionVsResult tests differentiation between Option and Result types
func TestSafeNavTypePlugin_OptionVsResult(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		expectOption bool
	}{
		{
			name: "Option type with OptionTag",
			source: `package main
type OptionTag uint8
type Option_User struct {
	tag OptionTag
	value *User
}
func (o Option_User) Unwrap() User { return *o.value }
type User struct{}`,
			expectOption: true,
		},
		{
			name: "Result type with ResultTag",
			source: `package main
type ResultTag uint8
type Result_User_Error struct {
	tag ResultTag
	value *User
	err *Error
}
func (r Result_User_Error) Unwrap() User { return *r.value }
type User struct{}
type Error struct{}`,
			expectOption: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.source, parser.ParseComments)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			conf := types.Config{}
			info := &types.Info{
				Types: make(map[ast.Expr]types.TypeAndValue),
				Defs:  make(map[*ast.Ident]types.Object),
			}
			pkg, err := conf.Check("test", fset, []*ast.File{file}, info)
			if err != nil {
				t.Logf("Type check warnings: %v", err)
			}

			// Find the struct type
			var typeName string
			if tt.expectOption {
				typeName = "Option_User"
			} else {
				typeName = "Result_User_Error"
			}

			obj := pkg.Scope().Lookup(typeName)
			if obj == nil {
				t.Fatalf("Type %s not found", typeName)
			}

			named, ok := obj.Type().(*types.Named)
			if !ok {
				t.Fatalf("Not a named type")
			}

			p := NewSafeNavTypePlugin()
			result := p.isOptionType(named)

			if result != tt.expectOption {
				t.Errorf("Expected isOptionType=%v, got %v for %s", tt.expectOption, result, typeName)
			}
		})
	}
}

// TestSafeNavTypePlugin_TypeInfoFallback tests behavior when TypeInfo is not available
func TestSafeNavTypePlugin_TypeInfoFallback(t *testing.T) {
	source := `package main
func main() {
	x := __INFER__.field
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Create context WITHOUT TypeInfo
	ctx := &plugin.Context{
		FileSet:  fset,
		TypeInfo: nil, // No type info
		Logger:   plugin.NewNoOpLogger(),
	}

	p := NewSafeNavTypePlugin()
	p.SetContext(ctx)

	// Should create type inference service even without TypeInfo
	if p.typeInference == nil {
		t.Error("Expected type inference service to be created even without TypeInfo")
	}

	// Process should still work
	err = p.Process(file)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Should find __INFER__ placeholder
	if len(p.inferNodes) != 1 {
		t.Errorf("Expected 1 __INFER__ node, got %d", len(p.inferNodes))
	}
}

// Helper function to check if error message contains substring
func containsSubstring(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) &&
		(s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
