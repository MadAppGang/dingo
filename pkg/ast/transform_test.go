package ast

import (
	"go/parser"
	gotoken "go/token"
	"strings"
	"testing"
)

// TestTransformSource_MethodReceiverTypeAnnotations exercises the fix for
// type-annotated parameter lists on methods (receiver functions).
//
// Without the receiver-aware branch in TransformSource, the param-list detector
// only recognised `func name(...)` and `func(...)`. As soon as a receiver was
// present, e.g. `func (r *T) Foo(x: int)`, the `(x: int)` group was treated as
// an arbitrary parenthesised expression, so the `:` was never converted into a
// space and the generated Go was syntactically invalid.
func TestTransformSource_MethodReceiverTypeAnnotations(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantAbsent []string
	}{
		{
			name:  "pointer receiver single param",
			input: "package p\nfunc (r *T) Foo(x: int) {}\ntype T struct{}\n",
			wantAbsent: []string{
				"x: int",
			},
		},
		{
			name:  "value receiver multiple params",
			input: "package p\nfunc (r T) Bar(x: int, y: string) {}\ntype T struct{}\n",
			wantAbsent: []string{
				"x: int",
				"y: string",
			},
		},
		{
			name: "method body struct literal colons are preserved",
			input: "package p\n" +
				"type T struct{}\n" +
				"type rw struct{ X int }\n" +
				"func (r *T) Foo(x: int) { _ = rw{X: 1} }\n",
			wantAbsent: []string{
				"x: int",
			},
		},
		{
			name:  "named receiver pointer to generic type",
			input: "package p\nfunc (r *T[U]) Foo(x: U) {}\ntype T[U any] struct{}\n",
			wantAbsent: []string{
				"x: U",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := TransformSource([]byte(tt.input), "test.dingo")
			if err != nil {
				t.Fatalf("TransformSource() error = %v", err)
			}
			got := string(out)
			// Goal: transformer must produce valid Go for method declarations
			// with type-annotated params. Without the fix, the `:` survives and
			// go/parser rejects it.
			fset := gotoken.NewFileSet()
			if _, err := parser.ParseFile(fset, "out.go", got, 0); err != nil {
				t.Errorf("transformed output is not valid Go: %v\n---got---\n%s", err, got)
			}
			for _, bad := range tt.wantAbsent {
				if strings.Contains(got, bad) {
					t.Errorf("output unexpectedly contains %q\n---got---\n%s", bad, got)
				}
			}
		})
	}
}
