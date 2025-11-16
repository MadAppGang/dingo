// Package ast defines the Abstract Syntax Tree for Dingo language
package ast

import (
	"go/token"
)

// Node is the base interface for all AST nodes
type Node interface {
	Pos() token.Pos    // position of first character belonging to the node
	End() token.Pos    // position of first character immediately after the node
	String() string    // string representation for debugging
}

// Expr represents an expression node
type Expr interface {
	Node
	exprNode()
}

// Stmt represents a statement node
type Stmt interface {
	Node
	stmtNode()
}

// Decl represents a declaration node
type Decl interface {
	Node
	declNode()
}

// ============================================================================
// File and Package
// ============================================================================

// File represents a Dingo source file
type File struct {
	Package    *PackageDecl  // package declaration
	Imports    []*ImportDecl // import declarations
	Decls      []Decl        // top-level declarations
	Comments   []*CommentGroup
	StartPos   token.Pos
	EndPos     token.Pos
}

func (f *File) Pos() token.Pos { return f.StartPos }
func (f *File) End() token.Pos { return f.EndPos }
func (f *File) String() string { return "File" }

// PackageDecl represents a package declaration
type PackageDecl struct {
	Name     *Ident
	StartPos token.Pos
}

func (p *PackageDecl) Pos() token.Pos { return p.StartPos }
func (p *PackageDecl) End() token.Pos { return p.Name.End() }
func (p *PackageDecl) String() string { return "package " + p.Name.Name }
func (p *PackageDecl) declNode()      {}

// ImportDecl represents an import declaration
type ImportDecl struct {
	Path     *BasicLit // string literal
	Alias    *Ident    // optional alias
	StartPos token.Pos
	EndPos   token.Pos
}

func (i *ImportDecl) Pos() token.Pos { return i.StartPos }
func (i *ImportDecl) End() token.Pos { return i.EndPos }
func (i *ImportDecl) String() string { return "import" }
func (i *ImportDecl) declNode()      {}

// ============================================================================
// Declarations
// ============================================================================

// FuncDecl represents a function declaration
type FuncDecl struct {
	Name       *Ident
	TypeParams []*TypeParam  // generic type parameters (for future)
	Params     []*Field      // function parameters
	Results    []*Field      // return types
	Body       *BlockStmt
	StartPos   token.Pos
	EndPos     token.Pos
}

func (f *FuncDecl) Pos() token.Pos { return f.StartPos }
func (f *FuncDecl) End() token.Pos { return f.EndPos }
func (f *FuncDecl) String() string { return "func " + f.Name.Name }
func (f *FuncDecl) declNode()      {}

// VarDecl represents a variable declaration (let/var)
type VarDecl struct {
	Names    []*Ident  // variable names
	Type     TypeExpr  // optional type annotation
	Values   []Expr    // initial values
	Mutable  bool      // true for 'var', false for 'let'
	StartPos token.Pos
	EndPos   token.Pos
}

func (v *VarDecl) Pos() token.Pos { return v.StartPos }
func (v *VarDecl) End() token.Pos { return v.EndPos }
func (v *VarDecl) String() string { return "var/let" }
func (v *VarDecl) declNode()      {}
func (v *VarDecl) stmtNode()      {} // VarDecl can also be a statement

// TypeDecl represents a type declaration
type TypeDecl struct {
	Name     *Ident
	Type     TypeExpr
	StartPos token.Pos
	EndPos   token.Pos
}

func (t *TypeDecl) Pos() token.Pos { return t.StartPos }
func (t *TypeDecl) End() token.Pos { return t.EndPos }
func (t *TypeDecl) String() string { return "type " + t.Name.Name }
func (t *TypeDecl) declNode()      {}

// Field represents a parameter or struct field
type Field struct {
	Name     *Ident   // can be nil for return types
	Type     TypeExpr
	Tags     string   // struct tags (for Go interop)
	StartPos token.Pos
	EndPos   token.Pos
}

func (f *Field) Pos() token.Pos { return f.StartPos }
func (f *Field) End() token.Pos { return f.EndPos }
func (f *Field) String() string {
	if f.Name != nil {
		return f.Name.Name
	}
	return "field"
}

