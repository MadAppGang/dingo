// Package parser implements a participle-based parser for Dingo
package parser

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"unicode"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
	dingoast "github.com/MadAppGang/dingo/pkg/ast"
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

// Declaration can be a function, variable, enum, or type declaration
type Declaration struct {
	Func     *Function     `parser:"  @@"`
	Var      *Variable     `parser:"| @@"`
	Enum     *Enum         `parser:"| @@"`
	TypeDecl *TypeDecl     `parser:"| @@"`
}

// TypeDecl represents a type declaration
type TypeDecl struct {
	Name       string      `parser:"'type' @Ident"`
	Struct     *StructType `parser:"( @@"`         // Struct definition
	Type       *Type       `parser:"  | @@ )"`     // Or regular type
}

// StructType represents a struct type definition
type StructType struct {
	Struct bool          `parser:"@'struct'"`
	Fields []*StructField `parser:"'{' ( @@ )* '}'"`
}

// StructField represents a field in a struct
type StructField struct {
	Name string `parser:"@Ident"`
	Type *Type  `parser:"@@"`
}

// Function represents a function declaration
type Function struct {
	Name        string       `parser:"'func' @Ident"`
	Params      []*Parameter `parser:"'(' ( @@ ( ',' @@ )* )? ')'"`
	HasTupleRet bool         `parser:"@'('?"`  // Detect if return is tuple
	Results     []*Type      `parser:"( @@ ( ',' @@ )* )? ')'?"`  // Capture results (with optional closing paren if tuple)
	Body        *Block       `parser:"@@"`
}

// Variable represents a variable declaration (let/var)
type Variable struct {
	Mutable bool        `parser:"( @'var' | 'let' )"`
	Name    string      `parser:"@Ident"`
	Type    *Type       `parser:"( ':'? @@ )?"`  // Type can be with or without colon
	Value   *Expression `parser:"( '=' @@ )?"`   // Value is optional for var declarations
}

// Parameter represents a function parameter
type Parameter struct {
	Name string `parser:"@Ident"`
	Type *Type  `parser:"':' @@"`
}

// Type represents a type expression
type Type struct {
	MapType    *MapType      `parser:"  @@"`           // Map type
	PointerType *PointerType `parser:"| @@"`           // Pointer type
	ArrayType  *ArrayType    `parser:"| @@"`           // Array/Slice type
	NamedType  *NamedType    `parser:"| @@"`           // Named type (must be last)
}

// MapType represents map[K]V
type MapType struct {
	Map   bool  `parser:"@'map'"`
	Key   *Type `parser:"'[' @@"`
	Value *Type `parser:"']' @@"`
}

// PointerType represents *T
type PointerType struct {
	Star bool  `parser:"@'*'"`
	Type *Type `parser:"@@"`
}

// ArrayType represents []T or [N]T
type ArrayType struct {
	Open  bool   `parser:"@'['"`
	Size  *int64 `parser:"@Int?"`
	Close bool   `parser:"@']'"`
	Elem  *Type  `parser:"@@"`
}

// NamedType represents a named type with optional generic parameters or empty interface
type NamedType struct {
	Name          string  `parser:"@Ident"`
	TypeParams    []*Type `parser:"( '<' @@ ( ',' @@ )* '>' )?"`
	EmptyInterface bool   `parser:"@( '{' '}' )?"`  // Support interface{} syntax
}

// Enum represents an enum declaration (sum type)
type Enum struct {
	Name       string         `parser:"'enum' @Ident"`
	TypeParams []*TypeParam   `parser:"( '<' @@ ( ',' @@ )* '>' )?"`  // Generic type parameters
	Variants   []*Variant     `parser:"'{' ( @@ ( ',' @@ )* ','? )? '}'"`  // Trailing comma allowed
}

// TypeParam represents a generic type parameter
type TypeParam struct {
	Name string `parser:"@Ident"`
}

// Variant represents an enum variant
type Variant struct {
	Name         string        `parser:"@Ident"`
	TupleFields  []*Field      `parser:"( '(' ( @@ ( ',' @@ )* ','? )? ')' )?"`  // Tuple variant: Circle(float64)
	StructFields []*NamedField `parser:"( '{' ( @@ ( ',' @@ )* ','? )? '}' )?"`  // Struct variant: Circle { radius: float64 }
}

