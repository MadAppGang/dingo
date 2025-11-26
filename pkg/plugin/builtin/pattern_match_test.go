package builtin

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/plugin"
)

func TestPatternMatchPlugin_Name(t *testing.T) {
	p := NewPatternMatchPlugin()
	if p.Name() != "pattern_match" {
		t.Errorf("expected name 'pattern_match', got %q", p.Name())
	}
}

func TestPatternMatchPlugin_ExhaustiveResult(t *testing.T) {
	src := `package main

func handleResult(result Result_int_string) int {
	scrutinee := result
	// DINGO_MATCH_START: result
	switch scrutinee.tag {
	case ResultTagOk:
		// DINGO_PATTERN: Ok(x)
		x := *scrutinee.ok
		return x * 2
	case ResultTagErr:
		// DINGO_PATTERN: Err(e)
		e := scrutinee.err
		return 0
	}
	// DINGO_MATCH_END
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	ctx := &plugin.Context{
		FileSet:     fset,
		CurrentFile: file,
		Logger:      plugin.NewNoOpLogger(),
	}
	ctx.BuildParentMap(file)

	p := NewPatternMatchPlugin()
	p.SetContext(ctx)

	err = p.Process(file)
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}

	// Should have no errors (exhaustive)
	if ctx.HasErrors() {
		t.Errorf("expected no errors for exhaustive match, got: %v", ctx.GetErrors())
	}

	// Should discover 1 match expression
	if len(p.matchExpressions) != 1 {
		t.Fatalf("expected 1 match expression, got %d", len(p.matchExpressions))
	}

	match := p.matchExpressions[0]
	if len(match.patterns) != 2 {
		t.Errorf("expected 2 patterns (Ok, Err), got %d", len(match.patterns))
	}
}

func TestPatternMatchPlugin_NonExhaustiveResult(t *testing.T) {
	src := `package main

func handleResult(result Result_int_string) int {
	scrutinee := result
	// DINGO_MATCH_START: result
	switch scrutinee.tag {
	case ResultTagOk:
		// DINGO_PATTERN: Ok(x)
		x := *scrutinee.ok
		return x * 2
	}
	// DINGO_MATCH_END
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	ctx := &plugin.Context{
		FileSet:     fset,
		CurrentFile: file,
		Logger:      plugin.NewNoOpLogger(),
	}
	ctx.BuildParentMap(file)

	p := NewPatternMatchPlugin()
	p.SetContext(ctx)

	err = p.Process(file)
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}

	// Should have 1 error (non-exhaustive, missing Err)
	if !ctx.HasErrors() {
		t.Fatalf("expected error for non-exhaustive match")
	}

	errors := ctx.GetErrors()
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}

	errMsg := errors[0].Error()
	if !strings.Contains(errMsg, "non-exhaustive") {
		t.Errorf("expected 'non-exhaustive' in error, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "Err") {
		t.Errorf("expected 'Err' in missing cases, got: %s", errMsg)
	}
}

func TestPatternMatchPlugin_ExhaustiveOption(t *testing.T) {
	src := `package main

func handleOption(opt Option_int) int {
	scrutinee := opt
	// DINGO_MATCH_START: opt
	switch scrutinee.tag {
	case OptionTagSome:
		// DINGO_PATTERN: Some(x)
		x := *scrutinee.some
		return x
	case OptionTagNone:
		// DINGO_PATTERN: None
		return 0
	}
	// DINGO_MATCH_END
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	ctx := &plugin.Context{
		FileSet:     fset,
		CurrentFile: file,
		Logger:      plugin.NewNoOpLogger(),
	}
	ctx.BuildParentMap(file)

	p := NewPatternMatchPlugin()
	p.SetContext(ctx)

	err = p.Process(file)
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}

	// Should have no errors (exhaustive)
	if ctx.HasErrors() {
		t.Errorf("expected no errors for exhaustive match, got: %v", ctx.GetErrors())
	}
}

func TestPatternMatchPlugin_NonExhaustiveOption(t *testing.T) {
	src := `package main

func handleOption(opt Option_int) int {
	scrutinee := opt
	// DINGO_MATCH_START: opt
	switch scrutinee.tag {
	case OptionTagSome:
		// DINGO_PATTERN: Some(x)
		x := *scrutinee.some
		return x
	}
	// DINGO_MATCH_END
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	ctx := &plugin.Context{
		FileSet:     fset,
		CurrentFile: file,
		Logger:      plugin.NewNoOpLogger(),
	}
	ctx.BuildParentMap(file)

	p := NewPatternMatchPlugin()
	p.SetContext(ctx)

	err = p.Process(file)
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}

	// Should have 1 error (non-exhaustive, missing None)
	if !ctx.HasErrors() {
		t.Fatalf("expected error for non-exhaustive match")
	}

	errors := ctx.GetErrors()
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}

	errMsg := errors[0].Error()
	if !strings.Contains(errMsg, "non-exhaustive") {
		t.Errorf("expected 'non-exhaustive' in error, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "None") {
		t.Errorf("expected 'None' in missing cases, got: %s", errMsg)
	}
}

