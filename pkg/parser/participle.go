// Package parser implements a participle-based parser for Dingo
package parser

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
	dingoast "github.com/yourusername/dingo/pkg/ast"
)

// ============================================================================
// Participle Grammar Definitions (Simplified for Phase 1)
// ============================================================================

// DingoFile represents a complete Dingo source file
type DingoFile struct {
	Package string         `parser:"'package' @Ident"`
	Imports []*Import      `parser:"@@*"`
	Decls   []*Declaration `parser:"@@*"`
}

// Import represents an import declaration
type Import struct {
	Path string `parser:"'import' @String"`
}

// Declaration can be a function or variable declaration
type Declaration struct {
	Func *Function `parser:"@@"`
	Var  *Variable `parser:"| @@"`
}

// Function represents a function declaration
type Function struct {
	Name   string       `parser:"'func' @Ident"`
	Params []*Parameter `parser:"'(' ( @@ ( ',' @@ )* )? ')'"`
	Result *Type        `parser:"( @@ )?"`  // No arrow - clean Go-style syntax
	Body   *Block       `parser:"@@"`
}

// Variable represents a variable declaration (let/var)
type Variable struct {
	Mutable bool        `parser:"( @'var' | 'let' )"`
	Name    string      `parser:"@Ident"`
	Type    *Type       `parser:"( ':' @@ )?"`
	Value   *Expression `parser:"'=' @@"`
}

// Parameter represents a function parameter
type Parameter struct {
	Name string `parser:"@Ident"`
	Type *Type  `parser:"':' @@"`
}

// Type represents a type expression
type Type struct {
	Name    string `parser:"@Ident"`
	Pointer bool   `parser:"@'*'?"`
	Array   bool   `parser:"@'['? ']'?"`
}

// Block represents a block of statements
type Block struct {
	Stmts []*Statement `parser:"'{' @@* '}'"`
}

// Statement can be various kinds of statements
type Statement struct {
	Var    *Variable   `parser:"  @@"`
	Return *ReturnStmt `parser:"| @@"`
	Expr   *Expression `parser:"| @@"`
}

// ReturnStmt represents a return statement
type ReturnStmt struct {
	Value *Expression `parser:"'return' @@?"`
}

// Expression represents an expression (simplified - just basic operations for now)
type Expression struct {
	Comparison *ComparisonExpression `parser:"@@"`
}

// ComparisonExpression handles ==, !=, <, >, <=, >=
type ComparisonExpression struct {
	Left  *AddExpression `parser:"@@"`
	Op    string         `parser:"( @( '=' '=' | '!' '=' | '<' '=' | '>' '=' | '<' | '>' )"`
	Right *AddExpression `parser:"  @@ )?"`
}

// AddExpression handles + and -
type AddExpression struct {
	Left  *MultiplyExpression `parser:"@@"`
	Op    string              `parser:"( @( '+' | '-' )"`
	Right *MultiplyExpression `parser:"  @@ )?"`
}

// MultiplyExpression handles * and /
type MultiplyExpression struct {
	Left  *UnaryExpression `parser:"@@"`
	Op    string           `parser:"( @( '*' | '/' )"`
	Right *UnaryExpression `parser:"  @@ )?"`
}

// UnaryExpression handles !, -, etc.
type UnaryExpression struct {
	Op      string             `parser:"( @( '!' | '-' )"`
	Primary *PrimaryExpression `parser:"  @@ ) | @@"`
}

// PrimaryExpression is the base expression
type PrimaryExpression struct {
	Call    *CallExpression    `parser:"  @@"`        // Try call first (has lookahead)
	Number  *int64             `parser:"| @Int"`
	String  *string            `parser:"| @String"`
	Bool    *bool              `parser:"| ( @'true' | 'false' )"`
	Subexpr *Expression        `parser:"| '(' @@ ')'"`
	Ident   *string            `parser:"| @Ident"`     // Ident last (most general)
}

// CallExpression represents a function call
type CallExpression struct {
	Func string        `parser:"@Ident"`
	Args []*Expression `parser:"'(' ( @@ ( ',' @@ )* )? ')'"`
}

// ============================================================================
// Participle Parser Implementation
// ============================================================================

type participleParser struct {
	parser *participle.Parser[DingoFile]
	mode   Mode
}

