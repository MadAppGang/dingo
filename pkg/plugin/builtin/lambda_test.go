package builtin

import (
	"go/ast"
	"go/token"
	"strings"
	"testing"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/plugin"
)

func TestNewLambdaPlugin(t *testing.T) {
	p := NewLambdaPlugin()

	if p == nil {
		t.Fatal("Expected plugin to be non-nil")
	}

	if p.Name() != "lambda" {
		t.Errorf("Expected name 'lambda', got %q", p.Name())
	}
}

func TestLambdaTransformNonLambdaNode(t *testing.T) {
	p := NewLambdaPlugin()
	ctx := &plugin.Context{
		FileSet: token.NewFileSet(),
	}

	// Create a regular function literal
	funcLit := &ast.FuncLit{
		Type: &ast.FuncType{
			Params: &ast.FieldList{},
		},
		Body: &ast.BlockStmt{},
	}

	// Transform should return the node unchanged
	result, err := p.Transform(ctx, funcLit)
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}

	if result != funcLit {
		t.Error("Expected node to be returned unchanged for non-LambdaExpr")
	}
}

func TestLambdaTransformBasic(t *testing.T) {
	p := NewLambdaPlugin()

	cfg := config.DefaultConfig()
	ctx := &plugin.Context{
		FileSet:     token.NewFileSet(),
		DingoConfig: cfg,
	}

	// Create LambdaExpr: |x| x * 2
	lambdaExpr := &dingoast.LambdaExpr{
		Pipe: token.NoPos,
		Params: &ast.FieldList{
			List: []*ast.Field{
				{Names: []*ast.Ident{ast.NewIdent("x")}},
			},
		},
		Arrow: token.NoPos,
		Body: &ast.BinaryExpr{
			X:  ast.NewIdent("x"),
			Op: token.MUL,
			Y:  &ast.BasicLit{Kind: token.INT, Value: "2"},
		},
		Rpipe: token.NoPos,
	}

	// Transform
	result, err := p.Transform(ctx, lambdaExpr)
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}

	// Result should be FuncLit
	funcLit, ok := result.(*ast.FuncLit)
	if !ok {
		t.Fatalf("Expected *ast.FuncLit, got %T", result)
	}

	// Check parameters match
	if funcLit.Type.Params == nil {
		t.Fatal("Expected function to have parameters")
	}

	if len(funcLit.Type.Params.List) != 1 {
		t.Fatalf("Expected 1 parameter, got %d", len(funcLit.Type.Params.List))
	}

	param := funcLit.Type.Params.List[0]
	if len(param.Names) != 1 || param.Names[0].Name != "x" {
		t.Error("Expected parameter to be named 'x'")
	}

	// Check body is wrapped in return statement
	if len(funcLit.Body.List) != 1 {
		t.Fatalf("Expected 1 statement in function body, got %d", len(funcLit.Body.List))
	}

	returnStmt, ok := funcLit.Body.List[0].(*ast.ReturnStmt)
	if !ok {
		t.Fatalf("Expected return statement in body, got %T", funcLit.Body.List[0])
	}

	if len(returnStmt.Results) != 1 {
		t.Fatalf("Expected 1 return value, got %d", len(returnStmt.Results))
	}

	// Check return contains the expression
	binExpr, ok := returnStmt.Results[0].(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("Expected binary expression in return, got %T", returnStmt.Results[0])
	}

	if binExpr.Op != token.MUL {
		t.Errorf("Expected * operator, got %v", binExpr.Op)
	}
}