// TypeParam represents a generic type parameter
type TypeParam struct {
	Name       *Ident
	Constraint TypeExpr // optional constraint
	StartPos   token.Pos
	EndPos     token.Pos
}

func (t *TypeParam) Pos() token.Pos { return t.StartPos }
func (t *TypeParam) End() token.Pos { return t.EndPos }
func (t *TypeParam) String() string { return t.Name.Name }

// ============================================================================
// Statements
// ============================================================================

// BlockStmt represents a block of statements
type BlockStmt struct {
	Stmts    []Stmt
	StartPos token.Pos
	EndPos   token.Pos
}

func (b *BlockStmt) Pos() token.Pos { return b.StartPos }
func (b *BlockStmt) End() token.Pos { return b.EndPos }
func (b *BlockStmt) String() string { return "block" }
func (b *BlockStmt) stmtNode()      {}

// ExprStmt represents an expression used as a statement
type ExprStmt struct {
	X Expr
}

func (e *ExprStmt) Pos() token.Pos { return e.X.Pos() }
func (e *ExprStmt) End() token.Pos { return e.X.End() }
func (e *ExprStmt) String() string { return "expr stmt" }
func (e *ExprStmt) stmtNode()      {}

// ReturnStmt represents a return statement
type ReturnStmt struct {
	Results  []Expr
	StartPos token.Pos
	EndPos   token.Pos
}

func (r *ReturnStmt) Pos() token.Pos { return r.StartPos }
func (r *ReturnStmt) End() token.Pos { return r.EndPos }
func (r *ReturnStmt) String() string { return "return" }
func (r *ReturnStmt) stmtNode()      {}

// IfStmt represents an if statement
type IfStmt struct {
	Init     Stmt      // optional initialization statement
	Cond     Expr      // condition
	Body     *BlockStmt
	Else     Stmt      // can be *BlockStmt or *IfStmt
	StartPos token.Pos
	EndPos   token.Pos
}

func (i *IfStmt) Pos() token.Pos { return i.StartPos }
func (i *IfStmt) End() token.Pos { return i.EndPos }
func (i *IfStmt) String() string { return "if" }
func (i *IfStmt) stmtNode()      {}

// ForStmt represents a for loop
type ForStmt struct {
	Init     Stmt      // initialization
	Cond     Expr      // condition
	Post     Stmt      // post iteration
	Body     *BlockStmt
	StartPos token.Pos
	EndPos   token.Pos
}

func (f *ForStmt) Pos() token.Pos { return f.StartPos }
func (f *ForStmt) End() token.Pos { return f.EndPos }
func (f *ForStmt) String() string { return "for" }
func (f *ForStmt) stmtNode()      {}

// AssignStmt represents an assignment statement
type AssignStmt struct {
	Lhs      []Expr    // left-hand side
	Op       token.Token // assignment operator (=, +=, etc.)
	Rhs      []Expr    // right-hand side
	StartPos token.Pos
	EndPos   token.Pos
}

func (a *AssignStmt) Pos() token.Pos { return a.StartPos }
func (a *AssignStmt) End() token.Pos { return a.EndPos }
func (a *AssignStmt) String() string { return "assign" }
func (a *AssignStmt) stmtNode()      {}

// ============================================================================
// Expressions
// ============================================================================

// Ident represents an identifier
type Ident struct {
	Name     string
	StartPos token.Pos
	EndPos   token.Pos
}

func (i *Ident) Pos() token.Pos { return i.StartPos }
func (i *Ident) End() token.Pos { return i.EndPos }
func (i *Ident) String() string { return i.Name }
func (i *Ident) exprNode()      {}

// BasicLit represents a literal value (number, string, bool, etc.)
type BasicLit struct {
	Kind     token.Token // token.INT, token.FLOAT, token.STRING, etc.
	Value    string
	StartPos token.Pos
	EndPos   token.Pos
}

func (b *BasicLit) Pos() token.Pos { return b.StartPos }
func (b *BasicLit) End() token.Pos { return b.EndPos }
func (b *BasicLit) String() string { return b.Value }
func (b *BasicLit) exprNode()      {}