func newParticipleParser(mode Mode) Parser {
	// Define custom lexer for Dingo
	dingoLexer := lexer.MustSimple([]lexer.SimpleRule{
		{Name: "Whitespace", Pattern: `[ \t\r\n]+`},
		{Name: "Comment", Pattern: `//[^\n]*`},
		{Name: "String", Pattern: `"[^"]*"`},
		{Name: "Int", Pattern: `\d+`},
		{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
		{Name: "Punct", Pattern: `[{}()\[\],;:?*+\-/!=<>&|]`},
	})

	// Build parser
	p := participle.MustBuild[DingoFile](
		participle.Lexer(dingoLexer),
		participle.Elide("Whitespace", "Comment"),
		participle.UseLookahead(2),
	)

	return &participleParser{
		parser: p,
		mode:   mode,
	}
}

func (p *participleParser) ParseFile(fset *token.FileSet, filename string, src []byte) (*dingoast.File, error) {
	// Parse with participle
	dingoFile, err := p.parser.ParseBytes(filename, src)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Convert participle AST to go/ast
	file := fset.AddFile(filename, -1, len(src))

	return p.convertToGoAST(dingoFile, file), nil
}

func (p *participleParser) ParseExpr(fset *token.FileSet, expr string) (dingoast.DingoNode, error) {
	// For now, wrap in a dummy function to parse
	src := []byte(fmt.Sprintf("package main\nfunc __dummy() { %s }", expr))
	file, err := p.ParseFile(fset, "expr.dingo", src)
	if err != nil {
		return nil, err
	}

	// Extract the expression from the function body
	if len(file.Decls) > 0 {
		if fn, ok := file.Decls[0].(*ast.FuncDecl); ok {
			if fn.Body != nil && len(fn.Body.List) > 0 {
				if exprStmt, ok := fn.Body.List[0].(*ast.ExprStmt); ok {
					// Check if it's a Dingo node
					if dn, ok := file.GetDingoNode(exprStmt.X); ok {
						return dn, nil
					}
					// Return as a placeholder - not a true DingoNode but implements the interface
					return nil, fmt.Errorf("parsed expression is not a Dingo node")
				}
			}
		}
	}

	return nil, fmt.Errorf("failed to extract expression")
}

// ============================================================================
// Participle AST -> go/ast Conversion
// ============================================================================

func (p *participleParser) convertToGoAST(dingoFile *DingoFile, file *token.File) *dingoast.File {
	goFile := &ast.File{
		Name: &ast.Ident{
			Name:    dingoFile.Package,
			NamePos: file.Pos(0),
		},
		Decls: make([]ast.Decl, 0, len(dingoFile.Decls)),
	}

	// Create Dingo file wrapper
	result := dingoast.NewFile(goFile)

	// Convert imports
	for _, imp := range dingoFile.Imports {
		goFile.Decls = append(goFile.Decls, &ast.GenDecl{
			Tok: token.IMPORT,
			Specs: []ast.Spec{
				&ast.ImportSpec{
					Path: &ast.BasicLit{
						Kind:  token.STRING,
						Value: imp.Path,
					},
				},
			},
		})
	}

	// Convert declarations
	for _, decl := range dingoFile.Decls {
		if decl.Func != nil {
			goFile.Decls = append(goFile.Decls, p.convertFunction(decl.Func, file))
		} else if decl.Var != nil {
			goFile.Decls = append(goFile.Decls, p.convertVariable(decl.Var, file))
		}
	}

	return result
}

func (p *participleParser) convertFunction(fn *Function, file *token.File) *ast.FuncDecl {
	funcDecl := &ast.FuncDecl{
		Name: &ast.Ident{Name: fn.Name},
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: make([]*ast.Field, 0, len(fn.Params)),
			},
		},
		Body: p.convertBlock(fn.Body, file),
	}

	// Convert parameters
	for _, param := range fn.Params {
		funcDecl.Type.Params.List = append(funcDecl.Type.Params.List, &ast.Field{
			Names: []*ast.Ident{{Name: param.Name}},
			Type:  p.convertType(param.Type, file),
		})
	}

	// Convert return type
	if fn.Result != nil {
		funcDecl.Type.Results = &ast.FieldList{
			List: []*ast.Field{
				{Type: p.convertType(fn.Result, file)},
			},
		}
	}

	return funcDecl
}

func (p *participleParser) convertVariable(v *Variable, file *token.File) ast.Decl {
	spec := &ast.ValueSpec{
		Names: []*ast.Ident{{Name: v.Name}},
	}

	if v.Type != nil {
		spec.Type = p.convertType(v.Type, file)
	}

	if v.Value != nil {
		spec.Values = []ast.Expr{p.convertExpression(v.Value, file)}
	}

	return &ast.GenDecl{
		Tok:   token.VAR,
		Specs: []ast.Spec{spec},
	}
}

