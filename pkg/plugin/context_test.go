package plugin

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

const packageMainSrc = "package main"

func TestContext_ReportError(t *testing.T) {
	ctx := &Context{}

	// Initially, no errors
	if ctx.HasErrors() {
		t.Error("HasErrors() should be false initially")
	}

	// Report an error
	ctx.ReportError("test error 1", token.Pos(10))

	if !ctx.HasErrors() {
		t.Error("HasErrors() should be true after reporting error")
	}

	errors := ctx.GetErrors()
	if len(errors) != 1 {
		t.Errorf("GetErrors() returned %d errors, want 1", len(errors))
	}

	// Report another error
	ctx.ReportError("test error 2", token.Pos(20))

	errors = ctx.GetErrors()
	if len(errors) != 2 {
		t.Errorf("GetErrors() returned %d errors, want 2", len(errors))
	}
}

func TestContext_GetErrors_Empty(t *testing.T) {
	ctx := &Context{}

	errors := ctx.GetErrors()
	if errors == nil {
		t.Error("GetErrors() should return empty slice, not nil")
	}
	if len(errors) != 0 {
		t.Errorf("GetErrors() returned %d errors, want 0", len(errors))
	}
}

func TestContext_ClearErrors(t *testing.T) {
	ctx := &Context{}

	// Add some errors
	ctx.ReportError("error 1", token.Pos(10))
	ctx.ReportError("error 2", token.Pos(20))

	if !ctx.HasErrors() {
		t.Error("HasErrors() should be true")
	}

	// Clear errors
	ctx.ClearErrors()

	if ctx.HasErrors() {
		t.Error("HasErrors() should be false after ClearErrors()")
	}

	errors := ctx.GetErrors()
	if len(errors) != 0 {
		t.Errorf("GetErrors() returned %d errors after clear, want 0", len(errors))
	}
}

func TestContext_NextTempVar(t *testing.T) {
	ctx := &Context{}

	// First call should return tmp (no number - "No-Number-First Pattern")
	name1 := ctx.NextTempVar()
	if name1 != "tmp" {
		t.Errorf("NextTempVar() = %q, want %q", name1, "tmp")
	}

	// Counter should be 2 (initialized to 1, then incremented to 2)
	if ctx.TempVarCounter != 2 {
		t.Errorf("TempVarCounter = %d, want 2", ctx.TempVarCounter)
	}

	// Second call should return tmp1
	name2 := ctx.NextTempVar()
	if name2 != "tmp1" {
		t.Errorf("NextTempVar() = %q, want %q", name2, "tmp1")
	}

	// Third call should return tmp2
	name3 := ctx.NextTempVar()
	if name3 != "tmp2" {
		t.Errorf("NextTempVar() = %q, want %q", name3, "tmp2")
	}

	// Counter should be 4 (after 3 calls: init to 1, then 2, 3, 4)
	if ctx.TempVarCounter != 4 {
		t.Errorf("TempVarCounter = %d, want 4", ctx.TempVarCounter)
	}
}

func TestContext_NextTempVar_UniqueNames(t *testing.T) {
	ctx := &Context{}

	// Generate 10 temp var names
	names := make(map[string]bool)
	for i := 0; i < 10; i++ {
		name := ctx.NextTempVar()
		if names[name] {
			t.Errorf("Duplicate temp var name: %s", name)
		}
		names[name] = true
	}

	// Should have 10 unique names
	if len(names) != 10 {
		t.Errorf("Generated %d unique names, want 10", len(names))
	}
}

func TestContext_ErrorsWithLocation(t *testing.T) {
	ctx := &Context{}

	pos1 := token.Pos(100)
	pos2 := token.Pos(200)

	ctx.ReportError("error at pos 100", pos1)
	ctx.ReportError("error at pos 200", pos2)

	errors := ctx.GetErrors()
	if len(errors) != 2 {
		t.Fatalf("Expected 2 errors, got %d", len(errors))
	}

	// Check that error messages contain position info
	err1Str := errors[0].Error()
	if !contains(err1Str, "100") {
		t.Errorf("Error message missing position 100: %s", err1Str)
	}

	err2Str := errors[1].Error()
	if !contains(err2Str, "200") {
		t.Errorf("Error message missing position 200: %s", err2Str)
	}
}

// Helper function to check substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// PHASE 4 - Task B: AST Parent Tracking Tests