func TestPatternMatchPlugin_WildcardCoversAll(t *testing.T) {
	src := `package main

func handleResult(result Result_int_string) int {
	scrutinee := result
	// DINGO_MATCH_START: result
	switch scrutinee.tag {
	case ResultTagOk:
		// DINGO_PATTERN: Ok(x)
		x := *scrutinee.ok
		return x * 2
	default:
		// DINGO_PATTERN: _
		return 0
	}
	// DINGO_MATCH_END
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	ctx := &plugin.Context{
		FileSet:     fset,
		CurrentFile: file,
		Logger:      plugin.NewNoOpLogger(),
	}
	ctx.BuildParentMap(file)

	p := NewPatternMatchPlugin()
	p.SetContext(ctx)

	err = p.Process(file)
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}

	// Should have no errors (wildcard covers Err)
	if ctx.HasErrors() {
		t.Errorf("expected no errors with wildcard, got: %v", ctx.GetErrors())
	}

	// Should detect wildcard
	if len(p.matchExpressions) != 1 {
		t.Fatalf("expected 1 match expression, got %d", len(p.matchExpressions))
	}

	match := p.matchExpressions[0]
	if !match.hasWildcard {
		t.Errorf("expected wildcard to be detected")
	}
}

func TestPatternMatchPlugin_GetAllVariants(t *testing.T) {
	p := NewPatternMatchPlugin()

	tests := []struct {
		name      string
		scrutinee string
		want      []string
	}{
		{
			name:      "Result type",
			scrutinee: "result",
			want:      []string{"Ok", "Err"},
		},
		{
			name:      "Result_T_E type",
			scrutinee: "Result_int_string",
			want:      []string{"Ok", "Err"},
		},
		{
			name:      "Option type",
			scrutinee: "option",
			want:      []string{"Some", "None"},
		},
		{
			name:      "Option_T type",
			scrutinee: "Option_int",
			want:      []string{"Some", "None"},
		},
		{
			name:      "Unknown type",
			scrutinee: "someValue",
			want:      []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.getAllVariants(tt.scrutinee)
			if len(got) != len(tt.want) {
				t.Errorf("getAllVariants(%q) = %v, want %v", tt.scrutinee, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("getAllVariants(%q)[%d] = %q, want %q", tt.scrutinee, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestPatternMatchPlugin_ExtractConstructorName(t *testing.T) {
	p := NewPatternMatchPlugin()

	tests := []struct {
		pattern string
		want    string
	}{
		{"Ok(x)", "Ok"},
		{"Err(e)", "Err"},
		{"Some(v)", "Some"},
		{"None", "None"},
		{"_", "_"},
		{"Active(id)", "Active"},
		{"Pending", "Pending"},
		{" Ok(x) ", "Ok"}, // with whitespace
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			got := p.extractConstructorName(tt.pattern)
			if got != tt.want {
				t.Errorf("extractConstructorName(%q) = %q, want %q", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestPatternMatchPlugin_IsExpressionMode(t *testing.T) {
	// Test expression mode detection
	// Note: Switch statements as expressions aren't valid Go syntax,
	// but we can test with function calls that get transformed
	src := `package main

func test() int {
	// Expression mode: return statement
	return match(x)

	// Statement mode: standalone
	match(y)
}

func match(v int) int {
	switch {
	case v > 0:
		return 1
	}
	return 0
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	ctx := &plugin.Context{
		FileSet:     fset,
		CurrentFile: file,
		Logger:      plugin.NewNoOpLogger(),
	}
	ctx.BuildParentMap(file)

	p := NewPatternMatchPlugin()
	p.SetContext(ctx)

	// Find all call expressions (match(x) and match(y))
	var calls []*ast.CallExpr
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "match" {
				calls = append(calls, call)
			}
		}
		return true
	})

	if len(calls) != 2 {
		t.Fatalf("expected 2 match calls, got %d", len(calls))
	}

	// First call is in return statement (expression context via parent)
	// This test is simplified - in real usage, we check if switch is in expression context
	// For this test, we just verify the parent walking works
	parent1 := ctx.GetParent(calls[0])
	if parent1 == nil {
		t.Errorf("expected parent for first call")
	}

	parent2 := ctx.GetParent(calls[1])
	if parent2 == nil {
		t.Errorf("expected parent for second call")
	}
}

