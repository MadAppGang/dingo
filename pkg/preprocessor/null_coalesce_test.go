package preprocessor

import (
	"strings"
	"testing"
)

func TestNullCoalesceProcessor_SimpleIdentifier(t *testing.T) {
	processor := NewNullCoalesceProcessor()

	source := `let x = value ?? "default"`
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	output := string(result)

	// Should generate inline IIFE
	if !strings.Contains(output, "func()") {
		t.Errorf("Expected IIFE, got: %s", output)
	}

	// Should check and use __UNWRAP__ placeholder
	if !strings.Contains(output, "IsSome()") || !strings.Contains(output, "__UNWRAP__") {
		t.Errorf("Expected Option check, got: %s", output)
	}

	// Should return default
	if !strings.Contains(output, `return "default"`) {
		t.Errorf("Expected default return, got: %s", output)
	}
}

func TestNullCoalesceProcessor_ChainedCoalesce(t *testing.T) {
	processor := NewNullCoalesceProcessor()

	source := `let x = value ?? fallback ?? "default"`
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	output := string(result)

	// Should generate IIFE (complex case)
	if !strings.Contains(output, "func()") {
		t.Errorf("Expected IIFE, got: %s", output)
	}

	// Should check value first
	if !strings.Contains(output, "value") {
		t.Errorf("Expected value check, got: %s", output)
	}

	// Should check fallback second
	if !strings.Contains(output, "fallback") {
		t.Errorf("Expected fallback check, got: %s", output)
	}

	// Should return default last
	if !strings.Contains(output, `"default"`) {
		t.Errorf("Expected default return, got: %s", output)
	}
}

func TestNullCoalesceProcessor_ComplexLeft(t *testing.T) {
	processor := NewNullCoalesceProcessor()

	source := `let x = getValue() ?? "default"`
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	output := string(result)

	// Should generate IIFE (complex case)
	if !strings.Contains(output, "func()") {
		t.Errorf("Expected IIFE, got: %s", output)
	}

	// Should call getValue() once
	if !strings.Contains(output, "getValue()") {
		t.Errorf("Expected getValue() call, got: %s", output)
	}
}

func TestNullCoalesceProcessor_SafeNavChain(t *testing.T) {
	processor := NewNullCoalesceProcessor()

	source := `let x = user?.name ?? "Unknown"`
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	output := string(result)

	// Should generate IIFE (complex case)
	if !strings.Contains(output, "func()") {
		t.Errorf("Expected IIFE, got: %s", output)
	}

	// Should reference safe nav result
	if !strings.Contains(output, "user?.name") {
		t.Errorf("Expected safe nav reference, got: %s", output)
	}

	// Should return default
	if !strings.Contains(output, `"Unknown"`) {
		t.Errorf("Expected default return, got: %s", output)
	}
}

func TestNullCoalesceProcessor_NumberLiteral(t *testing.T) {
	processor := NewNullCoalesceProcessor()

	source := `let x = count ?? 0`
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	output := string(result)

	// Should generate inline IIFE
	if !strings.Contains(output, "func()") {
		t.Errorf("Expected IIFE, got: %s", output)
	}

	// Should return 0
	if !strings.Contains(output, "return 0") {
		t.Errorf("Expected 0 return, got: %s", output)
	}
}

func TestNullCoalesceProcessor_NoOperator(t *testing.T) {
	processor := NewNullCoalesceProcessor()

	source := `let x = value`
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	output := string(result)

	// Should be unchanged
	if output != source {
		t.Errorf("Expected unchanged, got: %s", output)
	}
}

func TestNullCoalesceProcessor_MultipleOnSameLine(t *testing.T) {
	processor := NewNullCoalesceProcessor()

	source := `let x = a ?? "x"; let y = b ?? "y"`
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	output := string(result)

	// Should process both
	if strings.Count(output, "func()") < 2 {
		t.Errorf("Expected 2 IIFEs, got: %s", output)
	}
}

func TestNullCoalesceProcessor_BooleanLiteral(t *testing.T) {
	processor := NewNullCoalesceProcessor()

	source := `let x = flag ?? true`
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	output := string(result)

	// Should generate inline IIFE
	if !strings.Contains(output, "func()") {
		t.Errorf("Expected IIFE, got: %s", output)
	}

	// Should return true
	if !strings.Contains(output, "return true") {
		t.Errorf("Expected true return, got: %s", output)
	}
}

func TestNullCoalesceProcessor_ComplexChain(t *testing.T) {
	processor := NewNullCoalesceProcessor()

	source := `let x = getUser() ?? getFallback() ?? "none"`
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	output := string(result)

	// Should generate IIFE
	if !strings.Contains(output, "func()") {
		t.Errorf("Expected IIFE, got: %s", output)
	}

	// Should check getUser() first
	if !strings.Contains(output, "getUser()") {
		t.Errorf("Expected getUser() call, got: %s", output)
	}

	// Should check getFallback() second
	if !strings.Contains(output, "getFallback()") {
		t.Errorf("Expected getFallback() call, got: %s", output)
	}

	// Should return "none" last
	if !strings.Contains(output, `"none"`) {
		t.Errorf("Expected none return, got: %s", output)
	}
}

func TestIsIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"user", true},
		{"_value", true},
		{"count123", true},
		{"123count", false},
		{"user.name", false},
		{"getValue()", false},
		{"", false},
		{"a", true},
		{"_", true},
	}

	for _, tt := range tests {
		result := isIdentifier(tt.input)
		if result != tt.expected {
			t.Errorf("isIdentifier(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestIsLiteral(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{`"hello"`, true},
		{`'hello'`, true},
		{"`hello`", true},
		{"123", true},
		{"123.45", true},
		{"true", true},
		{"false", true},
		{"nil", true},
		{"user", false},
		{"getValue()", false},
		{`"unclosed`, false},
		{"", false},
	}

	for _, tt := range tests {
		result := isLiteral(tt.input)
		if result != tt.expected {
			t.Errorf("isLiteral(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestClassifyComplexity(t *testing.T) {
	processor := NewNullCoalesceProcessor()

	tests := []struct {
		chain    []string
		expected CoalesceComplexity
	}{
		{[]string{"value", `"default"`}, ComplexitySimple},
		{[]string{"count", "0"}, ComplexitySimple},
		{[]string{"flag", "true"}, ComplexitySimple},
		{[]string{"getValue()", `"default"`}, ComplexityComplex},
		{[]string{"value", "getFallback()"}, ComplexityComplex},
		{[]string{"user?.name", `"Unknown"`}, ComplexityComplex},
		{[]string{"a", "b", "c"}, ComplexityComplex}, // Chained
	}

	for _, tt := range tests {
		result := processor.classifyComplexity(tt.chain)
		if result != tt.expected {
			t.Errorf("classifyComplexity(%v) = %v, want %v", tt.chain, result, tt.expected)
		}
	}
}

func TestExtractOperandBefore(t *testing.T) {
	tests := []struct {
		line     string
		end      int
		expected int
	}{
		{"value ?? default", 6, 0},             // "value "
		{"  user ?? default", 7, 2},            // "  user "
		{`"hello" ?? default`, 8, 0},           // "\"hello\" "
		{"getValue() ?? default", 11, 0},       // "getValue() "
		{"user?.name ?? default", 11, 0},       // "user?.name "
		{"123 ?? default", 4, 0},               // "123 "
		{"123.45 ?? default", 7, 0},            // "123.45 "
		{"a + b ?? default", 6, 4},             // Extract "b" only
		{"x = value ?? default", 10, 4},        // Extract "value"
		{"obj.method() ?? default", 12, 0},     // "obj.method()"
		{"user?.getName() ?? default", 16, 0}, // "user?.getName()"
	}

	for _, tt := range tests {
		result := extractOperandBefore(tt.line, tt.end)
		if result != tt.expected {
			t.Errorf("extractOperandBefore(%q, %d) = %d, want %d (extracted: %q)",
				tt.line, tt.end, result, tt.expected,
				tt.line[result:tt.end])
		}
	}
}

func TestExtractOperandAfter(t *testing.T) {
	tests := []struct {
		line     string
		start    int
		expected int
	}{
		{"value ?? default", 9, 16},                  // "default"
		{`value ?? "default"`, 9, 18},                // "\"default\""
		{"value ?? 123", 9, 12},                      // "123"
		{"value ?? 123.45", 9, 15},                   // "123.45"
		{"value ?? true", 9, 13},                     // "true"
		{"value ?? getValue()", 9, 19},               // "getValue()"
		{"value ?? getX(a, b)", 9, 19},               // "getX(a, b)"
		{`value ?? "hello world"`, 9, 22},            // "\"hello world\""
		{"value ?? fallback ?? default", 9, 17},      // "fallback"
		{"value ?? user?.name", 9, 19},               // "user?.name" (safe nav chain)
		{"value ?? obj.method()", 9, 21},             // "obj.method()" (method chain)
		{`value ?? "quoted \"string\""`, 9, 28},      // Escaped quotes
		{"value ?? getData(nested())", 9, 26},        // Nested parens
		{"value ?? getData(a, f(x))", 9, 25},         // Nested with comma
		{`value ?? "string with, comma"`, 9, 29},     // Comma in string
		{"value ?? a + b", 9, 10},                    // Just "a"
		{"value ?? -123", 9, 13},                     // "-123" (negative number)
		{`value ?? "multi\nline"`, 9, 22},            // Escape sequences
		{`value ?? 'single'`, 9, 17},                 // Single quotes
		{"value ?? `backtick`", 9, 19},               // Backticks
		{"value ?? getData(f(g(h())))", 9, 27},       // Deep nesting
		{`value ?? "a(b)c"`, 9, 16},                  // Parens in string
		{"value ?? func(a, b, c)", 9, 22},            // Multiple args
		{`value ?? "test\"quote"`, 9, 22},            // Escaped quote
		{"value ?? 3.14159", 9, 16},                  // Float
		{"value ?? nil", 9, 12},                      // nil keyword
		{"value ?? user", 9, 13},                     // Simple identifier
		{"value ?? _private", 9, 17},                 // Underscore prefix
		{"value ?? count123", 9, 17},                 // Alphanumeric
		{"value ?? User", 9, 13},                     // Uppercase identifier
		{"value ?? CONSTANT", 9, 17},                 // All uppercase
		{`value ?? ""`, 9, 11},                       // Empty string
		{"value ?? 0", 9, 10},                        // Zero
		{"value ?? false", 9, 14},                    // false literal
		{"value ?? getData()", 9, 18},                // No args
		{"value ?? f()", 9, 12},                      // Short function
		{`value ?? "a\"b\"c"`, 9, 18},                // Multiple escapes
		{"value ?? num.method()", 9, 21},             // "num.method()" (method chain)
		{"value ?? arr[0]", 9, 12},                   // Stops at [ (array access)
		{"value ?? ptr->field", 9, 12},               // Stops at - (not valid Go but test boundary)
		{"value ?? x", 9, 10},                        // Single char
		{`value ?? "\""`, 9, 13},                     // Just escaped quote
		{"value ?? 1.0", 9, 12},                      // Simple float
		{"value ?? .5", 9, 11},                       // ".5" (decimal starting with .)
		{`value ?? "\n\t"`, 9, 15},                   // Escape sequences
		{"value ?? func()", 9, 15},                   // 'func' as identifier
		{"value ?? return", 9, 15},                   // Keyword as identifier
		{`value ?? "hello`, 9, -1},                   // Unclosed string
		{"value ?? getData(", 9, -1},                 // Unclosed paren
		{"value ?? getData(a, b", 9, -1},             // Unclosed with args
		{`value ?? "quote\"`, 9, -1},                 // Unclosed with escape
		{"value ?? ((nested", 9, -1},                 // Unbalanced nested
		{"value ?? f(g(", 9, -1},                     // Unbalanced nested functions
	}

	for _, tt := range tests {
		result := extractOperandAfter(tt.line, tt.start)
		if result != tt.expected {
			extracted := ""
			if result != -1 {
				extracted = tt.line[tt.start:result]
			}
			t.Errorf("extractOperandAfter(%q, %d) = %d, want %d (extracted: %q)",
				tt.line, tt.start, result, tt.expected, extracted)
		}
	}
}

func TestNullCoalesceProcessor_TypeDetection(t *testing.T) {
	processor := NewNullCoalesceProcessor()

	source := `let user: *User = getUser()
let name = user ?? "Unknown"`

	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	output := string(result)

	// First line unchanged
	if !strings.Contains(output, "let user: *User = getUser()") {
		t.Errorf("Expected first line preserved, got: %s", output)
	}

	// Second line should have null coalesce
	if !strings.Contains(output, "func()") {
		t.Errorf("Expected IIFE in second line, got: %s", output)
	}
}

func TestNullCoalesceProcessor_CommentsIgnored(t *testing.T) {
	processor := NewNullCoalesceProcessor()

	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "single-line comment with ??",
			source: "// Use value ?? default\nlet x = a ?? b",
			want:   "// Use value ?? default",
		},
		{
			name:   "inline comment with ??",
			source: "let x = a ?? b // fallback: a ?? b",
			want:   "// fallback: a ?? b",
		},
		{
			name:   "multi-line comment with ??",
			source: "/* This is ?? not code */\nlet x = a ?? b",
			want:   "/* This is ?? not code */",
		},
		{
			name:   "code with inline comment",
			source: `let result = value ?? "default" // Use ?? for null coalescing`,
			want:   "// Use ?? for null coalescing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := processor.Process([]byte(tt.source))
			if err != nil {
				t.Fatalf("Process failed: %v", err)
			}

			output := string(result)

			// Comment should be preserved exactly
			if !strings.Contains(output, tt.want) {
				t.Errorf("Expected comment preserved: %q\nGot: %s", tt.want, output)
			}

			// Comment should NOT be transformed
			lines := strings.Split(output, "\n")
			for _, line := range lines {
				if strings.HasPrefix(strings.TrimSpace(line), "//") ||
				   strings.HasPrefix(strings.TrimSpace(line), "/*") {
					// This is a comment line - should not contain func() or IIFE
					if strings.Contains(line, "func()") {
						t.Errorf("Comment was transformed (contains func()): %s", line)
					}
				}
			}
		})
	}
}

func TestNullCoalesceProcessor_StringLiteralsWithCommentMarkers(t *testing.T) {
	processor := NewNullCoalesceProcessor()

	// String literals containing // should not be treated as comments
	source := `let url = value ?? "http://example.com"`
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	output := string(result)

	// Should preserve the URL in string literal
	if !strings.Contains(output, "http://example.com") {
		t.Errorf("Expected URL preserved, got: %s", output)
	}

	// Should still transform the ??
	if !strings.Contains(output, "func()") {
		t.Errorf("Expected ?? to be transformed, got: %s", output)
	}
}
