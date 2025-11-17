package preprocessor

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestEnumProcessor_SimpleEnum(t *testing.T) {
	source := `package main

enum Status {
	Pending,
	Active,
	Complete,
}
`

	processor := NewEnumProcessor()
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process() failed: %v", err)
	}

	output := string(result)

	// Verify tag type is generated
	if !strings.Contains(output, "type StatusTag uint8") {
		t.Error("Missing StatusTag type")
	}

	// Verify tag constants
	if !strings.Contains(output, "StatusTag_Pending StatusTag = iota") {
		t.Error("Missing StatusTag_Pending constant")
	}
	if !strings.Contains(output, "StatusTag_Active") {
		t.Error("Missing StatusTag_Active constant")
	}
	if !strings.Contains(output, "StatusTag_Complete") {
		t.Error("Missing StatusTag_Complete constant")
	}

	// Verify struct type
	if !strings.Contains(output, "type Status struct") {
		t.Error("Missing Status struct")
	}
	if !strings.Contains(output, "tag StatusTag") {
		t.Error("Missing tag field in Status struct")
	}

	// Verify constructors
	if !strings.Contains(output, "func Status_Pending() Status") {
		t.Error("Missing Status_Pending constructor")
	}
	if !strings.Contains(output, "func Status_Active() Status") {
		t.Error("Missing Status_Active constructor")
	}
	if !strings.Contains(output, "func Status_Complete() Status") {
		t.Error("Missing Status_Complete constructor")
	}

	// Verify Is* methods
	if !strings.Contains(output, "func (e Status) IsPending() bool") {
		t.Error("Missing IsPending method")
	}
	if !strings.Contains(output, "func (e Status) IsActive() bool") {
		t.Error("Missing IsActive method")
	}
	if !strings.Contains(output, "func (e Status) IsComplete() bool") {
		t.Error("Missing IsComplete method")
	}

	// Verify generated code compiles
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "", result, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("Generated code does not compile: %v\nGenerated code:\n%s", parseErr, output)
	}
}

func TestEnumProcessor_StructVariant(t *testing.T) {
	source := `package main

enum Shape {
	Point,
	Circle { radius: float64 },
	Rectangle { width: float64, height: float64 },
}
`

	processor := NewEnumProcessor()
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process() failed: %v", err)
	}

	output := string(result)

	// Verify tag type and constants
	if !strings.Contains(output, "type ShapeTag uint8") {
		t.Error("Missing ShapeTag type")
	}
	if !strings.Contains(output, "ShapeTag_Point") {
		t.Error("Missing ShapeTag_Point")
	}
	if !strings.Contains(output, "ShapeTag_Circle") {
		t.Error("Missing ShapeTag_Circle")
	}
	if !strings.Contains(output, "ShapeTag_Rectangle") {
		t.Error("Missing ShapeTag_Rectangle")
	}

	// Verify struct fields for variants
	if !strings.Contains(output, "circle_radius *float64") {
		t.Error("Missing circle_radius field")
	}
	if !strings.Contains(output, "rectangle_width *float64") {
		t.Error("Missing rectangle_width field")
	}
	if !strings.Contains(output, "rectangle_height *float64") {
		t.Error("Missing rectangle_height field")
	}

	// Verify constructors with parameters
	if !strings.Contains(output, "func Shape_Point() Shape") {
		t.Error("Missing Shape_Point constructor")
	}
	if !strings.Contains(output, "func Shape_Circle(radius float64) Shape") {
		t.Error("Missing Shape_Circle constructor with parameter")
	}
	if !strings.Contains(output, "func Shape_Rectangle(width float64, height float64) Shape") {
		t.Error("Missing Shape_Rectangle constructor with parameters")
	}

	// Verify Is* methods
	if !strings.Contains(output, "func (e Shape) IsPoint() bool") {
		t.Error("Missing IsPoint method")
	}
	if !strings.Contains(output, "func (e Shape) IsCircle() bool") {
		t.Error("Missing IsCircle method")
	}
	if !strings.Contains(output, "func (e Shape) IsRectangle() bool") {
		t.Error("Missing IsRectangle method")
	}

	// Verify generated code compiles
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "", result, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("Generated code does not compile: %v\nGenerated code:\n%s", parseErr, output)
	}
}

func TestEnumProcessor_GenericEnum(t *testing.T) {
	source := `package main

enum Option {
	None,
	Some { value: T },
}
`

	processor := NewEnumProcessor()
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process() failed: %v", err)
	}

	output := string(result)

	// Verify basic structure is generated
	if !strings.Contains(output, "type OptionTag uint8") {
		t.Error("Missing OptionTag type")
	}
	if !strings.Contains(output, "type Option struct") {
		t.Error("Missing Option struct")
	}

	// Verify None variant (unit variant)
	if !strings.Contains(output, "func Option_None() Option") {
		t.Error("Missing Option_None constructor")
	}

	// Verify Some variant (with generic type T)
	if !strings.Contains(output, "func Option_Some(value T) Option") {
		t.Error("Missing Option_Some constructor")
	}
	if !strings.Contains(output, "some_value *T") {
		t.Error("Missing some_value field with type T")
	}

	// Note: Generic code won't compile without type parameters on the enum itself
	// This test just verifies structure is generated correctly
}

