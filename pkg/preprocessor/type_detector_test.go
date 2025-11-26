package preprocessor

import (
	"testing"
)

func TestTypeDetector_InferType_Literals(t *testing.T) {
	td := NewTernaryTypeInferrer()

	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		// String literals
		{"string literal double quotes", `"hello"`, "string"},
		{"string literal with spaces", `"hello world"`, "string"},
		{"string literal empty", `""`, "string"},
		{"string literal raw", "`hello`", "string"},
		{"string literal raw multiline", "`hello\nworld`", "string"},

		// Integer literals
		{"int literal", "42", "int"},
		{"int literal zero", "0", "int"},
		{"int literal negative", "-42", "int"},
		{"int literal hex", "0x1A", "int"},
		{"int literal octal", "0o755", "int"},

		// Float literals
		{"float literal", "3.14", "float64"},
		{"float literal with exponent", "1.5e10", "float64"},
		{"float literal negative", "-2.71", "float64"},

		// Boolean literals
		{"bool true", "true", "bool"},
		{"bool false", "false", "bool"},

		// Rune literal
		{"rune literal", "'a'", "rune"},
		{"rune literal unicode", "'世'", "rune"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := td.InferType(tt.expr)
			if result != tt.expected {
				t.Errorf("InferType(%q) = %q, want %q", tt.expr, result, tt.expected)
			}
		})
	}
}

func TestTypeDetector_InferType_CompositeLiterals(t *testing.T) {
	td := NewTernaryTypeInferrer()

	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		// Array/slice literals
		{"slice literal int", "[]int{1, 2, 3}", "[]int"},
		{"slice literal string", "[]string{\"a\", \"b\"}", "[]string"},
		{"slice literal empty", "[]int{}", "[]int"},
		{"array literal", "[3]int{1, 2, 3}", "[3]int"},

		// Map literals
		{"map literal", "map[string]int{\"a\": 1}", "map[string]int"},
		{"map literal empty", "map[string]int{}", "map[string]int"},

		// Struct literals (no type info available, fallback to any)
		{"struct literal no type", "{Name: \"John\"}", "any"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := td.InferType(tt.expr)
			if result != tt.expected {
				t.Errorf("InferType(%q) = %q, want %q", tt.expr, result, tt.expected)
			}
		})
	}
}

func TestTypeDetector_InferType_Expressions(t *testing.T) {
	td := NewTernaryTypeInferrer()

	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		// Binary expressions
		{"add int", "1 + 2", "int"},
		{"subtract int", "10 - 5", "int"},
		{"multiply int", "3 * 4", "int"},
		{"divide int", "10 / 2", "int"},
		{"add float", "1.5 + 2.5", "float64"},

		// Comparison expressions (always bool)
		{"equals", "x == y", "bool"},
		{"not equals", "x != y", "bool"},
		{"less than", "x < y", "bool"},
		{"greater than", "x > y", "bool"},
		{"less or equal", "x <= y", "bool"},
		{"greater or equal", "x >= y", "bool"},

		// Logical expressions (always bool)
		{"logical not", "!true", "bool"},

		// Parenthesized expressions
		{"parenthesized int", "(42)", "int"},
		{"parenthesized string", `("hello")`, "string"},
		{"parenthesized bool", "(true)", "bool"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := td.InferType(tt.expr)
			if result != tt.expected {
				t.Errorf("InferType(%q) = %q, want %q", tt.expr, result, tt.expected)
			}
		})
	}
}

func TestTypeDetector_InferType_TypeIdentifiers(t *testing.T) {
	td := NewTernaryTypeInferrer()

	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		// Built-in type identifiers
		{"type int", "int", "int"},
		{"type int32", "int32", "int32"},
		{"type int64", "int64", "int64"},
		{"type uint", "uint", "uint"},
		{"type float32", "float32", "float32"},
		{"type float64", "float64", "float64"},
		{"type bool", "bool", "bool"},
		{"type string", "string", "string"},
		{"type byte", "byte", "byte"},
		{"type rune", "rune", "rune"},
		{"type error", "error", "error"},
		{"type any", "any", "any"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := td.InferType(tt.expr)
			if result != tt.expected {
				t.Errorf("InferType(%q) = %q, want %q", tt.expr, result, tt.expected)
			}
		})
	}
}

func TestTypeDetector_InferType_FallbackCases(t *testing.T) {
	td := NewTernaryTypeInferrer()

	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		// Unknown variables (fallback to any)
		{"variable", "x", "any"},
		{"field access", "user.Name", "any"},
		{"function call", "getAge()", "any"},
		{"method call", "user.GetName()", "any"},

		// Invalid expressions
		{"empty", "", "any"},
		{"whitespace only", "   ", "any"},
		{"invalid syntax", "1 +", "any"},

		// Complex expressions (fallback to any)
		{"chained calls", "foo().bar().baz()", "any"},
		{"index expression", "arr[0]", "any"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := td.InferType(tt.expr)
			if result != tt.expected {
				t.Errorf("InferType(%q) = %q, want %q", tt.expr, result, tt.expected)
			}
		})
	}
}