func TestContext_BuildParentMap(t *testing.T) {
	ctx := &Context{}

	// Parse a simple Go file
	src := `package main

func main() {
	x := 42
	println(x)
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse source: %v", err)
	}

	// Build parent map
	ctx.BuildParentMap(file)

	// Verify parent map was created
	if ctx.parentMap == nil {
		t.Fatal("BuildParentMap() did not create parent map")
	}

	// Verify root node (file) has no parent
	if parent := ctx.GetParent(file); parent != nil {
		t.Errorf("File node should have no parent, got %T", parent)
	}

	// Find the main function declaration
	var mainFunc *ast.FuncDecl
	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "main" {
			mainFunc = fn
			return false
		}
		return true
	})

	if mainFunc == nil {
		t.Fatal("Could not find main function")
	}

	// Verify main function's parent is the file
	parent := ctx.GetParent(mainFunc)
	if parent != file {
		t.Errorf("main function's parent should be file, got %T", parent)
	}
}

func TestContext_GetParent_NilMap(t *testing.T) {
	ctx := &Context{}
	// Parent map not built yet

	src := packageMainSrc
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse source: %v", err)
	}

	// GetParent should return nil when parent map not built
	parent := ctx.GetParent(file)
	if parent != nil {
		t.Errorf("GetParent() should return nil when parent map not built, got %T", parent)
	}
}

func TestContext_GetParent_VariousNodeTypes(t *testing.T) {
	ctx := &Context{}

	// Parse more complex code with various node types
	src := `package main

import "fmt"

type Person struct {
	Name string
	Age  int
}

func (p *Person) String() string {
	return fmt.Sprintf("%s (%d)", p.Name, p.Age)
}

func main() {
	x := 42
	y := x + 10

	if y > 50 {
		println("big")
	} else {
		println("small")
	}

	for i := 0; i < 10; i++ {
		println(i)
	}
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse source: %v", err)
	}

	ctx.BuildParentMap(file)

	// Test cases: find specific nodes and verify their parents
	tests := []struct {
		name       string
		findNode   func(*ast.File) ast.Node
		parentType string
	}{
		{
			name: "AssignStmt parent is BlockStmt",
			findNode: func(f *ast.File) ast.Node {
				var assignStmt *ast.AssignStmt
				ast.Inspect(f, func(n ast.Node) bool {
					if as, ok := n.(*ast.AssignStmt); ok && assignStmt == nil {
						// Get first assignment: x := 42
						assignStmt = as
						return false
					}
					return true
				})
				return assignStmt
			},
			parentType: "*ast.BlockStmt",
		},
		{
			name: "IfStmt parent is BlockStmt",
			findNode: func(f *ast.File) ast.Node {
				var ifStmt *ast.IfStmt
				ast.Inspect(f, func(n ast.Node) bool {
					if is, ok := n.(*ast.IfStmt); ok {
						ifStmt = is
						return false
					}
					return true
				})
				return ifStmt
			},
			parentType: "*ast.BlockStmt",
		},
		{
			name: "ForStmt parent is BlockStmt",
			findNode: func(f *ast.File) ast.Node {
				var forStmt *ast.ForStmt
				ast.Inspect(f, func(n ast.Node) bool {
					if fs, ok := n.(*ast.ForStmt); ok {
						forStmt = fs
						return false
					}
					return true
				})
				return forStmt
			},
			parentType: "*ast.BlockStmt",
		},
		{
			name: "StructType parent is TypeSpec",
			findNode: func(f *ast.File) ast.Node {
				var structType *ast.StructType
				ast.Inspect(f, func(n ast.Node) bool {
					if st, ok := n.(*ast.StructType); ok {
						structType = st
						return false
					}
					return true
				})
				return structType
			},
			parentType: "*ast.TypeSpec",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := tt.findNode(file)
			if node == nil {
				t.Fatalf("Could not find node")
			}

			parent := ctx.GetParent(node)
			if parent == nil {
				t.Fatalf("GetParent() returned nil")
			}

			parentTypeStr := fmt.Sprintf("%T", parent)
			if parentTypeStr != tt.parentType {
				t.Errorf("Expected parent type %s, got %s", tt.parentType, parentTypeStr)
			}
		})
	}
}

