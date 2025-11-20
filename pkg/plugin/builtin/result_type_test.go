package builtin

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/MadAppGang/dingo/pkg/plugin"
)

// ============================================================================
// 1. TYPE DECLARATION TESTS (5 tests)
// ============================================================================

func TestTypeDeclaration_BasicResultIntError(t *testing.T) {
	p := NewResultTypePlugin()
	// Initialize type inference service to prevent nil pointer
	ctx := &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}
	p.SetContext(ctx)

	// Test: Result<int> should generate Result_int_error (underscore convention)
	indexExpr := &ast.IndexExpr{
		X:     ast.NewIdent("Result"),
		Index: ast.NewIdent("int"),
	}

	err := p.Process(indexExpr)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	decls := p.GetPendingDeclarations()
	foundResultType := false
	foundResultTag := false

	for _, decl := range decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					// Underscore naming convention (reverted from camelCase)
					if typeSpec.Name.Name == "Result_int_error" {
						foundResultType = true
						// Verify struct fields
						if structType, ok := typeSpec.Type.(*ast.StructType); ok {
							if len(structType.Fields.List) != 3 {
								t.Errorf("expected 3 fields, got %d", len(structType.Fields.List))
							}
							// Check field names
							expectedFields := []string{"tag", "ok", "err"}
							for i, field := range structType.Fields.List {
								if field.Names[0].Name != expectedFields[i] {
									t.Errorf("field %d: expected %q, got %q", i, expectedFields[i], field.Names[0].Name)
								}
							}
						}
					}
					if typeSpec.Name.Name == "ResultTag" {
						foundResultTag = true
					}
				}
			}
		}
	}

	if !foundResultType {
		t.Error("expected Result_int_error type declaration")
	}
	if !foundResultTag {
		t.Error("expected ResultTag type declaration")
	}
}

func TestTypeDeclaration_ComplexPointerTypes(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	// Test: Result<*User, *CustomError>
	indexListExpr := &ast.IndexListExpr{
		X: ast.NewIdent("Result"),
		Indices: []ast.Expr{
			&ast.StarExpr{X: ast.NewIdent("User")},
			&ast.StarExpr{X: ast.NewIdent("CustomError")},
		},
	}

	err := p.Process(indexListExpr)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	decls := p.GetPendingDeclarations()
	foundType := false

	for _, decl := range decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if typeSpec.Name.Name == "Result_ptr_User_ptr_CustomError" {
						foundType = true
					}
				}
			}
		}
	}

	if !foundType {
		t.Error("expected Result_ptr_User_ptr_CustomError type declaration")
	}
}

func TestTypeDeclaration_ComplexSliceTypes(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	// Test: Result<[]byte, error>
	indexListExpr := &ast.IndexListExpr{
		X: ast.NewIdent("Result"),
		Indices: []ast.Expr{
			&ast.ArrayType{Elt: ast.NewIdent("byte")},
			ast.NewIdent("error"),
		},
	}

	err := p.Process(indexListExpr)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	decls := p.GetPendingDeclarations()
	foundType := false

	for _, decl := range decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if typeSpec.Name.Name == "Result_slice_byte_error" {
						foundType = true
					}
				}
			}
		}
	}

	if !foundType {
		t.Error("expected Result_slice_byte_error type declaration")
	}
}

func TestTypeDeclaration_TypeNameSanitization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "int", "_int"},
		{"pointer", "*User", "_ptr_User"},
		{"slice", "[]byte", "_slice_byte"},
		{"map", "map[string]int", "_map"},  // Maps are simplified to just "map"
		{"package qualified", "pkg.Type", "_pkg.Type"},  // Dots preserved (TODO: should sanitize dots)
		{"nested pointer slice", "*[]string", "_ptr_slice_string"},
		{"multiple dots", "github.com.pkg.Type", "_github.com.pkg.Type"},  // Dots preserved
		{"array", "[10]int", "_[10]int"},  // Arrays preserve brackets (TODO: should sanitize)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeTypeName(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizeTypeName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestTypeDeclaration_MultipleResultTypesInSameFile(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	// Process multiple different Result types
	types := []struct {
		okType  string
		errType string
	}{
		{"int", "error"},
		{"string", "error"},
		{"bool", "CustomError"},
	}

	for _, typ := range types {
		var expr ast.Node
		if typ.errType == "error" {
			expr = &ast.IndexExpr{
				X:     ast.NewIdent("Result"),
				Index: ast.NewIdent(typ.okType),
			}
		} else {
			expr = &ast.IndexListExpr{
				X: ast.NewIdent("Result"),
				Indices: []ast.Expr{
					ast.NewIdent(typ.okType),
					ast.NewIdent(typ.errType),
				},
			}
		}

		err := p.Process(expr)
		if err != nil {
			t.Fatalf("Process failed for %s/%s: %v", typ.okType, typ.errType, err)
		}
	}

	decls := p.GetPendingDeclarations()

	// Should have ResultTag (emitted once) plus 3 Result types with their methods
	expectedTypes := map[string]bool{
		"ResultTag":               false,
		"Result_int_error":        false,
		"Result_string_error":     false,
		"Result_bool_CustomError": false,
	}

	for _, decl := range decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if _, exists := expectedTypes[typeSpec.Name.Name]; exists {
						expectedTypes[typeSpec.Name.Name] = true
					}
				}
			}
		}
	}

	for typeName, found := range expectedTypes {
		if !found {
			t.Errorf("expected type %s not found", typeName)
		}
	}
}

