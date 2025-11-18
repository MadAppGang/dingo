package builtin

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/MadAppGang/dingo/pkg/plugin"
)

// TestFindFunctionReturnType tests return type inference from function declarations
func TestFindFunctionReturnType(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string // Expected type name
		wantErr  bool
	}{
		{
			name: "simple_int_return",
			code: `package test
func f() int {
	return 42
}`,
			expected: "int",
			wantErr:  false,
		},
		{
			name: "option_type_return",
			code: `package test
type Option_int struct { value *int }
func f() Option_int {
	return Option_int{}
}`,
			expected: "test.Option_int",
			wantErr:  false,
		},
		{
			name: "result_type_return",
			code: `package test
type Result_int_error struct { tag int }
func f() Result_int_error {
	return Result_int_error{}
}`,
			expected: "test.Result_int_error",
			wantErr:  false,
		},
		{
			name: "lambda_return",
			code: `package test
func main() {
	f := func() string { return "hello" }
	_ = f
}`,
			expected: "string",
			wantErr:  false,
		},
		{
			name: "no_return_type",
			code: `package test
func f() {
	return
}`,
			expected: "",
			wantErr:  true, // No return type
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse code
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.code, parser.AllErrors)
			if err != nil {
				t.Fatalf("failed to parse code: %v", err)
			}

			// Type check
			conf := types.Config{Importer: nil}
			info := &types.Info{
				Types: make(map[ast.Expr]types.TypeAndValue),
			}
			_, err = conf.Check("test", fset, []*ast.File{file}, info)
			if err != nil {
				// Type errors are ok for test setup
				t.Logf("type check errors (ok for test): %v", err)
			}

			// Create type inference service
			logger := plugin.NewNoOpLogger()
			service, err := NewTypeInferenceService(fset, file, logger)
			if err != nil {
				t.Fatalf("failed to create service: %v", err)
			}
			service.SetTypesInfo(info)

			// Build parent map
			parentMap := buildParentMap(file)
			service.SetParentMap(parentMap)

			// Find return statement
			var retStmt *ast.ReturnStmt
			ast.Inspect(file, func(n ast.Node) bool {
				if ret, ok := n.(*ast.ReturnStmt); ok {
					retStmt = ret
					return false
				}
				return true
			})

			if retStmt == nil {
				t.Fatal("no return statement found in code")
			}

			// Test findFunctionReturnType
			resultType := service.findFunctionReturnType(retStmt)

			if tt.wantErr {
				if resultType != nil {
					t.Errorf("expected nil result, got %v", resultType)
				}
				return
			}

			if resultType == nil {
				t.Fatal("expected non-nil result type")
			}

			gotType := resultType.String()
			if gotType != tt.expected {
				t.Errorf("expected type %q, got %q", tt.expected, gotType)
			}
		})
	}
}

// TestFindAssignmentType tests type inference from assignment statements
func TestFindAssignmentType(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
		wantErr  bool
	}{
		{
			name: "simple_assignment",
			code: `package test
func main() {
	var x int
	x = 42
}`,
			expected: "int",
			wantErr:  false,
		},
		{
			name: "parallel_assignment",
			code: `package test
func main() {
	var x, y int
	x, y = 1, 2
}`,
			expected: "int",
			wantErr:  false,
		},
		{
			name: "option_type_assignment",
			code: `package test
type Option_string struct { value *string }
func main() {
	var opt Option_string
	opt = Option_string{}
}`,
			expected: "test.Option_string",
			wantErr:  false,
		},
		{
			name: "result_type_assignment",
			code: `package test
type Result_int_error struct { tag int }
func main() {
	var result Result_int_error
	result = Result_int_error{}
}`,
			expected: "test.Result_int_error",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse code
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.code, parser.AllErrors)
			if err != nil {
				t.Fatalf("failed to parse code: %v", err)
			}

			// Type check
			conf := types.Config{Importer: nil}
			info := &types.Info{
				Types: make(map[ast.Expr]types.TypeAndValue),
			}
			_, err = conf.Check("test", fset, []*ast.File{file}, info)
			if err != nil {
				t.Logf("type check errors (ok for test): %v", err)
			}

			// Create type inference service
			logger := plugin.NewNoOpLogger()
			service, err := NewTypeInferenceService(fset, file, logger)
			if err != nil {
				t.Fatalf("failed to create service: %v", err)
			}
			service.SetTypesInfo(info)

			// Build parent map
			parentMap := buildParentMap(file)
			service.SetParentMap(parentMap)

			// Find assignment statement and RHS node
			var assignStmt *ast.AssignStmt
			var targetNode ast.Node
			ast.Inspect(file, func(n ast.Node) bool {
				if assign, ok := n.(*ast.AssignStmt); ok {
					assignStmt = assign
					if len(assign.Rhs) > 0 {
						targetNode = assign.Rhs[0] // Test first RHS
					}
					return false
				}
				return true
			})

			if assignStmt == nil {
				t.Fatal("no assignment statement found")
			}
			if targetNode == nil {
				t.Fatal("no RHS node found")
			}

			// Test findAssignmentType
			resultType := service.findAssignmentType(assignStmt, targetNode)

			if tt.wantErr {
				if resultType != nil {
					t.Errorf("expected nil result, got %v", resultType)
				}
				return
			}

			if resultType == nil {
				t.Fatal("expected non-nil result type")
			}

			gotType := resultType.String()
			if gotType != tt.expected {
				t.Errorf("expected type %q, got %q", tt.expected, gotType)
			}
		})
	}
}

