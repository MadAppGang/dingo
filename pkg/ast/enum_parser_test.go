package ast

import (
	"strings"
	"testing"
)

// TestEnumParser_TypePrefixes covers parseTypeExpr's handling of pointer (*),
// slice ([]), and any nesting/order of the two. The original implementation
// accepted only `**T` or `*[]T`-shaped prefixes (pointers first, then a single
// slice), which silently rejected `[]*T`, `[][]T`, `[]*[]T`, etc. The
// transpilation pipeline turned that rejection into a confusing
// "expected declaration, found enum" error downstream.
func TestEnumParser_TypePrefixes(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantType string // expected Text of the parsed field type
	}{
		{
			name:     "plain type",
			src:      `enum E { V { x: int } }`,
			wantType: "int",
		},
		{
			name:     "pointer",
			src:      `enum E { V { x: *Foo } }`,
			wantType: "*Foo",
		},
		{
			name:     "double pointer",
			src:      `enum E { V { x: **Foo } }`,
			wantType: "**Foo",
		},
		{
			name:     "slice of value",
			src:      `enum E { V { x: []Foo } }`,
			wantType: "[]Foo",
		},
		{
			name:     "slice of pointer",
			src:      `enum E { V { x: []*Foo } }`,
			wantType: "[]*Foo",
		},
		{
			name:     "pointer to slice",
			src:      `enum E { V { x: *[]Foo } }`,
			wantType: "*[]Foo",
		},
		{
			name:     "slice of slice",
			src:      `enum E { V { x: [][]int } }`,
			wantType: "[][]int",
		},
		{
			name:     "slice of slice of pointer",
			src:      `enum E { V { x: [][]*Foo } }`,
			wantType: "[][]*Foo",
		},
		{
			name:     "generic with slice arg",
			src:      `enum E { V { x: Result[[]int, error] } }`,
			wantType: "Result[[]int, error]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewEnumParser([]byte(tt.src), 0)
			decl, _, err := p.ParseEnumDecl()
			if err != nil {
				t.Fatalf("ParseEnumDecl(%q) returned error: %v", tt.src, err)
			}
			if len(decl.Variants) != 1 {
				t.Fatalf("expected 1 variant, got %d", len(decl.Variants))
			}
			v := decl.Variants[0]
			if v.Kind != StructVariant {
				t.Fatalf("expected struct variant, got %v", v.Kind)
			}
			if len(v.Fields) != 1 {
				t.Fatalf("expected 1 field, got %d", len(v.Fields))
			}
			got := v.Fields[0].Type.Text
			if got != tt.wantType {
				t.Errorf("field type = %q, want %q", got, tt.wantType)
			}
		})
	}
}

// TestEnumParser_TupleTypePrefixes covers the same prefix handling in tuple
// variant fields. parseTupleFields shares parseTypeExpr with parseStructFields,
// so a regression in either context surfaces here too.
func TestEnumParser_TupleTypePrefixes(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		wantTypes []string
	}{
		{
			name:      "single slice-of-pointer",
			src:       `enum E { V([]*Foo) }`,
			wantTypes: []string{"[]*Foo"},
		},
		{
			name:      "mixed prefixes",
			src:       `enum E { V(*Foo, []*Bar, [][]int) }`,
			wantTypes: []string{"*Foo", "[]*Bar", "[][]int"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewEnumParser([]byte(tt.src), 0)
			decl, _, err := p.ParseEnumDecl()
			if err != nil {
				t.Fatalf("ParseEnumDecl(%q) returned error: %v", tt.src, err)
			}
			if len(decl.Variants) != 1 {
				t.Fatalf("expected 1 variant, got %d", len(decl.Variants))
			}
			v := decl.Variants[0]
			if v.Kind != TupleVariant {
				t.Fatalf("expected tuple variant, got %v", v.Kind)
			}
			if len(v.Fields) != len(tt.wantTypes) {
				t.Fatalf("expected %d fields, got %d", len(tt.wantTypes), len(v.Fields))
			}
			for i, want := range tt.wantTypes {
				got := v.Fields[i].Type.Text
				if got != want {
					t.Errorf("field %d type = %q, want %q", i, got, want)
				}
			}
		})
	}
}

// TestTransformSource_SliceOfPointer is an end-to-end regression test:
// before the fix, a `[]*T` field on an enum variant caused the dingo→go
// transformation to leave the `enum` keyword in the output, producing a
// downstream go/parser error of the form
//
//	"expected declaration, found enum"
//
// After the fix the source is transformed cleanly and the generated Go
// contains the variant struct with the correct slice-of-pointer field.
func TestTransformSource_SliceOfPointer(t *testing.T) {
	src := []byte(`package main

enum D { Foo { xs: []*Name, n: int } }

type Name struct { v string }
`)
	got, err := TransformSource(src, "test.dingo")
	if err != nil {
		t.Fatalf("TransformSource returned error: %v", err)
	}
	out := string(got)
	if strings.Contains(out, "enum ") {
		t.Errorf("transformed output still contains 'enum' keyword:\n%s", out)
	}
	// The variant struct should carry the field with its original slice-of-pointer type.
	if !strings.Contains(out, "xs []*Name") {
		t.Errorf("expected `xs []*Name` field in generated Go, got:\n%s", out)
	}
}

// TestTransformSource_MultiLineVariantBody confirms that a struct variant
// whose fields are newline-separated (no trailing commas) still parses.
// This was suspected to be broken alongside the slice-of-pointer bug but is
// actually fine; this test guards against future regressions in
// parseStructFields, which relies on skipWhitespaceAndCommas treating
// newlines as separators.
func TestTransformSource_MultiLineVariantBody(t *testing.T) {
	src := []byte(`package main

enum D { Foo {
    a: int
    b: *Name
    c: []*Name
} }

type Name struct { v string }
`)
	got, err := TransformSource(src, "test.dingo")
	if err != nil {
		t.Fatalf("TransformSource returned error: %v", err)
	}
	out := string(got)
	if strings.Contains(out, "enum ") {
		t.Errorf("transformed output still contains 'enum' keyword:\n%s", out)
	}
	for _, want := range []string{"a int", "b *Name", "c []*Name"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in generated Go, got:\n%s", want, out)
		}
	}
}