func TestLambdaTransformMultipleParams(t *testing.T) {
	p := NewLambdaPlugin()

	cfg := config.DefaultConfig()
	ctx := &plugin.Context{
		FileSet:     token.NewFileSet(),
		DingoConfig: cfg,
	}

	// Create LambdaExpr: |a, b| a + b
	lambdaExpr := &dingoast.LambdaExpr{
		Pipe: token.NoPos,
		Params: &ast.FieldList{
			List: []*ast.Field{
				{Names: []*ast.Ident{ast.NewIdent("a")}},
				{Names: []*ast.Ident{ast.NewIdent("b")}},
			},
		},
		Arrow: token.NoPos,
		Body: &ast.BinaryExpr{
			X:  ast.NewIdent("a"),
			Op: token.ADD,
			Y:  ast.NewIdent("b"),
		},
		Rpipe: token.NoPos,
	}

	// Transform
	result, err := p.Transform(ctx, lambdaExpr)
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}

	// Result should be FuncLit
	funcLit, ok := result.(*ast.FuncLit)
	if !ok {
		t.Fatalf("Expected *ast.FuncLit, got %T", result)
	}

	// Check parameters
	if len(funcLit.Type.Params.List) != 2 {
		t.Fatalf("Expected 2 parameters, got %d", len(funcLit.Type.Params.List))
	}

	param1 := funcLit.Type.Params.List[0]
	if len(param1.Names) != 1 || param1.Names[0].Name != "a" {
		t.Error("Expected first parameter to be named 'a'")
	}

	param2 := funcLit.Type.Params.List[1]
	if len(param2.Names) != 1 || param2.Names[0].Name != "b" {
		t.Error("Expected second parameter to be named 'b'")
	}
}

func TestLambdaTransformNoParams(t *testing.T) {
	p := NewLambdaPlugin()

	cfg := config.DefaultConfig()
	ctx := &plugin.Context{
		FileSet:     token.NewFileSet(),
		DingoConfig: cfg,
	}

	// Create LambdaExpr: || 42
	lambdaExpr := &dingoast.LambdaExpr{
		Pipe:   token.NoPos,
		Params: nil, // No parameters
		Arrow:  token.NoPos,
		Body:   &ast.BasicLit{Kind: token.INT, Value: "42"},
		Rpipe:  token.NoPos,
	}

	// Transform
	result, err := p.Transform(ctx, lambdaExpr)
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}

	// Result should be FuncLit
	funcLit, ok := result.(*ast.FuncLit)
	if !ok {
		t.Fatalf("Expected *ast.FuncLit, got %T", result)
	}

	// Check parameters is empty list
	if funcLit.Type.Params == nil {
		t.Fatal("Expected function to have parameter list (even if empty)")
	}

	if len(funcLit.Type.Params.List) != 0 {
		t.Fatalf("Expected 0 parameters, got %d", len(funcLit.Type.Params.List))
	}

	// Check body contains return 42
	if len(funcLit.Body.List) != 1 {
		t.Fatalf("Expected 1 statement in function body, got %d", len(funcLit.Body.List))
	}

	returnStmt, ok := funcLit.Body.List[0].(*ast.ReturnStmt)
	if !ok {
		t.Fatalf("Expected return statement in body, got %T", funcLit.Body.List[0])
	}

	lit, ok := returnStmt.Results[0].(*ast.BasicLit)
	if !ok || lit.Value != "42" {
		t.Error("Expected return value to be 42")
	}
}

func TestLambdaRustSyntaxMode(t *testing.T) {
	p := NewLambdaPlugin()

	cfg := config.DefaultConfig()
	cfg.Features.LambdaSyntax = "rust"

	ctx := &plugin.Context{
		FileSet:     token.NewFileSet(),
		DingoConfig: cfg,
	}

	lambdaExpr := &dingoast.LambdaExpr{
		Pipe: token.NoPos,
		Params: &ast.FieldList{
			List: []*ast.Field{
				{Names: []*ast.Ident{ast.NewIdent("x")}},
			},
		},
		Arrow: token.NoPos,
		Body:  ast.NewIdent("x"),
		Rpipe: token.NoPos,
	}

	// Transform should succeed
	result, err := p.Transform(ctx, lambdaExpr)
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}

	if _, ok := result.(*ast.FuncLit); !ok {
		t.Fatalf("Expected *ast.FuncLit, got %T", result)
	}
}

