package builtin

import "testing"

func TestSanitizeTypeName(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		expected string
	}{
		// Single built-in types (leading underscore for type params)
		{
			name:     "int",
			parts:    []string{"int"},
			expected: "_int",
		},
		{
			name:     "string",
			parts:    []string{"string"},
			expected: "_string",
		},
		{
			name:     "error",
			parts:    []string{"error"},
			expected: "_error",
		},
		{
			name:     "bool",
			parts:    []string{"bool"},
			expected: "_bool",
		},
		{
			name:     "any → interface",
			parts:    []string{"any"},
			expected: "_interface",
		},

		// Two-part type names (leading underscore, underscore separated)
		{
			name:     "int + error",
			parts:    []string{"int", "error"},
			expected: "_int_error",
		},
		{
			name:     "string + option",
			parts:    []string{"string", "option"},
			expected: "_string_option",
		},
		{
			name:     "any + error",
			parts:    []string{"any", "error"},
			expected: "_interface_error",
		},

		// User-defined types (preserve capitalization with leading underscore)
		{
			name:     "User",
			parts:    []string{"User"},
			expected: "_User",
		},
		{
			name:     "CustomError",
			parts:    []string{"CustomError"},
			expected: "_CustomError",
		},
		{
			name:     "UserID",
			parts:    []string{"UserID"},
			expected: "_UserID",
		},

		// User types in compound names
		{
			name:     "CustomError + int",
			parts:    []string{"CustomError", "int"},
			expected: "_CustomError_int",
		},
		{
			name:     "int + CustomError",
			parts:    []string{"int", "CustomError"},
			expected: "_int_CustomError",
		},
		{
			name:     "UserID + error",
			parts:    []string{"UserID", "error"},
			expected: "_UserID_error",
		},

		// Pointer types
		{
			name:     "*User",
			parts:    []string{"*User"},
			expected: "_ptr_User",
		},
		{
			name:     "*int + error",
			parts:    []string{"*int", "error"},
			expected: "_ptr_int_error",
		},

		// Slice types
		{
			name:     "[]string",
			parts:    []string{"[]string"},
			expected: "_slice_string",
		},
		{
			name:     "[]int + error",
			parts:    []string{"[]int", "error"},
			expected: "_slice_int_error",
		},

		// Three-part names
		{
			name:     "int + string + error",
			parts:    []string{"int", "string", "error"},
			expected: "_int_string_error",
		},

		// Edge cases
		{
			name:     "numeric types",
			parts:    []string{"int64", "error"},
			expected: "_int64_error",
		},
		{
			name:     "uint types",
			parts:    []string{"uint32"},
			expected: "_uint32",
		},
		{
			name:     "float types",
			parts:    []string{"float64", "error"},
			expected: "_float64_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeTypeName(tt.parts...)
			if result != tt.expected {
				t.Errorf("SanitizeTypeName(%v) = %q, want %q",
					tt.parts, result, tt.expected)
			}
		})
	}
}

func TestGenerateTempVarName(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		index    int
		expected string
	}{
		// First variable (no number suffix)
		{
			name:     "ok first",
			base:     "ok",
			index:    0,
			expected: "ok",
		},
		{
			name:     "err first",
			base:     "err",
			index:    0,
			expected: "err",
		},
		{
			name:     "tmp first",
			base:     "tmp",
			index:    0,
			expected: "tmp",
		},

		// Second variable (add number)
		{
			name:     "ok second",
			base:     "ok",
			index:    1,
			expected: "ok1",
		},
		{
			name:     "err second",
			base:     "err",
			index:    1,
			expected: "err1",
		},
		{
			name:     "tmp second",
			base:     "tmp",
			index:    1,
			expected: "tmp1",
		},

		// Third variable
		{
			name:     "ok third",
			base:     "ok",
			index:    2,
			expected: "ok2",
		},
		{
			name:     "err third",
			base:     "err",
			index:    2,
			expected: "err2",
		},

		// Higher indices
		{
			name:     "ok tenth",
			base:     "ok",
			index:    9,
			expected: "ok9",
		},
		{
			name:     "err twentieth",
			base:     "err",
			index:    19,
			expected: "err19",
		},

		// Different base names
		{
			name:     "val first",
			base:     "val",
			index:    0,
			expected: "val",
		},
		{
			name:     "val second",
			base:     "val",
			index:    1,
			expected: "val1",
		},
		{
			name:     "result first",
			base:     "result",
			index:    0,
			expected: "result",
		},
		{
			name:     "result second",
			base:     "result",
			index:    1,
			expected: "result1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateTempVarName(tt.base, tt.index)
			if result != tt.expected {
				t.Errorf("GenerateTempVarName(%q, %d) = %q, want %q",
					tt.base, tt.index, result, tt.expected)
			}
		})
	}
}

func TestSanitizeTypeComponent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Built-in types (preserve lowercase)
		{
			name:     "int",
			input:    "int",
			expected: "int",
		},
		{
			name:     "string",
			input:    "string",
			expected: "string",
		},
		{
			name:     "error",
			input:    "error",
			expected: "error",
		},
		{
			name:     "any → interface",
			input:    "any",
			expected: "interface",
		},
		{
			name:     "interface{} → interface",
			input:    "interface{}",
			expected: "interface",
		},

		// Pointer types
		{
			name:     "*User",
			input:    "*User",
			expected: "ptr_User",
		},
		{
			name:     "*int",
			input:    "*int",
			expected: "ptr_int",
		},

		// Slice types
		{
			name:     "[]string",
			input:    "[]string",
			expected: "slice_string",
		},
		{
			name:     "[]int",
			input:    "[]int",
			expected: "slice_int",
		},

		// User types (preserve case)
		{
			name:     "User",
			input:    "User",
			expected: "User",
		},
		{
			name:     "CustomError",
			input:    "CustomError",
			expected: "CustomError",
		},

		// Edge cases
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeTypeComponent(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeTypeComponent(%q) = %q, want %q",
					tt.input, result, tt.expected)
			}
		})
	}
}