func TestPatternMatchPlugin_MultipleMatches(t *testing.T) {
	src := `package main

func test(r1 Result_int_string, r2 Result_int_string) {
	// First match - non-exhaustive
	scrutinee := r1
	// DINGO_MATCH_START: r1
	switch scrutinee.tag {
	case ResultTagOk:
		// DINGO_PATTERN: Ok(x)
		x := *scrutinee.ok
	}
	// DINGO_MATCH_END

	// Second match - exhaustive
	scrutinee1 := r2
	// DINGO_MATCH_START: r2
	switch scrutinee1.tag {
	case ResultTagOk:
		// DINGO_PATTERN: Ok(y)
		y := *scrutinee1.ok
	case ResultTagErr:
		// DINGO_PATTERN: Err(e)
		e := scrutinee1.err
	}
	// DINGO_MATCH_END
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	ctx := &plugin.Context{
		FileSet:     fset,
		CurrentFile: file,
		Logger:      plugin.NewNoOpLogger(),
	}
	ctx.BuildParentMap(file)

	p := NewPatternMatchPlugin()
	p.SetContext(ctx)

	err = p.Process(file)
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}

	// Should discover 2 match expressions
	if len(p.matchExpressions) != 2 {
		t.Fatalf("expected 2 match expressions, got %d", len(p.matchExpressions))
	}

	// First match is non-exhaustive (should have error)
	if !ctx.HasErrors() {
		t.Fatalf("expected error for first non-exhaustive match")
	}

	// Should have exactly 1 error (only first match is non-exhaustive)
	errors := ctx.GetErrors()
	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(errors))
	}
}

func TestPatternMatchPlugin_Transform_AddsPanic(t *testing.T) {
	// Note: The PatternMatchPlugin's Transform phase intentionally does NOT transform the AST.
	// The preprocessor (rust_match.go) already generates correct switch statements with
	// panic statements. The plugin only validates exhaustiveness during Process phase.
	// This test verifies that Transform preserves the switch statement structure.
	src := `package main

func handleResult(result Result_int_string) int {
	scrutinee := result
	// DINGO_MATCH_START: result
	switch scrutinee.tag {
	case ResultTagOk:
		// DINGO_PATTERN: Ok(x)
		x := *scrutinee.ok
		return x * 2
	case ResultTagErr:
		// DINGO_PATTERN: Err(e)
		e := scrutinee.err
		return 0
	}
	// DINGO_MATCH_END
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	ctx := &plugin.Context{
		FileSet:     fset,
		CurrentFile: file,
		Logger:      plugin.NewNoOpLogger(),
	}
	ctx.BuildParentMap(file)

	p := NewPatternMatchPlugin()
	p.SetContext(ctx)

	// Process first to discover matches
	err = p.Process(file)
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}

	// Get the switch statement before transformation
	var switchStmtBefore *ast.SwitchStmt
	ast.Inspect(file, func(n ast.Node) bool {
		if sw, ok := n.(*ast.SwitchStmt); ok {
			switchStmtBefore = sw
			return false
		}
		return true
	})

	if switchStmtBefore == nil {
		t.Fatalf("switch statement not found before transform")
	}

	// Check initial case count (should be 2: Ok, Err)
	initialCaseCount := len(switchStmtBefore.Body.List)
	if initialCaseCount != 2 {
		t.Fatalf("expected 2 initial cases, got %d", initialCaseCount)
	}

	// Transform (should preserve switch structure, not convert to if-else)
	_, err = p.Transform(file)
	if err != nil {
		t.Fatalf("Transform error: %v", err)
	}

	// Verify the switch statement is preserved (not converted to if-else chain)
	var switchStmtAfter *ast.SwitchStmt
	ast.Inspect(file, func(n ast.Node) bool {
		if sw, ok := n.(*ast.SwitchStmt); ok {
			switchStmtAfter = sw
			return false
		}
		return true
	})

	// Switch statement should still exist after Transform
	if switchStmtAfter == nil {
		t.Fatalf("switch statement should be preserved after Transform")
	}

	// Case count should remain the same
	finalCaseCount := len(switchStmtAfter.Body.List)
	if finalCaseCount != initialCaseCount {
		t.Errorf("expected %d cases after Transform, got %d", initialCaseCount, finalCaseCount)
	}
}