// TestFindVarDeclType tests type inference from variable declarations
func TestFindVarDeclType(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
		wantErr  bool
	}{
		{
			name: "explicit_type",
			code: `package test
func main() {
	var x int = 42
}`,
			expected: "int",
			wantErr:  false,
		},
		{
			name: "option_type_explicit",
			code: `package test
type Option_int struct { value *int }
func main() {
	var opt Option_int = Option_int{}
}`,
			expected: "test.Option_int",
			wantErr:  false,
		},
		{
			name: "result_type_explicit",
			code: `package test
type Result_string_error struct { tag int }
func main() {
	var result Result_string_error = Result_string_error{}
}`,
			expected: "test.Result_string_error",
			wantErr:  false,
		},
		{
			name: "multi_var_explicit",
			code: `package test
func main() {
	var x, y int = 1, 2
}`,
			expected: "int",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse code
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.code, parser.AllErrors)
			if err != nil {
				t.Fatalf("failed to parse code: %v", err)
			}

			// Type check
			conf := types.Config{Importer: nil}
			info := &types.Info{
				Types: make(map[ast.Expr]types.TypeAndValue),
				Defs:  make(map[*ast.Ident]types.Object),
				Uses:  make(map[*ast.Ident]types.Object),
			}
			_, err = conf.Check("test", fset, []*ast.File{file}, info)
			if err != nil {
				t.Logf("type check errors (ok for test): %v", err)
			}

			// Create type inference service
			logger := plugin.NewNoOpLogger()
			service, err := NewTypeInferenceService(fset, file, logger)
			if err != nil {
				t.Fatalf("failed to create service: %v", err)
			}
			service.SetTypesInfo(info)

			// Build parent map
			parentMap := buildParentMap(file)
			service.SetParentMap(parentMap)

			// Find var declaration and value node
			var genDecl *ast.GenDecl
			var targetNode ast.Node
			ast.Inspect(file, func(n ast.Node) bool {
				if decl, ok := n.(*ast.GenDecl); ok && decl.Tok == token.VAR {
					genDecl = decl
					for _, spec := range decl.Specs {
						if vs, ok := spec.(*ast.ValueSpec); ok && len(vs.Values) > 0 {
							targetNode = vs.Values[0] // Test first value
							return false
						}
					}
				}
				return true
			})

			if genDecl == nil {
				t.Fatal("no var declaration found")
			}
			if targetNode == nil {
				t.Fatal("no value node found")
			}

			// Test findVarDeclType
			resultType := service.findVarDeclType(genDecl, targetNode)

			if tt.wantErr {
				if resultType != nil {
					t.Errorf("expected nil result, got %v", resultType)
				}
				return
			}

			if resultType == nil {
				t.Fatal("expected non-nil result type")
			}

			gotType := resultType.String()
			if gotType != tt.expected {
				t.Errorf("expected type %q, got %q", tt.expected, gotType)
			}
		})
	}
}

