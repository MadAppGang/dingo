package preprocessor

import (
	"strings"
	"testing"
)

func TestTupleDestructuring_Simple(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:  "2-tuple destructuring",
			input: "let (x, y) = getCoords()",
			expected: `tmp := getCoords()
x, y := tmp._0, tmp._1`,
		},
		{
			name:  "3-tuple destructuring",
			input: "let (a, b, c) = getTriplet()",
			expected: `tmp := getTriplet()
a, b, c := tmp._0, tmp._1, tmp._2`,
		},
		{
			name:  "with wildcard",
			input: "let (x, _, z) = getTriplet()",
			expected: `tmp := getTriplet()
x, _, z := tmp._0, tmp._1, tmp._2`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewTupleProcessor()
			result, _, err := processor.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := strings.TrimSpace(string(result))
			want := strings.TrimSpace(tt.expected)

			if got != want {
				t.Errorf("got:\n%s\n\nwant:\n%s", got, want)
			}
		})
	}
}

func TestTupleDestructuring_Nested(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:  "nested 2-tuple in first position",
			input: "let ((a, b), c) = getNested()",
			expected: `tmp := getNested()
tmp1 := tmp._0
a, b := tmp1._0, tmp1._1
c := tmp._1`,
		},
		{
			name:  "nested 2-tuple in second position",
			input: "let (x, (y, z)) = getNested()",
			expected: `tmp := getNested()
tmp1 := tmp._1
y, z := tmp1._0, tmp1._1
x := tmp._0`,
		},
		{
			name:  "multiple nested tuples",
			input: "let ((a, b), (c, d)) = getDoubleNested()",
			expected: `tmp := getDoubleNested()
tmp1 := tmp._0
a, b := tmp1._0, tmp1._1
tmp2 := tmp._1
c, d := tmp2._0, tmp2._1`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewTupleProcessor()
			result, _, err := processor.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := strings.TrimSpace(string(result))
			want := strings.TrimSpace(tt.expected)

			if got != want {
				t.Errorf("got:\n%s\n\nwant:\n%s", got, want)
			}
		})
	}
}

func TestTupleDestructuring_WithWildcards(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:  "wildcard in nested pattern",
			input: "let ((a, _), c) = getNested()",
			expected: `tmp := getNested()
tmp1 := tmp._0
a, _ := tmp1._0, tmp1._1
c := tmp._1`,
		},
		{
			name:  "wildcard at top level",
			input: "let (_, (y, z)) = getNested()",
			expected: `tmp := getNested()
tmp1 := tmp._1
y, z := tmp1._0, tmp1._1
_ := tmp._0`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewTupleProcessor()
			result, _, err := processor.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := strings.TrimSpace(string(result))
			want := strings.TrimSpace(tt.expected)

			if got != want {
				t.Errorf("got:\n%s\n\nwant:\n%s", got, want)
			}
		})
	}
}

func TestTupleDestructuring_Arity(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "0-tuple (empty)",
			input:       "let () = getEmpty()",
			expectError: true,
			errorMsg:    "empty destructuring pattern",
		},
		{
			name:        "1-tuple (single element)",
			input:       "let (x) = getSingle()",
			expectError: true,
			errorMsg:    "single-element tuples are not supported",
		},
		{
			name:        "12-tuple (max allowed)",
			input:       "let (a, b, c, d, e, f, g, h, i, j, k, l) = getMax()",
			expectError: false,
		},
		{
			name:        "13-tuple (exceeds max)",
			input:       "let (a, b, c, d, e, f, g, h, i, j, k, l, m) = getTooMany()",
			expectError: true,
			errorMsg:    "maximum is 12",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewTupleProcessor()
			_, _, err := processor.Process([]byte(tt.input))

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestTupleDestructuring_Indentation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:  "indented destructuring",
			input: "    let (x, y) = getCoords()",
			expected: `    tmp := getCoords()
    x, y := tmp._0, tmp._1`,
		},
		{
			name:  "tab indented",
			input: "\tlet (a, b) = getPair()",
			expected: "\ttmp := getPair()\n\ta, b := tmp._0, tmp._1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewTupleProcessor()
			result, _, err := processor.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			want := tt.expected

			if got != want {
				t.Errorf("got:\n%q\n\nwant:\n%q", got, want)
			}
		})
	}
}

func TestTupleDestructuring_MultiLine(t *testing.T) {
	input := `func example() {
	let (x, y) = getCoords()
	let ((a, b), c) = getNested()
}`

	expected := `func example() {
	tmp := getCoords()
	x, y := tmp._0, tmp._1
	tmp1 := getNested()
	tmp2 := tmp1._0
	a, b := tmp2._0, tmp2._1
	c := tmp1._1
}`

	processor := NewTupleProcessor()
	result, _, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := strings.TrimSpace(string(result))
	want := strings.TrimSpace(expected)

	if got != want {
		t.Errorf("got:\n%s\n\nwant:\n%s", got, want)
	}
}

func TestTupleDestructuring_NamingConvention(t *testing.T) {
	// Test that temp variables follow camelCase naming (tmp, tmp1, tmp2)
	input := `let (x, y) = f1()
let (a, b) = f2()
let (p, q) = f3()`

	processor := NewTupleProcessor()
	result, _, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := string(result)

	// Verify naming: should have tmp, tmp1, tmp2 (not tmp0, tmp1, tmp2)
	if !strings.Contains(got, "tmp := f1()") {
		t.Errorf("expected 'tmp := f1()', not found in output:\n%s", got)
	}
	if !strings.Contains(got, "tmp1 := f2()") {
		t.Errorf("expected 'tmp1 := f2()', not found in output:\n%s", got)
	}
	if !strings.Contains(got, "tmp2 := f3()") {
		t.Errorf("expected 'tmp2 := f3()', not found in output:\n%s", got)
	}

	// Verify NO underscore prefixes
	if strings.Contains(got, "__tmp") || strings.Contains(got, "_tmp") {
		t.Errorf("unexpected underscore-prefixed variables in output:\n%s", got)
	}
}

func TestParseDestructurePattern_Simple(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{
			name:     "two identifiers",
			pattern:  "x, y",
			expected: []string{"x", "y"},
		},
		{
			name:     "with whitespace",
			pattern:  " a , b , c ",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "with wildcards",
			pattern:  "x, _, z",
			expected: []string{"x", "_", "z"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDestructurePattern(tt.pattern)
			if len(got) != len(tt.expected) {
				t.Errorf("length mismatch: got %d, want %d", len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("element %d: got %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestParseDestructurePattern_Nested(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{
			name:     "nested in first position",
			pattern:  "(a, b), c",
			expected: []string{"(a, b)", "c"},
		},
		{
			name:     "nested in second position",
			pattern:  "x, (y, z)",
			expected: []string{"x", "(y, z)"},
		},
		{
			name:     "double nested",
			pattern:  "(a, b), (c, d)",
			expected: []string{"(a, b)", "(c, d)"},
		},
		{
			name:     "deeply nested",
			pattern:  "((a, b), c), d",
			expected: []string{"((a, b), c)", "d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDestructurePattern(tt.pattern)
			if len(got) != len(tt.expected) {
				t.Errorf("length mismatch: got %d, want %d", len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("element %d: got %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}