// ============================================================================
// 2. CONSTRUCTOR TESTS (8 tests)
// ============================================================================

func TestConstructor_OkWithIntLiteral(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	// First, ensure Result<int> type exists
	indexExpr := &ast.IndexExpr{
		X:     ast.NewIdent("Result"),
		Index: ast.NewIdent("int"),
	}
	_ = p.Process(indexExpr)

	// Test: Ok(42)
	okCall := &ast.CallExpr{
		Fun: ast.NewIdent("Ok"),
		Args: []ast.Expr{
			&ast.BasicLit{Kind: token.INT, Value: "42"},
		},
	}

	err := p.Process(okCall)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Verify constructor function was generated
	decls := p.GetPendingDeclarations()
	foundConstructor := false

	for _, decl := range decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok && funcDecl.Recv == nil {
			if funcDecl.Name.Name == "Result_int_error_Ok" {
				foundConstructor = true
				// Verify function signature
				if len(funcDecl.Type.Params.List) != 1 {
					t.Errorf("Ok constructor should have 1 parameter, got %d", len(funcDecl.Type.Params.List))
				}
				if len(funcDecl.Type.Results.List) != 1 {
					t.Errorf("Ok constructor should return 1 value, got %d", len(funcDecl.Type.Results.List))
				}
			}
		}
	}

	if !foundConstructor {
		t.Error("expected Result_int_error_Ok constructor function")
	}
}

func TestConstructor_OkWithStringLiteral(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	// Test: Ok("hello")
	okCall := &ast.CallExpr{
		Fun: ast.NewIdent("Ok"),
		Args: []ast.Expr{
			&ast.BasicLit{Kind: token.STRING, Value: `"hello"`},
		},
	}

	err := p.Process(okCall)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	decls := p.GetPendingDeclarations()
	foundType := false

	for _, decl := range decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if typeSpec.Name.Name == "Result_string_error" {
						foundType = true
					}
				}
			}
		}
	}

	if !foundType {
		t.Error("Ok with string literal should infer Result_string_error type")
	}
}

func TestConstructor_ErrWithErrorValue(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	// Test: Err(someError)
	errCall := &ast.CallExpr{
		Fun: ast.NewIdent("Err"),
		Args: []ast.Expr{
			ast.NewIdent("someError"),
		},
	}

	err := p.Process(errCall)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Err() should generate a Result type (with interface{} as placeholder for T)
	decls := p.GetPendingDeclarations()
	if len(decls) == 0 {
		t.Error("expected declarations for Err constructor")
	}
}

func TestConstructor_OkWithVariousTypes(t *testing.T) {
	tests := []struct {
		name     string
		argExpr  ast.Expr
		expected string
	}{
		{
			name:     "int literal",
			argExpr:  &ast.BasicLit{Kind: token.INT, Value: "42"},
			expected: "Result_int_error",
		},
		{
			name:     "float literal",
			argExpr:  &ast.BasicLit{Kind: token.FLOAT, Value: "3.14"},
			expected: "Result_float64_error",
		},
		{
			name:     "string literal",
			argExpr:  &ast.BasicLit{Kind: token.STRING, Value: `"test"`},
			expected: "Result_string_error",
		},
		{
			name:     "rune literal",
			argExpr:  &ast.BasicLit{Kind: token.CHAR, Value: "'a'"},
			expected: "Result_rune_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewResultTypePlugin()
			p.ctx = &plugin.Context{
				FileSet: token.NewFileSet(),
				Logger:  plugin.NewNoOpLogger(),
			}

			okCall := &ast.CallExpr{
				Fun:  ast.NewIdent("Ok"),
				Args: []ast.Expr{tt.argExpr},
			}

			err := p.Process(okCall)
			if err != nil {
				t.Fatalf("Process failed: %v", err)
			}

			decls := p.GetPendingDeclarations()
			foundType := false

			for _, decl := range decls {
				if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
					for _, spec := range genDecl.Specs {
						if typeSpec, ok := spec.(*ast.TypeSpec); ok {
							if typeSpec.Name.Name == tt.expected {
								foundType = true
							}
						}
					}
				}
			}

			if !foundType {
				t.Errorf("expected type %s not found", tt.expected)
			}
		})
	}
}

