package transpiler

import (
	"go/ast"
	goparser "go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/typechecker"
)

// TestResultWrapper_SimpleOk tests wrapping a success value with dgo.Ok[T, E].
func TestResultWrapper_SimpleOk(t *testing.T) {
	source := `package main

func fetch() Result[string, error] {
	return "success"
}
`

	expected := `dgo.Ok[string, error]("success")`

	result := transformAndExtractReturn(t, source)
	if !strings.Contains(result, expected) {
		t.Errorf("Expected return to contain %q, got: %s", expected, result)
	}
}

// TestResultWrapper_SimpleErr tests wrapping an error value with dgo.Err[T, E].
// Both type parameters must be specified for correct type inference.
func TestResultWrapper_SimpleErr(t *testing.T) {
	source := `package main

import "errors"

func fetch() Result[string, error] {
	return errors.New("failed")
}
`

	expected := `dgo.Err[string, error](errors.New("failed"))`

	result := transformAndExtractReturn(t, source)
	if !strings.Contains(result, expected) {
		t.Errorf("Expected return to contain %q, got: %s", expected, result)
	}
}

// TestResultWrapper_CompositeErrLiteral tests wrapping custom error type composite literal.
func TestResultWrapper_CompositeErrLiteral(t *testing.T) {
	source := `package main

type DBError struct {
	Code string
}

func fetch() Result[string, DBError] {
	return DBError{Code: "ERR"}
}
`

	expected := `dgo.Err[string, DBError](DBError{Code: "ERR"})`

	result := transformAndExtractReturn(t, source)
	if !strings.Contains(result, expected) {
		t.Errorf("Expected return to contain %q, got: %s", expected, result)
	}
}

func TestResultWrapper_MustErrWithUnresolvedReceiver(t *testing.T) {
	source := `package main

type DBError struct{ Message string }

func source() Result[int, DBError] {
	return 1
}

func transfer() Result[bool, DBError] {
	result := source()
	return result.MustErr()
}
`

	generated := transformAndPrint(t, source)
	expected := `dgo.Err[bool, DBError](result.MustErr())`
	if !strings.Contains(generated, expected) {
		t.Fatalf("expected MustErr return to contain %q, got:\n%s", expected, generated)
	}
}

// TestResultWrapper_AlreadyWrapped_NoChange tests that already-wrapped returns are not double-wrapped.
func TestResultWrapper_AlreadyWrapped_NoChange(t *testing.T) {
	testCases := []struct {
		name     string
		source   string
		expected string
	}{
		{
			name: "Generic Ok form",
			source: `package main

func fetch() Result[string, error] {
	return dgo.Ok[string, error]("success")
}
`,
			expected: `dgo.Ok[string, error]("success")`,
		},
		{
			name: "Generic Err form",
			source: `package main

func fetch() Result[string, error] {
	return dgo.Err[string, error](errors.New("failed"))
}
`,
			expected: `dgo.Err[string, error](errors.New("failed"))`,
		},
		{
			name: "Non-generic Ok form",
			source: `package main

func fetch() Result[string, error] {
	return dgo.Ok("success")
}
`,
			expected: `dgo.Ok("success")`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := transformAndExtractReturn(t, tc.source)
			// Should NOT be double-wrapped
			if strings.Count(result, "dgo.Ok") > 1 || strings.Count(result, "dgo.Err") > 1 {
				t.Errorf("Return was double-wrapped: %s", result)
			}
			if !strings.Contains(result, tc.expected) {
				t.Errorf("Expected return to contain %q, got: %s", tc.expected, result)
			}
		})
	}
}

// TestOptionWrapper_SimpleValue tests wrapping a non-nil value with dgo.Some.
// Go infers T from the value argument, so no type argument is needed.
func TestOptionWrapper_SimpleValue(t *testing.T) {
	source := `package main

func find() Option[string] {
	return "found"
}
`

	expected := `dgo.Some("found")`

	result := transformAndExtractReturn(t, source)
	if !strings.Contains(result, expected) {
		t.Errorf("Expected return to contain %q, got: %s", expected, result)
	}
}

// TestOptionWrapper_NilReturnsNone tests that nil is wrapped with dgo.None[T]().
func TestOptionWrapper_NilReturnsNone(t *testing.T) {
	source := `package main

func find() Option[string] {
	return nil
}
`

	expected := `dgo.None[string]()`

	result := transformAndExtractReturn(t, source)
	if !strings.Contains(result, expected) {
		t.Errorf("Expected return to contain %q, got: %s", expected, result)
	}
}

