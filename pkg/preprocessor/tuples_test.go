package preprocessor

import (
	"strings"
	"testing"
)

func TestTupleProcessor_TupleLiterals(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantMatch string // Substring that should appear in output
		wantErr   bool
	}{
		{
			name:      "basic_pair",
			input:     "let x = (10, 20)",
			wantMatch: "__TUPLE_2__LITERAL__",
			wantErr:   false,
		},
		{
			name:      "triple",
			input:     "let point = (1, 2, 3)",
			wantMatch: "__TUPLE_3__LITERAL__",
			wantErr:   false,
		},
		{
			name:      "nested_tuples",
			input:     "let nested = ((1, 2), (3, 4))",
			wantMatch: "let nested = ((1, 2), (3, 4))", // SCOPE REDUCTION: Nested tuples not supported - passes through unchanged
			wantErr:   false,
		},
		{
			name:      "complex_expressions",
			input:     "let x = (a + b, c * d, e / f)",
			wantMatch: "__TUPLE_3__LITERAL__",
			wantErr:   false,
		},
		{
			name:      "function_call_not_tuple",
			input:     "let x = foo(10, 20)",
			wantMatch: "foo(10, 20)", // Should remain unchanged
			wantErr:   false,
		},
		{
			name:      "grouping_not_tuple",
			input:     "let x = (a + b) * c",
			wantMatch: "(a + b) * c", // No comma, should remain unchanged
			wantErr:   false,
		},
		{
			name:      "tuple_with_function_calls",
			input:     "let x = (foo(a), bar(b))",
			wantMatch: "__TUPLE_2__LITERAL__",
			wantErr:   false,
		},
		{
			name:    "empty_tuple_error",
			input:   "let x = ()",
			wantErr: true,
		},
		{
			name:    "single_element_error",
			input:   "let x = (42,)",
			wantErr: true,
		},
		{
			name:    "too_many_elements_error",
			input:   "let x = (1,2,3,4,5,6,7,8,9,10,11,12,13)",
			wantErr: true,
		},
		{
			name:      "max_elements_ok",
			input:     "let x = (1,2,3,4,5,6,7,8,9,10,11,12)",
			wantMatch: "__TUPLE_12__LITERAL__",
			wantErr:   false,
		},
		{
			name:      "strings_in_tuple",
			input:     `let x = ("hello", "world")`,
			wantMatch: "__TUPLE_2__LITERAL__",
			wantErr:   false,
		},
		{
			name:      "mixed_types",
			input:     `let x = (42, "hello", true)`,
			wantMatch: "__TUPLE_3__LITERAL__",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewTupleProcessor()
			output, _, err := p.Process([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			outputStr := string(output)
			if !strings.Contains(outputStr, tt.wantMatch) {
				t.Errorf("output missing expected substring\nwant substring: %s\ngot: %s", tt.wantMatch, outputStr)
			}
		})
	}
}

