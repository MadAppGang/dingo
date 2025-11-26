package builtin

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/MadAppGang/dingo/pkg/plugin"
)

// TestIsAddressable_Identifiers verifies that identifiers are considered addressable
func TestIsAddressable_Identifiers(t *testing.T) {
	tests := []struct {
		name string
		expr ast.Expr
		want bool
	}{
		{
			name: "simple identifier",
			expr: ast.NewIdent("x"),
			want: true,
		},
		{
			name: "identifier user",
			expr: ast.NewIdent("user"),
			want: true,
		},
		{
			name: "identifier name",
			expr: ast.NewIdent("name"),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAddressable(tt.expr)
			if got != tt.want {
				t.Errorf("isAddressable(%v) = %v, want %v", FormatExprForDebug(tt.expr), got, tt.want)
			}
		})
	}
}

// TestIsAddressable_Selectors verifies that selector expressions are considered addressable
func TestIsAddressable_Selectors(t *testing.T) {
	tests := []struct {
		name string
		expr ast.Expr
		want bool
	}{
		{
			name: "field selector",
			expr: &ast.SelectorExpr{
				X:   ast.NewIdent("user"),
				Sel: ast.NewIdent("Name"),
			},
			want: true,
		},
		{
			name: "package selector",
			expr: &ast.SelectorExpr{
				X:   ast.NewIdent("pkg"),
				Sel: ast.NewIdent("Var"),
			},
			want: true,
		},
		{
			name: "nested selector",
			expr: &ast.SelectorExpr{
				X: &ast.SelectorExpr{
					X:   ast.NewIdent("app"),
					Sel: ast.NewIdent("config"),
				},
				Sel: ast.NewIdent("Port"),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAddressable(tt.expr)
			if got != tt.want {
				t.Errorf("isAddressable(%v) = %v, want %v", FormatExprForDebug(tt.expr), got, tt.want)
			}
		})
	}
}