// TestOptionWrapper_AlreadyWrapped_NoChange tests that already-wrapped Option returns are unchanged.
func TestOptionWrapper_AlreadyWrapped_NoChange(t *testing.T) {
	testCases := []struct {
		name     string
		source   string
		expected string
	}{
		{
			name: "Generic Some form",
			source: `package main

func find() Option[string] {
	return dgo.Some[string]("found")
}
`,
			expected: `dgo.Some[string]("found")`,
		},
		{
			name: "Generic None form",
			source: `package main

func find() Option[string] {
	return dgo.None[string]()
}
`,
			expected: `dgo.None[string]()`,
		},
		{
			name: "Non-generic Some form",
			source: `package main

func find() Option[string] {
	return dgo.Some("found")
}
`,
			expected: `dgo.Some("found")`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := transformAndExtractReturn(t, tc.source)
			// Should NOT be double-wrapped
			if strings.Count(result, "dgo.Some") > 1 || strings.Count(result, "dgo.None") > 1 {
				t.Errorf("Return was double-wrapped: %s", result)
			}
			if !strings.Contains(result, tc.expected) {
				t.Errorf("Expected return to contain %q, got: %s", tc.expected, result)
			}
		})
	}
}

// TestResultWrapper_MultipleReturns tests that multiple return statements are all wrapped.
func TestResultWrapper_MultipleReturns(t *testing.T) {
	source := `package main

import "errors"

func fetch(id int) Result[string, error] {
	if id == 0 {
		return errors.New("invalid id")
	}
	return "success"
}
`

	result := transformAndPrint(t, source)

	// Both Ok and Err need both [T, E] type parameters
	okCount := strings.Count(result, "dgo.Ok[string, error]")
	errCount := strings.Count(result, "dgo.Err[string, error]")

	if okCount != 1 {
		t.Errorf("Expected 1 dgo.Ok wrapper, got %d in:\n%s", okCount, result)
	}
	if errCount != 1 {
		t.Errorf("Expected 1 dgo.Err wrapper, got %d in:\n%s", errCount, result)
	}
}

// TestResultWrapper_RegularFunction_NoChange tests that non-Result/Option functions are unchanged.
func TestResultWrapper_RegularFunction_NoChange(t *testing.T) {
	source := `package main

func regular() string {
	return "hello"
}
`

	result := transformAndExtractReturn(t, source)

	// Should NOT have any wrappers
	if strings.Contains(result, "dgo.") {
		t.Errorf("Regular function should not have dgo wrappers, got: %s", result)
	}
	if !strings.Contains(result, `"hello"`) {
		t.Errorf("Expected original return value, got: %s", result)
	}
}

// TestResultWrapper_NestedFunction tests that nested functions are handled independently.
func TestResultWrapper_NestedFunction(t *testing.T) {
	source := `package main

func outer() Result[int, error] {
	inner := func() int {
		return 42
	}
	return inner()
}
`

	result := transformAndPrint(t, source)

	// Outer should be wrapped with Ok[T, E], inner should not
	if !strings.Contains(result, "dgo.Ok[int, error](inner())") {
		t.Errorf("Expected outer function to wrap inner() call, got: %s", result)
	}
	if strings.Contains(result, "dgo.Ok[int, error](42)") {
		t.Errorf("Inner function should not be wrapped, got: %s", result)
	}
}

// Helper: transformAndExtractReturn transforms source and extracts the return statement.
func transformAndExtractReturn(t *testing.T, source string) string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, "test.go", source, goparser.ParseComments)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Create type checker (with best effort - ignore errors)
	checker, _ := typechecker.New(fset, file, "main")

	// Transform
	transformer := NewResultWrapperTransformer(fset, file, checker)
	transformer.Transform()

	// Extract first return statement
	var returnStmt string
	ast.Inspect(file, func(n ast.Node) bool {
		if ret, ok := n.(*ast.ReturnStmt); ok && returnStmt == "" {
			returnStmt = exprToString(fset, ret.Results[0])
			return false
		}
		return true
	})

	if returnStmt == "" {
		t.Fatal("No return statement found")
	}

	return returnStmt
}

// Helper: transformAndPrint transforms source and returns the full output.
func transformAndPrint(t *testing.T, source string) string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, "test.go", source, goparser.ParseComments)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Create type checker (with best effort - ignore errors)
	checker, _ := typechecker.New(fset, file, "main")

	// Transform
	transformer := NewResultWrapperTransformer(fset, file, checker)
	transformer.Transform()

	// Print AST
	var buf strings.Builder
	cfg := printer.Config{Mode: printer.UseSpaces, Tabwidth: 4}
	if err := cfg.Fprint(&buf, fset, file); err != nil {
		t.Fatalf("Print error: %v", err)
	}

	return buf.String()
}

// Helper: exprToString converts an expression to a string.
func exprToString(fset *token.FileSet, expr ast.Expr) string {
	var buf strings.Builder
	cfg := printer.Config{Mode: printer.UseSpaces, Tabwidth: 4}
	if err := cfg.Fprint(&buf, fset, expr); err != nil {
		return "<print error>"
	}
	return buf.String()
}