// Field represents a field in a tuple variant (just type)
type Field struct {
	Name string `parser:"( @Ident ':' )?"`  // Optional name for tuple fields
	Type *Type  `parser:"@@"`
}

// NamedField represents a named field in a struct variant
type NamedField struct {
	Name string `parser:"@Ident"`
	Type *Type  `parser:"':' @@"`
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
	Values []*Expression `parser:"'return' ( @@ ( ',' @@ )* )?"`
}

// Expression represents an expression with ternary as the lowest precedence
type Expression struct {
	Ternary *TernaryExpression `parser:"@@"`
}

// TernaryExpression handles the ternary operator (? :)
type TernaryExpression struct {
	NullCoalesce *NullCoalesceExpression `parser:"@@"`
	HasTernary   bool                    `parser:"@'?'?"`
	Then         *NullCoalesceExpression `parser:"( @@ ':'"`
	Else         *TernaryExpression      `parser:"  @@ )?"`
}

// NullCoalesceExpression handles the null coalescing operator (??)
// Right-associative: a ?? b ?? c is parsed as a ?? (b ?? c)
type NullCoalesceExpression struct {
	Left  *ComparisonExpression   `parser:"@@"`
	Op    string                  `parser:"( @NullCoalesce"`
	Right *NullCoalesceExpression `parser:"  @@ )?"`
}

// ComparisonExpression handles ==, !=, <, >, <=, >=
type ComparisonExpression struct {
	Left  *AddExpression `parser:"@@"`
	Op    string         `parser:"( @( EqEq | NotEq | LessEq | GreaterEq | '<' | '>' )"`
	Right *AddExpression `parser:"  @@ )?"`
}

// AddExpression handles + and - (left-associative, allows chaining)
type AddExpression struct {
	Left  *MultiplyExpression `parser:"@@"`
	Rest  []*AddOp            `parser:"@@*"`
}

type AddOp struct {
	Op    string              `parser:"@( '+' | '-' )"`
	Right *MultiplyExpression `parser:"@@"`
}

// MultiplyExpression handles *, /, and % (left-associative, allows chaining)
type MultiplyExpression struct {
	Left  *UnaryExpression `parser:"@@"`
	Rest  []*MultiplyOp    `parser:"@@*"`
}

type MultiplyOp struct {
	Op    string           `parser:"@( '*' | '/' | '%' )"`
	Right *UnaryExpression `parser:"@@"`
}

// UnaryExpression handles !, -, &, *, etc.
type UnaryExpression struct {
	Op      string              `parser:"( @( '!' | '-' | '&' | '*' )"`
	Postfix *PostfixExpression  `parser:"  @@ ) | @@"`
}

// PrimaryExpression is the base expression
type PrimaryExpression struct {
	Match       *Match             `parser:"  @@"`        // Match expression
	Lambda      *LambdaExpression  `parser:"| @@"`        // Lambda expression
	Composite   *CompositeLit      `parser:"| @@"`        // Composite literal (e.g., &User{...}, []int{...})
	TypeCast    *TypeCast          `parser:"| @@"`        // Type cast (e.g., string(data))
	Call        *CallExpression    `parser:"| @@"`        // Try call (has lookahead)
	Number      *int64             `parser:"| @Int"`
	String      *string            `parser:"| @String"`
	Bool        *bool              `parser:"| ( @'true' | 'false' )"`
	Subexpr     *Expression        `parser:"| '(' @@ ')'"`
	Ident       *string            `parser:"| @Ident"`     // Ident last (most general)
}

// CompositeLit represents composite literals like User{...} or []int{1,2,3}
type CompositeLit struct {
	Type     *Type                `parser:"@@"`
	Elements []*CompositeLitElem  `parser:"'{' ( @@ ( ',' @@ )* ','? )? '}'"`
}

// CompositeLitElem represents an element in a composite literal
type CompositeLitElem struct {
	Key   *string     `parser:"( @Ident ':'"`  // Optional key for struct literals
	Value *Expression `parser:"  @@ ) | @@"`   // Value (required)
}