// TestFindCallArgType tests type inference from function call arguments
func TestFindCallArgType(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
		wantErr  bool
	}{
		{
			name: "regular_call",
			code: `package test
func process(x int) {}
func main() {
	process(42)
}`,
			expected: "int",
			wantErr:  false,
		},
		{
			name: "option_type_param",
			code: `package test
type Option_int struct { value *int }
func process(opt Option_int) {}
func main() {
	process(Option_int{})
}`,
			expected: "test.Option_int",
			wantErr:  false,
		},
		{
			name: "result_type_param",
			code: `package test
type Result_string_error struct { tag int }
func handle(result Result_string_error) {}
func main() {
	handle(Result_string_error{})
}`,
			expected: "test.Result_string_error",
			wantErr:  false,
		},
		{
			name: "multiple_params",
			code: `package test
func process(x int, y string) {}
func main() {
	process(42, "hello")
}`,
			expected: "int", // First arg
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse code
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.code, parser.AllErrors)
			if err != nil {
				t.Fatalf("failed to parse code: %v", err)
			}

			// Type check
			conf := types.Config{Importer: nil}
			info := &types.Info{
				Types: make(map[ast.Expr]types.TypeAndValue),
			}
			_, err = conf.Check("test", fset, []*ast.File{file}, info)
			if err != nil {
				t.Logf("type check errors (ok for test): %v", err)
			}

			// Create type inference service
			logger := plugin.NewNoOpLogger()
			service, err := NewTypeInferenceService(fset, file, logger)
			if err != nil {
				t.Fatalf("failed to create service: %v", err)
			}
			service.SetTypesInfo(info)

			// Build parent map
			parentMap := buildParentMap(file)
			service.SetParentMap(parentMap)

			// Find call expression and first argument
			var callExpr *ast.CallExpr
			var targetNode ast.Node
			ast.Inspect(file, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					// Skip the function declaration, find the call in main
					if len(call.Args) > 0 {
						callExpr = call
						targetNode = call.Args[0] // Test first argument
						return false
					}
				}
				return true
			})

			if callExpr == nil {
				t.Fatal("no call expression found")
			}
			if targetNode == nil {
				t.Fatal("no argument node found")
			}

			// Test findCallArgType
			resultType := service.findCallArgType(callExpr, targetNode)

			if tt.wantErr {
				if resultType != nil {
					t.Errorf("expected nil result, got %v", resultType)
				}
				return
			}

			if resultType == nil {
				t.Fatal("expected non-nil result type")
			}

			gotType := resultType.String()
			if gotType != tt.expected {
				t.Errorf("expected type %q, got %q", tt.expected, gotType)
			}
		})
	}
}

// TestContainsNode tests the containsNode helper function
func TestContainsNode(t *testing.T) {
	code := `package test
func main() {
	x := 42 + 10
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.AllErrors)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	logger := plugin.NewNoOpLogger()
	service, err := NewTypeInferenceService(fset, file, logger)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// Find the binary expression (42 + 10) and the literal (42)
	var binaryExpr *ast.BinaryExpr
	var literal *ast.BasicLit
	ast.Inspect(file, func(n ast.Node) bool {
		if be, ok := n.(*ast.BinaryExpr); ok {
			binaryExpr = be
		}
		if lit, ok := n.(*ast.BasicLit); ok && lit.Value == "42" {
			literal = lit
		}
		return true
	})

	if binaryExpr == nil || literal == nil {
		t.Fatal("failed to find test nodes")
	}

	// Test: literal is contained in binary expression
	if !service.containsNode(binaryExpr, literal) {
		t.Error("expected containsNode to return true for literal in binary expr")
	}

	// Test: binary expression is not contained in literal
	if service.containsNode(literal, binaryExpr) {
		t.Error("expected containsNode to return false for binary expr in literal")
	}

	// Test: node contains itself
	if !service.containsNode(literal, literal) {
		t.Error("expected containsNode to return true for node containing itself")
	}

	// Test: nil handling
	if service.containsNode(nil, literal) {
		t.Error("expected containsNode to return false for nil root")
	}
	if service.containsNode(literal, nil) {
		t.Error("expected containsNode to return false for nil target")
	}
}

// TestStrictGoTypesRequirement tests that helpers return nil when go/types.Info is unavailable
func TestStrictGoTypesRequirement(t *testing.T) {
	code := `package test
func f() int {
	var x int = 42
	x = 10
	process(20)
	return x
}
func process(x int) {}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.AllErrors)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	logger := plugin.NewNoOpLogger()
	service, err := NewTypeInferenceService(fset, file, logger)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// DO NOT set typesInfo - test strict requirement
	parentMap := buildParentMap(file)
	service.SetParentMap(parentMap)

	// Find nodes for each helper
	var retStmt *ast.ReturnStmt
	var assignStmt *ast.AssignStmt
	var genDecl *ast.GenDecl
	var callExpr *ast.CallExpr

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ReturnStmt:
			retStmt = node
		case *ast.AssignStmt:
			assignStmt = node
		case *ast.GenDecl:
			if node.Tok == token.VAR {
				genDecl = node
			}
		case *ast.CallExpr:
			if callExpr == nil { // Get first call
				callExpr = node
			}
		}
		return true
	})

	// Test 1: findFunctionReturnType returns nil without go/types
	if result := service.findFunctionReturnType(retStmt); result != nil {
		t.Errorf("findFunctionReturnType should return nil without go/types, got %v", result)
	}

	// Test 2: findAssignmentType returns nil without go/types
	if assignStmt != nil && len(assignStmt.Rhs) > 0 {
		if result := service.findAssignmentType(assignStmt, assignStmt.Rhs[0]); result != nil {
			t.Errorf("findAssignmentType should return nil without go/types, got %v", result)
		}
	}

	// Test 3: findVarDeclType returns nil without go/types
	if genDecl != nil {
		for _, spec := range genDecl.Specs {
			if vs, ok := spec.(*ast.ValueSpec); ok && len(vs.Values) > 0 {
				if result := service.findVarDeclType(genDecl, vs.Values[0]); result != nil {
					t.Errorf("findVarDeclType should return nil without go/types, got %v", result)
				}
				break
			}
		}
	}

	// Test 4: findCallArgType returns nil without go/types
	if callExpr != nil && len(callExpr.Args) > 0 {
		if result := service.findCallArgType(callExpr, callExpr.Args[0]); result != nil {
			t.Errorf("findCallArgType should return nil without go/types, got %v", result)
		}
	}
}