func TestTupleProcessor_Destructuring(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLines []string // Substrings that should appear in output lines
		wantErr   bool
	}{
		{
			name:  "basic_destructure",
			input: "let (x, y) = getCoords()",
			wantLines: []string{
				"tmp := getCoords()",
				"x, y := tmp._0, tmp._1",
			},
			wantErr: false,
		},
		{
			name:  "triple_destructure",
			input: "let (a, b, c) = getTriple()",
			wantLines: []string{
				"tmp := getTriple()",
				"a, b, c := tmp._0, tmp._1, tmp._2",
			},
			wantErr: false,
		},
		{
			name:  "wildcard_pattern",
			input: "let (x, _, z) = getData()",
			wantLines: []string{
				"tmp := getData()",
				"x, _, z := tmp._0, tmp._1, tmp._2",
			},
			wantErr: false,
		},
		{
			name:  "all_wildcards",
			input: "let (_, _, _) = getData()",
			wantLines: []string{
				"tmp := getData()",
				"_, _, _ := tmp._0, tmp._1, tmp._2",
			},
			wantErr: false,
		},
		{
			name:  "indented_destructure",
			input: "	let (x, y) = getCoords()",
			wantLines: []string{
				"	tmp := getCoords()",
				"	x, y := tmp._0, tmp._1",
			},
			wantErr: false,
		},
		{
			name:    "empty_destructure_error",
			input:   "let () = getData()",
			wantErr: true,
		},
		{
			name:    "single_destructure_error",
			input:   "let (x) = getData()",
			wantErr: true,
		},
		{
			name:    "too_many_destructure_error",
			input:   "let (a,b,c,d,e,f,g,h,i,j,k,l,m) = getData()",
			wantErr: true,
		},
		{
			name:  "max_destructure_ok",
			input: "let (a,b,c,d,e,f,g,h,i,j,k,l) = getData()",
			wantLines: []string{
				"tmp := getData()",
				"a, b, c, d, e, f, g, h, i, j, k, l := tmp._0, tmp._1, tmp._2, tmp._3, tmp._4, tmp._5, tmp._6, tmp._7, tmp._8, tmp._9, tmp._10, tmp._11",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewTupleProcessor()
			output, _, err := p.Process([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			outputStr := string(output)
			for _, wantLine := range tt.wantLines {
				if !strings.Contains(outputStr, wantLine) {
					t.Errorf("output missing expected line\nwant: %s\ngot: %s", wantLine, outputStr)
				}
			}
		})
	}
}

func TestTupleProcessor_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantMatch string
		wantErr   bool
	}{
		{
			name:      "no_tuples",
			input:     "let x = 10",
			wantMatch: "let x = 10",
			wantErr:   false,
		},
		{
			name:      "empty_line",
			input:     "",
			wantMatch: "",
			wantErr:   false,
		},
		{
			name:      "comment_line",
			input:     "// This is a comment",
			wantMatch: "// This is a comment",
			wantErr:   false,
		},
		{
			name:      "multiple_tuples_same_line",
			input:     "let x = (1, 2); let y = (3, 4)",
			wantMatch: "__TUPLE_2__LITERAL__",
			wantErr:   false,
		},
		{
			name:      "nested_function_calls",
			input:     "let x = (foo(bar(a)), baz(qux(b)))",
			wantMatch: "__TUPLE_2__LITERAL__",
			wantErr:   false,
		},
		{
			name:      "trailing_comma_ignored",
			input:     "let x = (1, 2,)",
			wantMatch: "__TUPLE_2__LITERAL__",
			wantErr:   false,
		},
		{
			name:      "whitespace_in_tuple",
			input:     "let x = ( 1 , 2 , 3 )",
			wantMatch: "__TUPLE_3__LITERAL__",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewTupleProcessor()
			output, _, err := p.Process([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			outputStr := string(output)
			if !strings.Contains(outputStr, tt.wantMatch) {
				t.Errorf("output missing expected substring\nwant: %s\ngot: %s", tt.wantMatch, outputStr)
			}
		})
	}
}

