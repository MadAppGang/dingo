package preprocessor

import (
	"strings"
	"testing"
)

// TestUnqualifiedTransform_MethodDeclaration verifies that method declarations
// are not treated as unqualified function calls
func TestUnqualifiedTransform_MethodDeclaration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "method declaration with Map",
			input: `package main

type Result struct{}

func (r Result) Map(f func(interface{}) interface{}) Result {
	return r
}
`,
			expected: `package main

type Result struct{}

func (r Result) Map(f func(interface{}) interface{}) Result {
	return r
}
`,
		},
		{
			name: "method call should not be transformed",
			input: `package main

func process() {
	result := getResult()
	transformed := result.Map(func(v interface{}) interface{} {
		return v
	})
}
`,
			expected: `package main

func process() {
	result := getResult()
	transformed := result.Map(func(v interface{}) interface{} {
		return v
	})
}
`,
		},
		{
			name: "stdlib method call should not be transformed",
			input: `package main

func process(s string) {
	mapped := strings.Map(func(r rune) rune {
		return r
	}, s)
}
`,
			expected: `package main

func process(s string) {
	mapped := strings.Map(func(r rune) rune {
		return r
	}, s)
}
`,
		},
		{
			name: "standalone function call should be transformed",
			input: `package main

func process() {
	Printf("hello")
}
`,
			expected: `package main

func process() {
	fmt.Printf("hello")
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewFunctionExclusionCache("/tmp/test")
			// Mark local functions (getResult, process) as local
			cache.localFunctions = map[string]bool{
				"getResult": true,
				"process":   true,
			}

			processor := NewUnqualifiedImportProcessor(cache)
			result, _, err := processor.Process([]byte(tt.input))

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if string(result) != tt.expected {
				t.Errorf("unexpected result:\nGot:\n%s\n\nExpected:\n%s", string(result), tt.expected)
			}
		})
	}
}

// TestUnqualifiedTransform_ComplexMethodDeclarations tests various method declaration patterns
func TestUnqualifiedTransform_ComplexMethodDeclarations(t *testing.T) {
	input := `package main

type Option struct{}
type Result struct{}

// Simple method
func (o Option) Map(f func(interface{}) interface{}) Option {
	return o
}

// Pointer receiver
func (o *Option) Filter(f func(interface{}) bool) Option {
	return *o
}

// Generic-style method
func (r Result) AndThen(f func(interface{}) Result) Result {
	return r
}

// Multiple receivers in same file
func (r *Result) OrElse(f func() Result) Result {
	return *r
}
`

	cache := NewFunctionExclusionCache("/tmp/test")
	// No local functions in this test
	cache.localFunctions = map[string]bool{}

	processor := NewUnqualifiedImportProcessor(cache)
	result, _, err := processor.Process([]byte(input))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not transform any of the method names
	if strings.Contains(string(result), ".Map(") ||
		strings.Contains(string(result), ".Filter(") ||
		strings.Contains(string(result), ".AndThen(") ||
		strings.Contains(string(result), ".OrElse(") {
		t.Errorf("method declarations were incorrectly transformed:\n%s", string(result))
	}

	// Should be identical to input (no transformations)
	if string(result) != input {
		t.Errorf("unexpected transformations:\nGot:\n%s\n\nExpected:\n%s", string(result), input)
	}
}