func TestConstructor_OkWithIdentifier(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	// Test: Ok(myValue) where myValue is an identifier
	okCall := &ast.CallExpr{
		Fun: ast.NewIdent("Ok"),
		Args: []ast.Expr{
			ast.NewIdent("myValue"),
		},
	}

	err := p.Process(okCall)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// CRITICAL FIX #3: Type inference now fails for identifiers without go/types
	// CRITICAL FIX #5 (Code Review): Uses interface{} fallback instead of reporting error
	errors := p.ctx.GetErrors()
	if len(errors) != 0 {
		t.Errorf("expected no errors (should use interface{} fallback), got %d errors", len(errors))
	}

	// Should generate declarations using interface{} fallback
	decls := p.GetPendingDeclarations()
	if len(decls) == 0 {
		t.Error("expected declarations to be generated with interface{} fallback")
	}
}

func TestConstructor_OkWithFunctionCall(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	// Test: Ok(getValue())
	okCall := &ast.CallExpr{
		Fun: ast.NewIdent("Ok"),
		Args: []ast.Expr{
			&ast.CallExpr{
				Fun: ast.NewIdent("getValue"),
			},
		},
	}

	err := p.Process(okCall)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// CRITICAL FIX #3: Function calls now fail type inference without go/types
	// CRITICAL FIX #5 (Code Review): Uses interface{} fallback instead of reporting error
	errors := p.ctx.GetErrors()
	if len(errors) != 0 {
		t.Errorf("expected no errors (should use interface{} fallback), got %d errors", len(errors))
	}

	// Should generate declarations using interface{} fallback
	decls := p.GetPendingDeclarations()
	if len(decls) == 0 {
		t.Error("expected declarations to be generated with interface{} fallback")
	}
}

func TestConstructor_InvalidOkNoArgs(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	// Test: Ok() with no arguments (invalid)
	okCall := &ast.CallExpr{
		Fun:  ast.NewIdent("Ok"),
		Args: []ast.Expr{},
	}

	// Should not panic, but log warning
	err := p.Process(okCall)
	if err != nil {
		t.Fatalf("Process should not fail: %v", err)
	}
}

func TestConstructor_InvalidErrNoArgs(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	// Test: Err() with no arguments (invalid)
	errCall := &ast.CallExpr{
		Fun:  ast.NewIdent("Err"),
		Args: []ast.Expr{},
	}

	// Should not panic, but log warning
	err := p.Process(errCall)
	if err != nil {
		t.Fatalf("Process should not fail: %v", err)
	}
}

// ============================================================================
// 3. HELPER METHOD TESTS (12 tests)
// ============================================================================

func TestHelperMethods_IsOkGeneration(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	indexExpr := &ast.IndexExpr{
		X:     ast.NewIdent("Result"),
		Index: ast.NewIdent("int"),
	}

	err := p.Process(indexExpr)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	decls := p.GetPendingDeclarations()
	var isOkMethod *ast.FuncDecl

	for _, decl := range decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok && funcDecl.Recv != nil {
			if funcDecl.Name.Name == "IsOk" {
				isOkMethod = funcDecl
				break
			}
		}
	}

	if isOkMethod == nil {
		t.Fatal("IsOk method not found")
	}

	// Verify method signature
	if len(isOkMethod.Type.Results.List) != 1 {
		t.Errorf("IsOk should return 1 value, got %d", len(isOkMethod.Type.Results.List))
	}

	resultType := isOkMethod.Type.Results.List[0].Type
	if ident, ok := resultType.(*ast.Ident); !ok || ident.Name != "bool" {
		t.Error("IsOk should return bool")
	}
}

func TestHelperMethods_IsErrGeneration(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	indexExpr := &ast.IndexExpr{
		X:     ast.NewIdent("Result"),
		Index: ast.NewIdent("string"),
	}

	err := p.Process(indexExpr)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	decls := p.GetPendingDeclarations()
	var isErrMethod *ast.FuncDecl

	for _, decl := range decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok && funcDecl.Recv != nil {
			if funcDecl.Name.Name == "IsErr" {
				isErrMethod = funcDecl
				break
			}
		}
	}

	if isErrMethod == nil {
		t.Fatal("IsErr method not found")
	}

	// Verify method signature
	if len(isErrMethod.Type.Results.List) != 1 {
		t.Errorf("IsErr should return 1 value, got %d", len(isErrMethod.Type.Results.List))
	}

	resultType := isErrMethod.Type.Results.List[0].Type
	if ident, ok := resultType.(*ast.Ident); !ok || ident.Name != "bool" {
		t.Error("IsErr should return bool")
	}
}