// TypeCast represents a type conversion like string(data)
type TypeCast struct {
	Type *Type       `parser:"@@"`
	Arg  *Expression `parser:"'(' @@ ')'"`
}

// LambdaExpression represents a lambda function
// Supports both Rust-style |x| expr and arrow-style (x) => expr
type LambdaExpression struct {
	// Rust-style: |x, y| x + y
	RustParams  []*LambdaParam  `parser:"( '|' ( @@ ( ',' @@ )* )? '|'"`
	RustBody    *Expression     `parser:"  @@ )"`
	// Arrow-style: (x, y) => x + y
	ArrowParams []*LambdaParam  `parser:"| ( '(' ( @@ ( ',' @@ )* )? ')' Arrow"`
	ArrowBody   *Expression     `parser:"  @@ )"`
}

// LambdaParam represents a lambda parameter (name with optional type)
type LambdaParam struct {
	Name string `parser:"@Ident"`
	Type *Type  `parser:"( ':' @@ )?"`  // Optional type annotation
}

// Match represents a match expression
type Match struct {
	Expr *Expression `parser:"'match' @@"`
	Arms []*MatchArm `parser:"'{' ( @@ ( ',' )? )+ '}'"`  // One or more arms, optional trailing comma
}

// MatchArm represents a single arm of a match expression
type MatchArm struct {
	Pattern *MatchPattern `parser:"@@"`
	Guard   *Expression   `parser:"( 'if' @@ )?"`  // Optional guard
	Body    *Expression   `parser:"'=' '>' @@"`    // Use => for match arms
}

// MatchPattern represents a pattern in a match arm
type MatchPattern struct {
	Wildcard     bool              `parser:"@'_'"`                                     // Wildcard pattern
	VariantName  string            `parser:"| @Ident"`                                 // Variant or unit pattern
	TupleFields  []*PatternBinding `parser:"( '(' ( @@ ( ',' @@ )* ','? )? ')' )?"`  // Tuple destructuring
	StructFields []*NamedPatternBinding `parser:"( '{' ( @@ ( ',' @@ )* ','? )? '}' )?"`  // Struct destructuring
}

// PatternBinding represents a binding in a tuple pattern
type PatternBinding struct {
	Name string `parser:"@Ident"`  // Variable to bind to
}

// NamedPatternBinding represents a named binding in a struct pattern
type NamedPatternBinding struct {
	FieldName string `parser:"@Ident"`
	Binding   string `parser:"( ':' @Ident )?"`  // Optional explicit binding (defaults to field name)
}

// PostfixExpression handles postfix operators like ?, ?., and method calls
type PostfixExpression struct {
	Primary         *PrimaryExpression `parser:"@@"`
	PostfixOps      []*PostfixOp       `parser:"@@*"` // Zero or more postfix operations
}

// PostfixOp represents a postfix operation (method call, safe nav, error propagation)
type PostfixOp struct {
	SafeNav        *SafeNavOp    `parser:"  @@"`       // Safe navigation ?.field
	MethodCall     *MethodCall   `parser:"| @@"`       // Method call .method()
	ErrorPropagate *ErrorPropOp  `parser:"| @@"`       // Error propagation ?
}

// SafeNavOp represents safe navigation operator
type SafeNavOp struct {
	Op    string  `parser:"@SafeNav"`
	Field string  `parser:"@Ident"`
}

// ErrorPropOp represents error propagation operator
type ErrorPropOp struct {
	Op      string  `parser:"@'?'"`
	Message *string `parser:"( @String )?"`  // Optional error message
}

// MethodCall represents a method call like .map(fn)
type MethodCall struct {
	Dot    string         `parser:"@'.'"`
	Method string         `parser:"@Ident"`
	Args   []*Expression  `parser:"'(' ( @@ ( ',' @@ )* )? ')'"`
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
	parser      *participle.Parser[DingoFile]
	mode        Mode
	currentFile *dingoast.File // Thread-safe: instance variable instead of global
}

