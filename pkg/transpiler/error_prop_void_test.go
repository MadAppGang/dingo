package transpiler

import (
	"strings"
	"testing"
)

// TestErrorPropInVoidFunction verifies that the ? operator produces a clear
// transpilation error when used inside functions that cannot propagate errors.
// This covers void functions, non-error-returning functions, and validates
// that functions with error returns still work correctly.
func TestErrorPropInVoidFunction(t *testing.T) {
	tests := []struct {
		name           string
		source         string
		expectError    bool
		errorContains  []string // substrings the error message must contain
		outputContains []string // substrings the output must contain (for success cases)
	}{
		{
			name: "? in main() produces error",
			source: `package main

import "strconv"

func main() {
	val := strconv.Atoi("42")?
	_ = val
}
`,
			expectError:   true,
			errorContains: []string{"cannot use ? operator", "main"},
		},
		{
			name: "? in init() produces error",
			source: `package main

func mayFail() error {
	return nil
}

func init() {
	mayFail()?
}
`,
			expectError:   true,
			errorContains: []string{"cannot use ? operator", "init"},
		},
		{
			name: "? in custom void function produces error",
			source: `package main

func mayFail() error {
	return nil
}

func processItems() {
	mayFail()?
}
`,
			expectError:   true,
			errorContains: []string{"cannot use ? operator", "processItems"},
		},
		{
			name: "? with assignment in void function produces error",
			source: `package main

import "strconv"

func main() {
	val := strconv.Atoi("42")?
	_ = val
}
`,
			expectError:   true,
			errorContains: []string{"cannot use ? operator", "main"},
		},
		{
			name: "? in function returning int (no error) produces error",
			source: `package main

import "strconv"

func getCount() int {
	n := strconv.Atoi("42")?
	return n
}
`,
			expectError:   true,
			errorContains: []string{"cannot use ? operator", "getCount"},
		},
		{
			name: "? in function returning error - succeeds",
			source: `package main

func mayFail() error {
	return nil
}

func wrapper() error {
	mayFail()?
	return nil
}
`,
			expectError:    false,
			outputContains: []string{"func wrapper() error"},
		},
		{
			name: "? in function returning (int, error) - succeeds",
			source: `package main

import "strconv"

func parseNumber() (int, error) {
	val := strconv.Atoi("42")?
	return val, nil
}
`,
			expectError:    false,
			outputContains: []string{"func parseNumber() (int, error)"},
		},
		{
			name: "? in function returning (string, error) - succeeds",
			source: `package main

import "os"

func readConfig() (string, error) {
	data := os.ReadFile("config.txt")?
	return string(data), nil
}
`,
			expectError:    false,
			outputContains: []string{"func readConfig() (string, error)"},
		},
		{
			name: "error message suggests alternatives",
			source: `package main

func mayFail() error {
	return nil
}

func main() {
	mayFail()?
}
`,
			expectError: true,
			// Error message should include helpful guidance
			errorContains: []string{"cannot use ? operator", "handle the error explicitly"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := PureASTTranspile([]byte(tt.source), "test_void.dingo")

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected transpilation error for ? in non-error function, got nil\nGenerated output:\n%s", string(result))
				}
				for _, substr := range tt.errorContains {
					if !strings.Contains(err.Error(), substr) {
						t.Errorf("error should contain %q, got: %s", substr, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Fatalf("expected successful transpilation, got error: %v", err)
				}
				goCode := string(result)
				for _, substr := range tt.outputContains {
					if !strings.Contains(goCode, substr) {
						t.Errorf("output should contain %q, got:\n%s", substr, goCode)
					}
				}
			}
		})
	}
}