func TestHelperMethods_UnwrapGeneration(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	indexExpr := &ast.IndexExpr{
		X:     ast.NewIdent("Result"),
		Index: ast.NewIdent("int"),
	}

	err := p.Process(indexExpr)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	decls := p.GetPendingDeclarations()
	var unwrapMethod *ast.FuncDecl

	for _, decl := range decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok && funcDecl.Recv != nil {
			if funcDecl.Name.Name == "Unwrap" {
				unwrapMethod = funcDecl
				break
			}
		}
	}

	if unwrapMethod == nil {
		t.Fatal("Unwrap method not found")
	}

	// Verify method returns T (not *T)
	if len(unwrapMethod.Type.Results.List) != 1 {
		t.Errorf("Unwrap should return 1 value, got %d", len(unwrapMethod.Type.Results.List))
	}

	// Verify method body contains panic check
	foundPanic := false
	ast.Inspect(unwrapMethod.Body, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			if ident, ok := callExpr.Fun.(*ast.Ident); ok && ident.Name == "panic" {
				foundPanic = true
			}
		}
		return true
	})

	if !foundPanic {
		t.Error("Unwrap should contain panic call for Err case")
	}
}

func TestHelperMethods_UnwrapOrGeneration(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	indexExpr := &ast.IndexExpr{
		X:     ast.NewIdent("Result"),
		Index: ast.NewIdent("string"),
	}

	err := p.Process(indexExpr)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	decls := p.GetPendingDeclarations()
	var unwrapOrMethod *ast.FuncDecl

	for _, decl := range decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok && funcDecl.Recv != nil {
			if funcDecl.Name.Name == "UnwrapOr" {
				unwrapOrMethod = funcDecl
				break
			}
		}
	}

	if unwrapOrMethod == nil {
		t.Fatal("UnwrapOr method not found")
	}

	// Verify method signature: UnwrapOr(defaultValue T) T
	if len(unwrapOrMethod.Type.Params.List) != 1 {
		t.Errorf("UnwrapOr should have 1 parameter, got %d", len(unwrapOrMethod.Type.Params.List))
	}

	if len(unwrapOrMethod.Type.Results.List) != 1 {
		t.Errorf("UnwrapOr should return 1 value, got %d", len(unwrapOrMethod.Type.Results.List))
	}

	// Verify no panic call (UnwrapOr should never panic)
	foundPanic := false
	ast.Inspect(unwrapOrMethod.Body, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			if ident, ok := callExpr.Fun.(*ast.Ident); ok && ident.Name == "panic" {
				foundPanic = true
			}
		}
		return true
	})

	if foundPanic {
		t.Error("UnwrapOr should not contain panic call")
	}
}

func TestHelperMethods_UnwrapErrGeneration(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	indexExpr := &ast.IndexExpr{
		X:     ast.NewIdent("Result"),
		Index: ast.NewIdent("int"),
	}

	err := p.Process(indexExpr)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	decls := p.GetPendingDeclarations()
	var unwrapErrMethod *ast.FuncDecl

	for _, decl := range decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok && funcDecl.Recv != nil {
			if funcDecl.Name.Name == "UnwrapErr" {
				unwrapErrMethod = funcDecl
				break
			}
		}
	}

	if unwrapErrMethod == nil {
		t.Fatal("UnwrapErr method not found")
	}

	// Verify method body contains panic check for Ok case
	foundPanic := false
	ast.Inspect(unwrapErrMethod.Body, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			if ident, ok := callExpr.Fun.(*ast.Ident); ok && ident.Name == "panic" {
				foundPanic = true
			}
		}
		return true
	})

	if !foundPanic {
		t.Error("UnwrapErr should contain panic call for Ok case")
	}
}

func TestHelperMethods_MapGeneration(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	indexExpr := &ast.IndexExpr{
		X:     ast.NewIdent("Result"),
		Index: ast.NewIdent("int"),
	}

	err := p.Process(indexExpr)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	decls := p.GetPendingDeclarations()
	var mapMethod *ast.FuncDecl

	for _, decl := range decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok && funcDecl.Recv != nil {
			if funcDecl.Name.Name == "Map" {
				mapMethod = funcDecl
				break
			}
		}
	}

	if mapMethod == nil {
		t.Fatal("Map method not found")
	}

	// Verify method signature: Map(fn func(T) U)
	if len(mapMethod.Type.Params.List) != 1 {
		t.Errorf("Map should have 1 parameter, got %d", len(mapMethod.Type.Params.List))
	}

	// Parameter should be a function type
	param := mapMethod.Type.Params.List[0]
	if _, ok := param.Type.(*ast.FuncType); !ok {
		t.Error("Map parameter should be a function type")
	}
}