// BinaryExpr represents a binary operation (a + b, a == b, etc.)
type BinaryExpr struct {
	X        Expr
	Op       token.Token
	Y        Expr
	StartPos token.Pos
	EndPos   token.Pos
}

func (b *BinaryExpr) Pos() token.Pos { return b.StartPos }
func (b *BinaryExpr) End() token.Pos { return b.EndPos }
func (b *BinaryExpr) String() string { return "binary expr" }
func (b *BinaryExpr) exprNode()      {}

// UnaryExpr represents a unary operation (!a, -b, etc.)
type UnaryExpr struct {
	Op       token.Token
	X        Expr
	StartPos token.Pos
	EndPos   token.Pos
}

func (u *UnaryExpr) Pos() token.Pos { return u.StartPos }
func (u *UnaryExpr) End() token.Pos { return u.EndPos }
func (u *UnaryExpr) String() string { return "unary expr" }
func (u *UnaryExpr) exprNode()      {}

// CallExpr represents a function call
type CallExpr struct {
	Func     Expr   // function expression
	Args     []Expr // arguments
	StartPos token.Pos
	EndPos   token.Pos
}

func (c *CallExpr) Pos() token.Pos { return c.StartPos }
func (c *CallExpr) End() token.Pos { return c.EndPos }
func (c *CallExpr) String() string { return "call" }
func (c *CallExpr) exprNode()      {}

// SelectorExpr represents a selector (a.b)
type SelectorExpr struct {
	X        Expr
	Sel      *Ident
	StartPos token.Pos
	EndPos   token.Pos
}

func (s *SelectorExpr) Pos() token.Pos { return s.StartPos }
func (s *SelectorExpr) End() token.Pos { return s.EndPos }
func (s *SelectorExpr) String() string { return "selector" }
func (s *SelectorExpr) exprNode()      {}

// IndexExpr represents an index operation (a[i])
type IndexExpr struct {
	X        Expr
	Index    Expr
	StartPos token.Pos
	EndPos   token.Pos
}

func (i *IndexExpr) Pos() token.Pos { return i.StartPos }
func (i *IndexExpr) End() token.Pos { return i.EndPos }
func (i *IndexExpr) String() string { return "index" }
func (i *IndexExpr) exprNode()      {}

// ============================================================================
// Dingo-Specific Expressions (Phase 1 Features)
// ============================================================================

// ErrorPropagationExpr represents the `?` operator (expr?)
type ErrorPropagationExpr struct {
	X        Expr
	StartPos token.Pos
	EndPos   token.Pos
}

func (e *ErrorPropagationExpr) Pos() token.Pos { return e.StartPos }
func (e *ErrorPropagationExpr) End() token.Pos { return e.EndPos }
func (e *ErrorPropagationExpr) String() string { return "error propagation (?)" }
func (e *ErrorPropagationExpr) exprNode()      {}

// NullCoalescingExpr represents the `??` operator (a ?? b)
type NullCoalescingExpr struct {
	X        Expr
	Y        Expr
	StartPos token.Pos
	EndPos   token.Pos
}

func (n *NullCoalescingExpr) Pos() token.Pos { return n.StartPos }
func (n *NullCoalescingExpr) End() token.Pos { return n.EndPos }
func (n *NullCoalescingExpr) String() string { return "null coalescing (??)" }
func (n *NullCoalescingExpr) exprNode()      {}

// TernaryExpr represents the ternary operator (cond ? a : b)
type TernaryExpr struct {
	Cond     Expr
	Then     Expr
	Else     Expr
	StartPos token.Pos
	EndPos   token.Pos
}

func (t *TernaryExpr) Pos() token.Pos { return t.StartPos }
func (t *TernaryExpr) End() token.Pos { return t.EndPos }
func (t *TernaryExpr) String() string { return "ternary (? :)" }
func (t *TernaryExpr) exprNode()      {}

// LambdaExpr represents a lambda function
type LambdaExpr struct {
	Params   []*Field
	Body     Expr      // single expression or BlockStmt
	StartPos token.Pos
	EndPos   token.Pos
}