func TestLambdaArrowSyntaxMode(t *testing.T) {
	p := NewLambdaPlugin()

	cfg := config.DefaultConfig()
	cfg.Features.LambdaSyntax = "arrow"

	ctx := &plugin.Context{
		FileSet:     token.NewFileSet(),
		DingoConfig: cfg,
	}

	lambdaExpr := &dingoast.LambdaExpr{
		Pipe: token.NoPos,
		Params: &ast.FieldList{
			List: []*ast.Field{
				{Names: []*ast.Ident{ast.NewIdent("x")}},
			},
		},
		Arrow: token.NoPos,
		Body:  ast.NewIdent("x"),
		Rpipe: token.NoPos,
	}

	// Transform should succeed (syntax validation happens in parser)
	result, err := p.Transform(ctx, lambdaExpr)
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}

	if _, ok := result.(*ast.FuncLit); !ok {
		t.Fatalf("Expected *ast.FuncLit, got %T", result)
	}
}

func TestLambdaBothSyntaxMode(t *testing.T) {
	p := NewLambdaPlugin()

	cfg := config.DefaultConfig()
	cfg.Features.LambdaSyntax = "both"

	ctx := &plugin.Context{
		FileSet:     token.NewFileSet(),
		DingoConfig: cfg,
	}

	lambdaExpr := &dingoast.LambdaExpr{
		Pipe: token.NoPos,
		Params: &ast.FieldList{
			List: []*ast.Field{
				{Names: []*ast.Ident{ast.NewIdent("x")}},
			},
		},
		Arrow: token.NoPos,
		Body:  ast.NewIdent("x"),
		Rpipe: token.NoPos,
	}

	// Transform should succeed
	result, err := p.Transform(ctx, lambdaExpr)
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}

	if _, ok := result.(*ast.FuncLit); !ok {
		t.Fatalf("Expected *ast.FuncLit, got %T", result)
	}
}

func TestLambdaInvalidSyntaxMode(t *testing.T) {
	p := NewLambdaPlugin()

	cfg := config.DefaultConfig()
	cfg.Features.LambdaSyntax = "invalid_mode"

	ctx := &plugin.Context{
		FileSet:     token.NewFileSet(),
		DingoConfig: cfg,
	}

	lambdaExpr := &dingoast.LambdaExpr{
		Pipe: token.NoPos,
		Params: &ast.FieldList{
			List: []*ast.Field{
				{Names: []*ast.Ident{ast.NewIdent("x")}},
			},
		},
		Arrow: token.NoPos,
		Body:  ast.NewIdent("x"),
		Rpipe: token.NoPos,
	}

	// Transform should return error
	_, err := p.Transform(ctx, lambdaExpr)
	if err == nil {
		t.Fatal("Expected error for invalid lambda syntax mode, got nil")
	}

	expectedMsg := "invalid lambda_syntax mode"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain %q, got %q", expectedMsg, err.Error())
	}
}

func TestLambdaNilConfig(t *testing.T) {
	p := NewLambdaPlugin()

	// Context with nil config should default to rust mode
	ctx := &plugin.Context{
		FileSet:     token.NewFileSet(),
		DingoConfig: nil,
	}

	lambdaExpr := &dingoast.LambdaExpr{
		Pipe: token.NoPos,
		Params: &ast.FieldList{
			List: []*ast.Field{
				{Names: []*ast.Ident{ast.NewIdent("x")}},
			},
		},
		Arrow: token.NoPos,
		Body:  ast.NewIdent("x"),
		Rpipe: token.NoPos,
	}

	// Transform should succeed with default behavior
	result, err := p.Transform(ctx, lambdaExpr)
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}

	if _, ok := result.(*ast.FuncLit); !ok {
		t.Fatalf("Expected *ast.FuncLit, got %T", result)
	}
}