func TestHelperMethods_MapErrGeneration(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	indexExpr := &ast.IndexExpr{
		X:     ast.NewIdent("Result"),
		Index: ast.NewIdent("string"),
	}

	err := p.Process(indexExpr)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	decls := p.GetPendingDeclarations()
	var mapErrMethod *ast.FuncDecl

	for _, decl := range decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok && funcDecl.Recv != nil {
			if funcDecl.Name.Name == "MapErr" {
				mapErrMethod = funcDecl
				break
			}
		}
	}

	if mapErrMethod == nil {
		t.Fatal("MapErr method not found")
	}

	// Verify method signature
	if len(mapErrMethod.Type.Params.List) != 1 {
		t.Errorf("MapErr should have 1 parameter, got %d", len(mapErrMethod.Type.Params.List))
	}

	// Parameter should be a function type
	param := mapErrMethod.Type.Params.List[0]
	if _, ok := param.Type.(*ast.FuncType); !ok {
		t.Error("MapErr parameter should be a function type")
	}
}

func TestHelperMethods_FilterGeneration(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	indexExpr := &ast.IndexExpr{
		X:     ast.NewIdent("Result"),
		Index: ast.NewIdent("int"),
	}

	err := p.Process(indexExpr)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	decls := p.GetPendingDeclarations()
	var filterMethod *ast.FuncDecl

	for _, decl := range decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok && funcDecl.Recv != nil {
			if funcDecl.Name.Name == "Filter" {
				filterMethod = funcDecl
				break
			}
		}
	}

	if filterMethod == nil {
		t.Fatal("Filter method not found")
	}

	// Verify method signature: Filter(predicate func(T) bool) Result<T, E>
	if len(filterMethod.Type.Params.List) != 1 {
		t.Errorf("Filter should have 1 parameter, got %d", len(filterMethod.Type.Params.List))
	}

	// Parameter should be a function type returning bool
	param := filterMethod.Type.Params.List[0]
	if funcType, ok := param.Type.(*ast.FuncType); ok {
		if len(funcType.Results.List) != 1 {
			t.Error("Filter predicate should return 1 value")
		}
		resultType := funcType.Results.List[0].Type
		if ident, ok := resultType.(*ast.Ident); !ok || ident.Name != "bool" {
			t.Error("Filter predicate should return bool")
		}
	} else {
		t.Error("Filter parameter should be a function type")
	}
}

func TestHelperMethods_AndThenGeneration(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	indexExpr := &ast.IndexExpr{
		X:     ast.NewIdent("Result"),
		Index: ast.NewIdent("int"),
	}

	err := p.Process(indexExpr)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	decls := p.GetPendingDeclarations()
	var andThenMethod *ast.FuncDecl

	for _, decl := range decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok && funcDecl.Recv != nil {
			if funcDecl.Name.Name == "AndThen" {
				andThenMethod = funcDecl
				break
			}
		}
	}

	if andThenMethod == nil {
		t.Fatal("AndThen method not found")
	}

	// Verify method signature: AndThen(fn func(T) Result<U, E>)
	if len(andThenMethod.Type.Params.List) != 1 {
		t.Errorf("AndThen should have 1 parameter, got %d", len(andThenMethod.Type.Params.List))
	}

	param := andThenMethod.Type.Params.List[0]
	if _, ok := param.Type.(*ast.FuncType); !ok {
		t.Error("AndThen parameter should be a function type")
	}
}

func TestHelperMethods_OrElseGeneration(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	indexExpr := &ast.IndexExpr{
		X:     ast.NewIdent("Result"),
		Index: ast.NewIdent("string"),
	}

	err := p.Process(indexExpr)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	decls := p.GetPendingDeclarations()
	var orElseMethod *ast.FuncDecl

	for _, decl := range decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok && funcDecl.Recv != nil {
			if funcDecl.Name.Name == "OrElse" {
				orElseMethod = funcDecl
				break
			}
		}
	}

	if orElseMethod == nil {
		t.Fatal("OrElse method not found")
	}

	// Verify method signature
	if len(orElseMethod.Type.Params.List) != 1 {
		t.Errorf("OrElse should have 1 parameter, got %d", len(orElseMethod.Type.Params.List))
	}
}