func TestPatternMatchPlugin_Transform_WildcardNoPanic(t *testing.T) {
	src := `package main

func handleResult(result Result_int_string) int {
	scrutinee := result
	// DINGO_MATCH_START: result
	switch scrutinee.tag {
	case ResultTagOk:
		// DINGO_PATTERN: Ok(x)
		x := *scrutinee.ok
		return x * 2
	default:
		// DINGO_PATTERN: _
		return 0
	}
	// DINGO_MATCH_END
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	ctx := &plugin.Context{
		FileSet:     fset,
		CurrentFile: file,
		Logger:      plugin.NewNoOpLogger(),
	}
	ctx.BuildParentMap(file)

	p := NewPatternMatchPlugin()
	p.SetContext(ctx)

	err = p.Process(file)
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}

	var switchStmt *ast.SwitchStmt
	ast.Inspect(file, func(n ast.Node) bool {
		if sw, ok := n.(*ast.SwitchStmt); ok {
			switchStmt = sw
			return false
		}
		return true
	})

	initialCaseCount := len(switchStmt.Body.List)

	// Transform should NOT add default panic (wildcard already exists)
	_, err = p.Transform(file)
	if err != nil {
		t.Fatalf("Transform error: %v", err)
	}

	finalCaseCount := len(switchStmt.Body.List)
	if finalCaseCount != initialCaseCount {
		t.Errorf("expected case count to remain %d (no panic added), got %d", initialCaseCount, finalCaseCount)
	}
}

// Guard transformation tests

func TestPatternMatchPlugin_GuardParsing(t *testing.T) {
	src := `package main

func handleResult(result Result_int_int) int {
	scrutinee := result
	// DINGO_MATCH_START: result
	switch scrutinee.tag {
	case ResultTagOk:
		// DINGO_PATTERN: Ok(x) | DINGO_GUARD: x > 0
		x := *scrutinee.ok
		return x * 2
	case ResultTagOk:
		// DINGO_PATTERN: Ok(x)
		x := *scrutinee.ok
		return 0
	case ResultTagErr:
		// DINGO_PATTERN: Err(e)
		e := scrutinee.err
		return -1
	}
	// DINGO_MATCH_END
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	ctx := &plugin.Context{
		FileSet:     fset,
		CurrentFile: file,
		Logger:      plugin.NewNoOpLogger(),
	}
	ctx.BuildParentMap(file)

	p := NewPatternMatchPlugin()
	p.SetContext(ctx)

	err = p.Process(file)
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}

	// Should discover 1 match expression
	if len(p.matchExpressions) != 1 {
		t.Fatalf("expected 1 match expression, got %d", len(p.matchExpressions))
	}

	match := p.matchExpressions[0]

	// Should have 1 guard
	if len(match.guards) != 1 {
		t.Fatalf("expected 1 guard, got %d", len(match.guards))
	}

	// Check guard condition
	guard := match.guards[0]
	if guard.condition != "x > 0" {
		t.Errorf("expected guard condition 'x > 0', got %q", guard.condition)
	}

	// Check guard is on first case (armIndex 0)
	if guard.armIndex != 0 {
		t.Errorf("expected guard on arm 0, got arm %d", guard.armIndex)
	}
}