func TestTypeDetector_InferBranchTypes_SameType(t *testing.T) {
	td := NewTernaryTypeInferrer()

	tests := []struct {
		name     string
		trueVal  string
		falseVal string
		expected string
	}{
		{"both string", `"adult"`, `"minor"`, "string"},
		{"both int", "100", "200", "int"},
		{"both bool", "true", "false", "bool"},
		{"both float", "3.14", "2.71", "float64"},
		{"both slice", "[]int{1}", "[]int{2}", "[]int"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := td.InferBranchTypes(tt.trueVal, tt.falseVal)
			if result != tt.expected {
				t.Errorf("InferBranchTypes(%q, %q) = %q, want %q",
					tt.trueVal, tt.falseVal, result, tt.expected)
			}
		})
	}
}

func TestTypeDetector_InferBranchTypes_DifferentTypes(t *testing.T) {
	td := NewTernaryTypeInferrer()

	tests := []struct {
		name     string
		trueVal  string
		falseVal string
		expected string
	}{
		{"string vs int", `"text"`, "42", "any"},
		{"int vs float", "42", "3.14", "float64"}, // Numeric promotion: int + float → float64
		{"bool vs string", "true", `"yes"`, "any"},
		{"int vs bool", "1", "true", "any"},
		{"slice vs int", "[]int{1}", "42", "any"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := td.InferBranchTypes(tt.trueVal, tt.falseVal)
			if result != tt.expected {
				t.Errorf("InferBranchTypes(%q, %q) = %q, want %q",
					tt.trueVal, tt.falseVal, result, tt.expected)
			}
		})
	}
}

func TestTypeDetector_InferBranchTypes_WithAny(t *testing.T) {
	td := NewTernaryTypeInferrer()

	tests := []struct {
		name     string
		trueVal  string
		falseVal string
		expected string
	}{
		{"any vs string", "x", `"text"`, "any"},
		{"int vs any", "42", "y", "any"},
		{"any vs any", "foo()", "bar()", "any"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := td.InferBranchTypes(tt.trueVal, tt.falseVal)
			if result != tt.expected {
				t.Errorf("InferBranchTypes(%q, %q) = %q, want %q",
					tt.trueVal, tt.falseVal, result, tt.expected)
			}
		})
	}
}

func TestTypeDetector_WhitespaceHandling(t *testing.T) {
	td := NewTernaryTypeInferrer()

	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{"leading whitespace", "  42", "int"},
		{"trailing whitespace", "42  ", "int"},
		{"both whitespace", "  42  ", "int"},
		{"string with whitespace", `  "hello"  `, "string"},
		{"bool with whitespace", "  true  ", "bool"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := td.InferType(tt.expr)
			if result != tt.expected {
				t.Errorf("InferType(%q) = %q, want %q", tt.expr, result, tt.expected)
			}
		})
	}
}

func TestTypeDetector_EdgeCases(t *testing.T) {
	td := NewTernaryTypeInferrer()

	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		// Special numeric formats
		{"hex integer", "0xFF", "int"},
		{"octal integer", "0o777", "int"},
		{"binary integer", "0b1010", "int"},

		// Float edge cases
		{"float scientific", "1.23e-4", "float64"},
		{"float negative exponent", "5E-10", "float64"},

		// String edge cases
		{"string with quotes inside", `"He said \"hello\""`, "string"},
		{"string with backslash", `"path\\to\\file"`, "string"},
		{"raw string with quotes", "`She said \"hi\"`", "string"},

		// Empty/whitespace in composites
		{"empty slice type", "[]int{}", "[]int"},
		{"empty map type", "map[string]int{}", "map[string]int"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := td.InferType(tt.expr)
			if result != tt.expected {
				t.Errorf("InferType(%q) = %q, want %q", tt.expr, result, tt.expected)
			}
		})
	}
}

func TestTypeDetector_NewInstance(t *testing.T) {
	td := NewTernaryTypeInferrer()

	if td == nil {
		t.Fatal("NewTernaryTypeInferrer() returned nil")
	}

	if td.fset == nil {
		t.Error("TypeDetector.fset is nil")
	}

	if td.config == nil {
		t.Error("TypeDetector.config is nil")
	}

	// Verify it's usable
	result := td.InferType("42")
	if result != "int" {
		t.Errorf("New TypeDetector failed basic test: got %q, want %q", result, "int")
	}
}

// Benchmark tests
func BenchmarkTypeDetector_StringLiteral(b *testing.B) {
	td := NewTernaryTypeInferrer()
	expr := `"hello world"`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = td.InferType(expr)
	}
}

func BenchmarkTypeDetector_IntLiteral(b *testing.B) {
	td := NewTernaryTypeInferrer()
	expr := "42"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = td.InferType(expr)
	}
}

func BenchmarkTypeDetector_BoolLiteral(b *testing.B) {
	td := NewTernaryTypeInferrer()
	expr := "true"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = td.InferType(expr)
	}
}

func BenchmarkTypeDetector_CompositeLiteral(b *testing.B) {
	td := NewTernaryTypeInferrer()
	expr := "[]int{1, 2, 3}"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = td.InferType(expr)
	}
}

func BenchmarkTypeDetector_BinaryExpression(b *testing.B) {
	td := NewTernaryTypeInferrer()
	expr := "1 + 2"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = td.InferType(expr)
	}
}

func BenchmarkTypeDetector_InferBranchTypes(b *testing.B) {
	td := NewTernaryTypeInferrer()
	trueVal := `"adult"`
	falseVal := `"minor"`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = td.InferBranchTypes(trueVal, falseVal)
	}
}