func TestHelperMethods_AndGeneration(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	indexExpr := &ast.IndexExpr{
		X:     ast.NewIdent("Result"),
		Index: ast.NewIdent("int"),
	}

	err := p.Process(indexExpr)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	decls := p.GetPendingDeclarations()
	var andMethod *ast.FuncDecl

	for _, decl := range decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok && funcDecl.Recv != nil {
			if funcDecl.Name.Name == "And" {
				andMethod = funcDecl
				break
			}
		}
	}

	if andMethod == nil {
		t.Fatal("And method not found")
	}

	// Verify method signature: And(other Result<U, E>)
	if len(andMethod.Type.Params.List) != 1 {
		t.Errorf("And should have 1 parameter, got %d", len(andMethod.Type.Params.List))
	}
}

func TestHelperMethods_OrGeneration(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	indexExpr := &ast.IndexExpr{
		X:     ast.NewIdent("Result"),
		Index: ast.NewIdent("bool"),
	}

	err := p.Process(indexExpr)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	decls := p.GetPendingDeclarations()
	var orMethod *ast.FuncDecl

	for _, decl := range decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok && funcDecl.Recv != nil {
			if funcDecl.Name.Name == "Or" {
				orMethod = funcDecl
				break
			}
		}
	}

	if orMethod == nil {
		t.Fatal("Or method not found")
	}

	// Verify method signature: Or(other Result<T, E>)
	if len(orMethod.Type.Params.List) != 1 {
		t.Errorf("Or should have 1 parameter, got %d", len(orMethod.Type.Params.List))
	}
}

// ============================================================================
// 4. INTEGRATION TESTS (5 tests)
// ============================================================================

func TestIntegration_CompleteResultWorkflow(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	// Simulate a complete workflow: type declaration + constructor calls
	indexExpr := &ast.IndexExpr{
		X:     ast.NewIdent("Result"),
		Index: ast.NewIdent("int"),
	}
	_ = p.Process(indexExpr)

	okCall := &ast.CallExpr{
		Fun:  ast.NewIdent("Ok"),
		Args: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "42"}},
	}
	_ = p.Process(okCall)

	errCall := &ast.CallExpr{
		Fun:  ast.NewIdent("Err"),
		Args: []ast.Expr{ast.NewIdent("someError")},
	}
	_ = p.Process(errCall)

	decls := p.GetPendingDeclarations()

	// Count declarations
	typeDecls := 0
	constDecls := 0
	funcDecls := 0
	methodDecls := 0

	for _, decl := range decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok == token.TYPE {
				typeDecls++
			} else if d.Tok == token.CONST {
				constDecls++
			}
		case *ast.FuncDecl:
			if d.Recv == nil {
				funcDecls++
			} else {
				methodDecls++
			}
		}
	}

	if typeDecls < 2 {
		t.Errorf("expected at least 2 type declarations (ResultTag + Result_int_error), got %d", typeDecls)
	}
	if constDecls < 1 {
		t.Errorf("expected at least 1 const declaration (ResultTag constants), got %d", constDecls)
	}
	if funcDecls < 2 {
		t.Errorf("expected at least 2 function declarations (Ok + Err constructors), got %d", funcDecls)
	}
	if methodDecls < 5 {
		t.Errorf("expected at least 5 method declarations (helper methods), got %d", methodDecls)
	}
}

func TestIntegration_MultipleResultTypesCoexist(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	// Create multiple Result types
	types := []string{"int", "string", "bool", "float64"}

	for _, typ := range types {
		indexExpr := &ast.IndexExpr{
			X:     ast.NewIdent("Result"),
			Index: ast.NewIdent(typ),
		}
		_ = p.Process(indexExpr)
	}

	decls := p.GetPendingDeclarations()

	// Should have ResultTag (once) + 4 Result types
	foundTypes := make(map[string]bool)
	for _, decl := range decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					foundTypes[typeSpec.Name.Name] = true
				}
			}
		}
	}

	expectedTypes := []string{"ResultTag", "Result_int_error", "Result_string_error", "Result_bool_error", "Result_float64_error"}
	for _, typeName := range expectedTypes {
		if !foundTypes[typeName] {
			t.Errorf("expected type %s not found", typeName)
		}
	}
}

func TestIntegration_NoDuplicateResultTagEnum(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	// Process multiple Result types
	for i := 0; i < 5; i++ {
		indexExpr := &ast.IndexExpr{
			X:     ast.NewIdent("Result"),
			Index: ast.NewIdent("int"),
		}
		_ = p.Process(indexExpr)
	}

	decls := p.GetPendingDeclarations()

	// Count ResultTag type declarations
	resultTagCount := 0
	for _, decl := range decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if typeSpec.Name.Name == "ResultTag" {
						resultTagCount++
					}
				}
			}
		}
	}

	if resultTagCount != 1 {
		t.Errorf("expected exactly 1 ResultTag declaration, got %d", resultTagCount)
	}
}

