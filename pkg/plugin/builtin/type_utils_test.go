package builtin

import (
	"go/types"
	"testing"
)

func TestTypeToString(t *testing.T) {
	tests := []struct {
		name     string
		typ      types.Type
		expected string
	}{
		{
			name:     "nil type",
			typ:      nil,
			expected: "unknown",
		},
		{
			name:     "basic int",
			typ:      types.Typ[types.Int],
			expected: "int",
		},
		{
			name:     "basic string",
			typ:      types.Typ[types.String],
			expected: "string",
		},
		{
			name:     "basic bool",
			typ:      types.Typ[types.Bool],
			expected: "bool",
		},
		{
			name:     "pointer to int",
			typ:      types.NewPointer(types.Typ[types.Int]),
			expected: "ptr_int",
		},
		{
			name:     "slice of string",
			typ:      types.NewSlice(types.Typ[types.String]),
			expected: "slice_string",
		},
		{
			name:     "array of int",
			typ:      types.NewArray(types.Typ[types.Int], 10),
			expected: "array_int",
		},
		{
			name:     "map string to int",
			typ:      types.NewMap(types.Typ[types.String], types.Typ[types.Int]),
			expected: "map_string_int",
		},
		{
			name:     "nested pointer",
			typ:      types.NewPointer(types.NewPointer(types.Typ[types.Int])),
			expected: "ptr_ptr_int",
		},
		{
			name:     "slice of pointers",
			typ:      types.NewSlice(types.NewPointer(types.Typ[types.String])),
			expected: "slice_ptr_string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := typeToString(tt.typ)
			if result != tt.expected {
				t.Errorf("typeToString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSanitizeTypeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple name",
			input:    "MyType",
			expected: "MyType",
		},
		{
			name:     "with dot",
			input:    "pkg.Type",
			expected: "pkg_Type",
		},
		{
			name:     "with pointer",
			input:    "*User",
			expected: "ptr_User",
		},
		{
			name:     "with brackets",
			input:    "map[string]int",
			expected: "map_string_int",
		},
		{
			name:     "with spaces",
			input:    "some type",
			expected: "some_type",
		},
		{
			name:     "with parens",
			input:    "func(int)",
			expected: "func_int_",
		},
		{
			name:     "complex type",
			input:    "map[*pkg.User][]string",
			expected: "map_ptr_pkg_User___string",
		},
		{
			name:     "multiple dots",
			input:    "github.com/pkg/errors.Error",
			expected: "github_com/pkg/errors_Error",
		},
		{
			name:     "nested pointers",
			input:    "**int",
			expected: "ptr_ptr_int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeTypeName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeTypeName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestTypeToStringNamedType tests typeToString with a named type
func TestTypeToStringNamedType(t *testing.T) {
	// Create a named type
	pkg := types.NewPackage("example.com/test", "test")
	typename := types.NewTypeName(0, pkg, "MyStruct", nil)
	named := types.NewNamed(typename, types.NewStruct(nil, nil), nil)

	result := typeToString(named)
	if result != "MyStruct" {
		t.Errorf("typeToString(named) = %q, want %q", result, "MyStruct")
	}
}
