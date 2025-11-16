// Package ast provides AST traversal utilities
package ast

// Visitor is called for each node during traversal
type Visitor interface {
	Visit(node Node) (w Visitor)
}

// Walk traverses an AST in depth-first order
func Walk(v Visitor, node Node) {
	if v = v.Visit(node); v == nil {
		return
	}

	// Walk children
	switch n := node.(type) {
	case *File:
		if n.Package != nil {
			Walk(v, n.Package)
		}
		for _, imp := range n.Imports {
			Walk(v, imp)
		}
		for _, decl := range n.Decls {
			Walk(v, decl)
		}

	case *FuncDecl:
		Walk(v, n.Name)
		for _, param := range n.Params {
			Walk(v, param)
		}
		for _, result := range n.Results {
			Walk(v, result)
		}
		if n.Body != nil {
			Walk(v, n.Body)
		}

	case *VarDecl:
		for _, name := range n.Names {
			Walk(v, name)
		}
		if n.Type != nil {
			Walk(v, n.Type)
		}
		for _, val := range n.Values {
			Walk(v, val)
		}

	case *BlockStmt:
		for _, stmt := range n.Stmts {
			Walk(v, stmt)
		}

	case *ExprStmt:
		Walk(v, n.X)

	case *ReturnStmt:
		for _, result := range n.Results {
			Walk(v, result)
		}

	case *IfStmt:
		if n.Init != nil {
			Walk(v, n.Init)
		}
		Walk(v, n.Cond)
		Walk(v, n.Body)
		if n.Else != nil {
			Walk(v, n.Else)
		}

	case *ForStmt:
		if n.Init != nil {
			Walk(v, n.Init)
		}
		if n.Cond != nil {
			Walk(v, n.Cond)
		}
		if n.Post != nil {
			Walk(v, n.Post)
		}
		Walk(v, n.Body)

	case *AssignStmt:
		for _, lhs := range n.Lhs {
			Walk(v, lhs)
		}
		for _, rhs := range n.Rhs {
			Walk(v, rhs)
		}

	case *BinaryExpr:
		Walk(v, n.X)
		Walk(v, n.Y)

	case *UnaryExpr:
		Walk(v, n.X)

	case *CallExpr:
		Walk(v, n.Func)
		for _, arg := range n.Args {
			Walk(v, arg)
		}

	case *SelectorExpr:
		Walk(v, n.X)
		Walk(v, n.Sel)

	case *IndexExpr:
		Walk(v, n.X)
		Walk(v, n.Index)

	case *ErrorPropagationExpr:
		Walk(v, n.X)

	case *NullCoalescingExpr:
		Walk(v, n.X)
		Walk(v, n.Y)

	case *TernaryExpr:
		Walk(v, n.Cond)
		Walk(v, n.Then)
		Walk(v, n.Else)

	case *LambdaExpr:
		for _, param := range n.Params {
			Walk(v, param)
		}
		Walk(v, n.Body)

	case *Field:
		if n.Name != nil {
			Walk(v, n.Name)
		}
		if n.Type != nil {
			Walk(v, n.Type)
		}

	// Type expressions
	case *ArrayType:
		if n.Len != nil {
			Walk(v, n.Len)
		}
		Walk(v, n.ElemType)

	case *MapType:
		Walk(v, n.KeyType)
		Walk(v, n.ValueType)

	case *FuncType:
		for _, param := range n.Params {
			Walk(v, param)
		}
		for _, result := range n.Results {
			Walk(v, result)
		}

	case *PointerType:
		Walk(v, n.ElemType)

	case *ResultType:
		Walk(v, n.ValueType)
		Walk(v, n.ErrorType)

	case *OptionType:
		Walk(v, n.ValueType)

	// Leaf nodes (no children to walk)
	case *Ident:
	case *BasicLit:
	case *TypeIdent:
	case *PackageDecl:
	case *ImportDecl:
	case *TypeDecl:
	case *Comment:
	case *CommentGroup:
	}
}

// Inspector is a helper for AST inspection with callback-based traversal
type Inspector func(Node) bool

// Inspect traverses AST and calls f for each node
func Inspect(node Node, f Inspector) {
	Walk(inspector(f), node)
}

type inspector func(Node) bool

func (f inspector) Visit(node Node) Visitor {
	if f(node) {
		return f
	}
	return nil
}