func newParticipleParser(mode Mode) Parser {
	// Define custom lexer for Dingo
	// IMPORTANT: Order matters! Longer patterns must come before shorter ones
	dingoLexer := lexer.MustSimple([]lexer.SimpleRule{
		{Name: "Whitespace", Pattern: `[ \t\r\n]+`},
		{Name: "Comment", Pattern: `//[^\n]*`},
		{Name: "String", Pattern: `"(?:[^"\\]|\\.)*"`}, // Support escape sequences like \", \\, \n, etc.
		{Name: "Int", Pattern: `\d+`},
		{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
		// Multi-character operators (must come before single-char Punct)
		{Name: "SafeNav", Pattern: `\?\.`},    // Safe navigation ?.
		{Name: "NullCoalesce", Pattern: `\?\?`}, // Null coalescing ??
		{Name: "EqEq", Pattern: `==`},
		{Name: "NotEq", Pattern: `!=`},
		{Name: "LessEq", Pattern: `<=`},
		{Name: "GreaterEq", Pattern: `>=`},
		{Name: "Arrow", Pattern: `=>`},        // Arrow function =>
		// Single-character punctuation (after multi-char operators)
		{Name: "Punct", Pattern: `[{}()\[\],;:?*+\-/!=<>&|.%]`},
	})

	// Build parser
	p := participle.MustBuild[DingoFile](
		participle.Lexer(dingoLexer),
		participle.Elide("Whitespace", "Comment"),
		participle.UseLookahead(4), // Increased for ternary operator
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

	// Set instance context for tracking Dingo nodes during conversion
	p.currentFile = result

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
		} else if decl.Enum != nil {
			// CRITICAL FIX #4: Enum declarations are Dingo-specific - store as placeholder
			// Generate a valid placeholder type to prevent go/types crashes
			enumDecl := p.convertEnum(decl.Enum, file)

			// Create a placeholder type declaration with a comment marker
			// This will be replaced by the sum_types plugin with the actual tagged union
			placeholder := &ast.GenDecl{
				Tok: token.TYPE,
				Specs: []ast.Spec{
					&ast.TypeSpec{
						Name: &ast.Ident{Name: enumDecl.Name.Name + "__PLACEHOLDER"},
						Type: &ast.StructType{
							Fields: &ast.FieldList{List: []*ast.Field{}},
						},
					},
				},
			}
			goFile.Decls = append(goFile.Decls, placeholder)
			// Register the enum as a Dingo node
			result.AddDingoNode(placeholder, enumDecl)
		} else if decl.TypeDecl != nil {
			// Regular type declarations convert directly to Go GenDecl
			goFile.Decls = append(goFile.Decls, p.convertTypeDecl(decl.TypeDecl, file))
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

	// Convert return types
	if len(fn.Results) > 0 {
		funcDecl.Type.Results = &ast.FieldList{
			List: make([]*ast.Field, 0, len(fn.Results)),
		}
		for _, result := range fn.Results {
			funcDecl.Type.Results.List = append(funcDecl.Type.Results.List, &ast.Field{
				Type: p.convertType(result, file),
			})
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

func (p *participleParser) convertTypeDecl(td *TypeDecl, file *token.File) ast.Decl {
	var typeExpr ast.Expr

	if td.Struct != nil {
		// Convert struct type
		fields := make([]*ast.Field, 0, len(td.Struct.Fields))
		for _, f := range td.Struct.Fields {
			fields = append(fields, &ast.Field{
				Names: []*ast.Ident{{Name: f.Name}},
				Type:  p.convertType(f.Type, file),
			})
		}
		typeExpr = &ast.StructType{
			Fields: &ast.FieldList{List: fields},
		}
	} else {
		// Convert regular type
		typeExpr = p.convertType(td.Type, file)
	}

	return &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{
			&ast.TypeSpec{
				Name: &ast.Ident{Name: td.Name},
				Type: typeExpr,
			},
		},
	}
}

func (p *participleParser) convertBlock(block *Block, file *token.File) *ast.BlockStmt {
	stmts := make([]ast.Stmt, 0, len(block.Stmts))

	for _, stmt := range block.Stmts {
		if stmt.Var != nil {
			stmts = append(stmts, p.convertVarStmt(stmt.Var, file))
		} else if stmt.Return != nil {
			returnStmt := &ast.ReturnStmt{}
			if len(stmt.Return.Values) > 0 {
				returnStmt.Results = make([]ast.Expr, 0, len(stmt.Return.Values))
				for _, val := range stmt.Return.Values {
					returnStmt.Results = append(returnStmt.Results, p.convertExpression(val, file))
				}
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
	if t.MapType != nil {
		return &ast.MapType{
			Key:   p.convertType(t.MapType.Key, file),
			Value: p.convertType(t.MapType.Value, file),
		}
	}

	if t.PointerType != nil {
		return &ast.StarExpr{
			X: p.convertType(t.PointerType.Type, file),
		}
	}

	if t.ArrayType != nil {
		var lenExpr ast.Expr
		if t.ArrayType.Size != nil {
			lenExpr = &ast.BasicLit{
				Kind:  token.INT,
				Value: fmt.Sprintf("%d", *t.ArrayType.Size),
			}
		}
		return &ast.ArrayType{
			Len: lenExpr,
			Elt: p.convertType(t.ArrayType.Elem, file),
		}
	}

	if t.NamedType != nil {
		// Handle generic type parameters (e.g., Result<T, E>)
		if len(t.NamedType.TypeParams) > 0 {
			// Construct composite name like Result_T_E
			typeName := t.NamedType.Name
			for _, param := range t.NamedType.TypeParams {
				typeName += "_" + p.typeToString(param)
			}
			return &ast.Ident{Name: typeName}
		}
		return &ast.Ident{Name: t.NamedType.Name}
	}

	// Fallback (should not reach here)
	return &ast.Ident{Name: "unknown"}
}

// Helper to convert a Type to a string representation for naming
func (p *participleParser) typeToString(t *Type) string {
	if t.MapType != nil {
		return fmt.Sprintf("map_%s_%s",
			p.typeToString(t.MapType.Key),
			p.typeToString(t.MapType.Value))
	}
	if t.PointerType != nil {
		return "ptr_" + p.typeToString(t.PointerType.Type)
	}
	if t.ArrayType != nil {
		if t.ArrayType.Size != nil {
			return fmt.Sprintf("array%d_%s", *t.ArrayType.Size, p.typeToString(t.ArrayType.Elem))
		}
		return "slice_" + p.typeToString(t.ArrayType.Elem)
	}
	if t.NamedType != nil {
		return t.NamedType.Name
	}
	return "unknown"
}

func (p *participleParser) convertExpression(expr *Expression, file *token.File) ast.Expr {
	return p.convertTernary(expr.Ternary, file)
}

func (p *participleParser) convertTernary(ternary *TernaryExpression, file *token.File) ast.Expr {
	// First evaluate null coalescing
	nullCoalesce := p.convertNullCoalesce(ternary.NullCoalesce, file)

	// Check if this is a ternary expression
	if ternary.HasTernary && ternary.Then != nil && ternary.Else != nil {
		// Create TernaryExpr Dingo node
		ternaryExpr := &dingoast.TernaryExpr{
			Cond:     nullCoalesce,
			Question: file.Pos(0), // Placeholder position
			Then:     p.convertNullCoalesce(ternary.Then, file),
			Colon:    file.Pos(0), // Placeholder position
			Else:     p.convertTernary(ternary.Else, file),
		}

		// Create placeholder expression
		placeholder := &ast.ParenExpr{X: ternaryExpr.Cond}

		// Track this Dingo node
		if p.currentFile != nil {
			p.currentFile.AddDingoNode(placeholder, ternaryExpr)
		}

		return placeholder
	}

	return nullCoalesce
}

func (p *participleParser) convertNullCoalesce(nc *NullCoalesceExpression, file *token.File) ast.Expr {
	left := p.convertComparison(nc.Left, file)

	if nc.Op != "" && nc.Right != nil {
		// Create NullCoalescingExpr Dingo node
		ncExpr := &dingoast.NullCoalescingExpr{
			X:     left,
			OpPos: file.Pos(0), // Placeholder position
			Y:     p.convertNullCoalesce(nc.Right, file), // Recursive for right-associativity
		}

		// Create placeholder expression
		placeholder := &ast.ParenExpr{X: left}

		// Track this Dingo node
		if p.currentFile != nil {
			p.currentFile.AddDingoNode(placeholder, ncExpr)
		}

		return placeholder
	}

	return left
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
	result := p.convertMultiply(add.Left, file)

	// Handle chained additions/subtractions (left-associative)
	for _, op := range add.Rest {
		result = &ast.BinaryExpr{
			X:  result,
			Op: stringToToken(op.Op),
			Y:  p.convertMultiply(op.Right, file),
		}
	}

	return result
}

func (p *participleParser) convertMultiply(mul *MultiplyExpression, file *token.File) ast.Expr {
	result := p.convertUnary(mul.Left, file)

	// Handle chained multiplications/divisions/modulos (left-associative)
	for _, op := range mul.Rest {
		result = &ast.BinaryExpr{
			X:  result,
			Op: stringToToken(op.Op),
			Y:  p.convertUnary(op.Right, file),
		}
	}

	return result
}

func (p *participleParser) convertUnary(unary *UnaryExpression, file *token.File) ast.Expr {
	postfix := p.convertPostfix(unary.Postfix, file)

	if unary.Op != "" {
		return &ast.UnaryExpr{
			Op: stringToToken(unary.Op),
			X:  postfix,
		}
	}

	return postfix
}

func (p *participleParser) convertPostfix(postfix *PostfixExpression, file *token.File) ast.Expr {
	expr := p.convertPrimary(postfix.Primary, file)

	// Process postfix operations in order
	for _, op := range postfix.PostfixOps {
		if op.SafeNav != nil {
			// Safe navigation operator
			safeNavExpr := &dingoast.SafeNavigationExpr{
				X:     expr,
				OpPos: file.Pos(0), // Placeholder position
				Sel:   &ast.Ident{Name: op.SafeNav.Field},
			}

			// Create placeholder expression
			placeholder := &ast.SelectorExpr{
				X:   expr,
				Sel: &ast.Ident{Name: op.SafeNav.Field},
			}

			// Track this Dingo node
			if p.currentFile != nil {
				p.currentFile.AddDingoNode(placeholder, safeNavExpr)
			}

			expr = placeholder

		} else if op.MethodCall != nil {
			// Method call
			selector := &ast.SelectorExpr{
				X:   expr,
				Sel: &ast.Ident{Name: op.MethodCall.Method},
			}

			// Convert arguments
			args := make([]ast.Expr, 0, len(op.MethodCall.Args))
			for _, arg := range op.MethodCall.Args {
				args = append(args, p.convertExpression(arg, file))
			}

			// Create call expression
			expr = &ast.CallExpr{
				Fun:  selector,
				Args: args,
			}

		} else if op.ErrorPropagate != nil {
			// Error propagation operator
			errExpr := &dingoast.ErrorPropagationExpr{
				X:      expr,
				OpPos:  expr.End(), // Position after expression
				Syntax: dingoast.SyntaxQuestion,
			}

			// Capture error message if provided
			if op.ErrorPropagate.Message != nil {
				// Remove quotes from the string literal
				msg := *op.ErrorPropagate.Message
				if len(msg) >= 2 && msg[0] == '"' && msg[len(msg)-1] == '"' {
					errExpr.Message = msg[1 : len(msg)-1]
				} else {
					errExpr.Message = msg
				}
				errExpr.MessagePos = expr.End() + 1 // Position after '?'
			}

			// Track this Dingo node
			if p.currentFile != nil {
				p.currentFile.AddDingoNode(expr, errExpr)
			}

			// Keep expr as-is (placeholder)
		}
	}

	return expr
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

	if primary.Lambda != nil {
		return p.convertLambda(primary.Lambda, file)
	}

	if primary.Composite != nil {
		// Convert composite literal
		elements := make([]ast.Expr, 0, len(primary.Composite.Elements))
		for _, elem := range primary.Composite.Elements {
			if elem.Key != nil {
				// Key-value element (struct literal)
				elements = append(elements, &ast.KeyValueExpr{
					Key:   &ast.Ident{Name: *elem.Key},
					Value: p.convertExpression(elem.Value, file),
				})
			} else {
				// Value-only element (array/slice literal)
				elements = append(elements, p.convertExpression(elem.Value, file))
			}
		}
		return &ast.CompositeLit{
			Type: p.convertType(primary.Composite.Type, file),
			Elts: elements,
		}
	}

	if primary.TypeCast != nil {
		// Convert type cast (type conversion in Go)
		return &ast.CallExpr{
			Fun:  p.convertType(primary.TypeCast.Type, file),
			Args: []ast.Expr{p.convertExpression(primary.TypeCast.Arg, file)},
		}
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

	if primary.Match != nil {
		// Match expressions are Dingo-specific
		matchExpr := p.convertMatch(primary.Match, file)
		// Create a placeholder call expression
		placeholder := &ast.CallExpr{
			Fun: &ast.Ident{Name: "__match"},
		}
		// Register as Dingo node
		if p.currentFile != nil {
			p.currentFile.AddDingoNode(placeholder, matchExpr)
		}
		return placeholder
	}

	// Fallback
	return &ast.Ident{Name: "nil"}
}

func (p *participleParser) convertLambda(lambda *LambdaExpression, file *token.File) ast.Expr {
	// Determine which style was used
	var params []*LambdaParam
	var body *Expression

	if lambda.RustBody != nil {
		params = lambda.RustParams
		body = lambda.RustBody
	} else if lambda.ArrowBody != nil {
		params = lambda.ArrowParams
		body = lambda.ArrowBody
	} else {
		// Invalid lambda
		return &ast.Ident{Name: "nil"}
	}

	// Create parameter field list
	fieldList := &ast.FieldList{
		List: make([]*ast.Field, 0, len(params)),
	}

	for _, param := range params {
		field := &ast.Field{
			Names: []*ast.Ident{{Name: param.Name}},
		}
		if param.Type != nil {
			field.Type = p.convertType(param.Type, file)
		}
		fieldList.List = append(fieldList.List, field)
	}

	// Create LambdaExpr Dingo node
	lambdaExpr := &dingoast.LambdaExpr{
		Pipe:   file.Pos(0), // Placeholder position
		Params: fieldList,
		Arrow:  file.Pos(0), // Placeholder position
		Body:   p.convertExpression(body, file),
		Rpipe:  file.Pos(0), // Placeholder position
	}

	// Create placeholder function literal
	placeholder := &ast.FuncLit{
		Type: &ast.FuncType{
			Params: fieldList,
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{lambdaExpr.Body},
				},
			},
		},
	}

	// Track this Dingo node
	if p.currentFile != nil {
		p.currentFile.AddDingoNode(placeholder, lambdaExpr)
	}

	return placeholder
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
	case "%":
		return token.REM
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

// ============================================================================
// Sum Types Conversion Functions
// ============================================================================

func (p *participleParser) convertEnum(enum *Enum, file *token.File) *dingoast.EnumDecl {
	enumDecl := &dingoast.EnumDecl{
		Enum:     file.Pos(0), // Placeholder position
		Name:     &ast.Ident{Name: enum.Name},
		Variants: make([]*dingoast.VariantDecl, 0, len(enum.Variants)),
	}

	// Convert type parameters if present
	if len(enum.TypeParams) > 0 {
		enumDecl.TypeParams = &ast.FieldList{
			List: make([]*ast.Field, 0, len(enum.TypeParams)),
		}
		for _, tp := range enum.TypeParams {
			enumDecl.TypeParams.List = append(enumDecl.TypeParams.List, &ast.Field{
				Names: []*ast.Ident{{Name: tp.Name}},
			})
		}
	}

	// Convert variants
	for _, v := range enum.Variants {
		variant := p.convertVariant(v, file)
		enumDecl.Variants = append(enumDecl.Variants, variant)
	}

	return enumDecl
}

func (p *participleParser) convertVariant(variant *Variant, file *token.File) *dingoast.VariantDecl {
	// Validate variant name is capitalized (Go export convention)
	// TODO: Add proper validation error reporting
	if variant.Name != "" && !unicode.IsUpper(rune(variant.Name[0])) {
		// Variant names should be capitalized for Go export
		// This will be caught by further validation or compilation
	}

	v := &dingoast.VariantDecl{
		Name: &ast.Ident{Name: variant.Name},
	}

	// Determine variant kind and convert fields
	if len(variant.TupleFields) > 0 {
		// Tuple variant
		v.Kind = dingoast.VariantTuple
		v.Fields = &ast.FieldList{
			List: make([]*ast.Field, 0, len(variant.TupleFields)),
		}
		for i, f := range variant.TupleFields {
			field := &ast.Field{
				Type: p.convertType(f.Type, file),
			}
			// Use consistent naming: just the index for tuple fields
			// The plugin will prefix with variantname_ later
			if f.Name != "" {
				field.Names = []*ast.Ident{{Name: f.Name}}
			} else {
				syntheticName := "_" + strconv.Itoa(i)
				field.Names = []*ast.Ident{{Name: syntheticName}}
			}
			v.Fields.List = append(v.Fields.List, field)
		}
	} else if len(variant.StructFields) > 0 {
		// Struct variant
		v.Kind = dingoast.VariantStruct
		v.Fields = &ast.FieldList{
			List: make([]*ast.Field, 0, len(variant.StructFields)),
		}
		for _, f := range variant.StructFields {
			v.Fields.List = append(v.Fields.List, &ast.Field{
				Names: []*ast.Ident{{Name: f.Name}},
				Type:  p.convertType(f.Type, file),
			})
		}
	} else {
		// Unit variant
		v.Kind = dingoast.VariantUnit
	}

	return v
}

func (p *participleParser) convertMatch(match *Match, file *token.File) *dingoast.MatchExpr {
	matchExpr := &dingoast.MatchExpr{
		Match: file.Pos(0), // Placeholder position
		Expr:  p.convertExpression(match.Expr, file),
		Arms:  make([]*dingoast.MatchArm, 0, len(match.Arms)),
	}

	for _, arm := range match.Arms {
		matchExpr.Arms = append(matchExpr.Arms, p.convertMatchArm(arm, file))
	}

	return matchExpr
}

func (p *participleParser) convertMatchArm(arm *MatchArm, file *token.File) *dingoast.MatchArm {
	ma := &dingoast.MatchArm{
		Pattern: p.convertPattern(arm.Pattern, file),
		Arrow:   file.Pos(0), // Placeholder position
		Body:    p.convertExpression(arm.Body, file),
	}

	if arm.Guard != nil {
		ma.Guard = p.convertExpression(arm.Guard, file)
	}

	return ma
}

func (p *participleParser) convertPattern(pattern *MatchPattern, file *token.File) *dingoast.Pattern {
	pat := &dingoast.Pattern{
		PatternPos: file.Pos(0), // Placeholder position
	}

	if pattern.Wildcard {
		// Wildcard pattern
		pat.Wildcard = true
		pat.Kind = dingoast.PatternWildcard
	} else if len(pattern.TupleFields) > 0 {
		// Tuple pattern
		pat.Kind = dingoast.PatternTuple
		pat.Variant = &ast.Ident{Name: pattern.VariantName}
		pat.Fields = make([]*dingoast.FieldPattern, 0, len(pattern.TupleFields))
		for _, b := range pattern.TupleFields {
			pat.Fields = append(pat.Fields, &dingoast.FieldPattern{
				Binding: &ast.Ident{Name: b.Name},
			})
		}
	} else if len(pattern.StructFields) > 0 {
		// Struct pattern
		pat.Kind = dingoast.PatternStruct
		pat.Variant = &ast.Ident{Name: pattern.VariantName}
		pat.Fields = make([]*dingoast.FieldPattern, 0, len(pattern.StructFields))
		for _, b := range pattern.StructFields {
			binding := b.Binding
			if binding == "" {
				// If no explicit binding, use field name
				binding = b.FieldName
			}
			pat.Fields = append(pat.Fields, &dingoast.FieldPattern{
				FieldName: &ast.Ident{Name: b.FieldName},
				Binding:   &ast.Ident{Name: binding},
			})
		}
	} else {
		// Unit pattern
		pat.Kind = dingoast.PatternUnit
		pat.Variant = &ast.Ident{Name: pattern.VariantName}
	}

	return pat
}