func (l *LambdaExpr) Pos() token.Pos { return l.StartPos }
func (l *LambdaExpr) End() token.Pos { return l.EndPos }
func (l *LambdaExpr) String() string { return "lambda" }
func (l *LambdaExpr) exprNode()      {}

// ============================================================================
// Type Expressions
// ============================================================================

// TypeExpr represents a type expression
type TypeExpr interface {
	Node
	typeExpr()
}

// TypeIdent represents a simple type identifier (int, string, User, etc.)
type TypeIdent struct {
	Name     string
	StartPos token.Pos
	EndPos   token.Pos
}

func (t *TypeIdent) Pos() token.Pos { return t.StartPos }
func (t *TypeIdent) End() token.Pos { return t.EndPos }
func (t *TypeIdent) String() string { return t.Name }
func (t *TypeIdent) typeExpr()      {}

// ArrayType represents an array or slice type ([]T, [N]T)
type ArrayType struct {
	Len      Expr     // nil for slices
	ElemType TypeExpr
	StartPos token.Pos
	EndPos   token.Pos
}

func (a *ArrayType) Pos() token.Pos { return a.StartPos }
func (a *ArrayType) End() token.Pos { return a.EndPos }
func (a *ArrayType) String() string { return "[]" }
func (a *ArrayType) typeExpr()      {}

// MapType represents a map type (map[K]V)
type MapType struct {
	KeyType   TypeExpr
	ValueType TypeExpr
	StartPos  token.Pos
	EndPos    token.Pos
}

func (m *MapType) Pos() token.Pos { return m.StartPos }
func (m *MapType) End() token.Pos { return m.EndPos }
func (m *MapType) String() string { return "map" }
func (m *MapType) typeExpr()      {}

// FuncType represents a function type
type FuncType struct {
	Params   []*Field
	Results  []*Field
	StartPos token.Pos
	EndPos   token.Pos
}

func (f *FuncType) Pos() token.Pos { return f.StartPos }
func (f *FuncType) End() token.Pos { return f.EndPos }
func (f *FuncType) String() string { return "func type" }
func (f *FuncType) typeExpr()      {}

// PointerType represents a pointer type (*T)
type PointerType struct {
	ElemType TypeExpr
	StartPos token.Pos
	EndPos   token.Pos
}

func (p *PointerType) Pos() token.Pos { return p.StartPos }
func (p *PointerType) End() token.Pos { return p.EndPos }
func (p *PointerType) String() string { return "*" }
func (p *PointerType) typeExpr()      {}

// ResultType represents Result<T, E> type (Dingo-specific)
type ResultType struct {
	ValueType TypeExpr
	ErrorType TypeExpr
	StartPos  token.Pos
	EndPos    token.Pos
}

func (r *ResultType) Pos() token.Pos { return r.StartPos }
func (r *ResultType) End() token.Pos { return r.EndPos }
func (r *ResultType) String() string { return "Result<T, E>" }
func (r *ResultType) typeExpr()      {}

// OptionType represents Option<T> type (Dingo-specific)
type OptionType struct {
	ValueType TypeExpr
	StartPos  token.Pos
	EndPos    token.Pos
}

func (o *OptionType) Pos() token.Pos { return o.StartPos }
func (o *OptionType) End() token.Pos { return o.EndPos }
func (o *OptionType) String() string { return "Option<T>" }
func (o *OptionType) typeExpr()      {}

// ============================================================================
// Comments
// ============================================================================

// Comment represents a single comment
type Comment struct {
	Text     string
	StartPos token.Pos
	EndPos   token.Pos
}

func (c *Comment) Pos() token.Pos { return c.StartPos }
func (c *Comment) End() token.Pos { return c.EndPos }
func (c *Comment) String() string { return c.Text }

// CommentGroup represents a sequence of comments
type CommentGroup struct {
	List []*Comment
}

func (c *CommentGroup) Pos() token.Pos {
	if len(c.List) > 0 {
		return c.List[0].Pos()
	}
	return token.NoPos
}

func (c *CommentGroup) End() token.Pos {
	if len(c.List) > 0 {
		return c.List[len(c.List)-1].End()
	}
	return token.NoPos
}

func (c *CommentGroup) String() string { return "comment group" }