// buildParentMap builds a map from AST nodes to their parents
func buildParentMap(file *ast.File) map[ast.Node]ast.Node {
	parentMap := make(map[ast.Node]ast.Node)
	var stack []ast.Node

	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			return false
		}

		if len(stack) > 0 {
			parentMap[n] = stack[len(stack)-1]
		}
		stack = append(stack, n)
		return true
	})

	return parentMap
}

// TestInferTypeFromContextIntegration tests end-to-end context inference
func TestInferTypeFromContextIntegration(t *testing.T) {
	code := `package test

type Option_int struct { value *int }

func getAge() Option_int {
	return Option_int{} // Context: return statement
}

func main() {
	var age Option_int
	age = Option_int{} // Context: assignment

	var name Option_int = Option_int{} // Context: var decl

	processAge(Option_int{}) // Context: call arg
}

func processAge(opt Option_int) {}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.AllErrors)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	// Type check
	conf := types.Config{Importer: nil}
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	_, err = conf.Check("test", fset, []*ast.File{file}, info)
	if err != nil {
		t.Logf("type check errors (ok for test): %v", err)
	}

	logger := plugin.NewNoOpLogger()
	service, err := NewTypeInferenceService(fset, file, logger)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	service.SetTypesInfo(info)

	parentMap := buildParentMap(file)
	service.SetParentMap(parentMap)

	// Collect all composite literals (should be in different contexts)
	var compositeLits []*ast.CompositeLit
	ast.Inspect(file, func(n ast.Node) bool {
		if cl, ok := n.(*ast.CompositeLit); ok {
			compositeLits = append(compositeLits, cl)
		}
		return true
	})

	if len(compositeLits) < 4 {
		t.Fatalf("expected at least 4 composite literals, got %d", len(compositeLits))
	}

	// Test InferTypeFromContext for each literal
	for i, lit := range compositeLits {
		resultType, ok := service.InferTypeFromContext(lit)
		if !ok {
			t.Errorf("context %d: failed to infer type", i)
			continue
		}
		if resultType == nil {
			t.Errorf("context %d: got nil type", i)
			continue
		}

		typeName := resultType.String()
		if typeName != "test.Option_int" {
			t.Errorf("context %d: expected Option_int, got %s", i, typeName)
		}
	}
}

// TestVariadicFunctionCallArgType tests findCallArgType with variadic functions
func TestVariadicFunctionCallArgType(t *testing.T) {
	code := `package test

type Option_int struct { value *int }

func process(args ...Option_int) {}

func main() {
	process(Option_int{}, Option_int{})
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.AllErrors)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	// Type check
	conf := types.Config{Importer: nil}
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
	}
	_, err = conf.Check("test", fset, []*ast.File{file}, info)
	if err != nil {
		t.Logf("type check errors (ok for test): %v", err)
	}

	logger := plugin.NewNoOpLogger()
	service, err := NewTypeInferenceService(fset, file, logger)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	service.SetTypesInfo(info)

	parentMap := buildParentMap(file)
	service.SetParentMap(parentMap)

	// Find the call expression
	var callExpr *ast.CallExpr
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && len(call.Args) > 0 {
			callExpr = call
			return false
		}
		return true
	})

	if callExpr == nil {
		t.Fatal("no call expression found")
	}

	// Test both arguments (both should be Option_int, not []Option_int)
	for i, arg := range callExpr.Args {
		resultType := service.findCallArgType(callExpr, arg)
		if resultType == nil {
			t.Fatalf("arg %d: expected non-nil type", i)
		}

		typeName := resultType.String()
		if typeName != "test.Option_int" {
			t.Errorf("arg %d: expected Option_int (variadic element type), got %s", i, typeName)
		}
	}
}