func TestTupleProcessor_TmpVarNaming(t *testing.T) {
	// Test that temporary variables follow camelCase convention
	// Pattern: tmp, tmp1, tmp2, tmp3, ...
	input := `let (a, b) = getFirst()
let (c, d) = getSecond()
let (e, f) = getThird()`

	p := NewTupleProcessor()
	output, _, err := p.Process([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outputStr := string(output)

	// Check for correct tmp variable names
	expectedVars := []string{"tmp", "tmp1", "tmp2"}
	for _, varName := range expectedVars {
		if !strings.Contains(outputStr, varName+" :=") {
			t.Errorf("missing expected tmp var: %s\noutput: %s", varName, outputStr)
		}
	}

	// Ensure no underscore-prefixed variables
	if strings.Contains(outputStr, "__tmp") || strings.Contains(outputStr, "_tmp") {
		t.Errorf("found underscore-prefixed tmp var (should use camelCase)\noutput: %s", outputStr)
	}
}

func TestTupleProcessor_NestedTuples(t *testing.T) {
	// SCOPE REDUCTION (Phase 8): Nested tuples are NOT supported
	// They are silently ignored (not processed as tuples)
	// This prevents generating invalid Go code from nested tuple syntax
	// Nested tuple support will be added in a future release
	tests := []struct {
		name      string
		input     string
		wantCount int // Number of tuple markers expected
	}{
		{
			name:      "two_level_nesting",
			input:     "let x = ((1, 2), (3, 4))",
			wantCount: 0, // SCOPE REDUCTION: Nested tuples not supported - ignored
		},
		{
			name:      "three_level_nesting",
			input:     "let x = (((1, 2), (3, 4)), ((5, 6), (7, 8)))",
			wantCount: 0, // SCOPE REDUCTION: Nested tuples not supported - ignored
		},
		{
			name:      "mixed_nesting",
			input:     "let x = ((1, 2), 3, (4, 5))",
			wantCount: 0, // SCOPE REDUCTION: Nested tuples not supported - ignored
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewTupleProcessor()
			output, _, err := p.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			outputStr := string(output)
			count := strings.Count(outputStr, "__TUPLE_")
			if count != tt.wantCount {
				t.Errorf("expected %d tuple markers, got %d\noutput: %s", tt.wantCount, count, outputStr)
			}
		})
	}
}

func TestTupleProcessor_SourceMappings(t *testing.T) {
	// Test that source mappings are created correctly
	input := "let (x, y) = getCoords()"

	p := NewTupleProcessor()
	_, mappings, err := p.Process([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mappings) == 0 {
		t.Errorf("expected mappings, got none")
	}

	// Check that all mappings reference the original line
	for _, m := range mappings {
		if m.OriginalLine != 1 {
			t.Errorf("expected original line 1, got %d", m.OriginalLine)
		}
		if m.GeneratedLine < 1 {
			t.Errorf("invalid generated line: %d", m.GeneratedLine)
		}
	}
}

func TestDetectTuple(t *testing.T) {
	tests := []struct {
		name         string
		line         string
		startIdx     int
		wantIsTuple  bool
		wantElements int
	}{
		{
			name:         "simple_tuple",
			line:         "(1, 2)",
			startIdx:     0,
			wantIsTuple:  true,
			wantElements: 2,
		},
		{
			name:         "function_call",
			line:         "foo(1, 2)",
			startIdx:     3,
			wantIsTuple:  false,
			wantElements: 0,
		},
		{
			name:         "grouping",
			line:         "(x + y)",
			startIdx:     0,
			wantIsTuple:  false,
			wantElements: 0,
		},
		{
			name:         "nested_tuple",
			line:         "((1, 2), 3)",
			startIdx:     0,
			wantIsTuple:  false, // SCOPE REDUCTION: Nested tuples not supported in Phase 8
			wantElements: 0,
		},
		{
			name:         "tuple_in_expression",
			line:         "x + (1, 2)",
			startIdx:     4,
			wantIsTuple:  true,
			wantElements: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewTupleProcessor()
			isTuple, elements, _ := p.detectTuple(tt.line, tt.startIdx)

			if isTuple != tt.wantIsTuple {
				t.Errorf("detectTuple() isTuple = %v, want %v", isTuple, tt.wantIsTuple)
			}

			if isTuple && len(elements) != tt.wantElements {
				t.Errorf("detectTuple() elements = %d, want %d", len(elements), tt.wantElements)
			}
		})
	}
}

func TestParseElements(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantLen  int
		wantElem string // One element to check
	}{
		{
			name:     "simple",
			content:  "1, 2, 3",
			wantLen:  3,
			wantElem: "1",
		},
		{
			name:     "nested_parens",
			content:  "foo(a, b), bar(c, d)",
			wantLen:  2,
			wantElem: "foo(a, b)",
		},
		{
			name:     "strings",
			content:  `"hello, world", "test"`,
			wantLen:  2,
			wantElem: `"hello, world"`,
		},
		{
			name:     "complex_expressions",
			content:  "a + b, c * (d + e), f",
			wantLen:  3,
			wantElem: "c * (d + e)",
		},
		{
			name:     "trailing_comma",
			content:  "1, 2,",
			wantLen:  3, // ["1", "2", ""] - will be filtered by caller
			wantElem: "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elements := parseElements(tt.content)

			if len(elements) != tt.wantLen {
				t.Errorf("parseElements() len = %d, want %d", len(elements), tt.wantLen)
			}

			// Check that expected element exists
			found := false
			for _, elem := range elements {
				if strings.TrimSpace(elem) == tt.wantElem {
					found = true
					break
				}
			}

			if !found && tt.wantElem != "" {
				t.Errorf("parseElements() missing element %q\ngot: %v", tt.wantElem, elements)
			}
		})
	}
}