func (p *participleParser) convertBlock(block *Block, file *token.File) *ast.BlockStmt {
	stmts := make([]ast.Stmt, 0, len(block.Stmts))

	for _, stmt := range block.Stmts {
		if stmt.Var != nil {
			stmts = append(stmts, p.convertVarStmt(stmt.Var, file))
		} else if stmt.Return != nil {
			returnStmt := &ast.ReturnStmt{}
			if stmt.Return.Value != nil {
				returnStmt.Results = []ast.Expr{p.convertExpression(stmt.Return.Value, file)}
			}
			stmts = append(stmts, returnStmt)
		} else if stmt.Expr != nil {
			stmts = append(stmts, &ast.ExprStmt{
				X: p.convertExpression(stmt.Expr, file),
			})
		}
	}

	return &ast.BlockStmt{List: stmts}
}

func (p *participleParser) convertVarStmt(v *Variable, file *token.File) *ast.DeclStmt {
	return &ast.DeclStmt{
		Decl: p.convertVariable(v, file),
	}
}

func (p *participleParser) convertType(t *Type, file *token.File) ast.Expr {
	baseType := ast.Expr(&ast.Ident{Name: t.Name})

	if t.Pointer {
		baseType = &ast.StarExpr{X: baseType}
	}

	if t.Array {
		baseType = &ast.ArrayType{Elt: baseType}
	}

	return baseType
}

func (p *participleParser) convertExpression(expr *Expression, file *token.File) ast.Expr {
	return p.convertComparison(expr.Comparison, file)
}

func (p *participleParser) convertComparison(comp *ComparisonExpression, file *token.File) ast.Expr {
	left := p.convertAdd(comp.Left, file)

	if comp.Op != "" {
		return &ast.BinaryExpr{
			X:  left,
			Op: stringToToken(comp.Op),
			Y:  p.convertAdd(comp.Right, file),
		}
	}

	return left
}

func (p *participleParser) convertAdd(add *AddExpression, file *token.File) ast.Expr {
	left := p.convertMultiply(add.Left, file)

	if add.Op != "" {
		return &ast.BinaryExpr{
			X:  left,
			Op: stringToToken(add.Op),
			Y:  p.convertMultiply(add.Right, file),
		}
	}

	return left
}

func (p *participleParser) convertMultiply(mul *MultiplyExpression, file *token.File) ast.Expr {
	left := p.convertUnary(mul.Left, file)

	if mul.Op != "" {
		return &ast.BinaryExpr{
			X:  left,
			Op: stringToToken(mul.Op),
			Y:  p.convertUnary(mul.Right, file),
		}
	}

	return left
}

func (p *participleParser) convertUnary(unary *UnaryExpression, file *token.File) ast.Expr {
	primary := p.convertPrimary(unary.Primary, file)

	if unary.Op != "" {
		return &ast.UnaryExpr{
			Op: stringToToken(unary.Op),
			X:  primary,
		}
	}

	return primary
}

func (p *participleParser) convertPrimary(primary *PrimaryExpression, file *token.File) ast.Expr {
	if primary.Number != nil {
		return &ast.BasicLit{
			Kind:  token.INT,
			Value: strconv.FormatInt(*primary.Number, 10),
		}
	}

	if primary.String != nil {
		return &ast.BasicLit{
			Kind:  token.STRING,
			Value: *primary.String,
		}
	}

	if primary.Bool != nil {
		value := "false"
		if *primary.Bool {
			value = "true"
		}
		return &ast.Ident{Name: value}
	}

	if primary.Ident != nil {
		return &ast.Ident{Name: *primary.Ident}
	}

	if primary.Call != nil {
		call := &ast.CallExpr{
			Fun:  &ast.Ident{Name: primary.Call.Func},
			Args: make([]ast.Expr, 0, len(primary.Call.Args)),
		}
		for _, arg := range primary.Call.Args {
			call.Args = append(call.Args, p.convertExpression(arg, file))
		}
		return call
	}

	if primary.Subexpr != nil {
		return &ast.ParenExpr{
			X: p.convertExpression(primary.Subexpr, file),
		}
	}

	// Fallback
	return &ast.Ident{Name: "nil"}
}

func stringToToken(s string) token.Token {
	switch s {
	case "+":
		return token.ADD
	case "-":
		return token.SUB
	case "*":
		return token.MUL
	case "/":
		return token.QUO
	case "==":
		return token.EQL
	case "!=":
		return token.NEQ
	case "<":
		return token.LSS
	case ">":
		return token.GTR
	case "<=":
		return token.LEQ
	case ">=":
		return token.GEQ
	case "!":
		return token.NOT
	default:
		return token.ILLEGAL
	}
}