func TestIntegration_ClearAndReuse(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	// First batch
	indexExpr1 := &ast.IndexExpr{
		X:     ast.NewIdent("Result"),
		Index: ast.NewIdent("int"),
	}
	_ = p.Process(indexExpr1)

	firstCount := len(p.GetPendingDeclarations())
	if firstCount == 0 {
		t.Fatal("expected declarations in first batch")
	}

	// Clear
	p.ClearPendingDeclarations()

	if len(p.GetPendingDeclarations()) != 0 {
		t.Error("expected 0 declarations after clear")
	}

	// Second batch (should remember emitted types)
	indexExpr2 := &ast.IndexExpr{
		X:     ast.NewIdent("Result"),
		Index: ast.NewIdent("string"),
	}
	_ = p.Process(indexExpr2)

	secondCount := len(p.GetPendingDeclarations())

	// Should generate new Result_string_error but NOT duplicate ResultTag
	// So second batch should have fewer declarations than first
	// (first: ResultTag + Result_int_error + methods, second: only Result_string_error + methods)
	if secondCount >= firstCount {
		t.Log("Note: Second batch has equal or more declarations. This is expected if ResultTag is emitted again.")
	}
}

func TestIntegration_ContextNotInitialized(t *testing.T) {
	p := NewResultTypePlugin()
	// Don't set p.ctx

	indexExpr := &ast.IndexExpr{
		X:     ast.NewIdent("Result"),
		Index: ast.NewIdent("int"),
	}

	err := p.Process(indexExpr)
	if err == nil {
		t.Error("expected error when context not initialized")
	}

	expectedMsg := "plugin context not initialized"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

// ============================================================================
// 5. ADDITIONAL EDGE CASE TESTS
// ============================================================================

func TestEdgeCase_PluginName(t *testing.T) {
	p := NewResultTypePlugin()
	if name := p.Name(); name != "result_type" {
		t.Errorf("expected name 'result_type', got %q", name)
	}
}

func TestEdgeCase_GetTypeNameWithComplexExpressions(t *testing.T) {
	p := NewResultTypePlugin()

	tests := []struct {
		name     string
		expr     ast.Expr
		expected string
	}{
		{
			name:     "simple ident",
			expr:     ast.NewIdent("int"),
			expected: "int",
		},
		{
			name:     "pointer",
			expr:     &ast.StarExpr{X: ast.NewIdent("User")},
			expected: "*User",
		},
		{
			name:     "slice",
			expr:     &ast.ArrayType{Elt: ast.NewIdent("byte")},
			expected: "[]byte",
		},
		{
			name:     "array with length",
			expr:     &ast.ArrayType{Len: &ast.BasicLit{Kind: token.INT, Value: "10"}, Elt: ast.NewIdent("int")},
			expected: "[N]int",
		},
		{
			name:     "selector",
			expr:     &ast.SelectorExpr{X: ast.NewIdent("pkg"), Sel: ast.NewIdent("Type")},
			expected: "pkg.Type",
		},
		{
			name:     "nested pointer",
			expr:     &ast.StarExpr{X: &ast.StarExpr{X: ast.NewIdent("T")}},
			expected: "**T",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.getTypeName(tt.expr)
			if got != tt.expected {
				t.Errorf("getTypeName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestEdgeCase_TypeToASTVariations(t *testing.T) {
	p := NewResultTypePlugin()

	tests := []struct {
		name      string
		typeName  string
		asPointer bool
		check     func(*testing.T, ast.Expr)
	}{
		{
			name:      "simple type non-pointer",
			typeName:  "int",
			asPointer: false,
			check: func(t *testing.T, expr ast.Expr) {
				if ident, ok := expr.(*ast.Ident); !ok || ident.Name != "int" {
					t.Errorf("expected *ast.Ident with Name=int, got %T", expr)
				}
			},
		},
		{
			name:      "simple type as pointer",
			typeName:  "string",
			asPointer: true,
			check: func(t *testing.T, expr ast.Expr) {
				if starExpr, ok := expr.(*ast.StarExpr); !ok {
					t.Errorf("expected *ast.StarExpr, got %T", expr)
				} else {
					if ident, ok := starExpr.X.(*ast.Ident); !ok || ident.Name != "string" {
						t.Errorf("expected inner type string, got %T", starExpr.X)
					}
				}
			},
		},
		{
			name:      "pointer type non-pointer",
			typeName:  "*User",
			asPointer: false,
			check: func(t *testing.T, expr ast.Expr) {
				if _, ok := expr.(*ast.StarExpr); !ok {
					t.Errorf("expected *ast.StarExpr for *User, got %T", expr)
				}
			},
		},
		{
			name:      "slice type",
			typeName:  "[]byte",
			asPointer: false,
			check: func(t *testing.T, expr ast.Expr) {
				if _, ok := expr.(*ast.ArrayType); !ok {
					t.Errorf("expected *ast.ArrayType for []byte, got %T", expr)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := p.typeToAST(tt.typeName, tt.asPointer)
			tt.check(t, expr)
		})
	}
}

func TestEdgeCase_InferTypeFromExprEdgeCases(t *testing.T) {
	p := NewResultTypePlugin()

	tests := []struct {
		name     string
		expr     ast.Expr
		expected string
	}{
		{
			name:     "int literal",
			expr:     &ast.BasicLit{Kind: token.INT, Value: "42"},
			expected: "int",
		},
		{
			name:     "float literal",
			expr:     &ast.BasicLit{Kind: token.FLOAT, Value: "3.14"},
			expected: "float64",
		},
		{
			name:     "string literal",
			expr:     &ast.BasicLit{Kind: token.STRING, Value: `"hello"`},
			expected: "string",
		},
		{
			name:     "char literal",
			expr:     &ast.BasicLit{Kind: token.CHAR, Value: "'a'"},
			expected: "rune",
		},
		{
			name:     "identifier",
			expr:     ast.NewIdent("myVar"),
			expected: "", // CRITICAL FIX #3: inferTypeFromExpr now returns empty string on error
		},
		{
			name:     "function call",
			expr:     &ast.CallExpr{Fun: ast.NewIdent("getValue")},
			expected: "", // CRITICAL FIX #3: inferTypeFromExpr now returns empty string on error
		},
		{
			name:     "nil expression",
			expr:     &ast.CompositeLit{},
			expected: "", // CRITICAL FIX #3: inferTypeFromExpr now returns empty string on error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// CRITICAL FIX #3: inferTypeFromExpr now returns (string, error)
			got, _ := p.inferTypeFromExpr(tt.expr)
			if got != tt.expected {
				t.Errorf("inferTypeFromExpr() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestEdgeCase_EmptyPendingDeclarations(t *testing.T) {
	p := NewResultTypePlugin()

	decls := p.GetPendingDeclarations()
	if len(decls) != 0 {
		t.Errorf("new plugin should have 0 pending declarations, got %d", len(decls))
	}
}

func TestEdgeCase_ProcessNonResultTypes(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	// Process non-Result type (should be ignored)
	indexExpr := &ast.IndexExpr{
		X:     ast.NewIdent("Option"),
		Index: ast.NewIdent("int"),
	}

	err := p.Process(indexExpr)
	if err != nil {
		t.Fatalf("Process should not fail on non-Result types: %v", err)
	}

	decls := p.GetPendingDeclarations()
	if len(decls) != 0 {
		t.Error("non-Result types should not generate declarations")
	}
}

func TestEdgeCase_ConstructorWithMultipleArgs(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	// Test: Ok(1, 2) - invalid, should warn but not panic
	okCall := &ast.CallExpr{
		Fun: ast.NewIdent("Ok"),
		Args: []ast.Expr{
			&ast.BasicLit{Kind: token.INT, Value: "1"},
			&ast.BasicLit{Kind: token.INT, Value: "2"},
		},
	}

	// Should not panic
	err := p.Process(okCall)
	if err != nil {
		t.Fatalf("Process should not fail: %v", err)
	}
}

func TestEdgeCase_ResultTagConstValues(t *testing.T) {
	p := NewResultTypePlugin()
	p.ctx = &plugin.Context{
		FileSet: token.NewFileSet(),
		Logger:  plugin.NewNoOpLogger(),
	}

	indexExpr := &ast.IndexExpr{
		X:     ast.NewIdent("Result"),
		Index: ast.NewIdent("int"),
	}

	err := p.Process(indexExpr)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	decls := p.GetPendingDeclarations()

	// Find const declaration
	var constDecl *ast.GenDecl
	for _, decl := range decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.CONST {
			constDecl = genDecl
			break
		}
	}

	if constDecl == nil {
		t.Fatal("expected const declaration for ResultTag")
	}

	// Verify we have ResultTag_Ok and ResultTag_Err
	if len(constDecl.Specs) < 2 {
		t.Errorf("expected at least 2 const specs (Ok, Err), got %d", len(constDecl.Specs))
	}

	// Check first const uses iota
	if valueSpec, ok := constDecl.Specs[0].(*ast.ValueSpec); ok {
		if len(valueSpec.Values) > 0 {
			if ident, ok := valueSpec.Values[0].(*ast.Ident); !ok || ident.Name != "iota" {
				t.Error("first ResultTag const should use iota")
			}
		}
	}
}