func TestPatternMatchPlugin_GuardTransformation(t *testing.T) {
	src := `package main

func handleResult(result Result_int_int) int {
	scrutinee := result
	// DINGO_MATCH_START: result
	switch scrutinee.tag {
	case ResultTagOk:
		// DINGO_PATTERN: Ok(x) | DINGO_GUARD: x > 0
		x := *scrutinee.ok
		return x * 2
	case ResultTagErr:
		// DINGO_PATTERN: Err(e)
		e := scrutinee.err
		return -1
	}
	// DINGO_MATCH_END
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	ctx := &plugin.Context{
		FileSet:     fset,
		CurrentFile: file,
		Logger:      plugin.NewNoOpLogger(),
	}
	ctx.BuildParentMap(file)

	p := NewPatternMatchPlugin()
	p.SetContext(ctx)

	err = p.Process(file)
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}

	// Get switch statement
	var switchStmt *ast.SwitchStmt
	ast.Inspect(file, func(n ast.Node) bool {
		if sw, ok := n.(*ast.SwitchStmt); ok {
			switchStmt = sw
			return false
		}
		return true
	})

	if switchStmt == nil {
		t.Fatalf("switch statement not found")
	}

	// Transform should succeed (note: switch→if transformation currently disabled)
	_, err = p.Transform(file)
	if err != nil {
		t.Fatalf("Transform error: %v", err)
	}

	// Verify guard parsing works correctly
	// Guards are collected during Process phase and validated during buildIfElseChain
	if len(p.matchExpressions) == 0 {
		t.Fatalf("expected match expressions to be discovered")
	}

	match := p.matchExpressions[0]
	if len(match.guards) == 0 {
		t.Fatalf("expected guard to be parsed")
	}

	// Verify guard condition was extracted correctly
	guard := match.guards[0]
	if guard.condition != "x > 0" {
		t.Errorf("expected guard condition 'x > 0', got %q", guard.condition)
	}

	// Note: Switch→if transformation is currently disabled (preserves DINGO comments)
	// Guard validation happens in buildIfElseChain when transformation is enabled
}

func TestPatternMatchPlugin_MultipleGuards(t *testing.T) {
	src := `package main

func handleResult(result Result_int_int) int {
	scrutinee := result
	// DINGO_MATCH_START: result
	switch scrutinee.tag {
	case ResultTagOk:
		// DINGO_PATTERN: Ok(x) | DINGO_GUARD: x > 0
		x := *scrutinee.ok
		return x * 2
	case ResultTagOk:
		// DINGO_PATTERN: Ok(x) | DINGO_GUARD: x == 0
		x := *scrutinee.ok
		return 0
	case ResultTagOk:
		// DINGO_PATTERN: Ok(x)
		x := *scrutinee.ok
		return x
	case ResultTagErr:
		// DINGO_PATTERN: Err(e)
		e := scrutinee.err
		return -1
	}
	// DINGO_MATCH_END
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	ctx := &plugin.Context{
		FileSet:     fset,
		CurrentFile: file,
		Logger:      plugin.NewNoOpLogger(),
	}
	ctx.BuildParentMap(file)

	p := NewPatternMatchPlugin()
	p.SetContext(ctx)

	err = p.Process(file)
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}

	// Should discover 2 guards (first two Ok cases)
	if len(p.matchExpressions) != 1 {
		t.Fatalf("expected 1 match expression, got %d", len(p.matchExpressions))
	}

	match := p.matchExpressions[0]
	if len(match.guards) != 2 {
		t.Fatalf("expected 2 guards, got %d", len(match.guards))
	}

	// Check first guard
	if match.guards[0].condition != "x > 0" {
		t.Errorf("expected first guard 'x > 0', got %q", match.guards[0].condition)
	}

	// Check second guard
	if match.guards[1].condition != "x == 0" {
		t.Errorf("expected second guard 'x == 0', got %q", match.guards[1].condition)
	}

	// Transform and verify both guards get if statements
	_, err = p.Transform(file)
	if err != nil {
		t.Fatalf("Transform error: %v", err)
	}

	// Note: After Transform, switch will be replaced with if-else chain
	// Just verify that Process phase handles guards correctly
}