func TestEnumProcessor_MultipleEnums(t *testing.T) {
	source := `package main

enum Color {
	Red,
	Green,
	Blue,
}

enum Size {
	Small,
	Medium,
	Large,
}
`

	processor := NewEnumProcessor()
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process() failed: %v", err)
	}

	output := string(result)

	// Verify both enums are generated
	if !strings.Contains(output, "type ColorTag uint8") {
		t.Error("Missing ColorTag type")
	}
	if !strings.Contains(output, "type SizeTag uint8") {
		t.Error("Missing SizeTag type")
	}

	// Verify Color variants
	if !strings.Contains(output, "func Color_Red() Color") {
		t.Error("Missing Color_Red constructor")
	}

	// Verify Size variants
	if !strings.Contains(output, "func Size_Small() Size") {
		t.Error("Missing Size_Small constructor")
	}

	// Verify generated code compiles
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "", result, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("Generated code does not compile: %v\nGenerated code:\n%s", parseErr, output)
	}
}

func TestEnumProcessor_NoEnums(t *testing.T) {
	source := `package main

func main() {
	println("Hello, world!")
}
`

	processor := NewEnumProcessor()
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process() failed: %v", err)
	}

	output := string(result)

	// Source should be unchanged
	if output != source {
		t.Errorf("Source was modified when no enums present\nExpected:\n%s\nGot:\n%s", source, output)
	}
}

func TestEnumProcessor_WithComments(t *testing.T) {
	source := `package main

enum Status {
	// Initial state
	Pending,
	// Currently processing
	Active,
	// Done
	Complete,
}
`

	processor := NewEnumProcessor()
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process() failed: %v", err)
	}

	output := string(result)

	// Verify all variants are still generated despite comments
	if !strings.Contains(output, "StatusTag_Pending") {
		t.Error("Missing StatusTag_Pending")
	}
	if !strings.Contains(output, "StatusTag_Active") {
		t.Error("Missing StatusTag_Active")
	}
	if !strings.Contains(output, "StatusTag_Complete") {
		t.Error("Missing StatusTag_Complete")
	}

	// Verify generated code compiles
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "", result, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("Generated code does not compile: %v\nGenerated code:\n%s", parseErr, output)
	}
}

func TestEnumProcessor_ComplexTypes(t *testing.T) {
	source := `package main

enum Result {
	Ok { value: []string },
	Err { error: error },
}
`

	processor := NewEnumProcessor()
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process() failed: %v", err)
	}

	output := string(result)

	// Verify complex type handling
	if !strings.Contains(output, "ok_value *[]string") {
		t.Error("Missing ok_value field with []string type")
	}
	if !strings.Contains(output, "err_error *error") {
		t.Error("Missing err_error field with error type")
	}

	// Verify constructors with complex types
	if !strings.Contains(output, "func Result_Ok(value []string) Result") {
		t.Error("Missing Result_Ok constructor with []string parameter")
	}
	if !strings.Contains(output, "func Result_Err(error error) Result") {
		t.Error("Missing Result_Err constructor with error parameter")
	}

	// Verify generated code compiles
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "", result, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("Generated code does not compile: %v\nGenerated code:\n%s", parseErr, output)
	}
}

func TestEnumProcessor_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		shouldFail  bool
		checkOutput func(string) bool
	}{
		{
			name: "single variant",
			source: `package main

enum Single {
	Only,
}
`,
			shouldFail: false,
			checkOutput: func(output string) bool {
				return strings.Contains(output, "type SingleTag uint8") &&
					strings.Contains(output, "func Single_Only() Single")
			},
		},
		{
			name: "trailing comma",
			source: `package main

enum Status {
	Pending,
	Active,
	Complete,
}
`,
			shouldFail: false,
			checkOutput: func(output string) bool {
				return strings.Contains(output, "StatusTag_Complete")
			},
		},
		{
			name: "no trailing comma",
			source: `package main

enum Status {
	Pending,
	Active,
	Complete
}
`,
			shouldFail: false,
			checkOutput: func(output string) bool {
				return strings.Contains(output, "StatusTag_Complete")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewEnumProcessor()
			result, _, err := processor.Process([]byte(tt.source))

			if tt.shouldFail && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.shouldFail && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.shouldFail && tt.checkOutput != nil {
				if !tt.checkOutput(string(result)) {
					t.Errorf("Output check failed for %s\nGenerated:\n%s", tt.name, string(result))
				}
			}
		})
	}
}