func TestTupleProcessor_GoCompatibility(t *testing.T) {
	// CRITICAL: Test that valid Go syntax is NOT transformed
	tests := []struct {
		name      string
		input     string
		wantMatch string // Should remain unchanged (no TUPLE marker)
	}{
		{
			name:      "function_return_type_basic",
			input:     "func foo() (int, string) {",
			wantMatch: "func foo() (int, string) {",
		},
		{
			name:      "function_return_type_named",
			input:     "func bar() (x int, y string) {",
			wantMatch: "func bar() (x int, y string) {",
		},
		{
			name:      "function_return_type_error",
			input:     "func baz() (int, error) {",
			wantMatch: "func baz() (int, error) {",
		},
		{
			name:      "return_statement_multi",
			input:     "	return (42, nil)",
			wantMatch: "return (42, nil)",
		},
		{
			name:      "return_statement_complex",
			input:     "	return (x + y, err)",
			wantMatch: "return (x + y, err)",
		},
		{
			name:      "return_statement_function_calls",
			input:     "	return (getData(), getError())",
			wantMatch: "return (getData(), getError())",
		},
		{
			name:      "method_return_type",
			input:     "func (s *Server) Handle() (Response, error) {",
			wantMatch: "func (s *Server) Handle() (Response, error) {",
		},
		{
			name:      "multiline_function_sig",
			input:     "	) (int, string) {",
			wantMatch: ") (int, string) {",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewTupleProcessor()
			output, _, err := p.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			outputStr := string(output)

			// Should NOT contain tuple marker
			if strings.Contains(outputStr, "__TUPLE_") {
				t.Errorf("INVALID TRANSFORMATION: Go syntax should not be transformed\ninput: %s\noutput: %s", tt.input, outputStr)
			}

			// Should match expected output (unchanged)
			if !strings.Contains(outputStr, tt.wantMatch) {
				t.Errorf("output missing expected content\nwant: %s\ngot: %s", tt.wantMatch, outputStr)
			}
		})
	}
}

func TestTupleProcessor_TupleLiteralsVsGo(t *testing.T) {
	// Test that we correctly distinguish tuple literals from Go syntax
	tests := []struct {
		name        string
		input       string
		shouldMark  bool // true = should transform to __TUPLE__, false = keep as-is
		description string
	}{
		{
			name:        "tuple_literal_assignment",
			input:       "let x = (10, 20)",
			shouldMark:  true,
			description: "Tuple literal in assignment",
		},
		{
			name:        "function_signature",
			input:       "func foo() (int, string) {",
			shouldMark:  false,
			description: "Function return type signature",
		},
		{
			name:        "return_statement",
			input:       "return (x, y)",
			shouldMark:  false,
			description: "Multi-return statement",
		},
		{
			name:        "tuple_in_expression",
			input:       "x := getValue() + (1, 2)",
			shouldMark:  true,
			description: "Tuple literal in expression",
		},
		{
			name:        "grouping_expression",
			input:       "x := (a + b) * c",
			shouldMark:  false,
			description: "Grouping expression (no comma)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewTupleProcessor()
			output, _, err := p.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			outputStr := string(output)
			hasMarker := strings.Contains(outputStr, "__TUPLE_")

			if hasMarker != tt.shouldMark {
				t.Errorf("%s:\nInput: %s\nExpected marker: %v, Got marker: %v\nOutput: %s",
					tt.description, tt.input, tt.shouldMark, hasMarker, outputStr)
			}
		})
	}
}