func TestContext_WalkParents(t *testing.T) {
	ctx := &Context{}

	src := `package main

func outer() {
	inner := func() {
		x := 42
	}
	inner()
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse source: %v", err)
	}

	ctx.BuildParentMap(file)

	// Find the assignment statement x := 42
	var assignStmt *ast.AssignStmt
	ast.Inspect(file, func(n ast.Node) bool {
		if as, ok := n.(*ast.AssignStmt); ok {
			assignStmt = as
			return false
		}
		return true
	})

	if assignStmt == nil {
		t.Fatal("Could not find assignment statement")
	}

	// Walk up from assignment and collect parent types
	var parentTypes []string
	ctx.WalkParents(assignStmt, func(parent ast.Node) bool {
		parentTypes = append(parentTypes, fmt.Sprintf("%T", parent))
		return true
	})

	// Should have: BlockStmt -> FuncLit -> AssignStmt -> BlockStmt -> FuncDecl -> File
	// Verify we got multiple parents
	if len(parentTypes) < 3 {
		t.Errorf("Expected at least 3 parents in chain, got %d: %v", len(parentTypes), parentTypes)
	}

	// Verify first parent is BlockStmt (immediate parent of assignment)
	if parentTypes[0] != "*ast.BlockStmt" {
		t.Errorf("First parent should be *ast.BlockStmt, got %s", parentTypes[0])
	}

	// Verify last parent is File (root)
	if parentTypes[len(parentTypes)-1] != "*ast.File" {
		t.Errorf("Last parent should be *ast.File, got %s", parentTypes[len(parentTypes)-1])
	}
}

func TestContext_WalkParents_StopsEarly(t *testing.T) {
	ctx := &Context{}

	src := `package main

func main() {
	x := 42
	println(x)
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse source: %v", err)
	}

	ctx.BuildParentMap(file)

	// Find the assignment statement
	var assignStmt *ast.AssignStmt
	ast.Inspect(file, func(n ast.Node) bool {
		if as, ok := n.(*ast.AssignStmt); ok {
			assignStmt = as
			return false
		}
		return true
	})

	// Walk up and stop at FuncDecl
	var visitedCount int
	var foundFunc bool
	reachedRoot := ctx.WalkParents(assignStmt, func(parent ast.Node) bool {
		visitedCount++
		if _, ok := parent.(*ast.FuncDecl); ok {
			foundFunc = true
			return false // Stop here
		}
		return true
	})

	if !foundFunc {
		t.Error("Did not find FuncDecl during walk")
	}

	if reachedRoot {
		t.Error("WalkParents() should return false when stopped early, got true")
	}

	// Should have visited BlockStmt and FuncDecl before stopping
	if visitedCount < 2 {
		t.Errorf("Expected at least 2 visits, got %d", visitedCount)
	}
}

func TestContext_WalkParents_NilMap(t *testing.T) {
	ctx := &Context{}
	// Parent map not built

	src := packageMainSrc
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse source: %v", err)
	}

	// WalkParents should return true immediately (no parent map)
	visitedCount := 0
	reachedRoot := ctx.WalkParents(file, func(parent ast.Node) bool {
		visitedCount++
		return true
	})

	if !reachedRoot {
		t.Error("WalkParents() should return true when parent map not built")
	}

	if visitedCount != 0 {
		t.Errorf("Visitor should not be called when parent map not built, got %d calls", visitedCount)
	}
}

func TestContext_BuildParentMap_EmptyFile(t *testing.T) {
	ctx := &Context{}

	src := packageMainSrc
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse source: %v", err)
	}

	// Should not panic on minimal file
	ctx.BuildParentMap(file)

	if ctx.parentMap == nil {
		t.Error("BuildParentMap() should create map even for empty file")
	}
}

func TestContext_BuildParentMap_LargeFile(t *testing.T) {
	ctx := &Context{}

	// Build a larger file to test performance
	src := `package main

import "fmt"

type Node struct {
	Value int
	Left  *Node
	Right *Node
}

func (n *Node) Insert(value int) {
	if value < n.Value {
		if n.Left == nil {
			n.Left = &Node{Value: value}
		} else {
			n.Left.Insert(value)
		}
	} else {
		if n.Right == nil {
			n.Right = &Node{Value: value}
		} else {
			n.Right.Insert(value)
		}
	}
}

func (n *Node) Find(value int) bool {
	if n == nil {
		return false
	}
	if value == n.Value {
		return true
	}
	if value < n.Value {
		return n.Left.Find(value)
	}
	return n.Right.Find(value)
}

func (n *Node) InOrder() []int {
	if n == nil {
		return []int{}
	}
	result := []int{}
	result = append(result, n.Left.InOrder()...)
	result = append(result, n.Value)
	result = append(result, n.Right.InOrder()...)
	return result
}

func main() {
	root := &Node{Value: 50}
	values := []int{30, 70, 20, 40, 60, 80, 10, 25, 35, 65}

	for _, v := range values {
		root.Insert(v)
	}

	fmt.Println("Tree values (in-order):", root.InOrder())
	fmt.Println("Find 35:", root.Find(35))
	fmt.Println("Find 100:", root.Find(100))
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse source: %v", err)
	}

	// Measure rough performance (should be <10ms)
	ctx.BuildParentMap(file)

	// Count nodes in parent map
	nodeCount := len(ctx.parentMap)
	if nodeCount < 100 {
		t.Errorf("Expected at least 100 nodes for this file, got %d", nodeCount)
	}

	// Verify parent relationships are consistent
	for child, parent := range ctx.parentMap {
		if child == nil {
			t.Error("Parent map contains nil child")
		}
		if parent == nil {
			t.Error("Parent map contains nil parent")
		}
	}
}