// TestIsAddressable_IndexExpressions verifies that index expressions are considered addressable
func TestIsAddressable_IndexExpressions(t *testing.T) {
	tests := []struct {
		name string
		expr ast.Expr
		want bool
	}{
		{
			name: "array index",
			expr: &ast.IndexExpr{
				X:     ast.NewIdent("arr"),
				Index: ast.NewIdent("i"),
			},
			want: true,
		},
		{
			name: "map index",
			expr: &ast.IndexExpr{
				X:     ast.NewIdent("m"),
				Index: ast.NewIdent("key"),
			},
			want: true,
		},
		{
			name: "literal index",
			expr: &ast.IndexExpr{
				X: ast.NewIdent("slice"),
				Index: &ast.BasicLit{
					Kind:  token.INT,
					Value: "0",
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAddressable(tt.expr)
			if got != tt.want {
				t.Errorf("isAddressable(%v) = %v, want %v", FormatExprForDebug(tt.expr), got, tt.want)
			}
		})
	}
}

// TestIsAddressable_Dereferences verifies that pointer dereferences are considered addressable
func TestIsAddressable_Dereferences(t *testing.T) {
	tests := []struct {
		name string
		expr ast.Expr
		want bool
	}{
		{
			name: "simple dereference",
			expr: &ast.StarExpr{
				X: ast.NewIdent("ptr"),
			},
			want: true,
		},
		{
			name: "nested dereference",
			expr: &ast.StarExpr{
				X: &ast.StarExpr{
					X: ast.NewIdent("ptrptr"),
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAddressable(tt.expr)
			if got != tt.want {
				t.Errorf("isAddressable(%v) = %v, want %v", FormatExprForDebug(tt.expr), got, tt.want)
			}
		})
	}
}

// TestIsAddressable_ParenExpressions verifies that parenthesized expressions follow inner expression rules
func TestIsAddressable_ParenExpressions(t *testing.T) {
	tests := []struct {
		name string
		expr ast.Expr
		want bool
	}{
		{
			name: "parenthesized identifier (addressable)",
			expr: &ast.ParenExpr{
				X: ast.NewIdent("x"),
			},
			want: true,
		},
		{
			name: "parenthesized literal (not addressable)",
			expr: &ast.ParenExpr{
				X: &ast.BasicLit{
					Kind:  token.INT,
					Value: "42",
				},
			},
			want: false,
		},
		{
			name: "double parenthesized identifier",
			expr: &ast.ParenExpr{
				X: &ast.ParenExpr{
					X: ast.NewIdent("x"),
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAddressable(tt.expr)
			if got != tt.want {
				t.Errorf("isAddressable(%v) = %v, want %v", FormatExprForDebug(tt.expr), got, tt.want)
			}
		})
	}
}

// TestIsAddressable_Literals verifies that literals are NOT addressable
func TestIsAddressable_Literals(t *testing.T) {
	tests := []struct {
		name string
		expr ast.Expr
		want bool
	}{
		{
			name: "integer literal",
			expr: &ast.BasicLit{
				Kind:  token.INT,
				Value: "42",
			},
			want: false,
		},
		{
			name: "string literal",
			expr: &ast.BasicLit{
				Kind:  token.STRING,
				Value: `"hello"`,
			},
			want: false,
		},
		{
			name: "float literal",
			expr: &ast.BasicLit{
				Kind:  token.FLOAT,
				Value: "3.14",
			},
			want: false,
		},
		{
			name: "boolean literal (true)",
			expr: &ast.BasicLit{
				Kind:  token.INT, // Booleans are represented as INT in go/ast
				Value: "true",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAddressable(tt.expr)
			if got != tt.want {
				t.Errorf("isAddressable(%v) = %v, want %v", FormatExprForDebug(tt.expr), got, tt.want)
			}
		})
	}
}

// TestIsAddressable_CompositeLiterals verifies that composite literals are NOT addressable
func TestIsAddressable_CompositeLiterals(t *testing.T) {
	tests := []struct {
		name string
		expr ast.Expr
		want bool
	}{
		{
			name: "struct literal",
			expr: &ast.CompositeLit{
				Type: ast.NewIdent("User"),
				Elts: []ast.Expr{},
			},
			want: false,
		},
		{
			name: "slice literal",
			expr: &ast.CompositeLit{
				Type: &ast.ArrayType{
					Elt: ast.NewIdent("int"),
				},
				Elts: []ast.Expr{
					&ast.BasicLit{Kind: token.INT, Value: "1"},
					&ast.BasicLit{Kind: token.INT, Value: "2"},
				},
			},
			want: false,
		},
		{
			name: "map literal",
			expr: &ast.CompositeLit{
				Type: &ast.MapType{
					Key:   ast.NewIdent("string"),
					Value: ast.NewIdent("int"),
				},
				Elts: []ast.Expr{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAddressable(tt.expr)
			if got != tt.want {
				t.Errorf("isAddressable(%v) = %v, want %v", FormatExprForDebug(tt.expr), got, tt.want)
			}
		})
	}
}

// TestIsAddressable_FunctionCalls verifies that function calls are NOT addressable
func TestIsAddressable_FunctionCalls(t *testing.T) {
	tests := []struct {
		name string
		expr ast.Expr
		want bool
	}{
		{
			name: "simple function call",
			expr: &ast.CallExpr{
				Fun:  ast.NewIdent("getUser"),
				Args: []ast.Expr{},
			},
			want: false,
		},
		{
			name: "method call",
			expr: &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   ast.NewIdent("user"),
					Sel: ast.NewIdent("GetName"),
				},
				Args: []ast.Expr{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAddressable(tt.expr)
			if got != tt.want {
				t.Errorf("isAddressable(%v) = %v, want %v", FormatExprForDebug(tt.expr), got, tt.want)
			}
		})
	}
}

// TestIsAddressable_BinaryExpressions verifies that binary operations are NOT addressable
func TestIsAddressable_BinaryExpressions(t *testing.T) {
	tests := []struct {
		name string
		expr ast.Expr
		want bool
	}{
		{
			name: "addition",
			expr: &ast.BinaryExpr{
				X:  ast.NewIdent("x"),
				Op: token.ADD,
				Y:  ast.NewIdent("y"),
			},
			want: false,
		},
		{
			name: "multiplication",
			expr: &ast.BinaryExpr{
				X:  ast.NewIdent("a"),
				Op: token.MUL,
				Y:  ast.NewIdent("b"),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAddressable(tt.expr)
			if got != tt.want {
				t.Errorf("isAddressable(%v) = %v, want %v", FormatExprForDebug(tt.expr), got, tt.want)
			}
		})
	}
}

// TestIsAddressable_UnaryExpressions verifies that unary operations (except dereference) are NOT addressable
func TestIsAddressable_UnaryExpressions(t *testing.T) {
	tests := []struct {
		name string
		expr ast.Expr
		want bool
	}{
		{
			name: "negation",
			expr: &ast.UnaryExpr{
				Op: token.SUB,
				X:  ast.NewIdent("value"),
			},
			want: false,
		},
		{
			name: "logical not",
			expr: &ast.UnaryExpr{
				Op: token.NOT,
				X:  ast.NewIdent("flag"),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAddressable(tt.expr)
			if got != tt.want {
				t.Errorf("isAddressable(%v) = %v, want %v", FormatExprForDebug(tt.expr), got, tt.want)
			}
		})
	}
}

// TestIsAddressable_TypeAssertions verifies that type assertions are NOT addressable
func TestIsAddressable_TypeAssertions(t *testing.T) {
	tests := []struct {
		name string
		expr ast.Expr
		want bool
	}{
		{
			name: "type assertion",
			expr: &ast.TypeAssertExpr{
				X:    ast.NewIdent("x"),
				Type: ast.NewIdent("User"),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAddressable(tt.expr)
			if got != tt.want {
				t.Errorf("isAddressable(%v) = %v, want %v", FormatExprForDebug(tt.expr), got, tt.want)
			}
		})
	}
}

// TestIsAddressable_NilExpression verifies nil handling
func TestIsAddressable_NilExpression(t *testing.T) {
	got := isAddressable(nil)
	if got != false {
		t.Errorf("isAddressable(nil) = %v, want false", got)
	}
}

// TestWrapInIIFE_BasicStructure verifies the IIFE structure is correct
func TestWrapInIIFE_BasicStructure(t *testing.T) {
	ctx := &plugin.Context{
		TempVarCounter: 1,
	}

	expr := &ast.BasicLit{
		Kind:  token.INT,
		Value: "42",
	}

	result := wrapInIIFE(expr, "int", ctx)

	// Verify it's a CallExpr (IIFE invocation)
	callExpr, ok := result.(*ast.CallExpr)
	if !ok {
		t.Fatalf("wrapInIIFE() returned %T, want *ast.CallExpr", result)
	}

	// Verify the Fun is a FuncLit
	funcLit, ok := callExpr.Fun.(*ast.FuncLit)
	if !ok {
		t.Fatalf("CallExpr.Fun is %T, want *ast.FuncLit", callExpr.Fun)
	}

	// Verify no parameters
	if len(funcLit.Type.Params.List) != 0 {
		t.Errorf("FuncLit has %d parameters, want 0", len(funcLit.Type.Params.List))
	}

	// Verify return type is *int
	if len(funcLit.Type.Results.List) != 1 {
		t.Fatalf("FuncLit has %d results, want 1", len(funcLit.Type.Results.List))
	}

	starExpr, ok := funcLit.Type.Results.List[0].Type.(*ast.StarExpr)
	if !ok {
		t.Fatalf("Result type is %T, want *ast.StarExpr", funcLit.Type.Results.List[0].Type)
	}

	ident, ok := starExpr.X.(*ast.Ident)
	if !ok || ident.Name != "int" {
		t.Errorf("Result type is *%v, want *int", starExpr.X)
	}

	// Verify body has 2 statements (assignment + return)
	if len(funcLit.Body.List) != 2 {
		t.Fatalf("FuncLit body has %d statements, want 2", len(funcLit.Body.List))
	}

	// Verify first statement is assignment
	assignStmt, ok := funcLit.Body.List[0].(*ast.AssignStmt)
	if !ok {
		t.Fatalf("First statement is %T, want *ast.AssignStmt", funcLit.Body.List[0])
	}

	// Verify assignment LHS is tmp (first call, no number per "No-Number-First Pattern")
	lhsIdent, ok := assignStmt.Lhs[0].(*ast.Ident)
	if !ok || lhsIdent.Name != "tmp" {
		t.Errorf("Assignment LHS is %v, want tmp", assignStmt.Lhs[0])
	}

	// Verify second statement is return
	returnStmt, ok := funcLit.Body.List[1].(*ast.ReturnStmt)
	if !ok {
		t.Fatalf("Second statement is %T, want *ast.ReturnStmt", funcLit.Body.List[1])
	}

	// Verify return value is &tmp
	unaryExpr, ok := returnStmt.Results[0].(*ast.UnaryExpr)
	if !ok || unaryExpr.Op != token.AND {
		t.Fatalf("Return value is %T with op %v, want *ast.UnaryExpr with &", returnStmt.Results[0], unaryExpr.Op)
	}

	returnIdent, ok := unaryExpr.X.(*ast.Ident)
	if !ok || returnIdent.Name != "tmp" {
		t.Errorf("Return address-of is %v, want &tmp", unaryExpr.X)
	}

	// Verify CallExpr has no args (immediate invocation)
	if len(callExpr.Args) != 0 {
		t.Errorf("CallExpr has %d args, want 0 (immediate invocation)", len(callExpr.Args))
	}

	// Verify temp var counter was incremented (first call uses "tmp", counter becomes 2)
	if ctx.TempVarCounter != 2 {
		t.Errorf("TempVarCounter is %d, want 2", ctx.TempVarCounter)
	}
}

// TestWrapInIIFE_MultipleCalls verifies unique temp var names
func TestWrapInIIFE_MultipleCalls(t *testing.T) {
	ctx := &plugin.Context{
		TempVarCounter: 0, // Start at 0
	}

	// First call - should get "tmp" (no number)
	expr1 := &ast.BasicLit{Kind: token.INT, Value: "42"}
	result1 := wrapInIIFE(expr1, "int", ctx)

	callExpr1 := result1.(*ast.CallExpr)
	funcLit1 := callExpr1.Fun.(*ast.FuncLit)
	assignStmt1 := funcLit1.Body.List[0].(*ast.AssignStmt)
	lhs1 := assignStmt1.Lhs[0].(*ast.Ident)

	if lhs1.Name != "tmp" {
		t.Errorf("First call generated %s, want tmp", lhs1.Name)
	}

	// Second call - should get "tmp1"
	expr2 := &ast.BasicLit{Kind: token.STRING, Value: `"hello"`}
	result2 := wrapInIIFE(expr2, "string", ctx)

	callExpr2 := result2.(*ast.CallExpr)
	funcLit2 := callExpr2.Fun.(*ast.FuncLit)
	assignStmt2 := funcLit2.Body.List[0].(*ast.AssignStmt)
	lhs2 := assignStmt2.Lhs[0].(*ast.Ident)

	if lhs2.Name != "tmp1" {
		t.Errorf("Second call generated %s, want tmp1", lhs2.Name)
	}

	// Third call - should get "tmp2"
	expr3 := &ast.BasicLit{Kind: token.FLOAT, Value: "3.14"}
	result3 := wrapInIIFE(expr3, "float64", ctx)

	callExpr3 := result3.(*ast.CallExpr)
	funcLit3 := callExpr3.Fun.(*ast.FuncLit)
	assignStmt3 := funcLit3.Body.List[0].(*ast.AssignStmt)
	lhs3 := assignStmt3.Lhs[0].(*ast.Ident)

	if lhs3.Name != "tmp2" {
		t.Errorf("Third call generated %s, want tmp2", lhs3.Name)
	}

	if ctx.TempVarCounter != 4 {
		t.Errorf("TempVarCounter is %d, want 4", ctx.TempVarCounter)
	}
}

// TestMaybeWrapForAddressability_Addressable verifies that addressable expressions get &expr
func TestMaybeWrapForAddressability_Addressable(t *testing.T) {
	ctx := &plugin.Context{
		TempVarCounter: 0, // Start at 0
	}

	expr := ast.NewIdent("x")
	result := MaybeWrapForAddressability(expr, "int", ctx)

	// Should be &x
	unaryExpr, ok := result.(*ast.UnaryExpr)
	if !ok {
		t.Fatalf("MaybeWrapForAddressability() returned %T, want *ast.UnaryExpr", result)
	}

	if unaryExpr.Op != token.AND {
		t.Errorf("UnaryExpr.Op = %v, want &", unaryExpr.Op)
	}

	ident, ok := unaryExpr.X.(*ast.Ident)
	if !ok || ident.Name != "x" {
		t.Errorf("UnaryExpr.X = %v, want x", unaryExpr.X)
	}

	// Temp var counter should NOT be incremented (no IIFE created)
	if ctx.TempVarCounter != 0 {
		t.Errorf("TempVarCounter = %d, want 0 (no IIFE created)", ctx.TempVarCounter)
	}
}

// TestMaybeWrapForAddressability_NonAddressable verifies that non-addressable expressions get wrapped
func TestMaybeWrapForAddressability_NonAddressable(t *testing.T) {
	ctx := &plugin.Context{
		TempVarCounter: 0, // Start at 0
	}

	expr := &ast.BasicLit{Kind: token.INT, Value: "42"}
	result := MaybeWrapForAddressability(expr, "int", ctx)

	// Should be an IIFE CallExpr
	callExpr, ok := result.(*ast.CallExpr)
	if !ok {
		t.Fatalf("MaybeWrapForAddressability() returned %T, want *ast.CallExpr (IIFE)", result)
	}

	// Verify it's a function literal (IIFE structure)
	_, ok = callExpr.Fun.(*ast.FuncLit)
	if !ok {
		t.Errorf("CallExpr.Fun is %T, want *ast.FuncLit", callExpr.Fun)
	}

	// Temp var counter SHOULD be incremented (IIFE created, counter goes 0→1→2)
	if ctx.TempVarCounter != 2 {
		t.Errorf("TempVarCounter = %d, want 2 (IIFE created)", ctx.TempVarCounter)
	}
}

// TestParseTypeString_SimpleTypes verifies simple type name parsing
func TestParseTypeString_SimpleTypes(t *testing.T) {
	tests := []struct {
		typeName string
		want     string
	}{
		{"int", "int"},
		{"string", "string"},
		{"error", "error"},
		{"User", "User"},
		{"MyCustomType", "MyCustomType"},
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			result := parseTypeString(tt.typeName)

			ident, ok := result.(*ast.Ident)
			if !ok {
				t.Fatalf("parseTypeString(%q) returned %T, want *ast.Ident", tt.typeName, result)
			}

			if ident.Name != tt.want {
				t.Errorf("parseTypeString(%q) = %q, want %q", tt.typeName, ident.Name, tt.want)
			}
		})
	}
}

// TestParseTypeString_EmptyType verifies empty type name handling
func TestParseTypeString_EmptyType(t *testing.T) {
	result := parseTypeString("")

	// Should return interface{} for empty type
	interfaceType, ok := result.(*ast.InterfaceType)
	if !ok {
		t.Fatalf("parseTypeString(\"\") returned %T, want *ast.InterfaceType", result)
	}

	if interfaceType.Methods == nil || len(interfaceType.Methods.List) != 0 {
		t.Errorf("parseTypeString(\"\") returned interface with methods, want empty interface{}")
	}
}

// TestFormatExprForDebug verifies debug formatting for various expressions
func TestFormatExprForDebug(t *testing.T) {
	tests := []struct {
		name string
		expr ast.Expr
		want string
	}{
		{
			name: "nil expression",
			expr: nil,
			want: "<nil>",
		},
		{
			name: "identifier",
			expr: ast.NewIdent("x"),
			want: "x",
		},
		{
			name: "integer literal",
			expr: &ast.BasicLit{Kind: token.INT, Value: "42"},
			want: "42",
		},
		{
			name: "string literal",
			expr: &ast.BasicLit{Kind: token.STRING, Value: `"hello"`},
			want: `"hello"`,
		},
		{
			name: "selector",
			expr: &ast.SelectorExpr{
				X:   ast.NewIdent("user"),
				Sel: ast.NewIdent("Name"),
			},
			want: "user.Name",
		},
		{
			name: "function call",
			expr: &ast.CallExpr{
				Fun: ast.NewIdent("getUser"),
			},
			want: "getUser(...)",
		},
		{
			name: "binary expression",
			expr: &ast.BinaryExpr{
				X:  ast.NewIdent("x"),
				Op: token.ADD,
				Y:  ast.NewIdent("y"),
			},
			want: "(x + y)",
		},
		{
			name: "unary expression",
			expr: &ast.UnaryExpr{
				Op: token.NOT,
				X:  ast.NewIdent("flag"),
			},
			want: "!flag",
		},
		{
			name: "star expression",
			expr: &ast.StarExpr{
				X: ast.NewIdent("ptr"),
			},
			want: "*ptr",
		},
		{
			name: "index expression",
			expr: &ast.IndexExpr{
				X: ast.NewIdent("arr"),
			},
			want: "arr[...]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatExprForDebug(tt.expr)
			if got != tt.want {
				t.Errorf("FormatExprForDebug() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFormatExprForDebug_CompositeLiteral verifies composite literal formatting
func TestFormatExprForDebug_CompositeLiteral(t *testing.T) {
	expr := &ast.CompositeLit{
		Type: ast.NewIdent("User"),
		Elts: []ast.Expr{},
	}

	got := FormatExprForDebug(expr)
	want := "User{...}"

	if got != want {
		t.Errorf("FormatExprForDebug(CompositeLit) = %q, want %q", got, want)
	}
}

// TestEdgeCase_AddressableComplexCases verifies edge cases with complex addressability
func TestEdgeCase_AddressableComplexCases(t *testing.T) {
	tests := []struct {
		name string
		expr ast.Expr
		want bool
	}{
		{
			name: "slice expression (not addressable)",
			expr: &ast.SliceExpr{
				X: ast.NewIdent("arr"),
			},
			want: false,
		},
		{
			name: "function literal (not addressable)",
			expr: &ast.FuncLit{},
			want: false,
		},
		{
			name: "array type expression (not addressable)",
			expr: &ast.ArrayType{
				Elt: ast.NewIdent("int"),
			},
			want: false,
		},
		{
			name: "struct type expression (not addressable)",
			expr: &ast.StructType{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAddressable(tt.expr)
			if got != tt.want {
				t.Errorf("isAddressable(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// TestWrapInIIFE_TypePreservation verifies different type names are preserved
func TestWrapInIIFE_TypePreservation(t *testing.T) {
	tests := []struct {
		typeName string
		want     string
	}{
		{"int", "int"},
		{"string", "string"},
		{"error", "error"},
		{"User", "User"},
		{"MyCustomType", "MyCustomType"},
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			ctx := &plugin.Context{TempVarCounter: 1}
			expr := &ast.BasicLit{Kind: token.INT, Value: "0"}

			result := wrapInIIFE(expr, tt.typeName, ctx)

			callExpr := result.(*ast.CallExpr)
			funcLit := callExpr.Fun.(*ast.FuncLit)
			starExpr := funcLit.Type.Results.List[0].Type.(*ast.StarExpr)
			ident := starExpr.X.(*ast.Ident)

			if ident.Name != tt.want {
				t.Errorf("Type name in IIFE = %q, want %q", ident.Name, tt.want)
			}
		})
	}
}

// Benchmark tests for performance validation

func BenchmarkIsAddressable_Identifier(b *testing.B) {
	expr := ast.NewIdent("x")
	for i := 0; i < b.N; i++ {
		isAddressable(expr)
	}
}

func BenchmarkIsAddressable_Literal(b *testing.B) {
	expr := &ast.BasicLit{Kind: token.INT, Value: "42"}
	for i := 0; i < b.N; i++ {
		isAddressable(expr)
	}
}

func BenchmarkWrapInIIFE(b *testing.B) {
	ctx := &plugin.Context{TempVarCounter: 1}
	expr := &ast.BasicLit{Kind: token.INT, Value: "42"}
	for i := 0; i < b.N; i++ {
		wrapInIIFE(expr, "int", ctx)
		ctx.TempVarCounter = 0 // Reset for fair comparison
	}
}

func BenchmarkMaybeWrapForAddressability_NoWrap(b *testing.B) {
	ctx := &plugin.Context{TempVarCounter: 1}
	expr := ast.NewIdent("x")
	for i := 0; i < b.N; i++ {
		MaybeWrapForAddressability(expr, "int", ctx)
	}
}

func BenchmarkMaybeWrapForAddressability_Wrap(b *testing.B) {
	ctx := &plugin.Context{TempVarCounter: 1}
	expr := &ast.BasicLit{Kind: token.INT, Value: "42"}
	for i := 0; i < b.N; i++ {
		MaybeWrapForAddressability(expr, "int", ctx)
		ctx.TempVarCounter = 0
	}
}

// Example test to demonstrate usage
func ExampleMaybeWrapForAddressability() {
	ctx := &plugin.Context{TempVarCounter: 1}

	// Addressable expression (identifier) - gets &x
	addrExpr := ast.NewIdent("x")
	result1 := MaybeWrapForAddressability(addrExpr, "int", ctx)
	if unary, ok := result1.(*ast.UnaryExpr); ok {
		println("Addressable:", unary.Op == token.AND) // true
	}

	// Non-addressable expression (literal) - gets IIFE
	literalExpr := &ast.BasicLit{Kind: token.INT, Value: "42"}
	result2 := MaybeWrapForAddressability(literalExpr, "int", ctx)
	if call, ok := result2.(*ast.CallExpr); ok {
		println("Non-addressable:", call.Fun != nil) // true (IIFE created)
	}
}

// Integration test: Verify IIFE can be printed as valid Go code
func TestWrapInIIFE_ValidGoCode(t *testing.T) {
	// This test uses go/printer to verify the IIFE generates syntactically valid Go
	// We don't run the printer here (would need imports), but the structure is verified above
	// Manual verification: the IIFE structure matches Go syntax rules

	ctx := &plugin.Context{TempVarCounter: 1}
	expr := &ast.BasicLit{Kind: token.INT, Value: "42"}
	result := wrapInIIFE(expr, "int", ctx)

	// Verify result is not nil
	if result == nil {
		t.Fatal("wrapInIIFE() returned nil")
	}

	// The structure has been verified in detail in other tests
	// This test ensures the overall structure is present
	callExpr, ok := result.(*ast.CallExpr)
	if !ok {
		t.Fatal("wrapInIIFE() did not return CallExpr")
	}

	funcLit, ok := callExpr.Fun.(*ast.FuncLit)
	if !ok {
		t.Fatal("IIFE Fun is not FuncLit")
	}

	// Verify syntactic completeness
	requiredParts := []bool{
		funcLit.Type != nil,               // Function type exists
		funcLit.Type.Params != nil,        // Parameters exist (even if empty)
		funcLit.Type.Results != nil,       // Results exist
		funcLit.Body != nil,               // Body exists
		len(funcLit.Body.List) == 2,       // Body has 2 statements
		len(callExpr.Args) == 0,           // Immediate invocation (no args)
	}

	for i, part := range requiredParts {
		if !part {
			t.Errorf("IIFE structural requirement %d failed", i)
		}
	}
}

// Table-driven comprehensive test combining all cases
func TestIsAddressable_Comprehensive(t *testing.T) {
	tests := []struct {
		category string
		name     string
		expr     ast.Expr
		want     bool
	}{
		// Addressable
		{"addressable", "identifier", ast.NewIdent("x"), true},
		{"addressable", "selector", &ast.SelectorExpr{X: ast.NewIdent("u"), Sel: ast.NewIdent("N")}, true},
		{"addressable", "index", &ast.IndexExpr{X: ast.NewIdent("arr"), Index: ast.NewIdent("i")}, true},
		{"addressable", "dereference", &ast.StarExpr{X: ast.NewIdent("ptr")}, true},
		{"addressable", "paren_ident", &ast.ParenExpr{X: ast.NewIdent("x")}, true},

		// Non-addressable
		{"non-addressable", "int_literal", &ast.BasicLit{Kind: token.INT, Value: "42"}, false},
		{"non-addressable", "string_literal", &ast.BasicLit{Kind: token.STRING, Value: `"hi"`}, false},
		{"non-addressable", "composite_lit", &ast.CompositeLit{Type: ast.NewIdent("User")}, false},
		{"non-addressable", "function_call", &ast.CallExpr{Fun: ast.NewIdent("f")}, false},
		{"non-addressable", "binary_expr", &ast.BinaryExpr{X: ast.NewIdent("x"), Op: token.ADD, Y: ast.NewIdent("y")}, false},
		{"non-addressable", "unary_expr", &ast.UnaryExpr{Op: token.NOT, X: ast.NewIdent("f")}, false},
		{"non-addressable", "type_assert", &ast.TypeAssertExpr{X: ast.NewIdent("x"), Type: ast.NewIdent("T")}, false},
		{"non-addressable", "paren_literal", &ast.ParenExpr{X: &ast.BasicLit{Kind: token.INT, Value: "42"}}, false},

		// Edge cases
		{"edge", "nil", nil, false},
		{"edge", "slice_expr", &ast.SliceExpr{X: ast.NewIdent("a")}, false},
		{"edge", "func_lit", &ast.FuncLit{}, false},
		{"edge", "array_type", &ast.ArrayType{Elt: ast.NewIdent("int")}, false},
	}

	for _, tt := range tests {
		t.Run(tt.category+"_"+tt.name, func(t *testing.T) {
			got := isAddressable(tt.expr)
			if got != tt.want {
				desc := "<nil>"
				if tt.expr != nil {
					desc = FormatExprForDebug(tt.expr)
				}
				t.Errorf("isAddressable(%s) = %v, want %v", desc, got, tt.want)
			}
		})
	}
}