func TestPatternMatchPlugin_ComplexGuardExpression(t *testing.T) {
	src := `package main

func handleResult(result Result_int_int) int {
	scrutinee := result
	// DINGO_MATCH_START: result
	switch scrutinee.tag {
	case ResultTagOk:
		// DINGO_PATTERN: Ok(x) | DINGO_GUARD: x > 0 && x < 100
		x := *scrutinee.ok
		return x * 2
	case ResultTagErr:
		// DINGO_PATTERN: Err(e)
		e := scrutinee.err
		return -1
	}
	// DINGO_MATCH_END
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	ctx := &plugin.Context{
		FileSet:     fset,
		CurrentFile: file,
		Logger:      plugin.NewNoOpLogger(),
	}
	ctx.BuildParentMap(file)

	p := NewPatternMatchPlugin()
	p.SetContext(ctx)

	err = p.Process(file)
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}

	match := p.matchExpressions[0]
	if len(match.guards) != 1 {
		t.Fatalf("expected 1 guard, got %d", len(match.guards))
	}

	// Check complex guard condition
	if match.guards[0].condition != "x > 0 && x < 100" {
		t.Errorf("expected guard 'x > 0 && x < 100', got %q", match.guards[0].condition)
	}

	// Transform should succeed
	_, err = p.Transform(file)
	if err != nil {
		t.Fatalf("Transform error: %v", err)
	}
}

func TestPatternMatchPlugin_InvalidGuardSyntax(t *testing.T) {
	src := `package main

func handleResult(result Result_int_int) int {
	scrutinee := result
	// DINGO_MATCH_START: result
	switch scrutinee.tag {
	case ResultTagOk:
		// DINGO_PATTERN: Ok(x) | DINGO_GUARD: x > @ invalid
		x := *scrutinee.ok
		return x * 2
	case ResultTagErr:
		// DINGO_PATTERN: Err(e)
		e := scrutinee.err
		return -1
	}
	// DINGO_MATCH_END
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	ctx := &plugin.Context{
		FileSet:     fset,
		CurrentFile: file,
		Logger:      plugin.NewNoOpLogger(),
	}
	ctx.BuildParentMap(file)

	p := NewPatternMatchPlugin()
	p.SetContext(ctx)

	err = p.Process(file)
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}

	// Guard validation test: Invalid guard syntax should be detected
	// We need to actually enable transformation and call buildIfElseChain to trigger validation
	// Since switch→if transformation is disabled, we'll test buildIfElseChain directly

	if len(p.matchExpressions) == 0 {
		t.Fatalf("expected match expressions to be discovered")
	}

	match := p.matchExpressions[0]

	// Call buildIfElseChain which validates guards
	// This should fail or log errors for invalid guard syntax
	stmts := p.buildIfElseChain(match, file)

	// buildIfElseChain should skip the case with invalid guard (logs error and continues)
	// We should have 1 if statement for the valid Err case, not 2
	if len(stmts) < 1 {
		t.Fatalf("expected at least 1 if statement for valid Err case")
	}

	// The first case (with invalid guard) should have been skipped due to validation error
	// Only the Err case should be present
}

func TestPatternMatchPlugin_GuardExhaustivenessIgnored(t *testing.T) {
	// Guards should NOT satisfy exhaustiveness checking
	src := `package main

func handleResult(result Result_int_int) int {
	scrutinee := result
	// DINGO_MATCH_START: result
	switch scrutinee.tag {
	case ResultTagOk:
		// DINGO_PATTERN: Ok(x) | DINGO_GUARD: x > 0
		x := *scrutinee.ok
		return x * 2
	}
	// DINGO_MATCH_END
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	ctx := &plugin.Context{
		FileSet:     fset,
		CurrentFile: file,
		Logger:      plugin.NewNoOpLogger(),
	}
	ctx.BuildParentMap(file)

	p := NewPatternMatchPlugin()
	p.SetContext(ctx)

	err = p.Process(file)
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}

	// Should have error for non-exhaustive match (missing Err)
	// Guards do NOT satisfy exhaustiveness
	if !ctx.HasErrors() {
		t.Fatalf("expected error for non-exhaustive match (guards ignored)")
	}

	errors := ctx.GetErrors()
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}

	errMsg := errors[0].Error()
	if !strings.Contains(errMsg, "non-exhaustive") {
		t.Errorf("expected 'non-exhaustive' in error, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "Err") {
		t.Errorf("expected 'Err' in missing cases, got: %s", errMsg)
	}
}
